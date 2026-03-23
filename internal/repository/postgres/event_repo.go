package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepo struct {
	db *pgxpool.Pool
}

func NewEventRepo(db *pgxpool.Pool) *EventRepo {
	return &EventRepo{db: db}
}

const eventColumns = `
	id, name, description, location, starts_at, ends_at,
	total_tickets, remaining_tickets, price_cents, status,
	created_by, version, created_at, updated_at, deleted_at`

func scanEvent(row pgx.Row) (*domain.Event, error) {
	e := &domain.Event{}
	err := row.Scan(
		&e.ID, &e.Name, &e.Description, &e.Location,
		&e.StartsAt, &e.EndsAt,
		&e.TotalTickets, &e.RemainingTickets, &e.PriceCents, &e.Status,
		&e.CreatedBy, &e.Version, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrEventNotFound
		}
		return nil, err
	}
	return e, nil
}

func (r *EventRepo) Create(ctx context.Context, e *domain.Event) error {
	const q = `
		INSERT INTO events
			(name, description, location, starts_at, ends_at,
			 total_tickets, remaining_tickets, price_cents, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING ` + eventColumns

	ev, err := scanEvent(r.db.QueryRow(ctx, q,
		e.Name, e.Description, e.Location, e.StartsAt, e.EndsAt,
		e.TotalTickets, e.RemainingTickets, e.PriceCents, e.Status, e.CreatedBy,
	))
	if err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	*e = *ev
	return nil
}

func (r *EventRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	const q = `SELECT ` + eventColumns + ` FROM events WHERE id=$1 AND deleted_at IS NULL`
	ev, err := scanEvent(r.db.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("get event by id: %w", err)
	}
	return ev, nil
}

// GetByIDForUpdate acquires a row-level lock — call only inside a transaction.
func (r *EventRepo) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.Event, error) {
	const q = `SELECT ` + eventColumns + ` FROM events WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`
	ev, err := scanEvent(tx.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("get event for update: %w", err)
	}
	return ev, nil
}

// EventFilter holds optional filtering parameters for List.
type EventFilter struct {
	Status *domain.EventStatus
	Limit  int
	Offset int
}

func (r *EventRepo) List(ctx context.Context, f EventFilter) ([]domain.Event, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	q := `SELECT ` + eventColumns + ` FROM events WHERE deleted_at IS NULL`
	args := []any{}
	idx := 1

	if f.Status != nil {
		q += fmt.Sprintf(" AND status=$%d", idx)
		args = append(args, *f.Status)
		idx++
	}

	q += fmt.Sprintf(" ORDER BY starts_at ASC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		e := domain.Event{}
		if err := rows.Scan(
			&e.ID, &e.Name, &e.Description, &e.Location,
			&e.StartsAt, &e.EndsAt,
			&e.TotalTickets, &e.RemainingTickets, &e.PriceCents, &e.Status,
			&e.CreatedBy, &e.Version, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event row: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *EventRepo) Update(ctx context.Context, e *domain.Event) error {
	const q = `
		UPDATE events
		SET name=$1, description=$2, location=$3, starts_at=$4, ends_at=$5,
		    total_tickets=$6, remaining_tickets=$7, price_cents=$8, status=$9,
		    version=version+1, updated_at=NOW()
		WHERE id=$10 AND version=$11 AND deleted_at IS NULL
		RETURNING ` + eventColumns

	updated, err := scanEvent(r.db.QueryRow(ctx, q,
		e.Name, e.Description, e.Location, e.StartsAt, e.EndsAt,
		e.TotalTickets, e.RemainingTickets, e.PriceCents, e.Status,
		e.ID, e.Version,
	))
	if err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			return domain.ErrVersionConflict
		}
		return fmt.Errorf("update event: %w", err)
	}
	*e = *updated
	return nil
}

// UpdateTicketsInTx decrements remaining_tickets and increments version inside an existing transaction.
func (r *EventRepo) UpdateTicketsInTx(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, version, decrement int) error {
	const q = `
		UPDATE events
		SET remaining_tickets = remaining_tickets - $1,
		    version = version + 1,
		    updated_at = NOW()
		WHERE id = $2 AND version = $3 AND deleted_at IS NULL`

	tag, err := tx.Exec(ctx, q, decrement, eventID, version)
	if err != nil {
		return fmt.Errorf("update tickets in tx: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}
	return nil
}

// SoftDelete marks an event as deleted without removing the row.
func (r *EventRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE events SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("soft delete event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrEventNotFound
	}
	return nil
}

// BatchCreate inserts multiple events in a single round-trip using pgx Batch API.
func (r *EventRepo) BatchCreate(ctx context.Context, events []domain.Event) ([]domain.Event, error) {
	batch := &pgx.Batch{}
	const q = `
		INSERT INTO events
			(name, description, location, starts_at, ends_at,
			 total_tickets, remaining_tickets, price_cents, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING ` + eventColumns

	for i := range events {
		e := &events[i]
		batch.Queue(q,
			e.Name, e.Description, e.Location, e.StartsAt, e.EndsAt,
			e.TotalTickets, e.RemainingTickets, e.PriceCents, e.Status, e.CreatedBy,
		)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	result := make([]domain.Event, 0, len(events))
	for range events {
		ev, err := scanEvent(br.QueryRow())
		if err != nil {
			return nil, fmt.Errorf("batch create event row: %w", err)
		}
		result = append(result, *ev)
	}
	return result, nil
}
