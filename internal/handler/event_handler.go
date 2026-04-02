package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/Allmight-456/ticketflow/internal/middleware"
	"github.com/Allmight-456/ticketflow/internal/repository/postgres"
	"github.com/Allmight-456/ticketflow/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type EventHandler struct {
	svc *service.EventService
}

func NewEventHandler(svc *service.EventService) *EventHandler {
	return &EventHandler{svc: svc}
}

func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := postgres.EventFilter{
		Limit:  parseIntQuery(r, "limit", 20),
		Offset: parseIntQuery(r, "offset", 0),
	}
	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.EventStatus(s)
		filter.Status = &st
	}

	events, err := h.svc.List(r.Context(), filter)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "could not list events")
		return
	}
	render(w, http.StatusOK, map[string]any{"data": events})
}

func (h *EventHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	event, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			renderError(w, http.StatusNotFound, "event not found")
		} else {
			renderError(w, http.StatusInternalServerError, "could not fetch event")
		}
		return
	}
	render(w, http.StatusOK, event)
}

func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		renderError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.CreateEventRequest
	if !decode(w, r, &req) {
		return
	}

	event, err := h.svc.Create(r.Context(), req, claims.UserID)
	if err != nil {
		renderError(w, http.StatusBadRequest, err.Error())
		return
	}
	render(w, http.StatusCreated, event)
}

func (h *EventHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		renderError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.UpdateEventRequest
	if !decode(w, r, &req) {
		return
	}

	// Resolve event ID: URL path param takes precedence, body "id" is the fallback.
	var id uuid.UUID
	if urlParam := chi.URLParam(r, "id"); urlParam != "" {
		parsed, err := uuid.Parse(urlParam)
		if err != nil {
			renderError(w, http.StatusBadRequest, "invalid UUID in URL path")
			return
		}
		id = parsed
	} else if req.ID != uuid.Nil {
		id = req.ID
	} else {
		renderError(w, http.StatusBadRequest, "event id is required: provide it in the URL path (/events/{id}) or as 'id' in the request body")
		return
	}

	event, err := h.svc.Update(r.Context(), id, req, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEventNotFound):
			renderError(w, http.StatusNotFound, "event not found")
		case errors.Is(err, domain.ErrVersionConflict):
			renderError(w, http.StatusConflict, "version conflict — reload and retry")
		default:
			renderError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	render(w, http.StatusOK, event)
}

func (h *EventHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		renderError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.svc.Delete(r.Context(), id, claims.UserID); err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			renderError(w, http.StatusNotFound, "event not found")
		} else {
			renderError(w, http.StatusInternalServerError, "could not delete event")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EventHandler) BatchCreate(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		renderError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var reqs []service.CreateEventRequest
	if !decode(w, r, &reqs) {
		return
	}

	events, err := h.svc.BatchCreate(r.Context(), reqs, claims.UserID)
	if err != nil {
		renderError(w, http.StatusBadRequest, err.Error())
		return
	}
	render(w, http.StatusCreated, map[string]any{"data": events, "count": len(events)})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseUUID(w http.ResponseWriter, s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		renderError(w, http.StatusBadRequest, "invalid UUID")
		return uuid.UUID{}, false
	}
	return id, true
}

func parseIntQuery(r *http.Request, key string, fallback int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
