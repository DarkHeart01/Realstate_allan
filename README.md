# Real Estate Platform

Internal property management and brokerage tool for Allan & Aditya.

## Architecture

**Backend:** Go 1.22, chi router, pgx v5 (PostgreSQL + PostGIS), Redis, Asynq (background jobs)
**Frontend:** Flutter 3.22, Riverpod, GoRouter, Dio
**Infrastructure:** Docker Compose, Nginx, GCS, Asynq workers

```
infra/docker-compose.yml   — postgres+PostGIS, redis, nginx, api, worker, asynqmon
backend/                   — Go API + worker binary
app/                       — Flutter app
backend/migrations/        — golang-migrate SQL files (001–013)
```

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Docker + Docker Compose plugin | `docker compose version` ≥ 2.x |
| Go 1.22+ | For local backend development only |
| Flutter 3.22+ | `flutter --version` |
| GCP project | Cloud Vision API + GCS bucket + two Maps API keys |
| Twilio account | Optional — SMS/WhatsApp degrade gracefully if absent |
| Firebase project | Optional — FCM token registration only |

---

## Local Development Setup

```bash
# 1. Clone the repo
git clone <repo-url>
cd realestate-platform

# 2. Create .env from the example
cp .env.example .env
# Fill in at minimum: DB_*, REDIS_URL, JWT_SECRET, JWT_ACCESS_TTL_MINUTES,
# JWT_REFRESH_TTL_DAYS, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REDIRECT_URL

# 3. Start all services
cd infra
docker compose up -d

# 4. Migrations run automatically when the api container starts.
#    To run manually:
docker compose exec api /usr/local/bin/golang-migrate \
  -path /migrations -database "postgres://..." up

# 5. Flutter app
cd app
flutter pub get
flutter run --dart-define=GOOGLE_MAPS_API_KEY=<your-client-side-key>
```

---

## Environment Variables

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DB_HOST` | ✅ | — | Postgres hostname |
| `DB_PORT` | — | `5432` | |
| `DB_NAME` | ✅ | — | |
| `DB_USER` | ✅ | — | |
| `DB_PASSWORD` | ✅ | — | |
| `REDIS_URL` | ✅ | — | e.g. `redis://redis:6379` |
| `JWT_SECRET` | ✅ | — | Min 32 chars |
| `JWT_ACCESS_TTL_MINUTES` | ✅ | — | e.g. `15` |
| `JWT_REFRESH_TTL_DAYS` | ✅ | — | e.g. `30` |
| `GOOGLE_CLIENT_ID` | ✅ | — | Google OAuth |
| `GOOGLE_CLIENT_SECRET` | ✅ | — | |
| `GOOGLE_REDIRECT_URL` | ✅ | — | |
| `APP_ENV` | — | `development` | `production` enables Secure cookies |
| `PORT` | — | `8080` | |
| `GCS_BUCKET` | — | — | Photo upload bucket |
| `GCS_CDN_BASE_URL` | — | — | Public CDN base URL |
| `GOOGLE_APPLICATION_CREDENTIALS` | — | — | Path to service account JSON (local dev) |
| `GOOGLE_MAPS_API_KEY` | — | — | Server-side Geocoding API key |
| `TWILIO_ACCOUNT_SID` | — | — | WhatsApp/SMS notifications |
| `TWILIO_AUTH_TOKEN` | — | — | |
| `TWILIO_WHATSAPP_FROM` | — | — | e.g. `whatsapp:+14155238886` |
| `TWILIO_SMS_FROM` | — | — | e.g. `+14155238886` |
| `GOOGLE_VISION_ENABLED` | — | `false` | Set `true` to enable OCR autofill |
| `RATE_LIMIT_AUTH_LOGIN` | — | `10` | Requests per 15 min per IP |
| `RATE_LIMIT_AUTH_REGISTER` | — | `5` | Requests per 1 hour per IP |
| `RATE_LIMIT_AUTH_REFRESH` | — | `30` | Requests per 15 min per IP |
| `RATE_LIMIT_OCR_SCAN` | — | `20` | Requests per 1 hour per IP |
| `RATE_LIMIT_GLOBAL` | — | `300` | Requests per 1 min per IP |

---

## Running Tests

### Backend — unit tests (no Docker required)

```bash
cd backend
go test ./internal/services/... ./internal/utils/... ./internal/worker/... -v
```

### Backend — integration tests (requires Docker daemon)

Integration tests spin up real Postgres and Redis containers via testcontainers-go.

```bash
cd backend
go test ./internal/... -tags=integration -timeout=120s -v
```

### Coverage gate

```bash
go test ./internal/... -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | grep internal/services
```

CI enforces ≥ 80% average on `internal/services/`.

### Flutter

```bash
cd app
flutter test --coverage
```

### golangci-lint

```bash
cd backend
golangci-lint run --timeout=5m
```

---

## GCS Setup

```bash
# 1. Create a bucket
gsutil mb -l ASIA-SOUTH1 gs://your-bucket-name

# 2. Set CORS policy (allows Flutter PUT uploads)
gsutil cors set - gs://your-bucket-name <<'EOF'
[{"origin":["*"],"method":["GET","PUT","HEAD"],"responseHeader":["Content-Type"],"maxAgeSeconds":3600}]
EOF

# 3. Create a service account
gcloud iam service-accounts create realestate-sa \
  --display-name="Real Estate Platform SA"

# 4. Grant storage permissions
gsutil iam ch serviceAccount:realestate-sa@PROJECT.iam.gserviceaccount.com:objectAdmin \
  gs://your-bucket-name

# 5. Download key (local dev only)
gcloud iam service-accounts keys create key.json \
  --iam-account=realestate-sa@PROJECT.iam.gserviceaccount.com
# Set GOOGLE_APPLICATION_CREDENTIALS=/absolute/path/to/key.json in .env

# 6. Enable required APIs
gcloud services enable vision.googleapis.com storage.googleapis.com
```

---

## Google Maps API Keys

Two separate keys are required:

| Key | Used by | Restrict to |
|---|---|---|
| **Server-side key** (`GOOGLE_MAPS_API_KEY` in `.env`) | Share endpoint reverse geocoding | Geocoding API + IP restriction |
| **Client-side key** (`--dart-define=GOOGLE_MAPS_API_KEY=...`) | Flutter Maps SDK | Maps SDK for Android/iOS + app package restriction |

Never use the server-side key in the Flutter app.

---

## Production Deployment

### 1. Prepare the VPS (once)

```bash
# On a fresh Ubuntu 24.04 VPS as root:
bash infra/scripts/setup-vps.sh
```

### 2. Copy files to the VPS

```bash
scp .env deploy@VPS_HOST:/opt/realestate/.env
scp infra/docker-compose.yml infra/docker-compose.prod.yml deploy@VPS_HOST:/opt/realestate/
scp infra/scripts/backup.sh deploy@VPS_HOST:/opt/realestate/scripts/backup.sh
ssh deploy@VPS_HOST "chmod +x /opt/realestate/scripts/backup.sh"
```

### 3. Set GitHub Secrets

| Secret | Description |
|---|---|
| `GCP_SA_KEY` | Service account JSON key with GCR push permission |
| `GCP_PROJECT_ID` | Your GCP project ID |
| `VPS_HOST` | Production VPS IP or hostname |
| `VPS_USER` | SSH user (`deploy`) |
| `VPS_SSH_KEY` | Private SSH key for VPS access |
| `GOOGLE_MAPS_API_KEY` | Client-side Maps SDK key for Flutter APK build |

### 4. Push to `main`

CI/CD runs automatically:
1. Builds Docker image tagged with `BUILD_SHA`
2. Pushes to GCR
3. SSHes into VPS — **runs DB migrations before container swap**
4. Pulls new image, restarts `api` + `worker`
5. Health-checks `GET /health` for 30 seconds; rolls back on failure

> **Branch protection:** Require both `Backend CI` and `Flutter CI` to pass before merging to `main`.

### 5. Set up daily backup cron

```bash
ssh deploy@VPS_HOST crontab -e
# Add:
# 0 2 * * * /opt/realestate/scripts/backup.sh >> /var/log/realestate-backup.log 2>&1
```

---

## Accessing Internal Tools

**Asynqmon (local):** [http://localhost:8082](http://localhost:8082)

**Asynqmon (production)** — not publicly exposed:
```bash
ssh -L 8082:localhost:8082 deploy@VPS_HOST
# then open http://localhost:8082
```

---

## Production Operations

### View logs
```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml logs -f api
```

### Manual migration
```bash
docker compose exec api /usr/local/bin/golang-migrate \
  -path /migrations -database "$(grep DATABASE_URL .env | cut -d= -f2-)" up
```

### Restore from backup
```bash
gcloud storage cp gs://BUCKET/backups/2026-04-01-0200.sql.gz /tmp/restore.sql.gz
gunzip /tmp/restore.sql.gz
docker compose stop api worker
docker exec -i realestate-postgres psql -U "$DB_USER" -d "$DB_NAME" < /tmp/restore.sql
docker compose start api worker
```

### Update stale listing threshold
```bash
curl -X PATCH https://your-domain.com/api/admin/config/stale-threshold \
  -H "Authorization: Bearer $ADMIN_JWT" \
  -d '{"days": 45}'
```

---

## Project Structure

```
realestate-platform/
├── .env.example
├── .gitignore
├── .github/workflows/
│   ├── backend-ci.yml      # lint → test (80% coverage gate) → build
│   ├── flutter-ci.yml      # analyze → test → build-apk
│   └── deploy.yml          # migrate → deploy on push to main
├── backend/
│   ├── cmd/api/            # HTTP server entrypoint
│   ├── cmd/worker/         # Asynq worker entrypoint
│   ├── internal/
│   │   ├── config/         # Config struct + env loading
│   │   ├── handlers/       # HTTP handlers (auth, properties, tools, ocr, notifications)
│   │   ├── middleware/     # Auth JWT, RBAC, Redis rate limiter
│   │   ├── models/         # Domain types
│   │   ├── respond/        # JSON envelope helpers
│   │   ├── services/       # Business logic (calculator, ocr, stale, share, scrubber…)
│   │   ├── testutil/       # Integration test infra (testcontainers)
│   │   └── worker/         # Asynq client, dispatcher, TwilioSender interface
│   ├── migrations/         # 013 SQL migrations
│   ├── .golangci.yml
│   └── Dockerfile
├── app/
│   ├── lib/
│   │   ├── core/           # Router (GoRouter + 4-tab shell), Dio client, theme
│   │   └── features/       # auth, calculator, map, notifications, properties
│   └── test/
│       ├── unit/           # indian_format_test, time_ago_test
│       └── widgets/        # brokerage_calculator_test, notification_badge_test
└── infra/
    ├── docker-compose.yml
    ├── docker-compose.prod.yml   # Prod overrides (no exposed ports for DB/asynqmon)
    ├── nginx/nginx.conf          # Reverse proxy + security headers
    └── scripts/
        ├── setup-vps.sh          # One-time VPS setup (Ubuntu 24.04)
        └── backup.sh             # Daily Postgres → GCS backup
```
