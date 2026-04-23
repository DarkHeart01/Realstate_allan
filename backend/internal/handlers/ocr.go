// internal/handlers/ocr.go
package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/realestate/backend/internal/respond"
	"github.com/realestate/backend/internal/services"
	"github.com/realestate/backend/internal/worker"
)

// OCRHandler handles OCR scan requests for property photos.
type OCRHandler struct {
	ocrSvc   *services.OCRService
	wkClient *worker.Client
}

// NewOCRHandler constructs an OCRHandler.
func NewOCRHandler(ocrSvc *services.OCRService, wkClient *worker.Client) *OCRHandler {
	return &OCRHandler{ocrSvc: ocrSvc, wkClient: wkClient}
}

// ── POST /api/properties/{id}/photos/{photo_id}/ocr ───────────────────────────
// Synchronously calls the Vision API and returns suggestions.
// Also enqueues a background TaskOCRProcessImage for audit / retry logging.

func (h *OCRHandler) Scan(w http.ResponseWriter, r *http.Request) {
	if !h.ocrSvc.IsEnabled() {
		respond.Error(w, http.StatusServiceUnavailable, "OCR_NOT_ENABLED",
			"OCR is not enabled on this server — set GOOGLE_VISION_ENABLED=true")
		return
	}

	propertyID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid property ID")
		return
	}
	photoID, err := uuid.Parse(chi.URLParam(r, "photo_id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid photo ID")
		return
	}

	result, err := h.ocrSvc.ScanPhoto(r.Context(), photoID, propertyID)
	if errors.Is(err, services.ErrPhotoNotFound) {
		respond.Error(w, http.StatusNotFound, "PHOTO_NOT_FOUND",
			"photo not found or not yet confirmed")
		return
	}
	if errors.Is(err, services.ErrOCRNotEnabled) {
		respond.Error(w, http.StatusServiceUnavailable, "OCR_NOT_ENABLED",
			"Vision API not configured")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"OCR scan failed: "+err.Error())
		return
	}

	// Best-effort background task for audit logging — don't fail the request.
	_ = h.wkClient.EnqueueOCR(r.Context(), photoID.String(), propertyID.String(), "")

	respond.JSON(w, http.StatusOK, result, "OCR scan completed successfully")
}

// ── GET /api/properties/{id}/photos/{photo_id}/ocr ────────────────────────────
// Returns a previously stored OCR result for a photo.

func (h *OCRHandler) GetResult(w http.ResponseWriter, r *http.Request) {
	photoID, err := uuid.Parse(chi.URLParam(r, "photo_id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid photo ID")
		return
	}

	result, err := h.ocrSvc.GetResult(r.Context(), photoID)
	if errors.Is(err, services.ErrOCRResultNotFound) {
		respond.Error(w, http.StatusNotFound, "OCR_RESULT_NOT_FOUND",
			"no OCR result found for this photo")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"failed to retrieve OCR result")
		return
	}

	respond.JSON(w, http.StatusOK, result, "OCR result retrieved successfully")
}
