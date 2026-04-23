// internal/services/share.go
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/realestate/backend/internal/models"
	"github.com/realestate/backend/internal/utils"
)

const geocodeCacheTTL = 24 * time.Hour

// ShareableProperty is a scrubbed view of a property safe for client sharing:
// no owner name/contact, no exact coordinates, human-readable location label.
type ShareableProperty struct {
	ID              uuid.UUID  `json:"id"`
	ListingCategory string     `json:"listing_category"`
	PropertyType    string     `json:"property_type"`
	Price           float64    `json:"price"`
	PriceFormatted  string     `json:"price_formatted"`
	PlotArea        *float64   `json:"plot_area,omitempty"`
	BuiltUpArea     *float64   `json:"built_up_area,omitempty"`
	LocationLabel   string     `json:"location_label"`
	Description     *string    `json:"description,omitempty"`
	Tags            []string   `json:"tags"`
	IsDirectOwner   bool       `json:"is_direct_owner"`
	PhotoURLs       []string   `json:"photo_urls"`
	PricePerSqm     float64    `json:"price_per_sqm,omitempty"`
	PricePerSqmFmt  string     `json:"price_per_sqm_formatted,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ShareResult is the full response from ShareService.Generate.
type ShareResult struct {
	Property        ShareableProperty `json:"property"`
	PlainText       string            `json:"plain_text"`
	WhatsAppMessage string            `json:"whatsapp_message"`
}

// ShareService generates shareable property snapshots.
type ShareService struct {
	db          *pgxpool.Pool
	redisClient *redis.Client
	mapsAPIKey  string
}

// NewShareService constructs a ShareService.
func NewShareService(db *pgxpool.Pool, redisClient *redis.Client, mapsAPIKey string) *ShareService {
	return &ShareService{db: db, redisClient: redisClient, mapsAPIKey: mapsAPIKey}
}

// Generate fetches a property, scrubs it, reverse-geocodes its location, and
// returns a ShareResult with formatted message templates.
func (s *ShareService) Generate(ctx context.Context, propertyID uuid.UUID) (*ShareResult, error) {
	prop, err := s.fetchProperty(ctx, propertyID)
	if errors.Is(err, ErrPropertyNotFound) {
		return nil, ErrPropertyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("share: fetch property: %w", err)
	}

	photos, err := s.fetchConfirmedPhotos(ctx, propertyID)
	if err != nil {
		log.Printf("share: fetch photos for %s: %v (continuing)", propertyID, err)
		photos = []string{}
	}

	locationLabel := s.reverseGeocode(ctx, prop.LocationLat, prop.LocationLng)

	shareable := toShareable(prop, photos, locationLabel)
	result := &ShareResult{
		Property:        shareable,
		PlainText:       buildPlainText(shareable),
		WhatsAppMessage: buildWhatsApp(shareable),
	}

	return result, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (s *ShareService) fetchProperty(ctx context.Context, id uuid.UUID) (*models.Property, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, listing_category, property_type, owner_name, owner_contact,
		       price, plot_area, built_up_area, location_lat, location_lng,
		       description, tags, is_direct_owner, assigned_broker_id,
		       created_by, created_at, updated_at
		FROM properties
		WHERE id = $1 AND deleted_at IS NULL`, id)

	p := &models.Property{}
	err := row.Scan(
		&p.ID, &p.ListingCategory, &p.PropertyType, &p.OwnerName, &p.OwnerContact,
		&p.Price, &p.PlotArea, &p.BuiltUpArea, &p.LocationLat, &p.LocationLng,
		&p.Description, &p.Tags, &p.IsDirectOwner, &p.AssignedBrokerID,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPropertyNotFound
		}
		return nil, err
	}
	return p, nil
}

func (s *ShareService) fetchConfirmedPhotos(ctx context.Context, propertyID uuid.UUID) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT cdn_url FROM property_photos
		WHERE property_id = $1 AND status = $2
		ORDER BY display_order ASC`, propertyID, models.PhotoStatusConfirmed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		urls = append(urls, u)
	}
	return urls, rows.Err()
}

// reverseGeocode resolves a lat/lng to a human-readable label. It checks Redis
// first (24h TTL). If the Maps API key is not configured or the call fails, it
// returns the fallback string.
func (s *ShareService) reverseGeocode(ctx context.Context, lat, lng float64) string {
	const fallback = "Location available on request"

	if s.mapsAPIKey == "" {
		return fallback
	}

	cacheKey := fmt.Sprintf("geocode:%.4f:%.4f", lat, lng)

	// Check cache.
	if cached, err := s.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		return cached
	}

	label := s.callGeocodeAPI(lat, lng)
	if label == "" {
		return fallback
	}

	// Store in cache — best-effort, ignore errors.
	if err := s.redisClient.Set(ctx, cacheKey, label, geocodeCacheTTL).Err(); err != nil {
		log.Printf("share: geocode cache set: %v", err)
	}

	return label
}

// callGeocodeAPI makes a single reverse-geocode request to Google Maps.
func (s *ShareService) callGeocodeAPI(lat, lng float64) string {
	endpoint := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/geocode/json?latlng=%s&key=%s",
		url.QueryEscape(fmt.Sprintf("%.6f,%.6f", lat, lng)),
		url.QueryEscape(s.mapsAPIKey),
	)

	resp, err := http.Get(endpoint) //nolint:gosec // URL is constructed from validated config
	if err != nil {
		log.Printf("share: geocode API error: %v", err)
		return ""
	}
	defer resp.Body.Close()

	var payload struct {
		Status  string `json:"status"`
		Results []struct {
			FormattedAddress string `json:"formatted_address"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		log.Printf("share: geocode decode error: %v", err)
		return ""
	}
	if payload.Status != "OK" || len(payload.Results) == 0 {
		log.Printf("share: geocode status %s", payload.Status)
		return ""
	}

	return payload.Results[0].FormattedAddress
}

// toShareable converts a raw Property into a ShareableProperty, dropping all
// sensitive fields (owner info, exact coordinates).
func toShareable(p *models.Property, photoURLs []string, locationLabel string) ShareableProperty {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	if photoURLs == nil {
		photoURLs = []string{}
	}

	area := effectiveArea(p)
	pricePerSqm := utils.FormatPricePerSqm(p.Price, area)

	s := ShareableProperty{
		ID:              p.ID,
		ListingCategory: p.ListingCategory,
		PropertyType:    p.PropertyType,
		Price:           p.Price,
		PriceFormatted:  utils.FormatIndianNumber(p.Price),
		PlotArea:        p.PlotArea,
		BuiltUpArea:     p.BuiltUpArea,
		LocationLabel:   locationLabel,
		Description:     p.Description,
		Tags:            tags,
		IsDirectOwner:   p.IsDirectOwner,
		PhotoURLs:       photoURLs,
		PricePerSqm:     pricePerSqm,
		CreatedAt:       p.CreatedAt,
	}
	if pricePerSqm > 0 {
		s.PricePerSqmFmt = utils.FormatIndianNumber(pricePerSqm) + "/sqm"
	}
	return s
}

func effectiveArea(p *models.Property) float64 {
	if p.BuiltUpArea != nil && *p.BuiltUpArea > 0 {
		return *p.BuiltUpArea
	}
	if p.PlotArea != nil && *p.PlotArea > 0 {
		return *p.PlotArea
	}
	return 0
}

func buildPlainText(p ShareableProperty) string {
	var b strings.Builder

	action := "For Sale"
	if p.ListingCategory == models.ListingCategoryBuying {
		action = "Wanted to Buy"
	}

	fmt.Fprintf(&b, "%s - %s\n", action, propertyTypeLabel(p.PropertyType))
	fmt.Fprintf(&b, "Price: %s\n", p.PriceFormatted)
	fmt.Fprintf(&b, "Location: %s\n", p.LocationLabel)

	if p.PricePerSqmFmt != "" {
		fmt.Fprintf(&b, "Price per sqm: %s\n", p.PricePerSqmFmt)
	}

	if areaLine := formatAreaLine(p); areaLine != "" {
		fmt.Fprintf(&b, "Area: %s\n", areaLine)
	}

	if p.Description != nil && *p.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", *p.Description)
	}

	if len(p.Tags) > 0 {
		fmt.Fprintf(&b, "\nTags: %s\n", strings.Join(p.Tags, ", "))
	}

	if p.IsDirectOwner {
		b.WriteString("\nDirect owner listing — no brokerage from owner side.\n")
	}

	b.WriteString("\nContact us for more details.")

	return b.String()
}

func buildWhatsApp(p ShareableProperty) string {
	var b strings.Builder

	action := "For Sale"
	if p.ListingCategory == models.ListingCategoryBuying {
		action = "Wanted to Buy"
	}

	fmt.Fprintf(&b, "*%s - %s*\n\n", action, propertyTypeLabel(p.PropertyType))
	fmt.Fprintf(&b, "💰 *Price:* %s\n", p.PriceFormatted)
	fmt.Fprintf(&b, "📍 *Location:* %s\n", p.LocationLabel)

	if p.PricePerSqmFmt != "" {
		fmt.Fprintf(&b, "📊 *Rate:* %s\n", p.PricePerSqmFmt)
	}

	if areaLine := formatAreaLine(p); areaLine != "" {
		fmt.Fprintf(&b, "📐 *Area:* %s\n", areaLine)
	}

	if p.Description != nil && *p.Description != "" {
		fmt.Fprintf(&b, "\n_%s_\n", *p.Description)
	}

	if len(p.Tags) > 0 {
		fmt.Fprintf(&b, "\n🏷️ %s\n", strings.Join(p.Tags, " · "))
	}

	if p.IsDirectOwner {
		b.WriteString("\n✅ _Direct owner — no brokerage from owner side._\n")
	}

	b.WriteString("\n_Contact us for more details._")

	return b.String()
}

func propertyTypeLabel(t string) string {
	switch t {
	case models.PropertyTypePlot:
		return "Plot"
	case models.PropertyTypeShop:
		return "Shop"
	case models.PropertyTypeFlat:
		return "Flat"
	default:
		return "Property"
	}
}

func formatAreaLine(p ShareableProperty) string {
	var parts []string
	if p.PlotArea != nil && *p.PlotArea > 0 {
		parts = append(parts, fmt.Sprintf("%.0f sqm plot", *p.PlotArea))
	}
	if p.BuiltUpArea != nil && *p.BuiltUpArea > 0 {
		parts = append(parts, fmt.Sprintf("%.0f sqm built-up", *p.BuiltUpArea))
	}
	return strings.Join(parts, " / ")
}
