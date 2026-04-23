// internal/models/notification.go
package models

import (
	"time"

	"github.com/google/uuid"
)

// Notification job type constants — mirror the DB CHECK constraint.
const (
	JobTypeStaleListing = "STALE_LISTING"
	JobTypeFollowUp     = "FOLLOW_UP"
	JobTypeDealAlert    = "DEAL_ALERT"
)

// Notification job status constants.
const (
	JobStatusPending = "PENDING"
	JobStatusSent    = "SENT"
	JobStatusFailed  = "FAILED"
)

// OCR status constants.
const (
	OCRStatusPending = "PENDING"
	OCRStatusDone    = "DONE"
	OCRStatusFailed  = "FAILED"
)

// NotificationJob maps to the notification_jobs table.
type NotificationJob struct {
	ID          uuid.UUID  `json:"id"`
	PropertyID  *uuid.UUID `json:"property_id,omitempty"`
	JobType     string     `json:"job_type"`
	Status      string     `json:"status"`
	TwilioSID   *string    `json:"twilio_sid,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// OCRSuggestions holds field name → suggested string value parsed from OCR text.
type OCRSuggestions map[string]string

// OCRResult maps to the ocr_results table.
type OCRResult struct {
	ID          uuid.UUID      `json:"id"`
	PhotoID     uuid.UUID      `json:"photo_id"`
	PropertyID  uuid.UUID      `json:"property_id"`
	RawText     string         `json:"raw_text"`
	Suggestions OCRSuggestions `json:"suggestions"`
	Status      string         `json:"status"`
	Error       *string        `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
