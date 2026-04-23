// internal/services/ocr.go
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2/google"

	"github.com/realestate/backend/internal/models"
)

// ErrOCRNotEnabled is returned when Vision API is not configured.
var ErrOCRNotEnabled = errors.New("ocr: Vision API not enabled")

// ErrOCRResultNotFound is returned when no ocr_result row exists for the photo.
var ErrOCRResultNotFound = errors.New("ocr: result not found")

// OCRService calls the Google Cloud Vision API to extract text from a photo
// and parses field suggestions for property autofill.
type OCRService struct {
	db      *pgxpool.Pool
	enabled bool
	bucket  string // GCS bucket name for building gs:// URIs
}

// NewOCRService constructs an OCRService.
// enabled should be cfg.GoogleVisionEnabled; bucket is cfg.GCSBucket.
func NewOCRService(db *pgxpool.Pool, enabled bool, bucket string) *OCRService {
	return &OCRService{db: db, enabled: enabled, bucket: bucket}
}

// IsEnabled reports whether Vision API calls are active.
func (s *OCRService) IsEnabled() bool { return s.enabled }

// ── ScanPhoto — main entry point ──────────────────────────────────────────────

// ScanPhoto calls Vision API for the given photo, parses suggestions, and
// upserts the result into ocr_results.  Returns the OCRResult.
func (s *OCRService) ScanPhoto(ctx context.Context, photoID, propertyID uuid.UUID) (*models.OCRResult, error) {
	if !s.enabled {
		return nil, ErrOCRNotEnabled
	}

	// Fetch the GCS key for the photo.
	var gcsKey, cdnURL string
	err := s.db.QueryRow(ctx,
		`SELECT gcs_key, cdn_url FROM property_photos WHERE id = $1 AND status = $2`,
		photoID, models.PhotoStatusConfirmed,
	).Scan(&gcsKey, &cdnURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPhotoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ocr: fetch photo: %w", err)
	}

	rawText, err := s.callVisionAPI(ctx, s.bucket, gcsKey)
	if err != nil {
		// Store failure in DB, then surface the error.
		errMsg := err.Error()
		s.upsertResult(ctx, photoID, propertyID, "", models.OCRSuggestions{}, models.OCRStatusFailed, &errMsg)
		return nil, fmt.Errorf("ocr: Vision API: %w", err)
	}

	suggestions := parseOCRText(rawText)
	result := s.upsertResult(ctx, photoID, propertyID, rawText, suggestions, models.OCRStatusDone, nil)
	return result, nil
}

// GetResult fetches an existing ocr_result row for the given photo.
func (s *OCRService) GetResult(ctx context.Context, photoID uuid.UUID) (*models.OCRResult, error) {
	var r models.OCRResult
	var suggestionsJSON []byte
	var errText *string

	err := s.db.QueryRow(ctx, `
		SELECT id, photo_id, property_id, raw_text, suggestions, status, error, created_at, updated_at
		FROM ocr_results WHERE photo_id = $1
		ORDER BY created_at DESC LIMIT 1`, photoID,
	).Scan(&r.ID, &r.PhotoID, &r.PropertyID, &r.RawText, &suggestionsJSON,
		&r.Status, &errText, &r.CreatedAt, &r.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOCRResultNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ocr: get result: %w", err)
	}

	r.Error = errText
	if err := json.Unmarshal(suggestionsJSON, &r.Suggestions); err != nil {
		r.Suggestions = models.OCRSuggestions{}
	}

	return &r, nil
}

// ── Vision API call ───────────────────────────────────────────────────────────

type visionRequest struct {
	Requests []visionImageRequest `json:"requests"`
}

type visionImageRequest struct {
	Image    visionImage    `json:"image"`
	Features []visionFeature `json:"features"`
}

type visionImage struct {
	Source *visionImageSource `json:"source,omitempty"`
}

type visionImageSource struct {
	GCSImageURI string `json:"gcsImageUri,omitempty"`
}

type visionFeature struct {
	Type       string `json:"type"`
	MaxResults int    `json:"maxResults"`
}

type visionResponse struct {
	Responses []struct {
		FullTextAnnotation *struct {
			Text string `json:"text"`
		} `json:"fullTextAnnotation"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"responses"`
}

func (s *OCRService) callVisionAPI(ctx context.Context, bucket, gcsKey string) (string, error) {
	// Get an ADC access token for the Vision API scope.
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-vision")
	if err != nil {
		return "", fmt.Errorf("ocr: get token source: %w", err)
	}
	token, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("ocr: get token: %w", err)
	}

	gcsURI := fmt.Sprintf("gs://%s/%s", bucket, gcsKey)
	reqBody := visionRequest{
		Requests: []visionImageRequest{{
			Image: visionImage{
				Source: &visionImageSource{GCSImageURI: gcsURI},
			},
			Features: []visionFeature{{
				Type:       "TEXT_DETECTION",
				MaxResults: 1,
			}},
		}},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ocr: marshal request: %w", err)
	}

	httpCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost,
		"https://vision.googleapis.com/v1/images:annotate",
		bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ocr: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ocr: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var vResp visionResponse
	if err := json.Unmarshal(raw, &vResp); err != nil {
		return "", fmt.Errorf("ocr: decode response: %w", err)
	}

	if len(vResp.Responses) == 0 {
		return "", fmt.Errorf("ocr: empty response from Vision API")
	}
	if vResp.Responses[0].Error != nil {
		return "", fmt.Errorf("ocr: Vision API error: %s", vResp.Responses[0].Error.Message)
	}
	if vResp.Responses[0].FullTextAnnotation == nil {
		return "", nil // no text detected — not an error
	}

	return vResp.Responses[0].FullTextAnnotation.Text, nil
}

// ── Text parsing ──────────────────────────────────────────────────────────────

var (
	// Price patterns: ₹50,00,000 / 50 lakh / 1.5 crore / Rs. 5000000
	priceRe = regexp.MustCompile(`(?i)(?:₹|rs\.?\s*)?\s*(\d[\d,\.]*)\s*(?:crore|cr\.?|lakh|lac|l\.?|lacs)?`)

	// Area patterns: 1200 sq ft / 1200 sqft / 1200 sq.m / 110 sqm
	areaRe = regexp.MustCompile(`(?i)(\d[\d,\.]*)\s*(?:sq\.?\s*(?:ft|feet|m|meter|metre)|sqft|sqm)`)

	// Indian phone: 10-digit starting with 6-9, with optional +91 or 0
	phoneRe = regexp.MustCompile(`(?:(?:\+91|0)\s*)?[6-9]\d{9}`)
)

// parseOCRText extracts property field suggestions from raw OCR text.
func parseOCRText(text string) models.OCRSuggestions {
	s := models.OCRSuggestions{}

	if text == "" {
		return s
	}

	// Price — take the largest number that looks like a property price (> 10000).
	if price := extractLargestPrice(text); price > 10000 {
		s["price"] = strconv.FormatInt(price, 10)
	}

	// Area — take the first area match.
	if m := areaRe.FindStringSubmatch(text); len(m) > 1 {
		clean := strings.ReplaceAll(m[1], ",", "")
		s["area"] = clean
	}

	// Phone — take the first match.
	if m := phoneRe.FindString(text); m != "" {
		// Normalise to 10 digits.
		digits := regexp.MustCompile(`\D`).ReplaceAllString(m, "")
		if len(digits) > 10 {
			digits = digits[len(digits)-10:]
		}
		s["owner_contact"] = digits
	}

	return s
}

// extractLargestPrice finds the largest price-like number in the text.
// It converts "lakh/crore" suffixes to absolute values.
func extractLargestPrice(text string) int64 {
	matches := priceRe.FindAllStringSubmatch(text, -1)
	lower := strings.ToLower(text)

	var best int64
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := strings.ReplaceAll(m[1], ",", "")
		raw = strings.TrimSpace(raw)
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}

		// Check surrounding context for unit multipliers.
		idx := strings.Index(lower, strings.ToLower(raw))
		suffix := ""
		if idx >= 0 && idx+len(raw) < len(lower) {
			suffix = lower[idx+len(raw) : min(idx+len(raw)+10, len(lower))]
		}

		var v int64
		switch {
		case strings.Contains(suffix, "crore") || strings.Contains(suffix, "cr"):
			v = int64(f * 1e7)
		case strings.Contains(suffix, "lakh") || strings.Contains(suffix, "lac") || strings.Contains(suffix, " l"):
			v = int64(f * 1e5)
		default:
			v = int64(f)
		}

		if v > best {
			best = v
		}
	}
	return best
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── DB upsert ─────────────────────────────────────────────────────────────────

func (s *OCRService) upsertResult(
	ctx context.Context,
	photoID, propertyID uuid.UUID,
	rawText string,
	suggestions models.OCRSuggestions,
	status string,
	errText *string,
) *models.OCRResult {
	sugJSON, _ := json.Marshal(suggestions)

	var r models.OCRResult
	err := s.db.QueryRow(ctx, `
		INSERT INTO ocr_results (photo_id, property_id, raw_text, suggestions, status, error)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6)
		ON CONFLICT (photo_id) DO UPDATE
		  SET raw_text = EXCLUDED.raw_text,
		      suggestions = EXCLUDED.suggestions,
		      status = EXCLUDED.status,
		      error = EXCLUDED.error,
		      updated_at = NOW()
		RETURNING id, photo_id, property_id, raw_text, suggestions, status, error, created_at, updated_at`,
		photoID, propertyID, rawText, string(sugJSON), status, errText,
	).Scan(&r.ID, &r.PhotoID, &r.PropertyID, &r.RawText, &sugJSON,
		&r.Status, &r.Error, &r.CreatedAt, &r.UpdatedAt)

	if err != nil {
		log.Printf("ocr: upsert result: %v", err)
		return &models.OCRResult{
			PhotoID:     photoID,
			PropertyID:  propertyID,
			RawText:     rawText,
			Suggestions: suggestions,
			Status:      status,
			Error:       errText,
		}
	}
	_ = json.Unmarshal(sugJSON, &r.Suggestions)
	return &r
}

