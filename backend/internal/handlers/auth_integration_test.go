// internal/handlers/auth_integration_test.go
//
//go:build integration

package handlers_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/realestate/backend/internal/testutil"
)

func TestRegister_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/register", map[string]interface{}{
		"email":     "newuser@example.com",
		"password":  "password123",
		"full_name": "New User",
	}, "")

	assert.Equal(t, http.StatusCreated, status)
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok, "response must have data object")
	assert.NotEmpty(t, data["access_token"])
	assert.Equal(t, "BROKER", data["user"].(map[string]interface{})["role"])
}

func TestRegister_DuplicateEmail(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	payload := map[string]interface{}{
		"email":     "duplicate@example.com",
		"password":  "password123",
		"full_name": "User One",
	}
	status1, _ := testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/register", payload, "")
	require.Equal(t, http.StatusCreated, status1)

	status2, body2 := testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/register", payload, "")
	assert.Equal(t, http.StatusConflict, status2)
	assert.Equal(t, "EMAIL_TAKEN", body2["error_code"])
}

func TestRegister_MissingFields(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	// Missing full_name.
	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/register", map[string]interface{}{
		"email":    "noname@example.com",
		"password": "password123",
	}, "")
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "VALIDATION_FAILED", body["error_code"])
}

func TestLogin_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	// Register first.
	testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/register", map[string]interface{}{
		"email":     "logintest@example.com",
		"password":  "password123",
		"full_name": "Login Test",
	}, "")

	status, body, cookies := testutil.MustRequestWithCookie(t, srv, http.MethodPost, "/api/auth/login",
		map[string]interface{}{
			"email":    "logintest@example.com",
			"password": "password123",
		}, "", nil)

	assert.Equal(t, http.StatusOK, status)
	data := body["data"].(map[string]interface{})
	assert.NotEmpty(t, data["access_token"])

	// Refresh cookie must be set.
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	assert.NotNil(t, refreshCookie, "refresh_token cookie must be set after login")
}

func TestLogin_WrongPassword(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/register", map[string]interface{}{
		"email": "wrongpass@example.com", "password": "correctpass123", "full_name": "WP",
	}, "")

	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/login", map[string]interface{}{
		"email": "wrongpass@example.com", "password": "wrongpassword",
	}, "")

	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "INVALID_CREDENTIALS", body["error_code"])
}

func TestLogin_UnknownEmail(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	status, body := testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/login", map[string]interface{}{
		"email": "nobody@nowhere.com", "password": "whatever",
	}, "")

	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "INVALID_CREDENTIALS", body["error_code"])
}

func TestRefresh_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/register", map[string]interface{}{
		"email": "refresh@example.com", "password": "password123", "full_name": "Refresh",
	}, "")

	_, _, cookies := testutil.MustRequestWithCookie(t, srv, http.MethodPost, "/api/auth/login",
		map[string]interface{}{"email": "refresh@example.com", "password": "password123"},
		"", nil)

	// Use refresh cookie to get a new access token.
	status, body, newCookies := testutil.MustRequestWithCookie(t, srv, http.MethodPost, "/api/auth/refresh",
		nil, "", cookies)

	assert.Equal(t, http.StatusOK, status)
	data := body["data"].(map[string]interface{})
	assert.NotEmpty(t, data["access_token"])

	// Old refresh token must be rotated (new cookie issued).
	var newRefreshCookie *http.Cookie
	for _, c := range newCookies {
		if c.Name == "refresh_token" {
			newRefreshCookie = c
		}
	}
	assert.NotNil(t, newRefreshCookie)
}

func TestRefresh_RevokedRefreshToken(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/register", map[string]interface{}{
		"email": "revoke@example.com", "password": "password123", "full_name": "Revoke",
	}, "")

	_, _, cookies := testutil.MustRequestWithCookie(t, srv, http.MethodPost, "/api/auth/login",
		map[string]interface{}{"email": "revoke@example.com", "password": "password123"},
		"", nil)

	// Use the token once — should succeed.
	status1, _, _ := testutil.MustRequestWithCookie(t, srv, http.MethodPost, "/api/auth/refresh",
		nil, "", cookies)
	require.Equal(t, http.StatusOK, status1)

	// Reuse the SAME original cookie — should fail (token rotated).
	status2, body2, _ := testutil.MustRequestWithCookie(t, srv, http.MethodPost, "/api/auth/refresh",
		nil, "", cookies)
	assert.Equal(t, http.StatusUnauthorized, status2)
	assert.Equal(t, "UNAUTHORIZED", body2["error_code"])
}

func TestLogout_BlacklistsJWT(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	srv := testutil.SetupTestServer(t, pool, rdb)

	// Register and login to get a JWT.
	testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/register", map[string]interface{}{
		"email": "logout@example.com", "password": "password123", "full_name": "Logout",
	}, "")

	loginStatus, loginBody := testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/login", map[string]interface{}{
		"email": "logout@example.com", "password": "password123",
	}, "")
	require.Equal(t, http.StatusOK, loginStatus)

	token := loginBody["data"].(map[string]interface{})["access_token"].(string)

	// Logout.
	logoutStatus, _ := testutil.MustRequest(t, srv, http.MethodPost, "/api/auth/logout", nil, token)
	assert.Equal(t, http.StatusNoContent, logoutStatus)

	// After logout, the same token must return 401 on a protected route.
	blacklistedStatus, _ := testutil.MustRequest(t, srv, http.MethodGet, "/api/properties/", nil, token)
	assert.Equal(t, http.StatusUnauthorized, blacklistedStatus)
}
