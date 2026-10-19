#!/usr/bin/env bash
# F6-7: restore dump ke database BARU lalu verifikasi — tidak pernah menimpa
# database berjalan tanpa --yes-overwrite. Lihat docs/RUNBOOK.md.
# Dipakai: ./scripts/ops/restore.sh <dump.sql.gz> [db_baru] [--yes-overwrite]
set -euo pipefail

DUMP="${1:?pemakaian: restore.sh <dump.sql.gz> [db_baru] [--yes-overwrite]}"
TARGET="${2:-mini_erp_restore_$(date -u +%Y%m%dT%H%M%SZ)}"
OVERWRITE=no
for a in "$@"; do [ "$a" = "--yes-overwrite" ] && OVERWRITE=yes; done
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-}"

CNF="$(mktemp)"; chmod 600 "$CNF"
trap 'rm -f "$CNF"' EXIT
printf '[client]\nhost=%s\nport=%s\nuser=%s\npassword=%s\n' \
  "$DB_HOST" "$DB_PORT" "$DB_USER" "$DB_PASSWORD" > "$CNF"

if mysql --defaults-extra-file="$CNF" -N -e "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='${TARGET}';" | grep -q .; then
  if [ "$OVERWRITE" != "yes" ]; then
    echo "restore: database ${TARGET} sudah ada; ulangi dengan --yes-overwrite atau pilih nama lain"
    exit 1
  fi
fi
echo "restore: buat ${TARGET}"
mysql --defaults-extra-file="$CNF" -e "CREATE DATABASE IF NOT EXISTS \`${TARGET}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"
echo "restore: impor ${DUMP} -> ${TARGET}"
gunzip -c "$DUMP" | mysql --defaults-extra-file="$CNF" "$TARGET"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
"$SCRIPT_DIR/verify-backup.sh" "$TARGET"
echo "restore: SEHAT di ${TARGET}. Untuk mengarahkan aplikasi: ubah DB_DSN/DB_NAME ke ${TARGET} lalu restart."
