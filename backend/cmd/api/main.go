// cmd/api/main.go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"github.com/realestate/backend/internal/config"
	"github.com/realestate/backend/internal/db"
	"github.com/realestate/backend/internal/gcs"
	"github.com/realestate/backend/internal/handlers"
	mw "github.com/realestate/backend/internal/middleware"
	"github.com/realestate/backend/internal/models"
	rdb "github.com/realestate/backend/internal/redis"
	"github.com/realestate/backend/internal/services"
	"github.com/realestate/backend/internal/worker"
)

func main() {
	ctx := context.Background()

	// ── Config ────────────────────────────────────────────────────────────────
	cfg := config.Load()

	// ── Database ──────────────────────────────────────────────────────────────
	pool, err := db.New(ctx, cfg)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}
	defer pool.Close()
	log.Println("database connected")

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisClient, err := rdb.New(ctx, cfg)
	if err != nil {
		log.Fatalf("redis init failed: %v", err)
	}
	defer redisClient.Close()
	log.Println("redis connected")

	// ── GCS ───────────────────────────────────────────────────────────────────
	gcsClient, err := gcs.New(ctx, cfg)
	if err != nil {
		log.Fatalf("gcs init failed: %v", err)
	}
	if gcsClient.IsConfigured() {
		log.Println("GCS configured")
	} else {
		log.Println("GCS not configured — media endpoints will return GCS_NOT_CONFIGURED")
	}

	// ── Worker client (enqueues background tasks) ─────────────────────────────
	workerClient, err := worker.NewClient(cfg)
	if err != nil {
		log.Fatalf("worker client init failed: %v", err)
	}
	defer workerClient.Close()
	log.Println("worker client connected")

	// ── Services ──────────────────────────────────────────────────────────────
	tokenSvc := services.NewTokenService(cfg, redisClient)
	authSvc := services.NewAuthService(pool, tokenSvc)
	propertySvc := services.NewPropertyService(pool)
	photoSvc := services.NewPhotoService(pool, gcsClient)
	userSvc := services.NewUserService(pool)
	calcSvc := services.NewCalculator()
	shareSvc := services.NewShareService(pool, redisClient, cfg.GoogleMapsAPIKey)
	notifSvc := services.NewNotificationService(pool)
	staleSvc := services.NewStaleService(pool)
	ocrSvc := services.NewOCRService(pool, cfg.GoogleVisionEnabled, cfg.GCSBucket)

	// ── Handlers ─────────────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(authSvc, tokenSvc, cfg)
	propHandler := handlers.NewPropertiesHandler(propertySvc)
	photoHandler := handlers.NewPhotosHandler(photoSvc)
	tagsHandler := handlers.NewTagsHandler(pool)
	usersHandler := handlers.NewUsersHandler(userSvc)
	toolsHandler := handlers.NewToolsHandler(calcSvc, pool)
	shareHandler := handlers.NewShareHandler(shareSvc)
	notifHandler := handlers.NewNotificationsHandler(notifSvc, staleSvc, workerClient)
	ocrHandler := handlers.NewOCRHandler(ocrSvc, workerClient)

	// ── Middleware ────────────────────────────────────────────────────────────
	authenticator := mw.NewAuthenticator(tokenSvc)

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		// AllowCredentials must be false when AllowedOrigins is "*" — browsers
		// reject the combination. Auth is handled via Bearer tokens, not cookies.
	})
	r.Use(corsMiddleware.Handler)

	// ── Routes ────────────────────────────────────────────────────────────────
	r.Route("/api", func(r chi.Router) {

		// Auth
		r.Route("/auth", func(r chi.Router) {
			r.With(mw.RateLimit(redisClient, "auth_register", cfg.RateLimitAuthRegister, 60*time.Minute)).
				Post("/register", authHandler.Register)
			r.With(mw.RateLimit(redisClient, "auth_login", cfg.RateLimitAuthLogin, 15*time.Minute)).
				Post("/login", authHandler.Login)
			r.With(mw.RateLimit(redisClient, "auth_refresh", cfg.RateLimitAuthRefresh, 15*time.Minute)).
				Post("/refresh", authHandler.Refresh)
			r.Get("/google", authHandler.GoogleLogin)
			r.Get("/google/callback", authHandler.GoogleCallback)
			r.Group(func(r chi.Router) {
				r.Use(authenticator.Authenticate)
				r.Post("/logout", authHandler.Logout)
			})
		})

		// Properties — all protected
		r.Route("/properties", func(r chi.Router) {
			r.Use(authenticator.Authenticate)

			r.Get("/nearby", propHandler.Nearby) // static route before /{id}
			r.Get("/", propHandler.List)
			r.Post("/", propHandler.Create)
			r.Get("/{id}", propHandler.Get)
			r.Patch("/{id}", propHandler.Update)
			r.Delete("/{id}", propHandler.Delete)

			// Photos
			r.Post("/{id}/photos/presign", photoHandler.Presign)
			r.Post("/{id}/photos/{photo_id}/confirm", photoHandler.Confirm)
			r.Delete("/{id}/photos/{photo_id}", photoHandler.Delete)

			// OCR (Phase 4)
			r.Post("/{id}/photos/{photo_id}/ocr", ocrHandler.Scan)
			r.Get("/{id}/photos/{photo_id}/ocr", ocrHandler.GetResult)

			// Share (Phase 3)
			r.Post("/{id}/share", shareHandler.Share)
		})

		// Tags — public (no auth required)
		r.Get("/tags", tagsHandler.Autocomplete)

		// Users — SUPER_ADMIN only
		r.With(authenticator.Authenticate).
			With(mw.Require(models.RoleSuperAdmin)).
			Get("/users", usersHandler.List)

		// Tools (Phase 3)
		r.Route("/tools", func(r chi.Router) {
			r.Use(authenticator.Authenticate)
			r.Post("/calculator", toolsHandler.Calculator)
			r.With(mw.Require(models.RoleSuperAdmin)).Get("/export/csv", toolsHandler.ExportCSV)
		})

		// Notifications (Phase 4) — authenticated
		r.Route("/notifications", func(r chi.Router) {
			r.Use(authenticator.Authenticate)
			r.Get("/", notifHandler.List)
			r.Get("/{job_id}", notifHandler.Get)
		})

		// Admin endpoints — SUPER_ADMIN only
		r.Route("/admin", func(r chi.Router) {
			r.Use(authenticator.Authenticate)
			r.Use(mw.Require(models.RoleSuperAdmin))
			r.Post("/notifications/scan-stale", notifHandler.TriggerStaleScan)
		})
	})

	// Health check — checks DB + Redis connectivity and reports BUILD_SHA.
	buildSHA := os.Getenv("BUILD_SHA")
	if buildSHA == "" {
		buildSHA = "unknown"
	}
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if err := pool.Ping(r.Context()); err != nil {
			dbStatus = "error: " + err.Error()
		}

		redisStatus := "ok"
		if err := redisClient.Ping(r.Context()).Err(); err != nil {
			redisStatus = "error: " + err.Error()
		}

		overall := "ok"
		statusCode := http.StatusOK
		if dbStatus != "ok" || redisStatus != "ok" {
			overall = "degraded"
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  overall,
			"db":      dbStatus,
			"redis":   redisStatus,
			"version": buildSHA,
		})
	})

	// ── Start server ──────────────────────────────────────────────────────────
	addr := ":" + cfg.Port
	log.Printf("API server listening on %s (env=%s)", addr, cfg.AppEnv)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
