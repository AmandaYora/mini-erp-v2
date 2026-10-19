#!/bin/bash
# Gladi ETL lokal (Windows/Git Bash + Laragon MySQL). Dipakai pada checkpoint 2026-10-16.
# Lihat docs/plan/menuju-pengganti-production/CHECKPOINT-2026-10-16.md untuk konteks dan urutan.
# Prasyarat: mini_erp_legacy_copy (dump + etl_copy_marker + collation seragam) sudah ada.
# Mereset target gladi lalu menjalankan ETL. Usage: etl-rehearsal.sh [phase]
#
# Variabel (semua punya default; timpa lewat environment):
#   MYSQL_BIN   klien mysql              OPS_ROOT   folder kerja besar (hasil, binari, cache)
#   MEDIA_SRC   folder media-export      WORKDIR    keluaran ETL (default $OPS_ROOT/etl-l1)
#   EXTRA       argumen tambahan, mis. EXTRA="--compare-with <dir-run1>" untuk L2
# Letakkan OPS_ROOT dan MEDIA_SRC di disk yang sehat (SSD internal), bukan USB.
set -e
REPO="$(cd "$(dirname "$0")/../.." && pwd)"
M="${MYSQL_BIN:-$(ls /c/laragon/bin/mysql/*/bin/mysql.exe | head -1)}"
OPS_ROOT="${OPS_ROOT:-$REPO/../ops-work}"
MEDIA_SRC="${MEDIA_SRC:-$REPO/../legacy-media/media-export}"
WORKDIR="${WORKDIR:-$OPS_ROOT/etl-l1}"
MEDIA_DST="${MEDIA_DST:-$OPS_ROOT/etl-media-dst}"
mkdir -p "$OPS_ROOT/etl-bin"
ETL_BIN="$OPS_ROOT/etl-bin/etl.exe"
cd "$REPO"
export GOFLAGS="-buildvcs=false"
(cd apps/api && gofmt -w internal/etl && go build -o "$ETL_BIN" ./cmd/etl)
"$M" -uroot -e "DROP DATABASE IF EXISTS mini_erp_rehearsal; CREATE DATABASE mini_erp_rehearsal CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
DB_DSN='root:@tcp(127.0.0.1:3306)/mini_erp_rehearsal?multiStatements=true' node scripts/migrate.mjs up >/dev/null
(cd apps/api && APP_ENV=development JWT_SECRET=rehearsal-secret-rehearsal-secret-123456 DB_DSN='root:@tcp(127.0.0.1:3306)/mini_erp_rehearsal' DB_NAME=mini_erp_rehearsal go run ./cmd/seed init 2>&1 | tail -5)
"$M" -uroot mini_erp_rehearsal -e "SET FOREIGN_KEY_CHECKS=0; TRUNCATE company_profile; TRUNCATE branch_document_sequences; TRUNCATE audit_logs; DELETE FROM user_branch_access; DELETE FROM branches; SET FOREIGN_KEY_CHECKS=1"
export ETL_LEGACY_DSN='root:@tcp(127.0.0.1:3306)/mini_erp_legacy_copy?parseTime=true' ETL_TARGET_DSN='root:@tcp(127.0.0.1:3306)/mini_erp_rehearsal?parseTime=true&multiStatements=true'
rm -rf "$WORKDIR" "$MEDIA_DST"; mkdir -p "$WORKDIR"
set +e
"$ETL_BIN" "${1:-rehearse}" --company-id 1 --opening-date "${OPENING_DATE:-2026-01-01}" --workdir "$WORKDIR" --media-src "$MEDIA_SRC" --media-dst "$MEDIA_DST" $EXTRA 2>&1 | grep -v "msg=extract\|msg=load table"
