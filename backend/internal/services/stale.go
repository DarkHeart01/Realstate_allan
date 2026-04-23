// internal/services/stale.go
package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// StaleProperty is a minimal projection used by the stale-listing scanner.
type StaleProperty struct {
	ID           string
	OwnerContact string
	DaysUnsold   int
}

// StaleService scans for listings that have not been updated within the
// configured threshold and returns them for notification dispatch.
type StaleService struct {
	db *pgxpool.Pool
}

// NewStaleService constructs a StaleService.
func NewStaleService(db *pgxpool.Pool) *StaleService {
	return &StaleService{db: db}
}

// FindStale queries properties that are active (deleted_at IS NULL) and have
// not been updated for more than stale_listing_threshold_days (from app_config).
// Only SELLING listings are considered — BUYING listings are client-initiated
// and do not require owner reminders.
func (s *StaleService) FindStale(ctx context.Context) ([]StaleProperty, error) {
	threshold, err := s.loadThresholdDays(ctx)
	if err != nil {
		return nil, fmt.Errorf("stale: load threshold: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -threshold)

	rows, err := s.db.Query(ctx, `
		SELECT id::text,
		       owner_contact,
		       EXTRACT(DAY FROM NOW() - updated_at)::int AS days_unsold
		FROM   properties
		WHERE  deleted_at IS NULL
		  AND  listing_category = 'SELLING'
		  AND  updated_at < $1
		ORDER  BY updated_at ASC`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("stale: query: %w", err)
	}
	defer rows.Close()

	var result []StaleProperty
	for rows.Next() {
		var p StaleProperty
		if err := rows.Scan(&p.ID, &p.OwnerContact, &p.DaysUnsold); err != nil {
			return nil, fmt.Errorf("stale: scan: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// loadThresholdDays reads stale_listing_threshold_days from app_config.
// Defaults to 30 if the key is missing or unparseable.
func (s *StaleService) loadThresholdDays(ctx context.Context) (int, error) {
	var raw string
	err := s.db.QueryRow(ctx,
		`SELECT value FROM app_config WHERE key = 'stale_listing_threshold_days'`,
	).Scan(&raw)
	if err != nil {
		// Key missing — use default.
		return 30, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return 30, nil
	}
	return days, nil
}
