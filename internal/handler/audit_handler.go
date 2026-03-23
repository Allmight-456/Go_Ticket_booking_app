package handler

import (
	"net/http"

	"github.com/Allmight-456/ticketflow/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AuditHandler struct {
	svc *service.AuditService
}

func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// GetHistory returns the audit trail for a resource.
// GET /audit/{resource_type}/{resource_id}
func (h *AuditHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	resourceType := chi.URLParam(r, "resource_type")
	resourceID, err := uuid.Parse(chi.URLParam(r, "resource_id"))
	if err != nil {
		renderError(w, http.StatusBadRequest, "invalid resource_id UUID")
		return
	}

	logs, err := h.svc.GetHistory(r.Context(), resourceType, resourceID)
	if err != nil {
		renderError(w, http.StatusInternalServerError, "could not fetch audit logs")
		return
	}

	render(w, http.StatusOK, map[string]any{"data": logs, "count": len(logs)})
}
