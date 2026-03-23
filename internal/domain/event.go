package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type EventStatus string

const (
	EventStatusDraft     EventStatus = "draft"
	EventStatusPublished EventStatus = "published"
	EventStatusCancelled EventStatus = "cancelled"
	EventStatusSoldOut   EventStatus = "sold_out"
)

// Event is the core aggregate for the booking system.
// PriceCents stores money as integer cents — never float — to avoid rounding errors.
// Version is used for optimistic locking on concurrent updates.
type Event struct {
	ID               uuid.UUID   `json:"id"`
	Name             string      `json:"name"`
	Description      string      `json:"description"`
	Location         string      `json:"location"`
	StartsAt         time.Time   `json:"starts_at"`
	EndsAt           time.Time   `json:"ends_at"`
	TotalTickets     int         `json:"total_tickets"`
	RemainingTickets int         `json:"remaining_tickets"`
	PriceCents       int         `json:"price_cents"`
	Status           EventStatus `json:"status"`
	CreatedBy        uuid.UUID   `json:"created_by"`
	Version          int         `json:"version"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	DeletedAt        *time.Time  `json:"deleted_at,omitempty"`
}

var (
	ErrEventNotFound       = errors.New("event not found")
	ErrEventNotAvailable   = errors.New("event is not available for booking")
	ErrInsufficientTickets = errors.New("not enough tickets remaining")
	ErrVersionConflict     = errors.New("version conflict — event was modified concurrently")
)
