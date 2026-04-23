// internal/handlers/notifications.go
package handlers

import (
	"errors"
	"net/http"

	"github.com/realestate/backend/internal/models"
	"github.com/realestate/backend/internal/respond"
	"github.com/realestate/backend/internal/services"
	"github.com/realestate/backend/internal/worker"
)

// NotificationsHandler exposes notification_jobs list and stale-scan trigger.
type NotificationsHandler struct {
	notifSvc *services.NotificationService
	staleSvc *services.StaleService
	wkClient *worker.Client
}

// NewNotificationsHandler constructs a NotificationsHandler.
func NewNotificationsHandler(
	notifSvc *services.NotificationService,
	staleSvc *services.StaleService,
	wkClient *worker.Client,
) *NotificationsHandler {
	return &NotificationsHandler{
		notifSvc: notifSvc,
		staleSvc: staleSvc,
		wkClient: wkClient,
	}
}

// ── GET /api/notifications ────────────────────────────────────────────────────

func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := clampInt(parseIntQuery(r.URL.Query().Get("limit"), 50), 1, 200)
	offset := max0(parseIntQuery(r.URL.Query().Get("offset"), 0))

	jobs, total, err := h.notifSvc.ListAll(r.Context(), limit, offset)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list notifications")
		return
	}
	if jobs == nil {
		jobs = []models.NotificationJob{}
	}

	respond.Paginated(w, jobs, "Notifications retrieved successfully", limit, offset, total)
}

// ── POST /api/admin/notifications/scan-stale ──────────────────────────────────
// SUPER_ADMIN only (enforced by mw.Require at the router level).
// Finds all stale SELLING listings and enqueues a TaskNotifyStale per property.

func (h *NotificationsHandler) TriggerStaleScan(w http.ResponseWriter, r *http.Request) {
	stale, err := h.staleSvc.FindStale(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "stale scan failed")
		return
	}

	queued := 0
	for _, p := range stale {
		if err := h.wkClient.EnqueueStaleNotification(
			r.Context(), p.ID, p.OwnerContact, p.DaysUnsold,
		); err != nil {
			// Log and continue — don't abort the whole scan for one failure.
			continue
		}
		queued++
	}

	respond.JSON(w, http.StatusAccepted, map[string]int{
		"stale_found": len(stale),
		"queued":      queued,
	}, "Stale scan complete")
}

// ── GET /api/notifications/:job_id ───────────────────────────────────────────

func (h *NotificationsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "job_id")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "invalid job ID")
		return
	}

	job, err := h.notifSvc.GetJob(r.Context(), id)
	if errors.Is(err, services.ErrJobNotFound) {
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "notification job not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get notification job")
		return
	}

	respond.JSON(w, http.StatusOK, job, "Notification job retrieved successfully")
}
