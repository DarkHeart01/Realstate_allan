// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	// Database
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string

	// Redis
	RedisURL string

	// JWT
	JWTSecret            string
	JWTAccessTTLMinutes  int
	JWTRefreshTTLDays    int

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// App
	AppEnv string
	Port   string

	// GCS (Phase 2)
	GCSBucket          string
	GCSProject         string
	GCSCredentialsFile string
	GCSCDNBaseURL      string

	// Google Maps — server-side Geocoding API key (Phase 3)
	// Separate from the client-side Maps SDK key used in Flutter.
	// Optional: if empty, share endpoint returns "Location available on request".
	GoogleMapsAPIKey string

	// Twilio — WhatsApp & SMS dispatch (Phase 4)
	// All three are required for notification dispatch; if any is empty, the
	// worker handlers log a warning and skip the Twilio call.
	TwilioAccountSID   string
	TwilioAuthToken    string
	TwilioWhatsAppFrom string // e.g. "whatsapp:+14155238886"
	TwilioSMSFrom      string // e.g. "+14155238886"

	// Google Vision — OCR autofill (Phase 4)
	// Optional: if empty, OCR endpoint returns an empty suggestion set.
	// Uses Application Default Credentials (same as GCS) — no separate key file needed.
	// Requires the Cloud Vision API to be enabled in the GCP project.
	GoogleVisionEnabled bool

	// Rate limiting (Phase 5) — requests per window per IP.
	// Defaults are safe minimums; override via env vars.
	RateLimitAuthLogin    int // POST /api/auth/login      — default 10 per 15 min
	RateLimitAuthRegister int // POST /api/auth/register   — default 5 per 1 hour
	RateLimitAuthRefresh  int // POST /api/auth/refresh    — default 30 per 15 min
	RateLimitOCRScan      int // POST …/ocr                — default 20 per 1 hour
	RateLimitGlobal       int // all other auth'd routes   — default 300 per 1 min
}

// Load reads all environment variables, validates required fields, and returns
// a populated Config. It panics with a descriptive message if any required field
// is missing or unparseable.
func Load() *Config {
	cfg := &Config{
		DBHost:             requireEnv("DB_HOST"),
		DBPort:             getEnvOrDefault("DB_PORT", "5432"),
		DBName:             requireEnv("DB_NAME"),
		DBUser:             requireEnv("DB_USER"),
		DBPassword:         requireEnv("DB_PASSWORD"),
		RedisURL:           requireEnv("REDIS_URL"),
		JWTSecret:          requireEnv("JWT_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		AppEnv:             getEnvOrDefault("APP_ENV", "development"),
		Port:               getEnvOrDefault("PORT", "8080"),

		// Optional — Phase 2 & 3 (not required on startup)
		GCSBucket:          os.Getenv("GCS_BUCKET"),
		GCSProject:         os.Getenv("GCS_PROJECT"),
		GCSCredentialsFile: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		GCSCDNBaseURL:      firstNonEmpty(os.Getenv("GCS_CDN_BASE_URL"), os.Getenv("CDN_BASE_URL")),
		GoogleMapsAPIKey:   os.Getenv("GOOGLE_MAPS_API_KEY"),

		// Optional — Phase 4
		TwilioAccountSID:    os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:     os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioWhatsAppFrom:  os.Getenv("TWILIO_WHATSAPP_FROM"),
		TwilioSMSFrom:       os.Getenv("TWILIO_SMS_FROM"),
		GoogleVisionEnabled: os.Getenv("GOOGLE_VISION_ENABLED") == "true",
	}

	cfg.JWTAccessTTLMinutes = envIntOrDefault("JWT_ACCESS_TTL_MINUTES", 60)
	cfg.JWTRefreshTTLDays = envIntOrDefault("JWT_REFRESH_TTL_DAYS", 30)

	// Rate limits — optional, fall back to safe defaults.
	cfg.RateLimitAuthLogin = envIntOrDefault("RATE_LIMIT_AUTH_LOGIN", 10)
	cfg.RateLimitAuthRegister = envIntOrDefault("RATE_LIMIT_AUTH_REGISTER", 5)
	cfg.RateLimitAuthRefresh = envIntOrDefault("RATE_LIMIT_AUTH_REFRESH", 30)
	cfg.RateLimitOCRScan = envIntOrDefault("RATE_LIMIT_OCR_SCAN", 20)
	cfg.RateLimitGlobal = envIntOrDefault("RATE_LIMIT_GLOBAL", 300)

	return cfg
}

// DBConnString returns a PostgreSQL DSN string compatible with pgx.
func (c *Config) DBConnString() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBName, c.DBUser, c.DBPassword,
	)
}

// IsProduction returns true when APP_ENV is "production".
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// ── helpers ──────────────────────────────────────────────────────────────────

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("config: required environment variable %q is not set", key))
	}
	return v
}

func requireEnvInt(key string) int {
	raw := requireEnv(key)
	v, err := strconv.Atoi(raw)
	if err != nil {
		panic(fmt.Sprintf("config: environment variable %q must be an integer, got %q", key, raw))
	}
	return v
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
