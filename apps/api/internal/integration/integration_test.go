// Package integration wires the real modules against a disposable MySQL
// database (mini_erp_test) for money-path tests. Nothing here ships:
// all files are _test.go. Rule A (contracts-only) does not apply outside
// internal/modules, so tests may use application input types directly.
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"mini-erp/internal/config"
	"mini-erp/internal/modules/audit"
	"mini-erp/internal/modules/branch"
	branchapp "mini-erp/internal/modules/branch/application"
	"mini-erp/internal/modules/company"
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
	"mini-erp/internal/modules/sales"
	salesapp "mini-erp/internal/modules/sales/application"
	"mini-erp/internal/modules/salesreturn"
	"mini-erp/internal/modules/stock"
	stockapp "mini-erp/internal/modules/stock/application"
	"mini-erp/internal/modules/user"
)

// mods mirrors cmd/server/main.go wiring over the test database.
type mods struct {
	db             *sql.DB
	audit          *audit.Module
	user           *user.Module
	branch         *branch.Module
	company        *company.Module
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
}

var testMods *mods

// testDBDSN reads TEST_DB_DSN or defaults to localhost. The database must
// exist or be creatable by these credentials (CI: grant CREATE).
func testDBDSN() string {
	if v := os.Getenv("TEST_DB_DSN"); v != "" {
		return v
	}
	return "root:@tcp(localhost:3306)/mini_erp_test"
}

func splitDSN(dsn string) (server, name string) {
	name = dsn
	if i := strings.LastIndex(dsn, "/"); i >= 0 {
		server, name = dsn[:i+1], dsn[i+1:]
	}
	if i := strings.Index(name, "?"); i >= 0 {
		name = name[:i]
	}
	return server, name
}

func truncateAll(t *testing.T, db *sql.DB, dbName string) {
	t.Helper()
	rows, err := db.Query(
		`SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE'`, dbName)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		tables = append(tables, name)
	}
	_ = rows.Close()
	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS=0"); err != nil {
		t.Fatalf("fk off: %v", err)
	}
	for _, tbl := range tables {
		if _, err := db.Exec("TRUNCATE TABLE `" + tbl + "`"); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}
	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS=1"); err != nil {
		t.Fatalf("fk on: %v", err)
	}
}

func TestMain(m *testing.M) {
	dsn := testDBDSN()
	serverDSN, dbName := splitDSN(dsn)
	server, err := sql.Open("mysql", serverDSN)
	if err != nil {
		fmt.Println("integration: open server:", err)
		os.Exit(1)
	}
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
	mdsn += sep + "multiStatements=true&parseTime=true"
	db, err := sql.Open("mysql", mdsn)
	if err != nil {
		fmt.Println("integration: open db:", err)
		os.Exit(1)
	}
	// Apply migrations once via a throwaway testing.T shim.
	t := &testing.T{}
	_ = t
	entries, err := os.ReadDir("../../migrations")
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
	var tableCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&tableCount); err != nil {
		fmt.Println("integration: count tables:", err)
		os.Exit(1)
	}
	if tableCount > 0 {
		fmt.Println("integration: reusing migrated test db")
	} else {
		for _, name := range ups {
			raw, err := os.ReadFile("../../migrations/" + name)
			if err != nil {
				fmt.Println("integration: read", name, err)
				os.Exit(1)
			}
			if _, err := db.Exec(string(raw)); err != nil {
				fmt.Println("integration: apply", name, err)
				os.Exit(1)
			}
		}
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	auditMod := audit.NewModule(db, log)
	userMod := user.NewModule(db, auditMod.Audit())
	branchMod := branch.NewModule(db, userMod.Users(), auditMod.Audit())
	companyMod := company.NewModule(db, auditMod.Audit())
	mediaMod, err := media.NewModule(db, config.Storage{Driver: "local"}, os.TempDir(), auditMod.Audit())
	if err != nil {
		fmt.Println("integration: media:", err)
		os.Exit(1)
	}
	productMod := product.NewModule(db, auditMod.Audit())
	partyMod := party.NewModule(db, productMod.Products(), auditMod.Audit())
	stockMod := stock.NewModule(db, productMod.Products(), branchMod.Branches(),
		userMod.Users(), userMod.Roles(), companyMod.Company(), auditMod.Audit())
	purchasingMod := purchasing.NewModule(db, partyMod.Parties(), productMod.Products(), branchMod.Branches(), auditMod.Audit())
	salesMod := sales.NewModule(db, partyMod.Parties(), productMod.Products(), partyMod.Pricing(), branchMod.Branches(), auditMod.Audit())
	goodsreceiptMod := goodsreceipt.NewModule(db, purchasingMod.PurchaseOrders(), productMod.Products(), stockMod.Stock(), auditMod.Audit())
	deliveryMod := delivery.NewModule(db, salesMod.Service(), productMod.Products(), stockMod.Stock(), branchMod.Branches(), mediaMod.Media(), auditMod.Audit())
	salesreturnMod := salesreturn.NewModule(db, salesMod.SalesOrders(), deliveryMod.Deliveries(), productMod.Products(), stockMod.Stock(), branchMod.Branches(), auditMod.Audit())
	purchasereturnMod := purchasereturn.NewModule(db, purchasingMod.PurchaseOrders(), goodsreceiptMod.GoodsReceipts(), productMod.Products(), stockMod.Stock(), branchMod.Branches(), auditMod.Audit())
	paymentMod := payment.NewModule(db, partyMod.Parties(), purchasingMod.PurchaseOrders(), salesMod.SalesOrders(), salesreturnMod.Returns(), purchasereturnMod.Returns(), branchMod.Branches(), mediaMod.Media(), auditMod.Audit())
	salesreturnMod.SetPayments(paymentMod.Payments())
	purchasereturnMod.SetPayments(paymentMod.Payments())
	financeMod := finance.NewModule(db, branchMod.Branches(), productMod.Products(), partyMod.Parties(),
		purchasingMod.PurchaseOrders(), salesMod.SalesOrders(),
		salesreturnMod.SalesReturns(), purchasereturnMod.PurchaseReturns(),
		goodsreceiptMod.GoodsReceipts(), deliveryMod.Deliveries(),
		paymentMod.Payments(), stockMod.Stock(), auditMod.Audit())
	financeMod.SetOps(salesMod.Service(), purchasingMod.Service())

	testMods = &mods{db: db, audit: auditMod, user: userMod, branch: branchMod,
		company: companyMod, media: mediaMod, product: productMod, party: partyMod,
		stock: stockMod, purchasing: purchasingMod, sales: salesMod,
		goodsreceipt: goodsreceiptMod, delivery: deliveryMod, salesreturn: salesreturnMod,
		purchasereturn: purchasereturnMod, payment: paymentMod, finance: financeMod}

	code := m.Run()
	_ = db.Close()
	os.Exit(code)
}

// freshDB truncates every table so each test starts empty.
func freshDB(t *testing.T) {
	t.Helper()
	_, dbName := splitDSN(testDBDSN())
	truncateAll(t, testMods.db, dbName)
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
		Name: "Customer Uji",
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
