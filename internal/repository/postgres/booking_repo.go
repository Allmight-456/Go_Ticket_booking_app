package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingRepo struct {
	db *pgxpool.Pool
}

func NewBookingRepo(db *pgxpool.Pool) *BookingRepo {
	return &BookingRepo{db: db}
}

const bookingColumns = `id, user_id, event_id, ticket_count, total_price_cents, status, version, booked_at, updated_at, cancelled_at`

func scanBooking(row pgx.Row) (*domain.Booking, error) {
	b := &domain.Booking{}
	err := row.Scan(
		&b.ID, &b.UserID, &b.EventID, &b.TicketCount, &b.TotalPriceCents,
		&b.Status, &b.Version, &b.BookedAt, &b.UpdatedAt, &b.CancelledAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, err
	}
	return b, nil
}

// CreateInTx inserts a booking record inside an existing transaction.
func (r *BookingRepo) CreateInTx(ctx context.Context, tx pgx.Tx, b *domain.Booking) error {
	const q = `
		INSERT INTO bookings (user_id, event_id, ticket_count, total_price_cents, status)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING ` + bookingColumns

	bk, err := scanBooking(tx.QueryRow(ctx, q,
		b.UserID, b.EventID, b.TicketCount, b.TotalPriceCents, b.Status,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrBookingAlreadyActive
		}
		return fmt.Errorf("create booking in tx: %w", err)
	}
	*b = *bk
	return nil
}

func (r *BookingRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Booking, error) {
	const q = `SELECT ` + bookingColumns + ` FROM bookings WHERE id=$1`
	bk, err := scanBooking(r.db.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("get booking by id: %w", err)
	}
	return bk, nil
}

func (r *BookingRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Booking, error) {
	const q = `SELECT ` + bookingColumns + ` FROM bookings WHERE user_id=$1 ORDER BY booked_at DESC`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list bookings by user: %w", err)
	}
	defer rows.Close()

	var bookings []domain.Booking
	for rows.Next() {
		b := domain.Booking{}
		if err := rows.Scan(
			&b.ID, &b.UserID, &b.EventID, &b.TicketCount, &b.TotalPriceCents,
			&b.Status, &b.Version, &b.BookedAt, &b.UpdatedAt, &b.CancelledAt,
		); err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

// Cancel marks a booking cancelled and restores the ticket count in the same transaction.
func (r *BookingRepo) Cancel(ctx context.Context, bookingID, userID uuid.UUID) (*domain.Booking, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin cancel tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Lock and fetch booking
	const lockQ = `
		SELECT ` + bookingColumns + `
		FROM bookings WHERE id=$1 AND user_id=$2 FOR UPDATE`
	bk, err := scanBooking(tx.QueryRow(ctx, lockQ, bookingID, userID))
	if err != nil {
		if errors.Is(err, domain.ErrBookingNotFound) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("lock booking: %w", err)
	}

	if bk.Status != domain.BookingStatusPending && bk.Status != domain.BookingStatusConfirmed {
		return nil, domain.ErrBookingCannotCancel
	}

	// Update booking status
	const cancelQ = `
		UPDATE bookings
		SET status='cancelled', cancelled_at=NOW(), updated_at=NOW(), version=version+1
		WHERE id=$1
		RETURNING ` + bookingColumns
	updated, err := scanBooking(tx.QueryRow(ctx, cancelQ, bookingID))
	if err != nil {
		return nil, fmt.Errorf("cancel booking update: %w", err)
	}

	// Restore tickets on the event
	const restoreQ = `
		UPDATE events
		SET remaining_tickets = remaining_tickets + $1, updated_at = NOW()
		WHERE id = $2`
	if _, err := tx.Exec(ctx, restoreQ, bk.TicketCount, bk.EventID); err != nil {
		return nil, fmt.Errorf("restore event tickets: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cancel: %w", err)
	}
	return updated, nil
}

// BeginTx exposes a transaction to the service layer for the booking flow.
func (r *BookingRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}
