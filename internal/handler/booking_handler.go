package handler

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/Allmight-456/ticketflow/internal/middleware"
	"github.com/Allmight-456/ticketflow/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type BookingHandler struct {
	svc *service.BookingService
}

func NewBookingHandler(svc *service.BookingService) *BookingHandler {
	return &BookingHandler{svc: svc}
}

type createBookingRequest struct {
	EventID     uuid.UUID `json:"event_id"`
	TicketCount int       `json:"ticket_count"`
}

func (h *BookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		renderError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createBookingRequest
	if !decode(w, r, &req) {
		return
	}

	booking, err := h.svc.BookTicket(
		r.Context(),
		claims.UserID,
		req.EventID,
		req.TicketCount,
		clientIP(r),
		r.UserAgent(),
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEventNotFound):
			renderError(w, http.StatusNotFound, "event not found")
		case errors.Is(err, domain.ErrEventNotAvailable):
			renderError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, domain.ErrInsufficientTickets):
			renderError(w, http.StatusConflict, err.Error())
		case errors.Is(err, domain.ErrBookingAlreadyActive):
			renderError(w, http.StatusConflict, err.Error())
		default:
			renderError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	render(w, http.StatusCreated, booking)
}

func (h *BookingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	bookingID, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		renderError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	updated, err := h.svc.CancelBooking(r.Context(), bookingID, claims.UserID, clientIP(r), r.UserAgent())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrBookingNotFound):
			renderError(w, http.StatusNotFound, "booking not found")
		case errors.Is(err, domain.ErrNotBookingOwner):
			renderError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, domain.ErrBookingCannotCancel):
			renderError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			renderError(w, http.StatusInternalServerError, "could not cancel booking")
		}
		return
	}
	render(w, http.StatusOK, updated)
}

func (h *BookingHandler) BatchCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		renderError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var reqs []service.BatchBookRequest
	if !decode(w, r, &reqs) {
		return
	}

	bookings, err := h.svc.BatchBook(r.Context(), reqs, claims.UserID, clientIP(r), r.UserAgent())
	if err != nil {
		renderError(w, http.StatusBadRequest, err.Error())
		return
	}
	render(w, http.StatusCreated, map[string]any{"data": bookings, "count": len(bookings)})
}

// clientIP extracts the real client IP, honouring X-Real-IP / X-Forwarded-For.
// r.RemoteAddr is always "IP:port" — we strip the port so Postgres INET accepts it.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// May be comma-separated list; take the first (leftmost = client).
		return strings.SplitN(ip, ",", 2)[0]
	}
	// RemoteAddr is "host:port" for TCP; strip the port.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
