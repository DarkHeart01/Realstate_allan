// internal/worker/client.go
package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/realestate/backend/internal/config"
)

// ── Task type constants ───────────────────────────────────────────────────────
// These are the only place task name strings are defined.  All callers use
// these constants — never raw string literals.

const (
	TaskOCRProcessImage = "ocr:process_image"
	TaskNotifyStale     = "notify:listing_stale"
	TaskNotifyWhatsApp  = "notify:whatsapp_send"
	TaskNotifySMS       = "notify:sms_send"
)

// ── Queue name constants ──────────────────────────────────────────────────────

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"
)

// ── Payload types ─────────────────────────────────────────────────────────────

// OCRPayload is the task payload for TaskOCRProcessImage.
type OCRPayload struct {
	PhotoID    string `json:"photo_id"`
	PropertyID string `json:"property_id"`
	GCSKey     string `json:"gcs_key"`
}

// StalePayload is the task payload for TaskNotifyStale.
type StalePayload struct {
	PropertyID   string `json:"property_id"`
	OwnerContact string `json:"owner_contact"`
	DaysUnsold   int    `json:"days_unsold"`
}

// WhatsAppPayload is the task payload for TaskNotifyWhatsApp.
type WhatsAppPayload struct {
	To                string `json:"to"`
	Message           string `json:"message"`
	NotificationJobID string `json:"notification_job_id"`
}

// SMSPayload is the task payload for TaskNotifySMS.
type SMSPayload struct {
	To                string `json:"to"`
	Message           string `json:"message"`
	NotificationJobID string `json:"notification_job_id"`
}

// ── Client ────────────────────────────────────────────────────────────────────

// Client wraps asynq.Client with type-safe enqueue methods.
// Callers never construct asynq.Task directly — use the enqueue methods.
type Client struct {
	inner *asynq.Client
}

// NewClient creates a Client connected to Redis.
func NewClient(cfg *config.Config) (*Client, error) {
	opts, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("worker: parse Redis URI: %w", err)
	}
	return &Client{inner: asynq.NewClient(opts)}, nil
}

// Close releases the underlying Redis connection.
func (c *Client) Close() error { return c.inner.Close() }

// ── Enqueue methods ───────────────────────────────────────────────────────────

// EnqueueOCR enqueues an OCR image processing task.
// gcsKey is the GCS object path used to identify the image via the Vision API.
func (c *Client) EnqueueOCR(ctx context.Context, photoID, propertyID, gcsKey string) error {
	payload, err := json.Marshal(OCRPayload{
		PhotoID:    photoID,
		PropertyID: propertyID,
		GCSKey:     gcsKey,
	})
	if err != nil {
		return fmt.Errorf("worker: marshal OCR payload: %w", err)
	}
	task := asynq.NewTask(TaskOCRProcessImage, payload)
	_, err = c.inner.EnqueueContext(ctx, task, asynq.Queue(QueueDefault))
	return err
}

// EnqueueStaleNotification enqueues a stale listing notification task.
func (c *Client) EnqueueStaleNotification(ctx context.Context, propertyID, ownerContact string, daysUnsold int) error {
	payload, err := json.Marshal(StalePayload{
		PropertyID:   propertyID,
		OwnerContact: ownerContact,
		DaysUnsold:   daysUnsold,
	})
	if err != nil {
		return fmt.Errorf("worker: marshal stale payload: %w", err)
	}
	task := asynq.NewTask(TaskNotifyStale, payload, asynq.MaxRetry(3))
	_, err = c.inner.EnqueueContext(ctx, task, asynq.Queue(QueueDefault))
	return err
}

// EnqueueWhatsApp enqueues a WhatsApp message dispatch task.
// notificationJobID links back to the notification_jobs row for status updates.
func (c *Client) EnqueueWhatsApp(ctx context.Context, to, message, notificationJobID string) error {
	payload, err := json.Marshal(WhatsAppPayload{
		To:                to,
		Message:           message,
		NotificationJobID: notificationJobID,
	})
	if err != nil {
		return fmt.Errorf("worker: marshal WhatsApp payload: %w", err)
	}
	task := asynq.NewTask(TaskNotifyWhatsApp, payload, asynq.MaxRetry(3))
	_, err = c.inner.EnqueueContext(ctx, task, asynq.Queue(QueueCritical))
	return err
}

// EnqueueSMS enqueues an SMS dispatch task.
func (c *Client) EnqueueSMS(ctx context.Context, to, message, notificationJobID string) error {
	payload, err := json.Marshal(SMSPayload{
		To:                to,
		Message:           message,
		NotificationJobID: notificationJobID,
	})
	if err != nil {
		return fmt.Errorf("worker: marshal SMS payload: %w", err)
	}
	task := asynq.NewTask(TaskNotifySMS, payload, asynq.MaxRetry(3))
	_, err = c.inner.EnqueueContext(ctx, task, asynq.Queue(QueueCritical))
	return err
}

// NewAsynqClient creates a raw asynq.Client (kept for backward compatibility
// with any code that constructs the server-side scheduler).
func NewAsynqClient(cfg *config.Config) (*asynq.Client, error) {
	opts, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("worker: parse Redis URI: %w", err)
	}
	return asynq.NewClient(opts), nil
}
