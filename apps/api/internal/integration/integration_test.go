// Package integration wires the real modules against a disposable MySQL
// database (mini_erp_test) for money-path tests. Nothing here ships:
// all files are _test.go. Rule A (contracts-only) does not apply outside
// internal/modules, so tests may use application input types directly.
package integration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"mini-erp/internal/bootstrap"
	"mini-erp/internal/config"
	"mini-erp/internal/modules/assistant"
	"mini-erp/internal/modules/audit"
	"mini-erp/internal/modules/auth"
	"mini-erp/internal/modules/branch"
	branchapp "mini-erp/internal/modules/branch/application"
	"mini-erp/internal/modules/company"
	"mini-erp/internal/modules/dashboard"
	"mini-erp/internal/modules/delivery"
	"mini-erp/internal/modules/finance"
	"mini-erp/internal/modules/goodsreceipt"
	"mini-erp/internal/modules/media"
	"mini-erp/internal/modules/party"
	partyapp "mini-erp/internal/modules/party/application"
	"mini-erp/internal/modules/payment"
	"mini-erp/internal/modules/product"
	productapp "mini-erp/internal/modules/product/application"
	"mini-erp/internal/modules/purchasereturn"
	"mini-erp/internal/modules/purchasing"
	"mini-erp/internal/modules/reporting"
	"mini-erp/internal/modules/sales"
	salesapp "mini-erp/internal/modules/sales/application"
	"mini-erp/internal/modules/salesreturn"
	"mini-erp/internal/modules/stock"
	stockapp "mini-erp/internal/modules/stock/application"
	"mini-erp/internal/modules/user"
	"mini-erp/internal/shared/signedurl"
	"mini-erp/internal/testutil/dsnguard"
)

// mods mirrors cmd/server/main.go wiring over the test database.
type mods struct {
	db             *sql.DB
	audit          *audit.Module
	user           *user.Module
	branch         *branch.Module
	company        *company.Module
	auth           *auth.Module
	assistant      *assistant.Module
	media          *media.Module
	product        *product.Module
	party          *party.Module
	stock          *stock.Module
	purchasing     *purchasing.Module
	sales          *sales.Module
	goodsreceipt   *goodsreceipt.Module
	delivery       *delivery.Module
	salesreturn    *salesreturn.Module
	purchasereturn *purchasereturn.Module
	payment        *payment.Module
	finance        *finance.Module
	dashboard      *dashboard.Module
	reporting      *reporting.Module
	// wired is the whole bootstrap graph, for steps the server runs at start.
	wired *bootstrap.Modules
}

var testMods *mods

// testDBDSN reads TEST_DB_DSN or defaults to the restricted account that
// scripts/test-db.mjs creates (it can only touch mini_erp_test, so a wrong DSN
// cannot reach another project's database on a shared MySQL server).
func testDBDSN() string {
	if v := os.Getenv("TEST_DB_DSN"); v != "" {
		return v
	}
	return "mini_erp_test:mini_erp_test@tcp(127.0.0.1:3306)/mini_erp_test"
}

func splitDSN(dsn string) (server, name string) { return dsnguard.Split(dsn) }

// metaTablePrefix marks bookkeeping tables of this harness; truncateAll and
// the rebuild path leave them alone.
const metaTablePrefix = "_it_"

// baseTables lists the application tables of the connected database.
func baseTables(ctx context.Context, conn *sql.Conn, dbName string) ([]string, error) {
	rows, err := conn.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE'`, dbName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// truncateAll empties every application table. It runs on ONE dedicated
// connection (SET FOREIGN_KEY_CHECKS is per session, so a pooled connection
// could silently lose it) and re-checks the live database name first.
func truncateAll(t *testing.T, db *sql.DB, dbName string) {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer func() { _ = conn.Close() }()
	var live string
	if err := conn.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&live); err != nil {
		t.Fatalf("select database: %v", err)
	}
	if err := dsnguard.CheckName(live); err != nil {
		t.Fatalf("%v", err)
	}
	tables, err := baseTables(ctx, conn, dbName)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		t.Fatalf("fk off: %v", err)
	}
	defer func() { _, _ = conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1") }()
	for _, tbl := range tables {
		if strings.HasPrefix(tbl, metaTablePrefix) || strings.HasPrefix(tbl, "legacy_") || strings.HasPrefix(tbl, "etl_") {
			continue
		}
		if _, err := conn.ExecContext(ctx, "TRUNCATE TABLE `"+tbl+"`"); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}
}

// dropAllTables removes every table (including bookkeeping) from the connected
// database. It never drops the database itself.
func dropAllTables(ctx context.Context, db *sql.DB, dbName string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	var live string
	if err := conn.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&live); err != nil {
		return err
	}
	if err := dsnguard.CheckName(live); err != nil {
		return err
	}
	tables, err := baseTables(ctx, conn, dbName)
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return err
	}
	defer func() { _, _ = conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1") }()
	for _, tbl := range tables {
		if _, err := conn.ExecContext(ctx, "DROP TABLE IF EXISTS `"+tbl+"`"); err != nil {
			return err
		}
	}
	return nil
}

// schemaHash fingerprints the ordered migration contents so a changed schema
// forces a rebuild instead of silently reusing a stale test database.
func schemaHash(dir string, names []string) (string, error) {
	h := sha256.New()
	for _, name := range names {
		raw, err := os.ReadFile(dir + "/" + name)
		if err != nil {
			return "", err
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(raw)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func TestMain(m *testing.M) {
	dsn := testDBDSN()
	// Guard first: nothing below (DROP/CREATE/TRUNCATE) may run against a
	// database that is not a *_test database on this machine.
	if err := dsnguard.Check(dsn, os.Getenv("ALLOW_REMOTE_TEST_DB") != ""); err != nil {
		fmt.Println("integration: DSN ditolak:", err)
		os.Exit(2)
	}
	serverDSN, dbName := splitDSN(dsn)
	server, err := sql.Open("mysql", serverDSN)
	if err != nil {
		fmt.Println("integration: open server:", err)
		os.Exit(1)
	}
	// Without MySQL fail loudly (a silent skip would look like a green run).
	// INTEGRATION_OPTIONAL=1 is for laptops without a database; CI must not set it.
	pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	if err := server.PingContext(pingCtx); err != nil {
		cancel()
		if os.Getenv("INTEGRATION_OPTIONAL") != "" {
			fmt.Println("integration: MySQL tidak terjangkau, test integrasi DILEWATI (INTEGRATION_OPTIONAL=1):", err)
			os.Exit(0)
		}
		fmt.Println("integration: MySQL tidak terjangkau:", err)
		fmt.Println("integration: nyalakan dengan `node scripts/test-db.mjs up`, atau set INTEGRATION_OPTIONAL=1 untuk melewati.")
		os.Exit(1)
	}
	cancel()
	// TESTDB_REBUILD=1 drops first (schema changed: re-migrate from zero).
	if os.Getenv("TESTDB_REBUILD") != "" {
		if _, err := server.Exec("DROP DATABASE IF EXISTS `" + dbName + "`"); err != nil {
			fmt.Println("integration: drop db:", err)
			os.Exit(1)
		}
	}
	if _, err := server.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "`"); err != nil {
		fmt.Println("integration: create db:", err)
		os.Exit(1)
	}
	_ = server.Close()

	// multiStatements so each migration file applies in one Exec.
	mdsn := dsn
	sep := "?"
	if strings.Contains(mdsn, "?") {
		sep = "&"
	}
	// READ COMMITTED + UTC mirror production (database.ensureParams); a test
	// DB on the server default (REPEATABLE READ) would hide or invent races.
	mdsn += sep + "multiStatements=true&parseTime=true" +
		"&transaction_isolation=%27READ-COMMITTED%27&time_zone=%27%2B00%3A00%27"
	db, err := sql.Open("mysql", mdsn)
	if err != nil {
		fmt.Println("integration: open db:", err)
		os.Exit(1)
	}
	// Mirror production pool bounds (F2a-11): concurrency tests must prove
	// the code works with 25 connections, not with an unbounded pool that
	// hides exhaustion (Error 1040) on the shared server.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	// Serialise with the ETL tests (they rebuild and truncate the same database):
	// hold the cross-process advisory lock for the whole run, taken BEFORE the
	// schema check below. Taking it only after the rebuild let `go test ./...`
	// run this package's DROP/CREATE concurrently with an ETL package's rebuild
	// on a fresh database (CI: "table already exists" / "doesn't exist").
	lockConn, lerr := db.Conn(context.Background())
	if lerr != nil {
		fmt.Println("integration: conn kunci:", lerr)
		os.Exit(1)
	}
	var got int
	if err := lockConn.QueryRowContext(context.Background(), "SELECT GET_LOCK('mini_erp_test_db', 3000)").Scan(&got); err != nil || got != 1 {
		fmt.Println("integration: kunci database test tidak didapat:", err, got)
		os.Exit(1)
	}
	const migrationsDir = "../../migrations"
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		fmt.Println("integration: read migrations:", err)
		os.Exit(1)
	}
	var ups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	want, err := schemaHash(migrationsDir, ups)
	if err != nil {
		fmt.Println("integration: hash migrations:", err)
		os.Exit(1)
	}
	// A stored hash that differs (or is absent) means the migrations changed
	// since this database was built: rebuild instead of reusing stale tables.
	var have string
	_ = db.QueryRow("SELECT hash FROM `" + metaTablePrefix + "schema` LIMIT 1").Scan(&have)
	if have == want {
		fmt.Println("integration: reusing migrated test db")
	} else {
		ctx := context.Background()
		if err := dropAllTables(ctx, db, dbName); err != nil {
			fmt.Println("integration: reset schema:", err)
			os.Exit(1)
		}
		for _, name := range ups {
			raw, err := os.ReadFile(migrationsDir + "/" + name)
			if err != nil {
				fmt.Println("integration: read", name, err)
				os.Exit(1)
			}
			if _, err := db.Exec(string(raw)); err != nil {
				fmt.Println("integration: apply", name, err)
				os.Exit(1)
			}
		}
		if _, err := db.Exec("CREATE TABLE `" + metaTablePrefix + "schema` (hash CHAR(64) NOT NULL)"); err != nil {
			fmt.Println("integration: create schema marker:", err)
			os.Exit(1)
		}
		if _, err := db.Exec("INSERT INTO `"+metaTablePrefix+"schema` (hash) VALUES (?)", want); err != nil {
			fmt.Println("integration: store schema hash:", err)
			os.Exit(1)
		}
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionDir, err := os.MkdirTemp("", "wa-test-*")
	if err != nil {
		fmt.Println("integration: session dir:", err)
		os.Exit(1)
	}
	// The very wiring the server runs (bootstrap.Wire): tests exercise the
	// production module graph, not a hand-copied one.
	w, err := bootstrap.Wire(db, bootstrap.Options{
		Log: log, JWTSecret: "integration-test-jwt-secret-minimal-32-karakter",
		AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour,
		Storage: config.Storage{Driver: "local", SignedURLTTLSecs: 300}, MediaDir: os.TempDir(),
		Signer: signedurl.New([]byte("integration-test-key")), SessionDir: sessionDir,
	})
	if err != nil {
		fmt.Println("integration: wire:", err)
		os.Exit(1)
	}
	testMods = &mods{db: db, audit: w.Audit, user: w.User, branch: w.Branch,
		company: w.Company, auth: w.Auth, assistant: w.Assistant, media: w.Media, product: w.Product, party: w.Party,
		stock: w.Stock, purchasing: w.Purchasing, sales: w.Sales,
		goodsreceipt: w.GoodsReceipt, delivery: w.Delivery, salesreturn: w.SalesReturn,
		purchasereturn: w.PurchaseReturn, payment: w.Payment, finance: w.Finance,
		dashboard: w.Dashboard, reporting: w.Reporting, wired: w}

	code := m.Run()
	if lockConn != nil {
		_, _ = lockConn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('mini_erp_test_db')")
		_ = lockConn.Close()
	}
	_ = db.Close()
	os.Exit(code)
}

// freshDB truncates every table so each test starts empty.
func freshDB(t *testing.T) {
	t.Helper()
	_, dbName := splitDSN(testDBDSN())
	truncateAll(t, testMods.db, dbName)
	// Auto-increment ids restart after TRUNCATE: drop sessions cached under ids the previous test used.
	testMods.auth.Service().InvalidateAll()
}

// shopFixture builds branch + product (1 variant) + customer + location +
// opening stock + chart. Amounts are small; stock delta stays under the
// default approval threshold.
type shopFixture struct {
	branchID   int64
	productID  int64
	variantID  int64
	locationID int64
	customerID int64
}

func newShop(t *testing.T, ctx context.Context) shopFixture {
	t.Helper()
	m := testMods
	// Fixtures adjust stock without an approver; the always-approve rule
	// (default in production) is exercised by its own tests.
	if _, err := m.db.Exec(`INSERT INTO company_settings (setting_key, setting_value) VALUES
		('stock_adjust_always_approve', 'false'), ('delivery_require_evidence', 'false')
		ON DUPLICATE KEY UPDATE setting_value = 'false'`); err != nil {
		t.Fatalf("fixture setting: %v", err)
	}
	br, err := m.branch.Service().CreateBranch(ctx, 0, branchapp.CreateBranchInput{
		Code: "TST", Name: "Test", IsHead: true,
	})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	p, err := m.product.Service().CreateProduct(ctx, 0, productapp.ProductInput{
		Code: "TST-001", Name: "Barang Uji", Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs",
		PurchaseFactor: 1, SalesFactor: 1,
		PurchasePrice: 10000, SellingPrice: 15000,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if len(p.Variants) == 0 {
		t.Fatal("product has no default variant")
	}
	cust, err := m.party.Service().CreateCustomer(ctx, 0, partyapp.PartyInput{
		Name: "Customer Uji", Phone: "081234567890",
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	loc, err := m.stock.Service().CreateLocation(ctx, 0, br.ID, stockapp.LocationInput{
		Code: "TST", Name: "Gudang Uji",
	})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	if err := m.stock.Service().Adjust(ctx, 0, br.ID, stockapp.AdjustInput{
		ProductID: p.ID, VariantID: p.Variants[0].ID, Location: loc.ID,
		Mode: "in", QtyDelta: 50, Reason: "stok awal uji",
	}); err != nil {
		t.Fatalf("adjust in: %v", err)
	}
	if err := m.finance.Service().EnsureChart(ctx, 0); err != nil {
		t.Fatalf("ensure chart: %v", err)
	}
	return shopFixture{
		branchID: br.ID, productID: p.ID, variantID: p.Variants[0].ID,
		locationID: loc.ID, customerID: cust.ID,
	}
}

// sellFixture creates + confirms an SO for qty units of the fixture product.
func sellFixture(t *testing.T, ctx context.Context, fx shopFixture, qty float64) int64 {
	t.Helper()
	so, err := testMods.sales.Service().CreateOrder(ctx, 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty,
		}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx, 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	return so.ID
}
