package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/Allmight-456/ticketflow/internal/repository/cache"
	"github.com/Allmight-456/ticketflow/internal/repository/postgres"
	"github.com/google/uuid"
)

type BookingService struct {
	bookings *postgres.BookingRepo
	events   *postgres.EventRepo
	cache    *cache.RedisRepo
	audit    *AuditService
}

func NewBookingService(
	bookings *postgres.BookingRepo,
	events *postgres.EventRepo,
	cache *cache.RedisRepo,
	audit *AuditService,
) *BookingService {
	return &BookingService{bookings: bookings, events: events, cache: cache, audit: audit}
}

// BookTicket is the core conflict-safe booking flow:
//
//  1. Acquire Redis distributed lock        → prevents thundering herd
//  2. BEGIN transaction
//  3. SELECT … FOR UPDATE                   → serialises concurrent writers at DB
//  4. Business validation (tickets, status)
//  5. INSERT booking
//  6. UPDATE events SET remaining = remaining - N, version = version + 1
//     WHERE id = $1 AND version = $2        → optimistic-lock safety net
//  7. INSERT audit_log (same TX)
//  8. COMMIT
//  9. Release Redis lock via Lua            → only releases own token
// 10. Invalidate availability cache
func (s *BookingService) BookTicket(ctx context.Context, userID, eventID uuid.UUID, ticketCount int, ipAddr, ua string) (*domain.Booking, error) {
	if ticketCount <= 0 {
		return nil, errors.New("ticket_count must be positive")
	}

	// ── 1. Redis distributed lock ──────────────────────────────────────────
	lockKey := fmt.Sprintf("lock:event:%s", eventID)
	lockToken, err := s.cache.AcquireLock(ctx, lockKey, 5*time.Second)
	if err != nil {
		if errors.Is(err, cache.ErrLockNotAcquired) {
			return nil, errors.New("event is currently being processed, try again shortly")
		}
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	defer s.cache.ReleaseLock(ctx, lockKey, lockToken) //nolint:errcheck

	// ── 2. Begin transaction ───────────────────────────────────────────────
	tx, err := s.bookings.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// ── 3. SELECT … FOR UPDATE ─────────────────────────────────────────────
	event, err := s.events.GetByIDForUpdate(ctx, tx, eventID)
	if err != nil {
		return nil, err
	}

	// ── 4. Business validation ─────────────────────────────────────────────
	if event.Status != domain.EventStatusPublished {
		return nil, domain.ErrEventNotAvailable
	}
	if event.RemainingTickets < ticketCount {
		return nil, domain.ErrInsufficientTickets
	}

	totalPrice := int64(ticketCount) * int64(event.PriceCents)

	// ── 5. INSERT booking ──────────────────────────────────────────────────
	booking := &domain.Booking{
		UserID:          userID,
		EventID:         eventID,
		TicketCount:     ticketCount,
		TotalPriceCents: totalPrice,
		Status:          domain.BookingStatusConfirmed,
	}
	if err := s.bookings.CreateInTx(ctx, tx, booking); err != nil {
		return nil, err
	}

	// ── 6. UPDATE events (optimistic lock check) ───────────────────────────
	if err := s.events.UpdateTicketsInTx(ctx, tx, eventID, event.Version, ticketCount); err != nil {
		return nil, err
	}

	// ── 7. Audit log (same transaction) ───────────────────────────────────
	s.audit.LogInTx(ctx, tx, &domain.AuditLog{ //nolint:errcheck
		ActorID:      &userID,
		ResourceType: "booking",
		ResourceID:   booking.ID,
		Action:       domain.AuditActionCreate,
		NewState:     ToMap(booking),
		IPAddress:    ipAddr,
		UserAgent:    ua,
	})

	// ── 8. COMMIT ──────────────────────────────────────────────────────────
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit booking: %w", err)
	}

	// ── 10. Invalidate cache ───────────────────────────────────────────────
	s.cache.InvalidateEvent(ctx, eventID)

	return booking, nil
}

// CancelBooking cancels an active booking and restores the ticket count.
func (s *BookingService) CancelBooking(ctx context.Context, bookingID, userID uuid.UUID, ipAddr, ua string) (*domain.Booking, error) {
	old, err := s.bookings.GetByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if old.UserID != userID {
		return nil, domain.ErrNotBookingOwner
	}

	updated, err := s.bookings.Cancel(ctx, bookingID, userID)
	if err != nil {
		return nil, err
	}

	s.cache.InvalidateEvent(ctx, old.EventID)

	oldMap := ToMap(old)
	newMap := ToMap(updated)
	s.audit.Log(ctx, &domain.AuditLog{ //nolint:errcheck
		ActorID:      &userID,
		ResourceType: "booking",
		ResourceID:   bookingID,
		Action:       domain.AuditActionUpdate,
		OldState:     oldMap,
		NewState:     newMap,
		Diff:         ComputeDiff(oldMap, newMap),
		IPAddress:    ipAddr,
		UserAgent:    ua,
	})

	return updated, nil
}

// BatchBook creates multiple bookings sequentially, stopping on the first error.
// Each booking goes through the full lock+transaction flow independently.
func (s *BookingService) BatchBook(ctx context.Context, reqs []BatchBookRequest, userID uuid.UUID, ipAddr, ua string) ([]domain.Booking, error) {
	if len(reqs) == 0 {
		return nil, errors.New("no bookings provided")
	}
	if len(reqs) > 10 {
		return nil, errors.New("batch size exceeds maximum of 10")
	}

	results := make([]domain.Booking, 0, len(reqs))
	for i, req := range reqs {
		b, err := s.BookTicket(ctx, userID, req.EventID, req.TicketCount, ipAddr, ua)
		if err != nil {
			return results, fmt.Errorf("booking[%d] (event %s): %w", i, req.EventID, err)
		}
		results = append(results, *b)
	}
	return results, nil
}

// BatchBookRequest is one entry in a batch booking request.
type BatchBookRequest struct {
	EventID     uuid.UUID `json:"event_id"`
	TicketCount int       `json:"ticket_count"`
}
