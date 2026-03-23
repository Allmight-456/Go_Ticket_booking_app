package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type BookingStatus string

const (
	BookingStatusPending   BookingStatus = "pending"
	BookingStatusConfirmed BookingStatus = "confirmed"
	BookingStatusCancelled BookingStatus = "cancelled"
	BookingStatusRefunded  BookingStatus = "refunded"
)

// Booking records a user's reservation of tickets for an event.
// TotalPriceCents is stored as int64 because ticket_count × price_cents can exceed int32.
type Booking struct {
	ID              uuid.UUID     `json:"id"`
	UserID          uuid.UUID     `json:"user_id"`
	EventID         uuid.UUID     `json:"event_id"`
	TicketCount     int           `json:"ticket_count"`
	TotalPriceCents int64         `json:"total_price_cents"`
	Status          BookingStatus `json:"status"`
	Version         int           `json:"version"`
	BookedAt        time.Time     `json:"booked_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	CancelledAt     *time.Time    `json:"cancelled_at,omitempty"`
}

var (
	ErrBookingNotFound      = errors.New("booking not found")
	ErrBookingAlreadyActive = errors.New("active booking already exists for this event")
	ErrBookingCannotCancel  = errors.New("booking cannot be cancelled in current status")
	ErrNotBookingOwner      = errors.New("only the booking owner can perform this action")
)
