// internal/services/ocr_parser_test.go
// Tests for parseOCRText and extractLargestPrice — pure functions in the same package.
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Price extraction ──────────────────────────────────────────────────────────

func TestParseOCRText_PriceWithRupeeSymbol(t *testing.T) {
	s := parseOCRText("₹ 50,00,000 flat for sale")
	price, ok := s["price"]
	require.True(t, ok, "price key must be present")
	assert.Equal(t, "5000000", price)
}

func TestParseOCRText_PriceWithRsPrefix(t *testing.T) {
	s := parseOCRText("Rs. 75 Lakh negotiable")
	price, ok := s["price"]
	require.True(t, ok, "price key must be present")
	assert.Equal(t, "7500000", price)
}

func TestParseOCRText_PriceLakhNotation(t *testing.T) {
	s := parseOCRText("50 lakh property available")
	price, ok := s["price"]
	require.True(t, ok)
	assert.Equal(t, "5000000", price)
}

func TestParseOCRText_PriceCroreNotation(t *testing.T) {
	s := parseOCRText("1.5 crore luxury villa")
	price, ok := s["price"]
	require.True(t, ok)
	assert.Equal(t, "15000000", price)
}

func TestParseOCRText_PriceCroreDecimal(t *testing.T) {
	s := parseOCRText("2.25 crore duplex")
	price, ok := s["price"]
	require.True(t, ok)
	assert.Equal(t, "22500000", price)
}

func TestParseOCRText_NoPriceFound(t *testing.T) {
	s := parseOCRText("beautiful property, contact owner")
	_, ok := s["price"]
	assert.False(t, ok, "price key should be absent when no price-like number found")
}

func TestParseOCRText_PriceMustExceedThreshold(t *testing.T) {
	// Numbers <= 10000 are filtered out (too small to be property prices).
	s := parseOCRText("5000 sq ft plot")
	// "5000" should NOT appear as price since it's <= 10000.
	// The area "5000" would appear as area, not price.
	_, priceOk := s["price"]
	assert.False(t, priceOk)
}

func TestParseOCRText_ExtractLargestPriceAmongMultiple(t *testing.T) {
	// Two prices present — should return the larger one.
	s := parseOCRText("reduced from 80 lakh to 70 lakh")
	price, ok := s["price"]
	require.True(t, ok)
	// 80 lakh = 8000000 > 70 lakh = 7000000
	assert.Equal(t, "8000000", price)
}

// ── Area extraction ───────────────────────────────────────────────────────────

func TestParseOCRText_AreaSqm(t *testing.T) {
	s := parseOCRText("1200 sqm built up area")
	area, ok := s["area"]
	require.True(t, ok)
	assert.Equal(t, "1200", area)
}

func TestParseOCRText_AreaSqft(t *testing.T) {
	// The parser stores the raw number without unit conversion.
	s := parseOCRText("1500 sqft plot available")
	area, ok := s["area"]
	require.True(t, ok)
	assert.Equal(t, "1500", area)
}

func TestParseOCRText_AreaWithCommas(t *testing.T) {
	s := parseOCRText("1,500 sq ft")
	area, ok := s["area"]
	require.True(t, ok)
	// Commas stripped.
	assert.Equal(t, "1500", area)
}

func TestParseOCRText_AreaTakesFirstMatch(t *testing.T) {
	// Two area mentions — first one wins.
	s := parseOCRText("Plot: 500 sqm, Built-up: 350 sqm")
	area, ok := s["area"]
	require.True(t, ok)
	assert.Equal(t, "500", area)
}

func TestParseOCRText_NoAreaFound(t *testing.T) {
	s := parseOCRText("luxury property priced at 1 crore")
	_, ok := s["area"]
	assert.False(t, ok)
}

// ── Phone / contact extraction ────────────────────────────────────────────────

func TestParseOCRText_Phone10Digit(t *testing.T) {
	s := parseOCRText("Call 9876543210 for details")
	contact, ok := s["owner_contact"]
	require.True(t, ok)
	assert.Equal(t, "9876543210", contact)
}

func TestParseOCRText_PhoneWithZeroPrefix(t *testing.T) {
	s := parseOCRText("Call 09876543210 for details")
	contact, ok := s["owner_contact"]
	require.True(t, ok)
	// Leading 0 stripped, last 10 digits kept.
	assert.Equal(t, "9876543210", contact)
}

func TestParseOCRText_PhoneWithPlus91Prefix(t *testing.T) {
	s := parseOCRText("Contact +919876543210")
	contact, ok := s["owner_contact"]
	require.True(t, ok)
	// +91 stripped, last 10 digits kept.
	assert.Equal(t, "9876543210", contact)
}

func TestParseOCRText_PhoneWith91Prefix(t *testing.T) {
	s := parseOCRText("WhatsApp: 919876543210")
	contact, ok := s["owner_contact"]
	require.True(t, ok)
	assert.Equal(t, "9876543210", contact)
}

func TestParseOCRText_MultiplePhones_FirstOnly(t *testing.T) {
	s := parseOCRText("Seller: 9111111111  Broker: 9222222222")
	contact, ok := s["owner_contact"]
	require.True(t, ok)
	assert.Equal(t, "9111111111", contact)
}

func TestParseOCRText_NoPhoneFound(t *testing.T) {
	s := parseOCRText("Luxury flat 1 crore, prime location")
	_, ok := s["owner_contact"]
	assert.False(t, ok)
}

// ── Realistic full-text inputs ────────────────────────────────────────────────

func TestParseOCRText_RealScreenshot_WhatsApp(t *testing.T) {
	// Simulates a forwarded WhatsApp property message.
	text := `
*FOR SALE*
3BHK Flat in Andheri West
Area: 1050 sqft
Price: ₹1.25 crore
Direct owner, no brokerage
Contact: 9876501234
`
	s := parseOCRText(text)

	// Price should be 1.25 crore = 12500000
	price, ok := s["price"]
	require.True(t, ok, "price should be extracted")
	assert.Equal(t, "12500000", price)

	// Area should be present
	_, areaOk := s["area"]
	assert.True(t, areaOk, "area should be extracted")

	// Contact
	contact, cOk := s["owner_contact"]
	require.True(t, cOk, "contact should be extracted")
	assert.Equal(t, "9876501234", contact)
}

func TestParseOCRText_RealScreenshot_MagicBricks(t *testing.T) {
	// Simulates OCR from a MagicBricks listing screenshot.
	text := `
2 BHK Apartment for Sale
Goregaon East, Mumbai
₹ 85,00,000
Super Built-up Area : 900 sq.ft
Contact Owner: 8765432109
Ready to Move
Spacious 2BHK with modular kitchen
`
	s := parseOCRText(text)

	price, priceOk := s["price"]
	require.True(t, priceOk)
	// 85,00,000 = 8500000
	assert.Equal(t, "8500000", price)

	contact, cOk := s["owner_contact"]
	require.True(t, cOk)
	assert.Equal(t, "8765432109", contact)
}

func TestParseOCRText_EmptyString(t *testing.T) {
	// Must not panic and must return an empty map.
	assert.NotPanics(t, func() {
		s := parseOCRText("")
		assert.Empty(t, s)
	})
}

func TestParseOCRText_WhitespaceOnly(t *testing.T) {
	s := parseOCRText("   \n\t  ")
	assert.Empty(t, s)
}

// ── extractLargestPrice unit tests ────────────────────────────────────────────

func TestExtractLargestPrice_LakhMultiplier(t *testing.T) {
	v := extractLargestPrice("50 lakh")
	assert.Equal(t, int64(5_000_000), v)
}

func TestExtractLargestPrice_CroreMultiplier(t *testing.T) {
	v := extractLargestPrice("2.5 crore")
	assert.Equal(t, int64(25_000_000), v)
}

func TestExtractLargestPrice_PlainNumber(t *testing.T) {
	v := extractLargestPrice("5000000")
	assert.Equal(t, int64(5_000_000), v)
}

func TestExtractLargestPrice_ReturnsLargest(t *testing.T) {
	v := extractLargestPrice("20 lakh or 15 lakh, negotiable")
	assert.Equal(t, int64(2_000_000), v)
}

func TestExtractLargestPrice_NoMatch(t *testing.T) {
	v := extractLargestPrice("beautiful view property")
	assert.Equal(t, int64(0), v)
}
