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

type EventService struct {
	events *postgres.EventRepo
	cache  *cache.RedisRepo
	audit  *AuditService
}

func NewEventService(events *postgres.EventRepo, cache *cache.RedisRepo, audit *AuditService) *EventService {
	return &EventService{events: events, cache: cache, audit: audit}
}

// CreateEventRequest carries validated input for creating a single event.
type CreateEventRequest struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Location     string            `json:"location"`
	StartsAt     time.Time         `json:"starts_at"`
	EndsAt       time.Time         `json:"ends_at"`
	TotalTickets int               `json:"total_tickets"`
	PriceCents   int               `json:"price_cents"`
	Status       domain.EventStatus `json:"status"`
}

// UpdateEventRequest carries validated input for updating an event.
type UpdateEventRequest struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Location     string            `json:"location"`
	StartsAt     time.Time         `json:"starts_at"`
	EndsAt       time.Time         `json:"ends_at"`
	TotalTickets int               `json:"total_tickets"`
	PriceCents   int               `json:"price_cents"`
	Status       domain.EventStatus `json:"status"`
	Version      int               `json:"version"`
}

func (s *EventService) Create(ctx context.Context, req CreateEventRequest, actorID uuid.UUID) (*domain.Event, error) {
	if err := validateCreateEvent(req); err != nil {
		return nil, err
	}

	status := req.Status
	if status == "" {
		status = domain.EventStatusDraft
	}

	event := &domain.Event{
		Name:             req.Name,
		Description:      req.Description,
		Location:         req.Location,
		StartsAt:         req.StartsAt,
		EndsAt:           req.EndsAt,
		TotalTickets:     req.TotalTickets,
		RemainingTickets: req.TotalTickets,
		PriceCents:       req.PriceCents,
		Status:           status,
		CreatedBy:        actorID,
	}

	if err := s.events.Create(ctx, event); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}

	s.audit.Log(ctx, &domain.AuditLog{ //nolint:errcheck
		ActorID:      &actorID,
		ResourceType: "event",
		ResourceID:   event.ID,
		Action:       domain.AuditActionCreate,
		NewState:     ToMap(event),
	})

	return event, nil
}

func (s *EventService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	return s.events.GetByID(ctx, id)
}

func (s *EventService) List(ctx context.Context, filter postgres.EventFilter) ([]domain.Event, error) {
	return s.events.List(ctx, filter)
}

func (s *EventService) Update(ctx context.Context, id uuid.UUID, req UpdateEventRequest, actorID uuid.UUID) (*domain.Event, error) {
	old, err := s.events.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := validateUpdateEvent(req); err != nil {
		return nil, err
	}

	updated := *old
	updated.Name = req.Name
	updated.Description = req.Description
	updated.Location = req.Location
	updated.StartsAt = req.StartsAt
	updated.EndsAt = req.EndsAt
	updated.TotalTickets = req.TotalTickets
	updated.PriceCents = req.PriceCents
	updated.Status = req.Status
	updated.Version = req.Version

	if err := s.events.Update(ctx, &updated); err != nil {
		return nil, err
	}

	s.cache.InvalidateEvent(ctx, id)

	oldMap := ToMap(old)
	newMap := ToMap(&updated)
	s.audit.Log(ctx, &domain.AuditLog{ //nolint:errcheck
		ActorID:      &actorID,
		ResourceType: "event",
		ResourceID:   id,
		Action:       domain.AuditActionUpdate,
		OldState:     oldMap,
		NewState:     newMap,
		Diff:         ComputeDiff(oldMap, newMap),
	})

	return &updated, nil
}

func (s *EventService) Delete(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	old, err := s.events.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.events.SoftDelete(ctx, id); err != nil {
		return err
	}

	s.cache.InvalidateEvent(ctx, id)

	s.audit.Log(ctx, &domain.AuditLog{ //nolint:errcheck
		ActorID:      &actorID,
		ResourceType: "event",
		ResourceID:   id,
		Action:       domain.AuditActionDelete,
		OldState:     ToMap(old),
	})

	return nil
}

// BatchCreate inserts multiple events atomically using pgx's batch API.
func (s *EventService) BatchCreate(ctx context.Context, reqs []CreateEventRequest, actorID uuid.UUID) ([]domain.Event, error) {
	if len(reqs) == 0 {
		return nil, errors.New("no events provided")
	}
	if len(reqs) > 50 {
		return nil, errors.New("batch size exceeds maximum of 50")
	}

	events := make([]domain.Event, len(reqs))
	for i, req := range reqs {
		if err := validateCreateEvent(req); err != nil {
			return nil, fmt.Errorf("event[%d]: %w", i, err)
		}
		status := req.Status
		if status == "" {
			status = domain.EventStatusDraft
		}
		events[i] = domain.Event{
			Name:             req.Name,
			Description:      req.Description,
			Location:         req.Location,
			StartsAt:         req.StartsAt,
			EndsAt:           req.EndsAt,
			TotalTickets:     req.TotalTickets,
			RemainingTickets: req.TotalTickets,
			PriceCents:       req.PriceCents,
			Status:           status,
			CreatedBy:        actorID,
		}
	}

	created, err := s.events.BatchCreate(ctx, events)
	if err != nil {
		return nil, fmt.Errorf("batch create events: %w", err)
	}

	for i := range created {
		e := created[i]
		s.audit.Log(ctx, &domain.AuditLog{ //nolint:errcheck
			ActorID:      &actorID,
			ResourceType: "event",
			ResourceID:   e.ID,
			Action:       domain.AuditActionCreate,
			NewState:     ToMap(&e),
		})
	}

	return created, nil
}

func validateCreateEvent(req CreateEventRequest) error {
	if len(req.Name) < 3 {
		return errors.New("event name must be at least 3 characters")
	}
	if req.Location == "" {
		return errors.New("location is required")
	}
	if req.TotalTickets <= 0 {
		return errors.New("total_tickets must be positive")
	}
	if req.PriceCents < 0 {
		return errors.New("price_cents cannot be negative")
	}
	if !req.EndsAt.After(req.StartsAt) {
		return errors.New("ends_at must be after starts_at")
	}
	return nil
}

func validateUpdateEvent(req UpdateEventRequest) error {
	if req.Version <= 0 {
		return errors.New("version is required for update (optimistic lock)")
	}
	return validateCreateEvent(CreateEventRequest{
		Name:         req.Name,
		Location:     req.Location,
		TotalTickets: req.TotalTickets,
		PriceCents:   req.PriceCents,
		StartsAt:     req.StartsAt,
		EndsAt:       req.EndsAt,
	})
}
