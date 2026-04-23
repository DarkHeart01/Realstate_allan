// internal/gcs/gcs.go
package gcs

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/storage"

	"github.com/realestate/backend/internal/config"
)

// GCSClient wraps the GCS storage client with app-specific helpers.
// If GCS_BUCKET is not configured, all presign/delete calls return ErrNotConfigured.
type GCSClient struct {
	client     *storage.Client
	bucket     string
	cdnBase    string
	configured bool
}

// ErrNotConfigured is returned when GCS_BUCKET is not set in config.
const ErrNotConfigured = gcsError("GCS_NOT_CONFIGURED")

type gcsError string

func (e gcsError) Error() string { return string(e) }

// New initialises a GCS client using Application Default Credentials (ADC).
// ADC resolves in this order:
//  1. GOOGLE_APPLICATION_CREDENTIALS env var → service account JSON key file
//  2. GCE/GKE metadata server (when deployed on GCP)
//  3. `gcloud auth application-default login` (local dev)
//
// The code contains zero explicit credential handling — no JSON parsing, no key embedding.
// If GCS_BUCKET is empty, the client is marked unconfigured and all media endpoints
// will return GCS_NOT_CONFIGURED without affecting other endpoints.
func New(ctx context.Context, cfg *config.Config) (*GCSClient, error) {
	if cfg.GCSBucket == "" {
		return &GCSClient{configured: false}, nil
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs: failed to create storage client: %w", err)
	}

	return &GCSClient{
		client:     client,
		bucket:     cfg.GCSBucket,
		cdnBase:    cfg.GCSCDNBaseURL,
		configured: true,
	}, nil
}

// IsConfigured reports whether GCS is ready for use.
func (g *GCSClient) IsConfigured() bool { return g.configured }

// CDNUrl returns the public CDN URL for a given GCS object key.
func (g *GCSClient) CDNUrl(objectKey string) string {
	return g.cdnBase + "/" + objectKey
}

// PresignPut generates a V4 signed PUT URL that lets the Flutter client upload
// a file directly to GCS without routing through the API server.
// The signed URL is self-authenticating — do NOT attach an Authorization header.
func (g *GCSClient) PresignPut(ctx context.Context, objectKey string, contentType string, ttl time.Duration) (string, error) {
	if !g.configured {
		return "", ErrNotConfigured
	}

	opts := &storage.SignedURLOptions{
		Scheme:      storage.SigningSchemeV4,
		Method:      "PUT",
		Expires:     time.Now().Add(ttl),
		ContentType: contentType,
	}

	url, err := g.client.Bucket(g.bucket).SignedURL(objectKey, opts)
	if err != nil {
		return "", fmt.Errorf("gcs: failed to generate signed URL: %w", err)
	}

	return url, nil
}

// DeleteObject removes an object from GCS.
func (g *GCSClient) DeleteObject(ctx context.Context, objectKey string) error {
	if !g.configured {
		return ErrNotConfigured
	}

	if err := g.client.Bucket(g.bucket).Object(objectKey).Delete(ctx); err != nil {
		return fmt.Errorf("gcs: failed to delete object %q: %w", objectKey, err)
	}

	return nil
}
