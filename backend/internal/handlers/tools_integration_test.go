// internal/handlers/tools_integration_test.go
//
//go:build integration

package handlers_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/realestate/backend/internal/models"
	"github.com/realestate/backend/internal/testutil"
)

// ── Calculator ────────────────────────────────────────────────────────────────

func TestCalculator_Sale_Integration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/tools/calculator",
		map[string]interface{}{
			"mode":            "SALE",
			"property_value":  5_000_000,
			"commission_rate": 2,
			"split_ratio":     "50:50",
		}, jwt)

	assert.Equal(t, http.StatusOK, status)
	data := body["data"].(map[string]interface{})
	assert.Equal(t, float64(100_000), data["total_commission"])
	assert.Equal(t, float64(50_000), data["split_a"])
	assert.Equal(t, float64(50_000), data["split_b"])
	assert.Equal(t, "₹1,00,000", data["total_commission_formatted"])
}

func TestCalculator_Rental_Integration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/tools/calculator",
		map[string]interface{}{
			"mode":            "RENTAL",
			"monthly_rent":    50_000,
			"commission_rate": 100,
		}, jwt)

	// commission_rate 100 is > 10 — should return 400.
	assert.Equal(t, http.StatusBadRequest, status)
	_ = body
}

func TestCalculator_InvalidSplit_Integration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/tools/calculator",
		map[string]interface{}{
			"mode": "SALE", "property_value": 1_000_000,
			"commission_rate": 2, "split_ratio": "60:41",
		}, jwt)

	assert.Equal(t, http.StatusBadRequest, status)
	assert.NotEmpty(t, body["error_code"])
}

// ── CSV Export ────────────────────────────────────────────────────────────────

func TestCSVExport_AdminOnly(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, brokerJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	status, _ := testutil.MustRequest(t, srv, http.MethodGet, "/api/tools/export/csv", nil, brokerJWT)
	assert.Equal(t, http.StatusForbidden, status)
}

func TestCSVExport_ReturnsCSVContentType(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tools/export/csv", nil)
	req.Header.Set("Authorization", "Bearer "+adminJWT)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/csv"),
		"expected text/csv, got %q", resp.Header.Get("Content-Type"))
}

func TestCSVExport_ExcludesSoftDeleted(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)

	// Create and then delete a property.
	_, createBody := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/",
		map[string]interface{}{
			"listing_category": "SELLING", "property_type": "FLAT",
			"owner_name": "CSV Delete", "owner_contact": "9876543210",
			"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
		}, adminJWT)
	propID := createBody["data"].(map[string]interface{})["id"].(string)
	testutil.MustRequest(t, srv, http.MethodDelete, "/api/properties/"+propID, nil, adminJWT)

	// Fetch CSV — deleted property must not appear.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/tools/export/csv", nil)
	req.Header.Set("Authorization", "Bearer "+adminJWT)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf := new(strings.Builder)
	_, _ = buf.ReadFrom(resp.Body)
	csv := buf.String()
	assert.NotContains(t, csv, propID, "deleted property must not appear in CSV export")
}
