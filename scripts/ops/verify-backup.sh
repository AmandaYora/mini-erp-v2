#!/usr/bin/env bash
# F6-7: verifikasi hasil backup/restore ke database tujuan.
# Dipakai: ./scripts/ops/verify-backup.sh <db_name> [--host H --port P --user U]
# Keluar 0 bila sehat; bukan-0 + pesan Indonesia bila ada yang salah.
set -euo pipefail

DB="${1:?pemakaian: verify-backup.sh <db_name>}"
shift || true
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-}"
while [ $# -gt 0 ]; do
  case "$1" in
    --host) DB_HOST="$2"; shift 2;;
    --port) DB_PORT="$2"; shift 2;;
    --user) DB_USER="$2"; shift 2;;
    *) echo "argumen tak dikenal: $1"; exit 2;;
  esac
done

CNF="$(mktemp)"; chmod 600 "$CNF"
trap 'rm -f "$CNF"' EXIT
printf '[client]\nhost=%s\nport=%s\nuser=%s\npassword=%s\n' \
  "$DB_HOST" "$DB_PORT" "$DB_USER" "$DB_PASSWORD" > "$CNF"
Q() { mysql --defaults-extra-file="$CNF" -N -e "$1" "$DB"; }

fail=0
check() { # check <nama> <sql> <kondisi-pesan-gagal-awk>
  local name="$1" sql="$2"
  local got
  got="$(Q "$sql")"
  echo "verify: ${name} = ${got}"
}

echo "verify: database ${DB}"
VER="$(Q "SELECT IFNULL(MAX(version),-1) FROM schema_migrations;" 2>/dev/null || echo MISSING)"
echo "verify: schema_migrations.version = ${VER}"
if [ "$VER" = "MISSING" ] || [ "$VER" = "-1" ]; then echo "verify: GAGAL — tabel schema_migrations tak ada"; fail=1; fi
DIRTY="$(Q "SELECT IFNULL(MAX(dirty),0) FROM schema_migrations;" 2>/dev/null || echo 1)"
if [ "$DIRTY" != "0" ]; then echo "verify: GAGAL — schema_migrations.dirty=${DIRTY}"; fail=1; fi

for t in branches users products parties stock_balances sales_orders purchase_orders delivery_notes payments finance_journal_entries finance_journal_lines; do
  N="$(Q "SELECT COUNT(*) FROM ${t};" 2>/dev/null || echo MISSING)"
  echo "verify: ${t} = ${N}"
  if [ "$N" = "MISSING" ]; then echo "verify: GAGAL — tabel ${t} tak ada"; fail=1; fi
done

BAL="$(Q "SELECT IFNULL(SUM(debit),0)-IFNULL(SUM(credit),0) FROM finance_journal_lines;" 2>/dev/null || echo MISSING)"
echo "verify: jurnal debit-credit = ${BAL}"
if [ "$BAL" != "0" ]; then echo "verify: GAGAL — jurnal tak seimbang"; fail=1; fi

NEG="$(Q "SELECT COUNT(*) FROM stock_balances WHERE on_hand_qty < 0;" 2>/dev/null || echo MISSING)"
echo "verify: saldo stok negatif = ${NEG}"

if [ "$fail" != "0" ]; then echo "verify: GAGAL"; exit 1; fi
echo "verify: SEHAT"
