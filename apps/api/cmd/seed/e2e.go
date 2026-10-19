package main

// `seed -profile=e2e` (F8-5): deterministic dataset for the Playwright suite.
//
// It is DESTRUCTIVE by design: it migrates the schema when the migration set
// changed and TRUNCATEs every application table before seeding. Therefore it
// only runs against a *_test database on this machine (dsnguard), and the
// caller (apps/e2e global-setup) must hold the 'mini_erp_test_db' advisory lock
// so it never overlaps the Go integration tests that share the database.
//
// Fixed data (codes, passwords) so specs can address it by name; dates stay
// relative to today because nothing here writes a fixed calendar date.

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
	"time"

	_ "github.com/go-sql-driver/mysql"

	"mini-erp/internal/bootstrap"
	"mini-erp/internal/config"
	branchapp "mini-erp/internal/modules/branch/application"
	companycontracts "mini-erp/internal/modules/company/contracts"
	partyapp "mini-erp/internal/modules/party/application"
	productapp "mini-erp/internal/modules/product/application"
	stockapp "mini-erp/internal/modules/stock/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/signedurl"
	"mini-erp/internal/testutil/dsnguard"
)

// E2EPassword is the fixed password of every e2e account (test database only).
const e2ePassword = "E2e-Rahasia-2026"

const e2eMeta = "_it_" // bookkeeping tables shared with the integration harness

// e2eProducts: code, name, selling price, purchase price, category index.
var e2eProducts = []struct {
	code, name string
	sell, buy  int64
}{
	{"P001", "Beras Premium 5kg", 70000, 60000},
	{"P002", "Minyak Goreng 1L", 18000, 15000},
	{"P003", "Gula Pasir 1kg", 16000, 13500},
	{"P004", "Tepung Terigu 1kg", 12000, 9500},
	{"P005", "Kopi Bubuk 200g", 22000, 17000},
	{"P006", "Teh Celup 25s", 9500, 7000},
	{"P007", "Susu Kental Manis", 13000, 10000},
	{"P008", "Mie Instan Goreng", 3500, 2700},
	{"P009", "Sabun Mandi", 5500, 4000},
	{"P010", "Sampo Sachet", 1500, 1000},
	{"P011", "Deterjen 800g", 21000, 16500},
	{"P012", "Pasta Gigi", 12500, 9000},
	{"P013", "Air Mineral 600ml", 4000, 2800},
	{"P014", "Biskuit Kelapa", 8500, 6000},
	{"P015", "Kecap Manis 275ml", 11000, 8000},
	{"P016", "Saus Sambal 335ml", 10500, 7600},
	{"P017", "Garam Halus 250g", 3000, 2000},
	{"P018", "Telur Ayam 1kg", 30000, 26000},
	{"P019", "Sarden Kaleng", 14500, 11000},
	{"P020", "Roti Tawar", 17000, 13000},
}

func runE2E(cfg *config.Config) {
	if cfg.Env != "test" && cfg.Env != "development" {
		check(fmt.Errorf("profile e2e menolak APP_ENV=%s", cfg.Env))
	}
	check(dsnguard.Check(cfg.DBDSN, false))
	serverDSN, dbName := dsnguard.Split(cfg.DBDSN)
	_ = serverDSN

	// Dedicated connection pool for DDL: multiStatements lets each migration
	// file apply in one Exec (same as the integration harness).
	mdsn := cfg.DBDSN
	sep := "?"
	if strings.Contains(mdsn, "?") {
		sep = "&"
	}
	mdsn += sep + "multiStatements=true&parseTime=true&transaction_isolation=%27READ-COMMITTED%27&time_zone=%27%2B00%3A00%27"
	db, err := sql.Open("mysql", mdsn)
	check(err)
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(10)
	check(db.Ping())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	check(ensureSchema(ctx, db, dbName))
	check(truncateAllTables(ctx, db, dbName))

	w, err := bootstrap.Wire(db, bootstrap.Options{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), JWTSecret: cfg.JWTSecret,
		AccessTTL: cfg.AccessTTL, RefreshTTL: cfg.RefreshTTL,
		Storage: config.Storage{Driver: "local", SignedURLTTLSecs: 300}, MediaDir: os.TempDir(),
		Signer: signedurl.New([]byte("e2e-seed-media-key")), SessionDir: os.TempDir(),
	})
	check(err)
	users, branches, products := w.User.Service(), w.Branch.Service(), w.Product.Service()
	parties, stocks := w.Party.Service(), w.Stock.Service()

	check(bootstrap.EnsureAuthz(ctx, users))
	check(w.Finance.Service().EnsureChart(ctx, 0))
	_, err = w.Company.Service().SaveProfile(ctx, 0, companycontracts.Profile{Name: "Toko E2E"})
	check(err)

	// Seeding adjusts stock without an approver; the always-approve rule is
	// restored (row removed -> default) at the end so E06 exercises it.
	// delivery_require_evidence stays false: proof photos are not part of the
	// suite's flows (E03 toggles it to assert the rule itself).
	_, err = db.ExecContext(ctx, `INSERT INTO company_settings (setting_key, setting_value) VALUES
		('stock_adjust_always_approve', 'false'), ('delivery_require_evidence', 'false') ON DUPLICATE KEY UPDATE setting_value = 'false'`)
	check(err)

	hq, err := branches.CreateBranch(ctx, 0, branchapp.CreateBranchInput{Code: "HQ", Name: "Pusat E2E", IsHead: true, City: "Jakarta"})
	check(err)
	br2, err := branches.CreateBranch(ctx, 0, branchapp.CreateBranchInput{Code: "CB2", Name: "Cabang Dua E2E", City: "Bandung"})
	check(err)

	// Roles: owner/kasir come from EnsureAuthz; gudang and akuntan are custom.
	roleID := func(code string) int64 {
		r, err := users.GetRoleByCode(ctx, code)
		check(err)
		if r == nil {
			check(fmt.Errorf("role %s tidak ada", code))
		}
		return r.ID
	}
	custom := func(code, name string, perms []string) int64 {
		r, err := users.CreateRole(ctx, 0, userapp.CreateRoleInput{Code: code, Name: name, Description: "Peran uji E2E"})
		check(err)
		check(users.AddRolePermissions(ctx, 0, r.ID, perms))
		return r.ID
	}
	gudang := custom("gudang", "Gudang", []string{
		"dashboard.view", "products.view", "suppliers.view", "customers.view",
		"stock.view", "stock.update", "stock.adjust", "stock.transfer", "stock.reserve",
		"purchasing.view", "goodsreceipt.view", "goodsreceipt.create",
		"delivery.view", "delivery.create", "sales.view",
	})
	akuntan := custom("akuntan", "Akuntan", []string{
		"dashboard.view", "reporting.view", "sales.view", "purchasing.view", "payment.view", "payment.create",
		"finance.view", "finance.manage", "finance.posting.view", "finance.posting.post",
		"finance.journal.view", "finance.journal.reverse", "finance.report.view", "finance.tax_report.view",
		"finance.expense.view", "finance.expense.create", "finance.tax_adjustment.view", "finance.tax_adjustment.manage",
		"customers.view", "suppliers.view", "products.view",
	})
	// The default kasir role has no payment.create, and a COD checkout then fails
	// at "confirm delivery" (see apps/e2e/README.md, known issue K1). The suite
	// grants it so the cash POS flow can be exercised.
	check(users.AddRolePermissions(ctx, 0, roleID("kasir"), []string{"payment.create", "payment.view"}))
	mk := func(username, full string, role int64, branchIDs ...int64) {
		acc := make([]usercontracts.BranchAccess, 0, len(branchIDs))
		for i, b := range branchIDs {
			acc = append(acc, usercontracts.BranchAccess{BranchID: b, IsDefault: i == 0})
		}
		_, err := users.CreateUser(ctx, 0, userapp.CreateUserInput{
			Username: username, FullName: full, Password: e2ePassword, RoleIDs: []int64{role}, Branches: acc,
		})
		check(err)
	}
	mk("admin", "Admin E2E", roleID("owner"), hq.ID, br2.ID)
	mk("kasir", "Kasir E2E", roleID("kasir"), hq.ID)
	mk("gudang", "Gudang E2E", gudang, hq.ID)
	mk("akuntan", "Akuntan E2E", akuntan, hq.ID)

	// Locations: two per branch (stock lives in leaves; the branch default
	// location stays as a third, empty one).
	var gudA int64
	for _, b := range []struct {
		id   int64
		code string
	}{{hq.ID, "HQ"}, {br2.ID, "CB2"}} {
		a, err := stocks.CreateLocation(ctx, 0, b.id, stockapp.LocationInput{Code: b.code + "-GA", Name: "Gudang A " + b.code})
		check(err)
		_, err = stocks.CreateLocation(ctx, 0, b.id, stockapp.LocationInput{Code: b.code + "-GB", Name: "Gudang B " + b.code})
		check(err)
		if b.id == hq.ID {
			gudA = a.ID
		}
	}

	// Catalog: 20 products, each with one visible variant carrying a barcode.
	cat, err := products.CreateCategory(ctx, 0, "SEMBAKO", "Sembako", nil, nil)
	check(err)
	for i, p := range e2eProducts {
		barcode := fmt.Sprintf("8990000%05d", i+1)
		prod, err := products.CreateProduct(ctx, 0, productapp.ProductInput{
			Code: p.code, Name: p.name, CategoryID: &cat.ID, Type: "barang", Tracked: true,
			BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs", PurchaseFactor: 1, SalesFactor: 1,
			PurchasePrice: p.buy, SellingPrice: p.sell, MinStock: 5,
			Variants: []productapp.VariantInput{{Code: p.code + "-STD", Name: "Standar", Barcode: barcode, IsDefault: true}},
		})
		check(err)
		var vid int64
		for _, v := range prod.Variants {
			vid = v.ID
		}
		if vid == 0 {
			check(fmt.Errorf("produk %s tanpa varian", p.code))
		}
		check(stocks.Adjust(ctx, 0, hq.ID, stockapp.AdjustInput{
			ProductID: prod.ID, VariantID: vid, Location: gudA, Mode: "in", QtyDelta: 100, Reason: "stok awal e2e",
		}))
	}

	// Parties: walk-in "Tunai" + five customers, three suppliers.
	for i, n := range []string{"Tunai", "Budi Santoso", "Siti Aminah", "Toko Maju Jaya", "CV Sentosa", "PT Abadi Makmur"} {
		code := fmt.Sprintf("C%03d", i+1)
		_, err := parties.CreateCustomer(ctx, 0, partyapp.PartyInput{
			Code: code, Name: n, Phone: fmt.Sprintf("0812000000%02d", i+1), Address: "Jl. Uji No. " + fmt.Sprint(i+1),
		})
		check(err)
	}
	for i, n := range []string{"PT Sumber Pangan", "CV Grosir Jaya", "UD Tani Makmur"} {
		_, err := parties.CreateSupplier(ctx, 0, partyapp.PartyInput{
			Code: fmt.Sprintf("S%03d", i+1), Name: n, Phone: fmt.Sprintf("0813000000%02d", i+1), Address: "Kawasan Industri " + fmt.Sprint(i+1),
		})
		check(err)
	}

	_, err = db.ExecContext(ctx, `DELETE FROM company_settings WHERE setting_key = 'stock_adjust_always_approve'`)
	check(err)
	fmt.Printf("seed e2e ok: branches HQ(%d) CB2(%d), users admin/kasir/gudang/akuntan, %d products\n", hq.ID, br2.ID, len(e2eProducts))
}

func baseTablesE2E(ctx context.Context, conn *sql.Conn, dbName string) ([]string, error) {
	rows, err := conn.QueryContext(ctx,
		`SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE'`, dbName)
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

// ensureSchema mirrors the integration harness: the stored hash of the ordered
// migration set decides between reuse and rebuild, so both harnesses agree.
func ensureSchema(ctx context.Context, db *sql.DB, dbName string) error {
	const dir = "migrations"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("baca migrations: %w", err)
	}
	var ups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	h := sha256.New()
	for _, name := range ups {
		raw, err := os.ReadFile(dir + "/" + name)
		if err != nil {
			return err
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(raw)
		h.Write([]byte{0})
	}
	want := hex.EncodeToString(h.Sum(nil))
	var have string
	_ = db.QueryRowContext(ctx, "SELECT hash FROM `"+e2eMeta+"schema` LIMIT 1").Scan(&have)
	if have == want {
		fmt.Println("seed e2e: skema sudah sesuai migrasi")
		return nil
	}
	fmt.Println("seed e2e: membangun ulang skema dari migrasi")
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
	tables, err := baseTablesE2E(ctx, conn, dbName)
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return err
	}
	for _, t := range tables {
		if _, err := conn.ExecContext(ctx, "DROP TABLE IF EXISTS `"+t+"`"); err != nil {
			return err
		}
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1"); err != nil {
		return err
	}
	for _, name := range ups {
		raw, err := os.ReadFile(dir + "/" + name)
		if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, string(raw)); err != nil {
			return fmt.Errorf("terapkan %s: %w", name, err)
		}
	}
	if _, err := conn.ExecContext(ctx, "CREATE TABLE `"+e2eMeta+"schema` (hash CHAR(64) NOT NULL)"); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "INSERT INTO `"+e2eMeta+"schema` (hash) VALUES (?)", want)
	return err
}

func truncateAllTables(ctx context.Context, db *sql.DB, dbName string) error {
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
	tables, err := baseTablesE2E(ctx, conn, dbName)
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return err
	}
	defer func() { _, _ = conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1") }()
	for _, t := range tables {
		if strings.HasPrefix(t, e2eMeta) || strings.HasPrefix(t, "legacy_") || strings.HasPrefix(t, "etl_") {
			continue
		}
		if _, err := conn.ExecContext(ctx, "TRUNCATE TABLE `"+t+"`"); err != nil {
			return fmt.Errorf("truncate %s: %w", t, err)
		}
	}
	return nil
}
