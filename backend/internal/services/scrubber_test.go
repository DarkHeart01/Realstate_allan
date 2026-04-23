// internal/services/scrubber_test.go
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/realestate/backend/internal/models"
)

// ── Role-based scrubber ───────────────────────────────────────────────────────

func TestScrubForRole_SuperAdmin_PreservesAllFields(t *testing.T) {
	p := &models.Property{
		OwnerName:    "Allan Dias",
		OwnerContact: "+919876543210",
		Price:        5_000_000,
	}
	ScrubForRole(models.RoleSuperAdmin, p)
	assert.Equal(t, "Allan Dias", p.OwnerName)
	assert.Equal(t, "+919876543210", p.OwnerContact)
}

func TestScrubForRole_Broker_RedactsOwnerFields(t *testing.T) {
	p := &models.Property{
		OwnerName:    "Private Owner",
		OwnerContact: "+919999999999",
		Price:        7_500_000,
	}
	ScrubForRole(models.RoleBroker, p)
	assert.Equal(t, "", p.OwnerName)
	assert.Equal(t, "", p.OwnerContact)
}

func TestScrubForRole_Broker_PreservesNonSensitiveFields(t *testing.T) {
	area := 1200.0
	p := &models.Property{
		OwnerName:    "Owner",
		OwnerContact: "9876543210",
		Price:        3_000_000,
		BuiltUpArea:  &area,
		Tags:         []string{"3BHK", "Parking"},
	}
	ScrubForRole(models.RoleBroker, p)
	assert.Equal(t, float64(3_000_000), p.Price)
	require.NotNil(t, p.BuiltUpArea)
	assert.Equal(t, 1200.0, *p.BuiltUpArea)
	assert.Equal(t, []string{"3BHK", "Parking"}, p.Tags)
}

func TestScrubForRole_UnknownRole_RedactsOwnerFields(t *testing.T) {
	p := &models.Property{
		OwnerName:    "Owner",
		OwnerContact: "9876543210",
	}
	ScrubForRole("UNKNOWN_ROLE", p)
	assert.Equal(t, "", p.OwnerName)
	assert.Equal(t, "", p.OwnerContact)
}

func TestScrubForRole_NilProperty_NoPanic(t *testing.T) {
	// Must not panic on nil input.
	assert.NotPanics(t, func() {
		ScrubForRole(models.RoleBroker, nil)
	})
}

// ── Share scrubber (via toShareable) ─────────────────────────────────────────
// toShareable is the internal function that produces a scrubbed ShareableProperty.
// The ShareableProperty struct itself does not carry owner_name, owner_contact,
// location_lat, or location_lng — so scrubbing is structural by design.

func makeTestProperty() *models.Property {
	plotArea := 200.0
	builtUpArea := 150.0
	desc := "Spacious 3BHK near metro"
	return &models.Property{
		ID:              uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ListingCategory: models.ListingCategorySelling,
		PropertyType:    models.PropertyTypeFlat,
		OwnerName:       "Secret Owner",
		OwnerContact:    "+919876543210",
		Price:           5_000_000,
		PlotArea:        &plotArea,
		BuiltUpArea:     &builtUpArea,
		LocationLat:     19.0760,
		LocationLng:     72.8777,
		Description:     &desc,
		Tags:            []string{"Metro", "3BHK"},
		IsDirectOwner:   true,
	}
}

func TestScrubForShare_RemovesOwnerName(t *testing.T) {
	p := makeTestProperty()
	s := toShareable(p, nil, "Mumbai, Maharashtra")
	// ShareableProperty has no OwnerName field — owner identity is structurally absent.
	// Verify the returned struct is of the expected type (no owner fields exposed).
	assert.Equal(t, p.ID, s.ID, "ID should be preserved")
	// Verify no reflection of owner_name in the formatted text.
	plain := buildPlainText(s)
	assert.NotContains(t, plain, "Secret Owner")
}

func TestScrubForShare_RemovesOwnerContact(t *testing.T) {
	p := makeTestProperty()
	s := toShareable(p, nil, "Mumbai")
	plain := buildPlainText(s)
	assert.NotContains(t, plain, "+919876543210")
	assert.NotContains(t, plain, "9876543210")
}

func TestScrubForShare_RemovesExactCoordinates(t *testing.T) {
	p := makeTestProperty()
	s := toShareable(p, nil, "Location available on request")
	// ShareableProperty only has LocationLabel — exact lat/lng are not present.
	// (There are no LocationLat/LocationLng fields on ShareableProperty.)
	assert.Equal(t, "Location available on request", s.LocationLabel)
}

func TestScrubForShare_PreservesPrice(t *testing.T) {
	p := makeTestProperty()
	s := toShareable(p, nil, "Mumbai")
	assert.Equal(t, float64(5_000_000), s.Price)
	assert.Equal(t, "₹50,00,000", s.PriceFormatted)
}

func TestScrubForShare_PreservesDescription(t *testing.T) {
	p := makeTestProperty()
	s := toShareable(p, nil, "Mumbai")
	require.NotNil(t, s.Description)
	assert.Equal(t, "Spacious 3BHK near metro", *s.Description)
}

func TestScrubForShare_CalculatesPricePerSqm_BuiltUp(t *testing.T) {
	// built_up_area = 100 sqm, price = 1_000_000 → 10_000 per sqm
	area := 100.0
	p := &models.Property{
		Price:       1_000_000,
		BuiltUpArea: &area,
	}
	s := toShareable(p, nil, "")
	assert.InDelta(t, 10_000.0, s.PricePerSqm, 0.01)
}

func TestScrubForShare_CalculatesPricePerSqm_PlotFallback(t *testing.T) {
	// No built_up_area — should fall back to plot_area.
	area := 100.0
	p := &models.Property{
		Price:    1_000_000,
		PlotArea: &area,
		// BuiltUpArea intentionally nil
	}
	s := toShareable(p, nil, "")
	assert.InDelta(t, 10_000.0, s.PricePerSqm, 0.01)
}

func TestScrubForShare_PricePerSqm_ZeroWhenNoArea(t *testing.T) {
	// Neither area set → PricePerSqm should be 0 and PricePerSqmFmt should be "".
	p := &models.Property{
		Price: 5_000_000,
		// PlotArea and BuiltUpArea both nil
	}
	s := toShareable(p, nil, "")
	assert.Equal(t, float64(0), s.PricePerSqm)
	assert.Equal(t, "", s.PricePerSqmFmt)
}

func TestScrubForShare_BuiltUpTakesPrecedenceOverPlot(t *testing.T) {
	plot := 200.0
	built := 150.0
	p := &models.Property{
		Price:       1_500_000,
		PlotArea:    &plot,
		BuiltUpArea: &built,
	}
	s := toShareable(p, nil, "")
	// Should use built_up_area (150), not plot_area (200).
	expected := 1_500_000.0 / 150.0
	assert.InDelta(t, expected, s.PricePerSqm, 0.01)
}

func TestScrubForShare_PhotoURLsPreserved(t *testing.T) {
	p := makeTestProperty()
	urls := []string{"https://cdn.example.com/photo1.jpg", "https://cdn.example.com/photo2.jpg"}
	s := toShareable(p, urls, "Mumbai")
	assert.Equal(t, urls, s.PhotoURLs)
}

func TestScrubForShare_NilPhotos_EmptySlice(t *testing.T) {
	p := makeTestProperty()
	s := toShareable(p, nil, "Mumbai")
	assert.NotNil(t, s.PhotoURLs)
	assert.Empty(t, s.PhotoURLs)
}
