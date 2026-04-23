// internal/services/notification.go
package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/realestate/backend/internal/models"
)

// ErrJobNotFound is returned when a notification_jobs row cannot be found.
var ErrJobNotFound = errors.New("notification: job not found")

// NotificationService handles DB operations for the notification_jobs table.
type NotificationService struct {
	db *pgxpool.Pool
}

// NewNotificationService constructs a NotificationService.
func NewNotificationService(db *pgxpool.Pool) *NotificationService {
	return &NotificationService{db: db}
}

// CreateJob inserts a new PENDING notification_jobs row and returns the job ID.
func (s *NotificationService) CreateJob(ctx context.Context, propertyID *uuid.UUID, jobType string) (uuid.UUID, error) {
	var id uuid.UUID
	const q = `
		INSERT INTO notification_jobs (property_id, job_type, status, scheduled_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id`
	if err := s.db.QueryRow(ctx, q, propertyID, jobType, models.JobStatusPending).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("notification: create job: %w", err)
	}
	return id, nil
}

// UpdateJobStatus sets the status (and optionally twilio_sid + sent_at) for a job.
func (s *NotificationService) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status, twilioSID string) error {
	var sentAt *time.Time
	if status == models.JobStatusSent {
		now := time.Now()
		sentAt = &now
	}
	_, err := s.db.Exec(ctx,
		`UPDATE notification_jobs SET status=$1, twilio_sid=NULLIF($2,''), sent_at=$3 WHERE id=$4`,
		status, twilioSID, sentAt, jobID,
	)
	return err
}

// ListAll returns notification_jobs ordered newest first, with optional pagination.
func (s *NotificationService) ListAll(ctx context.Context, limit, offset int) ([]models.NotificationJob, int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, property_id, job_type, status, twilio_sid,
		       scheduled_at, sent_at, created_at
		FROM notification_jobs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("notification: list: %w", err)
	}
	defer rows.Close()

	jobs, err := scanJobs(rows)
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM notification_jobs`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("notification: count: %w", err)
	}

	return jobs, total, nil
}

// ListForProperty returns notification_jobs for a specific property.
func (s *NotificationService) ListForProperty(ctx context.Context, propertyID uuid.UUID, limit, offset int) ([]models.NotificationJob, int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, property_id, job_type, status, twilio_sid,
		       scheduled_at, sent_at, created_at
		FROM notification_jobs
		WHERE property_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, propertyID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("notification: list for property: %w", err)
	}
	defer rows.Close()

	jobs, err := scanJobs(rows)
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM notification_jobs WHERE property_id = $1`, propertyID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("notification: count for property: %w", err)
	}

	return jobs, total, nil
}

// GetJob fetches a single notification_jobs row by ID.
func (s *NotificationService) GetJob(ctx context.Context, id uuid.UUID) (*models.NotificationJob, error) {
	var j models.NotificationJob
	err := s.db.QueryRow(ctx, `
		SELECT id, property_id, job_type, status, twilio_sid,
		       scheduled_at, sent_at, created_at
		FROM notification_jobs WHERE id = $1`, id,
	).Scan(&j.ID, &j.PropertyID, &j.JobType, &j.Status, &j.TwilioSID,
		&j.ScheduledAt, &j.SentAt, &j.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("notification: get job: %w", err)
	}
	return &j, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanJobs(rows pgx.Rows) ([]models.NotificationJob, error) {
	defer rows.Close()
	var jobs []models.NotificationJob
	for rows.Next() {
		var j models.NotificationJob
		if err := rows.Scan(
			&j.ID, &j.PropertyID, &j.JobType, &j.Status, &j.TwilioSID,
			&j.ScheduledAt, &j.SentAt, &j.CreatedAt,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// ── pgx.Rows adapter ─────────────────────────────────────────────────────────
// scanJobs accepts pgx.Rows, which is returned by pool.Query(). The two
// call sites (ListAll, ListForProperty) pass the result of s.db.Query() directly.

