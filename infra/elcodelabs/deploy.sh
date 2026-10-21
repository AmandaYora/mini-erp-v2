#!/usr/bin/env bash
# mini-erp-v2 — deploy di VPS elcodelabs (ADR-0019, /opt/elcodelabs/SERVER_PLAYBOOK.md).
# VPS 2 GB TIDAK PERNAH membangun image: CI (release-image.yml) membangun & push ke GHCR;
# skrip ini hanya: cek awal -> pull -> backup DB -> migrate -> up -> tunggu /readyz.
#
# Pakai (di VPS, dari ~/minierp):
#   ./deploy.sh <git-sha>          # deploy image yang persis & tak berubah (disarankan)
#   ./deploy.sh <git-sha-lama>     # rollback = deploy ulang sha sebelumnya (migrasi maju saja)
#
# Variabel opsional:
#   SKIP_BACKUP=1    lewati backup DB (hanya deploy PERTAMA saat database masih kosong)
#   SKIP_PULL=1      pakai image yang sudah ada di mesin (uji lokal; produksi selalu pull)
#   MYSQLDUMP="..."  perintah dump (default: sudo -n mysqldump — root MySQL via auth_socket)
set -euo pipefail
cd "$(dirname "$0")"

TAG="${1:-}"
if [ -z "$TAG" ]; then
  echo "pakai: ./deploy.sh <git-sha>   (lihat tag di GitHub Actions » release-image)" >&2
  exit 2
fi
if [ "$TAG" = "latest" ]; then
  echo "tolak: 'latest' bukan versi yang bisa di-rollback — pakai <git-sha> lengkap." >&2
  exit 2
fi

APP=minierp
PORT=8084
IMAGE="ghcr.io/amandayora/mini-erp-v2:${TAG}"
COMPOSE="docker compose -f docker-compose.prod.yml"
MYSQLDUMP="${MYSQLDUMP:-sudo -n mysqldump}"

envval() { grep -E "^$1=" .env | tail -1 | cut -d= -f2-; }

echo "▶ [0/5] cek awal"
[ -f .env ] || { echo "  ✗ .env tidak ada (salin dari .env.production.example di repo, isi nilai asli)"; exit 1; }
perm="$(stat -c %a .env 2>/dev/null || stat -f %Lp .env)"
[ "$perm" = "600" ] || { echo "  ✗ .env harus chmod 600 (sekarang $perm)"; exit 1; }
[ "$(envval APP_ENV)" = "production" ] || { echo "  ✗ APP_ENV di .env harus production"; exit 1; }
DB_NAME="$(envval DB_NAME)"
[ -n "$DB_NAME" ] || { echo "  ✗ DB_NAME kosong di .env"; exit 1; }
mkdir -p data/storage backups
owner="$(stat -c %u data/storage 2>/dev/null || stat -f %u data/storage)"
[ "$owner" = "10001" ] || { echo "  ✗ data/storage harus milik uid 10001 (user image): sudo chown -R 10001:10001 data/storage"; exit 1; }
PREV_TAG="$(envval IMAGE_TAG || true)"
echo "  ✓ .env 600, APP_ENV=production, DB_NAME=${DB_NAME}, storage uid 10001; versi sebelumnya: ${PREV_TAG:-(belum ada)}"

export IMAGE_TAG="$TAG"

echo "▶ [1/5] pull image: ${IMAGE}"
if [ "${SKIP_PULL:-0}" = "1" ]; then
  docker image inspect "$IMAGE" >/dev/null || { echo "  ✗ SKIP_PULL=1 tetapi image lokal tidak ada"; exit 1; }
  echo "  (SKIP_PULL=1 — memakai image lokal)"
else
  $COMPOSE pull app
fi

echo "▶ [2/5] backup database ${DB_NAME} sebelum migrasi"
if [ "${SKIP_BACKUP:-0}" = "1" ]; then
  echo "  (SKIP_BACKUP=1 — dilewati; hanya untuk deploy pertama di database kosong)"
else
  out="backups/pre-deploy-$(date +%Y%m%d-%H%M%S)-${TAG:0:12}.sql.gz"
  # pipefail: dump yang gagal menggagalkan deploy (jangan migrasi tanpa backup).
  $MYSQLDUMP --single-transaction --routines --triggers --events --hex-blob \
    --default-character-set=utf8mb4 --set-gtid-purged=OFF --no-tablespaces "$DB_NAME" | gzip -9 > "$out"
  chmod 600 "$out"
  echo "  ✓ ${out} ($(du -h "$out" | cut -f1))"
fi

echo "▶ [3/5] migrasi database (image yang sama, golang-migrate)"
$COMPOSE --profile ops run --rm migrate up

echo "▶ [4/5] start app"
$COMPOSE up -d app
# Catat versi yang berjalan agar `docker compose up` manual tetap di versi ini.
if grep -qE '^IMAGE_TAG=' .env; then
  sed -i.bak -E "s|^IMAGE_TAG=.*|IMAGE_TAG=${TAG}|" .env && rm -f .env.bak
else
  printf '\n# dikelola deploy.sh — versi image yang sedang berjalan\nIMAGE_TAG=%s\n' "$TAG" >> .env
fi
chmod 600 .env

echo "▶ [5/5] tunggu sehat (/readyz, maks 120 dtk)"
for _ in $(seq 1 24); do
  if curl -fsS "http://127.0.0.1:${PORT}/readyz" >/dev/null 2>&1; then
    echo "  ✓ sehat: $(curl -fsS "http://127.0.0.1:${PORT}/api/v1/health" | head -c 200)"
    docker image prune -f >/dev/null 2>&1 || true
    echo "✓ selesai. ${APP} berjalan di ${IMAGE}"
    exit 0
  fi
  sleep 5
done
echo "  ✗ /readyz tak kunjung sehat — log terakhir:"
docker logs --tail 60 "${APP}-app" || true
if [ -n "$PREV_TAG" ]; then
  echo "  Rollback: ./deploy.sh ${PREV_TAG}   (migrasi bersifat maju; lihat docs/RUNBOOK.md)"
fi
exit 1
