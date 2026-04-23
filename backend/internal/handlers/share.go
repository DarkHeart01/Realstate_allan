// internal/handlers/share.go
package handlers

import (
	"errors"
	"net/http"

	"github.com/realestate/backend/internal/respond"
	"github.com/realestate/backend/internal/services"
)

// ShareHandler handles the property share endpoint.
type ShareHandler struct {
	svc *services.ShareService
}

// NewShareHandler constructs a ShareHandler.
func NewShareHandler(svc *services.ShareService) *ShareHandler {
	return &ShareHandler{svc: svc}
}

// ── POST /api/properties/{id}/share ──────────────────────────────────────────

func (h *ShareHandler) Share(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid property ID")
		return
	}

	result, err := h.svc.Generate(r.Context(), id)
	if errors.Is(err, services.ErrPropertyNotFound) {
		respond.Error(w, http.StatusNotFound, "PROPERTY_NOT_FOUND", "property not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate share content")
		return
	}

	respond.JSON(w, http.StatusOK, result, "Share content generated successfully")
}
