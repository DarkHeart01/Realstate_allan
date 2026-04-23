// internal/services/photos.go
package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/realestate/backend/internal/gcs"
	"github.com/realestate/backend/internal/models"
)

// Sentinel errors for photos.
var (
	ErrPhotoNotFound    = errors.New("photo: not found")
	ErrGCSNotConfigured = errors.New("photo: GCS not configured")
)

// PresignResult holds the response data for a successful presign call.
type PresignResult struct {
	PhotoID   uuid.UUID
	UploadURL string
	CDNUrl    string
}

// PhotoService handles photo presign/confirm/delete operations.
type PhotoService struct {
	db  *pgxpool.Pool
	gcs *gcs.GCSClient
}

// NewPhotoService constructs a PhotoService.
func NewPhotoService(db *pgxpool.Pool, gcsClient *gcs.GCSClient) *PhotoService {
	return &PhotoService{db: db, gcs: gcsClient}
}

// IsConfigured reports whether GCS storage is ready.
func (s *PhotoService) IsConfigured() bool { return s.gcs.IsConfigured() }

// allowedContentTypes for photo uploads.
var allowedContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9.\-]`)

// sanitiseFilename lowercases and replaces any non-alphanumeric/dot/dash char with "_".
func sanitiseFilename(name string) string {
	lower := strings.ToLower(name)
	return nonAlphanumRe.ReplaceAllString(lower, "_")
}

// Presign creates a PENDING photo record and returns a GCS V4 signed PUT URL.
func (s *PhotoService) Presign(ctx context.Context, propertyID uuid.UUID, filename, contentType string) (*PresignResult, error) {
	if !s.gcs.IsConfigured() {
		return nil, ErrGCSNotConfigured
	}
	if !allowedContentTypes[contentType] {
		return nil, fmt.Errorf("photo: unsupported content type %q", contentType)
	}

	photoID := uuid.New()
	objectKey := fmt.Sprintf("properties/%s/%s-%s", propertyID, photoID, sanitiseFilename(filename))
	cdnUrl := s.gcs.CDNUrl(objectKey)

	uploadURL, err := s.gcs.PresignPut(ctx, objectKey, contentType, 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("photo presign: %w", err)
	}

	const q = `
		INSERT INTO property_photos (id, property_id, cdn_url, gcs_key, status)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := s.db.Exec(ctx, q, photoID, propertyID, cdnUrl, objectKey, models.PhotoStatusPending); err != nil {
		return nil, fmt.Errorf("photo presign insert: %w", err)
	}

	return &PresignResult{
		PhotoID:   photoID,
		UploadURL: uploadURL,
		CDNUrl:    cdnUrl,
	}, nil
}

// Confirm marks a photo as CONFIRMED after the client successfully PUTs to GCS.
func (s *PhotoService) Confirm(ctx context.Context, propertyID, photoID uuid.UUID) (*models.PropertyPhoto, error) {
	const q = `
		UPDATE property_photos
		SET status = $1
		WHERE id = $2 AND property_id = $3
		RETURNING id, property_id, cdn_url, gcs_key, display_order, status, created_at`

	var p models.PropertyPhoto
	err := s.db.QueryRow(ctx, q, models.PhotoStatusConfirmed, photoID, propertyID).
		Scan(&p.ID, &p.PropertyID, &p.CDNUrl, &p.GCSKey, &p.DisplayOrder, &p.Status, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPhotoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("photo confirm: %w", err)
	}
	return &p, nil
}

// Delete hard-deletes a photo record. The GCS object is intentionally NOT
// deleted here — the gcs_key is logged for a background cleanup job (Phase 3).
func (s *PhotoService) Delete(ctx context.Context, propertyID, photoID uuid.UUID) error {
	var gcsKey string
	err := s.db.QueryRow(ctx,
		`DELETE FROM property_photos WHERE id = $1 AND property_id = $2 RETURNING gcs_key`,
		photoID, propertyID,
	).Scan(&gcsKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPhotoNotFound
	}
	if err != nil {
		return fmt.Errorf("photo delete: %w", err)
	}

	// Log for future GCS cleanup (background task in Phase 3).
	log.Printf("photo deleted from DB — GCS object pending cleanup: %s", gcsKey)
	return nil
}

// IsOwnerOrAdmin returns true if callerID owns the property or is a SUPER_ADMIN.
func (s *PhotoService) IsOwnerOrAdmin(ctx context.Context, propertyID uuid.UUID, callerID uuid.UUID, callerRole string) (bool, error) {
	if callerRole == models.RoleSuperAdmin {
		return true, nil
	}
	var createdBy uuid.UUID
	err := s.db.QueryRow(ctx,
		`SELECT created_by FROM properties WHERE id = $1 AND deleted_at IS NULL`, propertyID,
	).Scan(&createdBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrPropertyNotFound
	}
	if err != nil {
		return false, err
	}
	return createdBy == callerID, nil
}
