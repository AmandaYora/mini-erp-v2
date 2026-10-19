// Package etltest builds the scratch environment of the ETL tests inside the
// shared mini_erp_test database: it migrates the revamp schema (same marker as
// the integration harness), creates the `legacy_*` fixture tables next to it
// and tears them down again. Test-only; never imported by the program.
//
// Every test that uses it touches the shared database, so run them through
// scripts/db-lock.mjs (see the ETL README comment in cmd/etl).
package etltest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"mini-erp/internal/bootstrap"
	"mini-erp/internal/testutil/dsnguard"
)

// Prefix of the fixture tables.
const Prefix = "legacy_"

const metaTable = "_it_schema"

// CompanyID of the fixture company.
const CompanyID = 1

const tenantCol = "id_" + "company"

// Env is one prepared database.
type Env struct {
	DB     *sql.DB
	DSN    string
	DBName string
}

// DSN of the restricted test account (or TEST_DB_DSN).
func DSN() string {
	if v := os.Getenv("TEST_DB_DSN"); v != "" {
		return v
	}
	return "mini_erp_test:mini_erp_test@tcp(127.0.0.1:3306)/mini_erp_test"
}

func here() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

// TestdataPath returns a file under internal/etl/testdata.
func TestdataPath(name string) string {
	return filepath.Join(here(), "..", "testdata", name)
}

// Setup prepares schema + fixture. It skips the test when MySQL is not
// reachable and INTEGRATION_OPTIONAL is set, otherwise it fails.
func Setup(t *testing.T) *Env {
	t.Helper()
	dsn := DSN()
	if err := dsnguard.Check(dsn, os.Getenv("ALLOW_REMOTE_TEST_DB") != ""); err != nil {
		t.Fatalf("DSN ditolak: %v", err)
	}
	_, name := dsnguard.Split(dsn)
	full := dsn
	sep := "?"
	if strings.Contains(full, "?") {
		sep = "&"
	}
	full += sep + "multiStatements=true&parseTime=true&loc=UTC&interpolateParams=false"
	db, err := sql.Open("mysql", full)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(8)
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		if os.Getenv("INTEGRATION_OPTIONAL") != "" {
			t.Skipf("MySQL tidak terjangkau: %v", err)
		}
		t.Fatalf("MySQL tidak terjangkau: %v", err)
	}
	// The integration suite and every ETL test truncate the same database:
	// take the cross-process advisory lock for the whole test (released with
	// the dedicated connection).
	lockConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	var got int
	if err := lockConn.QueryRowContext(ctx, "SELECT GET_LOCK('mini_erp_test_db', 3000)").Scan(&got); err != nil || got != 1 {
		t.Fatalf("kunci database test tidak didapat: %v (%d)", err, got)
	}
	e := &Env{DB: db, DSN: dsn, DBName: name}
	t.Cleanup(func() {
		e.dropPrefixed(context.Background(), Prefix)
		e.dropPrefixed(context.Background(), "etl\\_")
		// Release the cross-process lock only after the scratch tables are gone.
		_, _ = lockConn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('mini_erp_test_db')")
		_ = lockConn.Close()
		_ = db.Close()
	})
	e.ensureSchema(t)
	e.dropPrefixed(ctx, Prefix)
	e.dropPrefixed(ctx, "etl\\_")
	e.truncateTarget(t)
	e.seedPermissions(t)
	e.loadFixture(t)
	return e
}

// seedPermissions restores the permission catalog the truncation removed (the
// running application seeds it at start; the role migration needs it).
func (e *Env) seedPermissions(t *testing.T) {
	t.Helper()
	for _, p := range bootstrap.AllPermissionSeeds() {
		if _, err := e.DB.Exec("INSERT INTO permissions (code, name) VALUES (?, ?)", p.Code, p.Name); err != nil {
			t.Fatalf("seed permission %s: %v", p.Code, err)
		}
	}
}

func (e *Env) ensureSchema(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	dir := filepath.Join(here(), "..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("migrations: %v", err)
	}
	var ups []string
	for _, en := range entries {
		if !en.IsDir() && strings.HasSuffix(en.Name(), ".up.sql") {
			ups = append(ups, en.Name())
		}
	}
	sort.Strings(ups)
	h := sha256.New()
	for _, n := range ups {
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		h.Write([]byte(n))
		h.Write([]byte{0})
		h.Write(raw)
		h.Write([]byte{0})
	}
	want := hex.EncodeToString(h.Sum(nil))
	var have string
	_ = e.DB.QueryRowContext(ctx, "SELECT hash FROM `"+metaTable+"` LIMIT 1").Scan(&have)
	if have == want {
		return
	}
	e.dropAll(ctx, t)
	for _, n := range ups {
		raw, _ := os.ReadFile(filepath.Join(dir, n))
		if _, err := e.DB.ExecContext(ctx, string(raw)); err != nil {
			t.Fatalf("apply %s: %v", n, err)
		}
	}
	if _, err := e.DB.ExecContext(ctx, "CREATE TABLE `"+metaTable+"` (hash CHAR(64) NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.DB.ExecContext(ctx, "INSERT INTO `"+metaTable+"` (hash) VALUES (?)", want); err != nil {
		t.Fatal(err)
	}
}

func (e *Env) tables(ctx context.Context) ([]string, error) {
	rows, err := e.DB.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// withConn runs fn on one connection with FK checks off and the name guard.
func (e *Env) withNoFK(ctx context.Context, t *testing.T, fn func(c *sql.Conn)) {
	t.Helper()
	c, err := e.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	var live string
	if err := c.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&live); err != nil {
		t.Fatal(err)
	}
	if err := dsnguard.CheckName(live); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = c.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1") }()
	fn(c)
}

func (e *Env) dropAll(ctx context.Context, t *testing.T) {
	tabs, err := e.tables(ctx)
	if err != nil {
		t.Fatal(err)
	}
	e.withNoFK(ctx, t, func(c *sql.Conn) {
		for _, n := range tabs {
			if _, err := c.ExecContext(ctx, "DROP TABLE IF EXISTS `"+n+"`"); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// dropPrefixed drops tables whose name starts with the LIKE-escaped prefix.
func (e *Env) dropPrefixed(ctx context.Context, likePrefix string) {
	pattern := strings.ReplaceAll(likePrefix, "_", "\\_")
	pattern = strings.ReplaceAll(pattern, "\\\\_", "\\_")
	rows, err := e.DB.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name LIKE ?`, pattern+"%")
	if err != nil {
		return
	}
	var names []string
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		names = append(names, n)
	}
	_ = rows.Close()
	c, err := e.DB.Conn(ctx)
	if err != nil {
		return
	}
	defer func() { _ = c.Close() }()
	_, _ = c.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0")
	for _, n := range names {
		_, _ = c.ExecContext(ctx, "DROP TABLE IF EXISTS `"+n+"`")
	}
	_, _ = c.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1")
}

func (e *Env) truncateTarget(t *testing.T) {
	ctx := context.Background()
	tabs, err := e.tables(ctx)
	if err != nil {
		t.Fatal(err)
	}
	e.withNoFK(ctx, t, func(c *sql.Conn) {
		for _, n := range tabs {
			if n == metaTable || strings.HasPrefix(n, Prefix) || strings.HasPrefix(n, "etl_") {
				continue
			}
			if _, err := c.ExecContext(ctx, "TRUNCATE TABLE `"+n+"`"); err != nil {
				t.Fatalf("truncate %s: %v", n, err)
			}
		}
	})
}

// TruncateTarget empties the revamp tables and drops the ETL bookkeeping so a
// test can run the pipeline from zero again.
func (e *Env) TruncateTarget(t *testing.T) {
	t.Helper()
	e.dropPrefixed(context.Background(), "etl\\_")
	e.truncateTarget(t)
	e.seedPermissions(t)
}

func (e *Env) loadFixture(t *testing.T) {
	t.Helper()
	for _, f := range []string{"legacy_schema.sql", "legacy_data.sql"} {
		raw, err := os.ReadFile(TestdataPath(f))
		if err != nil {
			t.Fatal(err)
		}
		src := strings.ReplaceAll(string(raw), "@@CO@@", tenantCol)
		if _, err := e.DB.Exec(src); err != nil {
			t.Fatalf("fixture %s: %v", f, err)
		}
	}
}

// Exec runs a statement and fails the test on error.
func (e *Env) Exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := e.DB.Exec(strings.ReplaceAll(q, "@@CO@@", tenantCol), args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

// Count returns SELECT COUNT(*) FROM table.
func (e *Env) Count(t *testing.T, table string) int64 {
	t.Helper()
	var n int64
	if err := e.DB.QueryRow("SELECT COUNT(*) FROM `" + table + "`").Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// HashTable returns a stable digest of a table's content (all columns, rows
// sorted by every column), used to prove a second run changes nothing.
func (e *Env) HashTable(t *testing.T, table string) string {
	t.Helper()
	cols, err := e.DB.Query(`SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? ORDER BY ordinal_position`, table)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for cols.Next() {
		var n string
		_ = cols.Scan(&n)
		names = append(names, n)
	}
	_ = cols.Close()
	if len(names) == 0 {
		t.Fatalf("tabel %s tidak ada", table)
	}
	order := "`" + strings.Join(names, "`, `") + "`"
	rows, err := e.DB.Query("SELECT " + order + " FROM `" + table + "` ORDER BY " + order)
	if err != nil {
		t.Fatalf("hash %s: %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	h := sha256.New()
	vals := make([]sql.NullString, len(names))
	ptrs := make([]any, len(names))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		for _, v := range vals {
			if v.Valid {
				fmt.Fprintf(h, "%s\x1f", v.String)
			} else {
				_, _ = h.Write([]byte("\x00\x1f"))
			}
		}
		_, _ = h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}
