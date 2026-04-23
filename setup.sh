#!/usr/bin/env bash
# setup.sh — First-time developer setup for the Real Estate Platform.
# Run from the repo root: bash setup.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== Real Estate Platform — Phase 1 Setup ==="

# ── Prerequisites check ───────────────────────────────────────────────────────
check_cmd() {
  if ! command -v "$1" &>/dev/null; then
    echo "  [MISSING] $1 — $2"
    return 1
  fi
  echo "  [OK]      $1 $(${1} version 2>/dev/null | head -1)"
  return 0
}

echo ""
echo "Checking prerequisites..."
missing=0
check_cmd go       "Install from https://go.dev/dl/ (v1.22+)"          || missing=1
check_cmd flutter  "Install from https://docs.flutter.dev/get-started" || missing=1
check_cmd docker   "Install from https://docs.docker.com/get-docker/"  || missing=1

if [[ $missing -ne 0 ]]; then
  echo ""
  echo "Please install the missing tools above, then re-run this script."
  exit 1
fi

# ── Copy .env ─────────────────────────────────────────────────────────────────
if [[ ! -f "$REPO_ROOT/.env" ]]; then
  cp "$REPO_ROOT/.env.example" "$REPO_ROOT/.env"
  echo ""
  echo "Copied .env.example → .env"
  echo "  IMPORTANT: Edit .env and set JWT_SECRET, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET"
fi

# ── Go dependencies ───────────────────────────────────────────────────────────
echo ""
echo "Resolving Go dependencies..."
cd "$REPO_ROOT/backend"
go mod tidy
echo "  go mod tidy — done"

# ── Flutter dependencies ──────────────────────────────────────────────────────
echo ""
echo "Resolving Flutter dependencies..."
cd "$REPO_ROOT/app"
flutter pub get
echo "  flutter pub get — done"

# ── Docker build ──────────────────────────────────────────────────────────────
echo ""
echo "Building Docker images (this may take a few minutes on first run)..."
cd "$REPO_ROOT/infra"
docker compose build
echo "  docker compose build — done"

echo ""
echo "=== Setup complete! ==="
echo ""
echo "Next steps:"
echo "  1. Edit .env with your real secrets"
echo "  2. cd infra && docker compose up -d"
echo "  3. Run migrations:"
echo "       docker exec \$(docker compose ps -q api) sh -c \\"
echo "         'golang-migrate -path /migrations -database \\"
echo "           \"postgres://\$DB_USER:\$DB_PASSWORD@\$DB_HOST:\$DB_PORT/\$DB_NAME?sslmode=disable\" up'"
echo ""
