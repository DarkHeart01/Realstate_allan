// internal/testutil/testutil.go
// Shared test helpers for integration tests.
// Spins up real Postgres (PostGIS) + Redis containers via testcontainers-go,
// runs all migrations, and wires the full chi router against them.
//
// Build tag: integration
// Run with: go test ./internal/... -tags=integration -timeout=120s
//
//go:build integration

package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"golang.org/x/crypto/bcrypt"

	"github.com/realestate/backend/internal/config"
	"github.com/realestate/backend/internal/handlers"
	mw "github.com/realestate/backend/internal/middleware"
	"github.com/realestate/backend/internal/models"
	"github.com/realestate/backend/internal/services"
)

// SetupTestDB spins up a postgis/postgis:16-3.4-alpine container, runs all
// migrations, and returns a *pgxpool.Pool.  The container is terminated on
// t.Cleanup().
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgis/postgis:16-3.4-alpine"),
		postgres.WithDatabase("realestate_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			testcontainers.NewLogWaitStrategy("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Run migrations.
	migrationsPath := migrationsDir(t)
	m, err := migrate.New("file://"+migrationsPath, connStr)
	require.NoError(t, err, "failed to init migrations")
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		require.NoError(t, err, "failed to run migrations")
	}

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

// SetupTestRedis spins up a redis:7-alpine container and returns a *redis.Client.
func SetupTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()

	redisContainer, err := tcredis.RunContainer(ctx,
		testcontainers.WithImage("redis:7-alpine"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = redisContainer.Terminate(ctx) })

	addr, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })

	return rdb
}

// SetupTestServer wires together the full chi router with all middleware and
// handlers pointing at the test DB and Redis, then wraps it in an httptest.Server.
func SetupTestServer(t *testing.T, pool *pgxpool.Pool, rdb *redis.Client) *httptest.Server {
	t.Helper()

	cfg := &config.Config{
		JWTSecret:           "test-secret-key-at-least-32-bytes!!",
		JWTAccessTTLMinutes: 15,
		JWTRefreshTTLDays:   30,
		AppEnv:              "test",
		Port:                "8080",
		// Rate limits high enough that tests never hit them.
		RateLimitAuthLogin:    10000,
		RateLimitAuthRegister: 10000,
		RateLimitAuthRefresh:  10000,
		RateLimitOCRScan:      10000,
		RateLimitGlobal:       10000,
	}

	tokenSvc := services.NewTokenService(cfg, rdb)
	authSvc := services.NewAuthService(pool, tokenSvc)
	propertySvc := services.NewPropertyService(pool)
	calcSvc := services.NewCalculator()
	notifSvc := services.NewNotificationService(pool)
	staleSvc := services.NewStaleService(pool)

	authHandler := handlers.NewAuthHandler(authSvc, tokenSvc, cfg)
	propHandler := handlers.NewPropertiesHandler(propertySvc)
	toolsHandler := handlers.NewToolsHandler(calcSvc, pool)
	notifHandler := handlers.NewNotificationsHandler(notifSvc, staleSvc, nil)
	authenticator := mw.NewAuthenticator(tokenSvc)

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.AllowAll().Handler)

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Group(func(r chi.Router) {
				r.Use(authenticator.Authenticate)
				r.Post("/logout", authHandler.Logout)
			})
		})

		r.Route("/properties", func(r chi.Router) {
			r.Use(authenticator.Authenticate)
			r.Get("/", propHandler.List)
			r.Post("/", propHandler.Create)
			r.Get("/{id}", propHandler.Get)
			r.Patch("/{id}", propHandler.Update)
			r.With(mw.Require(models.RoleSuperAdmin)).Delete("/{id}", propHandler.Delete)
		})

		r.Route("/tools", func(r chi.Router) {
			r.Use(authenticator.Authenticate)
			r.Post("/calculator", toolsHandler.Calculator)
			r.With(mw.Require(models.RoleSuperAdmin)).Get("/export/csv", toolsHandler.ExportCSV)
		})

		r.Route("/notifications", func(r chi.Router) {
			r.Use(authenticator.Authenticate)
			r.Get("/", notifHandler.List)
			r.Get("/{job_id}", notifHandler.Get)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(authenticator.Authenticate)
			r.Use(mw.Require(models.RoleSuperAdmin))
			r.Post("/notifications/scan-stale", notifHandler.TriggerStaleScan)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// CreateTestUser inserts a user row directly into the DB and returns a valid JWT
// for that user.  role must be "SUPER_ADMIN" or "BROKER".
func CreateTestUser(t *testing.T, pool *pgxpool.Pool, cfg *config.Config, rdb *redis.Client, role string) (userID string, jwt string) {
	t.Helper()
	ctx := context.Background()

	email := fmt.Sprintf("test-%d-%s@example.com", time.Now().UnixNano(), role)
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), 4) // cost 4 for speed in tests
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, full_name, role)
         VALUES ($1, $2, $3, $4)
         RETURNING id`,
		email, string(hash), "Test User", role,
	).Scan(&userID)
	require.NoError(t, err)

	// Issue a JWT for the new user.
	tokenSvc := services.NewTokenService(cfg, rdb)
	user := &models.User{Role: role}
	_, _ = fmt.Sscan(userID, &user.ID)
	user.Email = email

	tokenStr, _, err := tokenSvc.IssueAccessToken(user)
	require.NoError(t, err)
	return userID, tokenStr
}

// MustRequest builds and executes an HTTP request against the test server and
// returns the parsed response body as map[string]interface{}.
func MustRequest(
	t *testing.T,
	srv *httptest.Server,
	method, path string,
	body interface{},
	jwt string,
) (statusCode int, responseBody map[string]interface{}) {
	t.Helper()

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req, err := http.NewRequest(method, srv.URL+path, bytes.NewReader(bodyBytes))
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	_ = json.NewDecoder(resp.Body).Decode(&responseBody)
	return resp.StatusCode, responseBody
}

// MustRequestWithCookie is like MustRequest but also sends and captures cookies.
func MustRequestWithCookie(
	t *testing.T,
	srv *httptest.Server,
	method, path string,
	body interface{},
	jwt string,
	cookies []*http.Cookie,
) (statusCode int, responseBody map[string]interface{}, respCookies []*http.Cookie) {
	t.Helper()

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req, err := http.NewRequest(method, srv.URL+path, bytes.NewReader(bodyBytes))
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}

	// Don't follow redirects; capture cookies manually.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	_ = json.NewDecoder(resp.Body).Decode(&responseBody)
	return resp.StatusCode, responseBody, resp.Cookies()
}

// testCfg returns a *config.Config suitable for test usage.
func TestCfg() *config.Config {
	return &config.Config{
		JWTSecret:           "test-secret-key-at-least-32-bytes!!",
		JWTAccessTTLMinutes: 15,
		JWTRefreshTTLDays:   30,
		AppEnv:              "test",
	}
}

// migrationsDir returns the absolute path to the backend/migrations directory.
func migrationsDir(t *testing.T) string {
	t.Helper()
	// Walk up from this file's location to find the backend/migrations directory.
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "migrations")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate backend/migrations directory")
	return ""
}
