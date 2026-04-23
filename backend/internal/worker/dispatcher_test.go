// internal/worker/dispatcher_test.go
// Unit tests for pure helper functions in the worker package.
// HandleWhatsApp / HandleSMS integration tests (which require DB + Twilio mock)
// are in dispatcher_integration_test.go (build tag: integration).
package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── normalisePhone ────────────────────────────────────────────────────────────

func TestNormalisePhone_10Digit(t *testing.T) {
	assert.Equal(t, "+919876543210", normalisePhone("9876543210"))
}

func TestNormalisePhone_ZeroPrefix(t *testing.T) {
	assert.Equal(t, "+919876543210", normalisePhone("09876543210"))
}

func TestNormalisePhone_Plus91Prefix(t *testing.T) {
	assert.Equal(t, "+919876543210", normalisePhone("+919876543210"))
}

func TestNormalisePhone_91Prefix12Digit(t *testing.T) {
	assert.Equal(t, "+919876543210", normalisePhone("919876543210"))
}

func TestNormalisePhone_WithDashes(t *testing.T) {
	// Hyphens stripped — 10 digits remain.
	assert.Equal(t, "+919876543210", normalisePhone("98765-43210"))
}

func TestNormalisePhone_WithSpaces(t *testing.T) {
	assert.Equal(t, "+919876543210", normalisePhone("98765 43210"))
}

// ── normaliseWhatsAppNumber ───────────────────────────────────────────────────

func TestNormaliseWhatsAppNumber_10Digit(t *testing.T) {
	// WhatsApp normalisation uses the same rules as normalisePhone.
	assert.Equal(t, "+919876543210", normaliseWhatsAppNumber("9876543210"))
}

// ── maskContact ──────────────────────────────────────────────────────────────

func TestMaskContact_Standard(t *testing.T) {
	assert.Equal(t, "******3210", maskContact("9876543210"))
}

func TestMaskContact_ShortString(t *testing.T) {
	// Strings of ≤ 4 chars are fully masked.
	assert.Equal(t, "****", maskContact("1234"))
	assert.Equal(t, "****", maskContact("hi"))
	assert.Equal(t, "****", maskContact(""))
}

func TestMaskContact_FiveChars(t *testing.T) {
	// 5 chars: 1 masked + 4 visible.
	assert.Equal(t, "*1234", maskContact("51234"))
}

func TestMaskContact_PhoneWithPrefix(t *testing.T) {
	s := maskContact("+919876543210")
	assert.Equal(t, "*********3210", s)
}

// ── formatStaleMessage ────────────────────────────────────────────────────────

func TestFormatStaleMessage_ContainsDays(t *testing.T) {
	msg := formatStaleMessage(45)
	assert.Contains(t, msg, "45")
}

func TestFormatStaleMessage_ContainsDaysWord(t *testing.T) {
	msg := formatStaleMessage(10)
	assert.Contains(t, msg, "days")
}

func TestFormatStaleMessage_ContainsThresholdReminder(t *testing.T) {
	msg := formatStaleMessage(30)
	// Should mention the 30-day inactivity threshold.
	assert.Contains(t, msg, "30 days")
}

func TestFormatStaleMessage_NeverEmpty(t *testing.T) {
	for _, days := range []int{1, 30, 90, 365} {
		assert.NotEmpty(t, formatStaleMessage(days))
	}
}

func TestFormatStaleMessage_ContainsUpdatePrompt(t *testing.T) {
	msg := formatStaleMessage(35)
	// Must encourage the owner to update their listing.
	assert.Contains(t, msg, "update")
}
