#!/bin/bash
# infra/scripts/backup.sh
#
# Daily Postgres backup to GCS.
# Runs as a cron job on the VPS (not in Docker).
# Cron entry (as deploy user):
#   0 2 * * * /opt/realestate/scripts/backup.sh >> /var/log/realestate-backup.log 2>&1
#
# Required environment variables (read from /opt/realestate/.env):
#   GCS_BUCKET  — GCS bucket name for uploads (without gs:// prefix)
#   DB_USER     — Postgres user
#   DB_NAME     — Postgres database name
#
# Authentication: uses Application Default Credentials (ADC) configured on the
# VPS via `gcloud auth application-default login` or a service account key.
# No credentials are hardcoded in this script.
set -euo pipefail

# Load .env from the app directory (reads key=value lines, ignoring comments).
ENV_FILE="/opt/realestate/.env"
if [[ -f "$ENV_FILE" ]]; then
    set -a
    # shellcheck disable=SC1090
    source <(grep -v '^\s*#' "$ENV_FILE" | grep -v '^\s*$')
    set +a
fi

: "${GCS_BUCKET:?GCS_BUCKET env var required}"
: "${DB_USER:?DB_USER env var required}"
: "${DB_NAME:?DB_NAME env var required}"

DATE=$(date +%Y-%m-%d-%H%M)
BACKUP_FILE="/tmp/realestate-backup-${DATE}.sql.gz"

echo "[$(date)] Starting backup for database ${DB_NAME}..."

# Dump from the running postgres container.
docker exec realestate-postgres pg_dump \
    -U "${DB_USER}" "${DB_NAME}" \
    | gzip > "${BACKUP_FILE}"

echo "[$(date)] Dump complete: ${BACKUP_FILE}"

# Upload to GCS using ADC.
gcloud storage cp "${BACKUP_FILE}" "gs://${GCS_BUCKET}/backups/${DATE}.sql.gz"

echo "[$(date)] Uploaded to gs://${GCS_BUCKET}/backups/${DATE}.sql.gz"

# Remove local copy.
rm "${BACKUP_FILE}"

# Prune backups older than 30 days (files whose name does not match this or last month).
THIS_MONTH=$(date +%Y-%m)
LAST_MONTH=$(date -d "1 month ago" +%Y-%m 2>/dev/null || date -v-1m +%Y-%m)

gcloud storage ls "gs://${GCS_BUCKET}/backups/" \
    | grep -v "${THIS_MONTH}" \
    | grep -v "${LAST_MONTH}" \
    | xargs -r gcloud storage rm -- || true

echo "[$(date)] Backup complete: gs://${GCS_BUCKET}/backups/${DATE}.sql.gz"
