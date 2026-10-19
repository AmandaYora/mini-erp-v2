#!/usr/bin/env bash
# F6-7: backup database + storage sebelum setiap deploy/cutover.
# Dipakai: ./scripts/ops/backup.sh [--db-only|--storage-only]
# Keluaran: backups/db-<timestamp>-<schemaVersion>.sql.gz dan
#   backups/storage-<timestamp>.tar.gz (+ .sha256).
# Kredensial TIDAK lewat argumen proses: ditulis ke berkas sementara 0600.
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
KEEP_DAILY="${KEEP_DAILY:-14}"
KEEP_WEEKLY="${KEEP_WEEKLY:-8}"
KEEP_MONTHLY="${KEEP_MONTHLY:-6}"
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_NAME="${DB_NAME:-mini_erp_db}"
STORAGE_DIR="${STORAGE_DIR:-./data/storage}"

MODE="${1:-all}"
mkdir -p "$BACKUP_DIR"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
CNF="$(mktemp)"
chmod 600 "$CNF"
trap 'rm -f "$CNF"' EXIT
printf '[client]\nhost=%s\nport=%s\nuser=%s\npassword=%s\n' \
  "$DB_HOST" "$DB_PORT" "$DB_USER" "$DB_PASSWORD" > "$CNF"

if [ "$MODE" = "all" ] || [ "$MODE" = "--db-only" ]; then
  VER="$(mysql --defaults-extra-file="$CNF" -N -e "SELECT IFNULL(MAX(version),0) FROM ${DB_NAME}.schema_migrations;" 2>/dev/null || echo "unknown")"
  OUT="$BACKUP_DIR/db-${TS}-v${VER}.sql.gz"
  echo "backup: database ${DB_NAME} -> ${OUT} (schema v${VER})"
  mysqldump --defaults-extra-file="$CNF" \
    --single-transaction --routines --triggers --events --hex-blob \
    --default-character-set=utf8mb4 --set-gtid-purged=OFF --no-tablespaces \
    "$DB_NAME" | gzip -9 > "$OUT"
  sha256sum "$OUT" > "$OUT.sha256"
  echo "backup: selesai $(du -h "$OUT" | cut -f1)"
fi

if [ "$MODE" = "all" ] || [ "$MODE" = "--storage-only" ]; then
  if [ -d "$STORAGE_DIR/media" ]; then
    OUT="$BACKUP_DIR/storage-${TS}.tar.gz"
    echo "backup: storage ${STORAGE_DIR} -> ${OUT}"
    # _private (sesi WA) ikut terarsip dengan mode 0600; simpan terpisah/terenkripsi
    # di luar host bila berisi sesi produksi (lihat RUNBOOK).
    tar -czf "$OUT" -C "$(dirname "$STORAGE_DIR")" "$(basename "$STORAGE_DIR")"
    chmod 600 "$OUT"
    sha256sum "$OUT" > "$OUT.sha256"
  else
    echo "backup: ${STORAGE_DIR}/media tak ada, storage dilewati"
  fi
fi

echo "backup: retensi harian ${KEEP_DAILY} / mingguan ${KEEP_WEEKLY} / bulanan ${KEEP_MONTHLY} (bersihkan manual di luar host; skrip tidak menghapus otomatis)"
