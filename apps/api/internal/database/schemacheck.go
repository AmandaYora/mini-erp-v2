package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var migFileRe = regexp.MustCompile(`^(\d{6})_.*\.up\.sql$`)

// MaxMigrationVersion reads the largest NNNNNN prefix of *.up.sql in dir.
// The directory is the migrations carried by the image (/app/migrations) or
// the repo checkout (apps/api/migrations); callers try candidates in order.
func MaxMigrationVersion(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	max := 0
	n := 0
	for _, e := range entries {
		m := migFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, _ := strconv.Atoi(m[1])
		if v > max {
			max = v
		}
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("tidak ada berkas *.up.sql di %s", dir)
	}
	return max, nil
}

// FindMigrationsDir returns the first existing migrations directory among the
// container layout (/app/migrations) and the dev layouts.
func FindMigrationsDir() (string, error) {
	for _, c := range []string{"/app/migrations", "migrations", "apps/api/migrations"} {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", fmt.Errorf("direktori migrations tidak ditemukan (coba /app/migrations atau apps/api/migrations)")
}

// CheckMigrationVersion compares the golang-migrate bookkeeping table
// (schema_migrations: version, dirty) against the expected version carried
// by this build (F6-3). Rules:
//
//   - dirty=1 → error (migrasi terakhir gagal di tengah; jangan start).
//   - version < expected → error "jalankan migrasi" (biner baru di schema lama).
//   - version > expected → nil (rollback image; hanya peringatan oleh pemanggil).
//   - tabel tak ada → di production error, di luar production nil + skipped=true
//     (DB dev/test dibangun oleh loader uji, bukan CLI migrate).
func CheckMigrationVersion(ctx context.Context, db *sql.DB, expected int, isProduction bool) (warn string, skipped bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var version int
	var dirty bool
	qerr := db.QueryRowContext(ctx, "SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&version, &dirty)
	if qerr != nil {
		if isProduction {
			return "", false, fmt.Errorf("tabel schema_migrations tak terbaca (%v): jalankan migrasi dulu: docker compose --profile ops run --rm migrate up", qerr)
		}
		return "", true, nil
	}
	if dirty {
		return "", false, fmt.Errorf("schema_migrations.dirty=1 (migrasi %d gagal di tengah): perbaiki manual lalu jalankan migrate up; aplikasi menolak start", version)
	}
	if version < expected {
		return "", false, fmt.Errorf("schema versi %d, biner butuh %d: jalankan migrasi dulu: docker compose --profile ops run --rm migrate up", version, expected)
	}
	if version > expected {
		return fmt.Sprintf("schema versi %d lebih baru dari biner (%d): kemungkinan rollback image; tetap jalan", version, expected), false, nil
	}
	return "", false, nil
}
