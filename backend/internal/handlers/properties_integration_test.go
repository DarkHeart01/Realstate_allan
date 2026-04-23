// internal/handlers/properties_integration_test.go
//
//go:build integration

package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/realestate/backend/internal/models"
	"github.com/realestate/backend/internal/testutil"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func createProperty(t *testing.T, srv interface{ URL string }, jwt string, overrides map[string]interface{}) map[string]interface{} {
	t.Helper()
	payload := map[string]interface{}{
		"listing_category": "SELLING",
		"property_type":    "FLAT",
		"owner_name":       "Test Owner",
		"owner_contact":    "9876543210",
		"price":            5_000_000.0,
		"location_lat":     19.076,
		"location_lng":     72.877,
		"is_direct_owner":  true,
	}
	for k, v := range overrides {
		payload[k] = v
	}
	s := srv.(*struct{ URL string })
	status, body := testutil.MustRequest(t, nil, http.MethodPost, "/api/properties/", payload, jwt)
	_ = status
	return body
}

// ── CREATE ────────────────────────────────────────────────────────────────────

func TestCreateProperty_Success_Admin(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)

	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING",
		"property_type":    "FLAT",
		"owner_name":       "Admin Owner",
		"owner_contact":    "9876543210",
		"price":            5_000_000.0,
		"location_lat":     19.076,
		"location_lng":     72.877,
		"is_direct_owner":  true,
	}, adminJWT)

	assert.Equal(t, http.StatusCreated, status)
	data := body["data"].(map[string]interface{})
	assert.NotEmpty(t, data["id"])
}

func TestCreateProperty_Success_Broker(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, brokerJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	status, _ := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING",
		"property_type":    "PLOT",
		"owner_name":       "Broker Client",
		"owner_contact":    "9876543210",
		"price":            3_000_000.0,
		"location_lat":     19.076,
		"location_lng":     72.877,
		"is_direct_owner":  false,
	}, brokerJWT)
	assert.Equal(t, http.StatusCreated, status)
}

func TestCreateProperty_Unauthenticated(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	status, _ := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "x", "owner_contact": "9876543210",
		"price": 1.0, "location_lat": 19.0, "location_lng": 72.0,
	}, "")
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestCreateProperty_MissingRequiredFields(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	// Missing price.
	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING",
		"property_type":    "FLAT",
	}, jwt)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.NotEmpty(t, body["error_code"])
}

func TestCreateProperty_PostGISGeomPopulated(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Owner", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877,
		"is_direct_owner": true,
	}, jwt)
	require.Equal(t, http.StatusCreated, status)

	propID := body["data"].(map[string]interface{})["id"].(string)

	// Query DB directly to verify geom is not null.
	var geomNotNull bool
	err := pool.QueryRow(context.Background(),
		`SELECT geom IS NOT NULL FROM properties WHERE id = $1`, propID).Scan(&geomNotNull)
	require.NoError(t, err)
	assert.True(t, geomNotNull, "geom column must be populated after property creation")
}

// ── READ LIST ─────────────────────────────────────────────────────────────────

func TestGetProperties_Pagination(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	// Create 4 properties.
	for i := 0; i < 4; i++ {
		testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
			"listing_category": "SELLING", "property_type": "FLAT",
			"owner_name": fmt.Sprintf("Owner%d", i), "owner_contact": "9876543210",
			"price": float64(1_000_000 * (i + 1)), "location_lat": 19.076, "location_lng": 72.877,
			"is_direct_owner": true,
		}, jwt)
	}

	// First page.
	status1, body1 := testutil.MustRequest(t, srv, http.MethodGet, "/api/properties/?limit=2&offset=0", nil, jwt)
	assert.Equal(t, http.StatusOK, status1)
	items1 := body1["data"].([]interface{})
	assert.Len(t, items1, 2)

	// Second page.
	status2, body2 := testutil.MustRequest(t, srv, http.MethodGet, "/api/properties/?limit=2&offset=2", nil, jwt)
	assert.Equal(t, http.StatusOK, status2)
	items2 := body2["data"].([]interface{})
	assert.Len(t, items2, 2)

	// IDs must differ between pages.
	id1 := items1[0].(map[string]interface{})["id"]
	id3 := items2[0].(map[string]interface{})["id"]
	assert.NotEqual(t, id1, id3)
}

func TestGetProperties_FilterByCategory(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	// Create one SELLING and one BUYING.
	testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Seller", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, jwt)
	testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "BUYING", "property_type": "FLAT",
		"owner_name": "Buyer", "owner_contact": "9876543210",
		"price": 4_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, jwt)

	status, body := testutil.MustRequest(t, srv, http.MethodGet, "/api/properties/?category=BUYING", nil, jwt)
	assert.Equal(t, http.StatusOK, status)
	items := body["data"].([]interface{})
	for _, item := range items {
		prop := item.(map[string]interface{})
		assert.Equal(t, "BUYING", prop["listing_category"])
	}
}

func TestGetProperties_ScrubBrokerRole(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)
	_, brokerJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Secret Owner", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, adminJWT)

	_, brokerBody := testutil.MustRequest(t, srv, http.MethodGet, "/api/properties/", nil, brokerJWT)
	items := brokerBody["data"].([]interface{})
	require.NotEmpty(t, items)
	prop := items[0].(map[string]interface{})
	assert.Equal(t, "", prop["owner_name"], "owner_name must be scrubbed for BROKER")
	assert.Equal(t, "", prop["owner_contact"], "owner_contact must be scrubbed for BROKER")
}

func TestGetProperties_FullFieldsAdminRole(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)

	testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Real Owner", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, adminJWT)

	_, adminBody := testutil.MustRequest(t, srv, http.MethodGet, "/api/properties/", nil, adminJWT)
	items := adminBody["data"].([]interface{})
	require.NotEmpty(t, items)
	prop := items[0].(map[string]interface{})
	assert.Equal(t, "Real Owner", prop["owner_name"])
	assert.Equal(t, "9876543210", prop["owner_contact"])
}

func TestGetProperties_ExcludesSoftDeleted(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)

	_, createBody := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "To Delete", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, adminJWT)
	propID := createBody["data"].(map[string]interface{})["id"].(string)

	// Delete it.
	delStatus, _ := testutil.MustRequest(t, srv, http.MethodDelete, "/api/properties/"+propID, nil, adminJWT)
	require.Equal(t, http.StatusOK, delStatus)

	// Should not appear in list.
	_, listBody := testutil.MustRequest(t, srv, http.MethodGet, "/api/properties/", nil, adminJWT)
	items := listBody["data"].([]interface{})
	for _, item := range items {
		assert.NotEqual(t, propID, item.(map[string]interface{})["id"])
	}
}

// ── READ DETAIL ───────────────────────────────────────────────────────────────

func TestGetPropertyByID_Success_Admin(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)

	_, createBody := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Detailed Owner", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, adminJWT)
	propID := createBody["data"].(map[string]interface{})["id"].(string)

	status, body := testutil.MustRequest(t, srv, http.MethodGet, "/api/properties/"+propID, nil, adminJWT)
	assert.Equal(t, http.StatusOK, status)
	data := body["data"].(map[string]interface{})
	assert.Equal(t, propID, data["id"])
	assert.Equal(t, "Detailed Owner", data["owner_name"])
}

func TestGetPropertyByID_Success_Broker_Scrubbed(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)
	_, brokerJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	_, createBody := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Hidden Owner", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, adminJWT)
	propID := createBody["data"].(map[string]interface{})["id"].(string)

	status, body := testutil.MustRequest(t, srv, http.MethodGet, "/api/properties/"+propID, nil, brokerJWT)
	assert.Equal(t, http.StatusOK, status)
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "", data["owner_name"])
	assert.Equal(t, "", data["owner_contact"])
}

func TestGetPropertyByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	status, body := testutil.MustRequest(t, srv, http.MethodGet,
		"/api/properties/00000000-0000-0000-0000-000000000001", nil, jwt)
	assert.Equal(t, http.StatusNotFound, status)
	assert.NotEmpty(t, body["error_code"])
}

// ── PATCH ─────────────────────────────────────────────────────────────────────

func TestPatchProperty_Admin_AnyListing(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)
	_, brokerJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	_, createBody := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Broker Owner", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, brokerJWT)
	propID := createBody["data"].(map[string]interface{})["id"].(string)

	// Admin patches broker's listing.
	status, body := testutil.MustRequest(t, srv, http.MethodPatch, "/api/properties/"+propID,
		map[string]interface{}{"price": 6_000_000.0}, adminJWT)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, float64(6_000_000), body["data"].(map[string]interface{})["price"])
}

func TestPatchProperty_Broker_OthersListing(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)
	_, otherBrokerJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	_, createBody := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Owner", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, adminJWT)
	propID := createBody["data"].(map[string]interface{})["id"].(string)

	// A different broker tries to patch admin's listing.
	status, _ := testutil.MustRequest(t, srv, http.MethodPatch, "/api/properties/"+propID,
		map[string]interface{}{"price": 1.0}, otherBrokerJWT)
	assert.Equal(t, http.StatusForbidden, status)
}

// ── DELETE ────────────────────────────────────────────────────────────────────

func TestDeleteProperty_Admin(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)

	_, createBody := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Doomed", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, adminJWT)
	propID := createBody["data"].(map[string]interface{})["id"].(string)

	status, _ := testutil.MustRequest(t, srv, http.MethodDelete, "/api/properties/"+propID, nil, adminJWT)
	assert.Equal(t, http.StatusOK, status)

	// Verify deleted_at is set in DB.
	var deletedAt *string
	err := pool.QueryRow(context.Background(),
		`SELECT deleted_at::text FROM properties WHERE id = $1`, propID).Scan(&deletedAt)
	require.NoError(t, err)
	assert.NotNil(t, deletedAt, "deleted_at must be set after delete")
}

func TestDeleteProperty_Broker(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, adminJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleSuperAdmin)
	_, brokerJWT := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	_, createBody := testutil.MustRequest(t, srv, http.MethodPost, "/api/properties/", map[string]interface{}{
		"listing_category": "SELLING", "property_type": "FLAT",
		"owner_name": "Owner", "owner_contact": "9876543210",
		"price": 5_000_000.0, "location_lat": 19.076, "location_lng": 72.877, "is_direct_owner": true,
	}, adminJWT)
	propID := createBody["data"].(map[string]interface{})["id"].(string)

	status, _ := testutil.MustRequest(t, srv, http.MethodDelete, "/api/properties/"+propID, nil, brokerJWT)
	assert.Equal(t, http.StatusForbidden, status)
}

// ── SQL Injection ─────────────────────────────────────────────────────────────

func TestSQLInjection_SearchFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	// Pass a SQL injection payload as the category filter.
	// The validation layer should reject it (not a valid enum value).
	status, body := testutil.MustRequest(t, srv, http.MethodGet,
		"/api/properties/?category=%27%3B+DROP+TABLE+properties%3B+--", nil, jwt)

	// Must return 400 (invalid enum) or 200 empty (treated as literal tag).
	assert.True(t, status == http.StatusBadRequest || status == http.StatusOK,
		"unexpected status %d: %v", status, body)

	// Verify the properties table still exists.
	var count int
	err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM properties`).Scan(&count)
	assert.NoError(t, err, "properties table must still exist after injection attempt")
}

func TestSQLInjection_TagsFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)
	cfg := testutil.TestCfg()
	_, jwt := testutil.CreateTestUser(t, pool, cfg, rdb, models.RoleBroker)

	// Tags are treated as literal strings via pgx parameterisation.
	status, _ := testutil.MustRequest(t, srv, http.MethodGet,
		"/api/properties/?tags=%27%3B+DELETE+FROM+properties%3B+--", nil, jwt)

	assert.True(t, status == http.StatusOK || status == http.StatusBadRequest)

	// DB must be intact.
	var count int
	err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM properties`).Scan(&count)
	assert.NoError(t, err, "properties table must still exist after injection attempt")
}
