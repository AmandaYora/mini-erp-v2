#!/usr/bin/env bash
# F6-7: urutan deploy yang aman: backup -> migrate -> up -> tunggu sehat -> smoke.
# Dipakai dari root: APP_VERSION=<gitsha> ./scripts/ops/deploy.sh
# Berhenti di langkah pertama yang gagal; rollback manual lihat docs/RUNBOOK.md.
set -euo pipefail

APP_VERSION="${APP_VERSION:-dev}"
BASE="${BASE_URL:-http://127.0.0.1:8080}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "deploy: 1/5 backup"
"$SCRIPT_DIR/backup.sh" --db-only

echo "deploy: 2/5 build image mini-erp:${APP_VERSION}"
docker compose build --build-arg "VERSION=${APP_VERSION}" app

echo "deploy: 3/5 migrate up"
docker compose --profile ops run --rm migrate up

echo "deploy: 4/5 up -d app"
APP_VERSION="$APP_VERSION" docker compose up -d app

echo "deploy: 5/5 tunggu sehat (maks 120 dtk)"
for i in $(seq 1 24); do
  if curl -fsS "$BASE/readyz" >/dev/null 2>&1; then
    echo "deploy: healthy"
    curl -fsS "$BASE/api/v1/health" | head -c 200; echo
    exit 0
  fi
  sleep 5
done
echo "deploy: GAGAL — /readyz tak kunjung sehat; lihat docs/RUNBOOK.md (rollback)"
exit 1
