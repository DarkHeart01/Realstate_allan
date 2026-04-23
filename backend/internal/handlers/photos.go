// internal/handlers/photos.go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	mw "github.com/realestate/backend/internal/middleware"
	"github.com/realestate/backend/internal/respond"
	"github.com/realestate/backend/internal/services"
)

// PhotosHandler handles photo presign / confirm / delete.
type PhotosHandler struct {
	photos *services.PhotoService
}

// NewPhotosHandler constructs a PhotosHandler.
func NewPhotosHandler(photos *services.PhotoService) *PhotosHandler {
	return &PhotosHandler{photos: photos}
}

// ── POST /api/properties/{id}/photos/presign ──────────────────────────────────

func (h *PhotosHandler) Presign(w http.ResponseWriter, r *http.Request) {
	callerID, err := uuid.Parse(mw.UserIDFromCtx(r.Context()))
	if err != nil {
		respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid user identity")
		return
	}
	callerRole := mw.RoleFromCtx(r.Context())

	propID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid property ID")
		return
	}

	ok, err := h.photos.IsOwnerOrAdmin(r.Context(), propID, callerID, callerRole)
	if errors.Is(err, services.ErrPropertyNotFound) {
		respond.Error(w, http.StatusNotFound, "PROPERTY_NOT_FOUND", "property not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ownership check failed")
		return
	}
	if !ok {
		respond.Error(w, http.StatusForbidden, "FORBIDDEN", "you do not have permission to upload photos for this property")
		return
	}

	var body struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid request body")
		return
	}
	if strings.TrimSpace(body.Filename) == "" {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "filename is required")
		return
	}
	if strings.TrimSpace(body.ContentType) == "" {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "content_type is required")
		return
	}

	result, err := h.photos.Presign(r.Context(), propID, body.Filename, body.ContentType)
	if errors.Is(err, services.ErrGCSNotConfigured) {
		respond.Error(w, http.StatusServiceUnavailable, "GCS_NOT_CONFIGURED", "Media storage is not configured")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "unsupported content type") {
			respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
			return
		}
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate upload URL")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{
		"photo_id":   result.PhotoID.String(),
		"upload_url": result.UploadURL,
		"cdn_url":    result.CDNUrl,
	}, "Upload URL generated successfully")
}

// ── POST /api/properties/{id}/photos/{photo_id}/confirm ───────────────────────

func (h *PhotosHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	propID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid property ID")
		return
	}
	photoID, err := uuid.Parse(chi.URLParam(r, "photo_id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid photo ID")
		return
	}

	if !h.photos.IsConfigured() {
		respond.Error(w, http.StatusServiceUnavailable, "GCS_NOT_CONFIGURED", "Media storage is not configured")
		return
	}

	photo, err := h.photos.Confirm(r.Context(), propID, photoID)
	if errors.Is(err, services.ErrPhotoNotFound) {
		respond.Error(w, http.StatusNotFound, "PHOTO_NOT_FOUND", "photo not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to confirm photo")
		return
	}

	respond.JSON(w, http.StatusOK, photo, "Photo confirmed successfully")
}

// ── DELETE /api/properties/{id}/photos/{photo_id} ─────────────────────────────

func (h *PhotosHandler) Delete(w http.ResponseWriter, r *http.Request) {
	callerID, err := uuid.Parse(mw.UserIDFromCtx(r.Context()))
	if err != nil {
		respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid user identity")
		return
	}
	callerRole := mw.RoleFromCtx(r.Context())

	propID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid property ID")
		return
	}
	photoID, err := uuid.Parse(chi.URLParam(r, "photo_id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid photo ID")
		return
	}

	ok, err := h.photos.IsOwnerOrAdmin(r.Context(), propID, callerID, callerRole)
	if errors.Is(err, services.ErrPropertyNotFound) {
		respond.Error(w, http.StatusNotFound, "PROPERTY_NOT_FOUND", "property not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ownership check failed")
		return
	}
	if !ok {
		respond.Error(w, http.StatusForbidden, "FORBIDDEN", "you do not have permission to delete photos for this property")
		return
	}

	if err := h.photos.Delete(r.Context(), propID, photoID); errors.Is(err, services.ErrPhotoNotFound) {
		respond.Error(w, http.StatusNotFound, "PHOTO_NOT_FOUND", "photo not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete photo")
		return
	}

	respond.JSON(w, http.StatusOK, nil, "Photo deleted successfully")
}
