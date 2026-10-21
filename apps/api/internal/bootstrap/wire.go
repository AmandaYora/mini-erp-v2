package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"mini-erp/internal/config"
	"mini-erp/internal/modules/assistant"
	"mini-erp/internal/modules/audit"
	"mini-erp/internal/modules/auth"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/branch"
	"mini-erp/internal/modules/company"
	"mini-erp/internal/modules/dashboard"
	"mini-erp/internal/modules/delivery"
	"mini-erp/internal/modules/finance"
	"mini-erp/internal/modules/goodsreceipt"
	"mini-erp/internal/modules/media"
	"mini-erp/internal/modules/party"
	"mini-erp/internal/modules/payment"
	"mini-erp/internal/modules/product"
	"mini-erp/internal/modules/purchasereturn"
	"mini-erp/internal/modules/purchasing"
	"mini-erp/internal/modules/reporting"
	"mini-erp/internal/modules/sales"
	"mini-erp/internal/modules/salesreturn"
	"mini-erp/internal/modules/stock"
	"mini-erp/internal/modules/user"
	"mini-erp/internal/shared/signedurl"
)

// Options carries the environment-specific inputs of the composition root.
// The server and the integration harness differ only in these values, never
// in how the modules are connected.
type Options struct {
	Log        *slog.Logger
	JWTSecret  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Storage    config.Storage
	MediaDir   string
	Signer     *signedurl.Signer
	SessionDir string
}

// Modules is every constructed bounded context. Modules meet only through
// contracts/; Wire is the single place that sees all of them.
type Modules struct {
	// AuthzHub carries user/branch authorization announcements to auth
	// (built first, auth subscribes last).
	AuthzHub       *authcontracts.AuthzHub
	Audit          *audit.Module
	User           *user.Module
	Branch         *branch.Module
	Company        *company.Module
	Auth           *auth.Module
	Media          *media.Module
	Product        *product.Module
	Party          *party.Module
	Stock          *stock.Module
	Purchasing     *purchasing.Module
	Sales          *sales.Module
	GoodsReceipt   *goodsreceipt.Module
	Delivery       *delivery.Module
	SalesReturn    *salesreturn.Module
	PurchaseReturn *purchasereturn.Module
	Payment        *payment.Module
	Finance        *finance.Module
	Dashboard      *dashboard.Module
	Reporting      *reporting.Module
	Assistant      *assistant.Module
}

// Wire builds every module over db in dependency order and connects the
// late-bound edges that remain (see the list below). It starts no goroutine
// and registers no route: callers decide that (the server mounts routes and
// boots the WhatsApp gateway; tests do neither).
//
// Edges that are plain constructor arguments (finance ops/branch access,
// delivery settings, dashboard/reporting sources, user/branch -> auth hub)
// cannot be forgotten. The edges left as setters are genuine construction
// cycles (A needs B at build time and B needs A):
//
//	audit.NameResolvers      audit is first; user/branch need audit
//	product.StockState       stock needs product clients
//	stock.FinanceGate        finance needs stock clients
//	delivery/goodsreceipt/salesreturn/purchasereturn.Payments
//	                         payment needs the returns, returns need delivery/receipts
//	payment/salesreturn/purchasereturn.Reverser
//	                         finance needs payment and the returns
//	sales/purchasing.Dependents
//	                         delivery/receipts need the orders
//
// Wire finishes with Modules.Verify, so a forgotten setter fails here, at
// startup, instead of at the first request that needs the dependency.
func Wire(db *sql.DB, o Options) (*Modules, error) {
	m := &Modules{AuthzHub: authcontracts.NewAuthzHub()}
	// L1 identity package: auth ⇄ user ⇄ branch (+ company). Audit is built
	// first: every mutating module receives its client.
	m.Audit = audit.NewModule(db, o.Log)
	m.User = user.NewModule(db, m.Audit.Audit(), m.AuthzHub, m.AuthzHub)
	m.Branch = branch.NewModule(db, m.User.Users(), m.Audit.Audit(), m.AuthzHub)
	// Branch is built after user (it needs UserClient), so the admin user list
	// gets its branch-name lookup by setter (U-01).
	m.User.Service().SetBranchDirectory(m.Branch.Branches())
	m.Company = company.NewModule(db, m.Audit.Audit())
	m.Auth = auth.NewModule(db, m.User.Users(), m.User.Roles(), m.Branch.Branches(), m.Company.Company(),
		o.JWTSecret, o.AccessTTL, o.RefreshTTL, o.Log, m.Audit.Audit(), m.AuthzHub)
	// Auth resolves a session once and reuses it; user and branch announce
	// their writes through the hub (and revoke sessions through it), so a
	// revoked permission or closed branch applies on the very next request.
	// Auth subscribed itself to the hub inside auth.NewModule.

	var err error
	m.Media, err = media.NewModule(db, o.Storage, o.MediaDir, o.Signer, m.Audit.Audit())
	if err != nil {
		return nil, err
	}
	m.Product = product.NewModule(db, m.Audit.Audit())
	m.Party = party.NewModule(db, m.Product.Products(), m.Audit.Audit())
	m.Stock = stock.NewModule(db, m.Product.Products(), m.Branch.Branches(),
		m.User.Users(), m.User.Roles(), m.Company.Company(), m.Audit.Audit())
	m.Purchasing = purchasing.NewModule(db, m.Party.Parties(), m.Product.Products(), m.Branch.Branches(), m.Audit.Audit(), m.User.Users())
	m.Sales = sales.NewModule(db, m.Party.Parties(), m.Product.Products(), m.Party.Pricing(), m.Branch.Branches(), m.Audit.Audit(), m.User.Users())
	m.GoodsReceipt = goodsreceipt.NewModule(db, m.Purchasing.PurchaseOrders(), m.Product.Products(), m.Stock.Stock(), m.Branch.Branches(), m.Audit.Audit(), m.User.Users())
	m.Delivery = delivery.NewModule(db, m.Sales.Service(), m.Party.Parties(), m.Product.Products(), m.Stock.Stock(), m.Branch.Branches(), m.Media.Media(), m.Audit.Audit(), m.Company.Company())
	m.SalesReturn = salesreturn.NewModule(db, m.Sales.SalesOrders(), m.Delivery.Deliveries(), m.Product.Products(), m.Stock.Stock(), m.Branch.Branches(), m.Audit.Audit())
	m.PurchaseReturn = purchasereturn.NewModule(db, m.Purchasing.PurchaseOrders(), m.GoodsReceipt.GoodsReceipts(), m.Product.Products(), m.Stock.Stock(), m.Branch.Branches(), m.Audit.Audit())
	m.Payment = payment.NewModule(db, m.Party.Parties(), m.Purchasing.PurchaseOrders(), m.Sales.SalesOrders(),
		m.SalesReturn.Returns(), m.PurchaseReturn.Returns(), m.Branch.Branches(), m.Media.Media(), m.Audit.Audit())
	m.Delivery.SetPayments(m.Payment.Payments())
	m.GoodsReceipt.SetPayments(m.Payment.Payments())
	m.SalesReturn.SetPayments(m.Payment.Payments())
	m.PurchaseReturn.SetPayments(m.Payment.Payments())
	// Cancelling an order must see what hangs off it (shipments, receipts,
	// payments, returns). Those modules are built after the orders, so the
	// edge is closed here.
	m.Sales.SetDependents(m.Delivery.Deliveries(), m.Payment.Payments(), m.SalesReturn.SalesReturns())
	m.Purchasing.SetDependents(m.GoodsReceipt.GoodsReceipts(), m.Payment.Payments(), m.PurchaseReturn.PurchaseReturns())
	m.Finance = finance.NewModule(db, m.Branch.Branches(), m.Product.Products(), m.Party.Parties(),
		m.Purchasing.PurchaseOrders(), m.Sales.SalesOrders(),
		m.SalesReturn.SalesReturns(), m.PurchaseReturn.PurchaseReturns(),
		m.GoodsReceipt.GoodsReceipts(), m.Delivery.Deliveries(),
		m.Payment.Payments(), m.Stock.Stock(), m.Audit.Audit(),
		m.Sales.Service(), m.Purchasing.Service(), branchAccess(m.User), m.Media.Media())
	m.Stock.Service().SetFinanceGate(m.Finance.Finance())
	// The order export summary shows the books' margin (SL-04); finance depends on
	// sales, so sales only learns the source here.
	m.Sales.Service().SetMarginReader(salesMargin{fin: m.Finance.Finance()})
	// Product guards (UOM/type lock, archive, variant retirement) need stock
	// facts; stock is built after product, so the edge is closed here.
	m.Product.SetStockState(m.Stock.ProductStockState())
	// A cancelled payment/return takes its journal with it (F2b-5).
	m.Payment.SetReverser(m.Finance.Reverser())
	m.SalesReturn.SetReverser(m.Finance.Reverser())
	m.PurchaseReturn.SetReverser(m.Finance.Reverser())
	m.Purchasing.SetCorrectionGate(m.Finance.PurchaseCorrectionGate())
	// Replacement goods need a known cost (moving average, then catalog price).
	m.SalesReturn.SetCosts(m.Finance.Finance())
	// L8 readers: dashboard (finance figures + operational widgets) and
	// reporting (finance figures).
	m.Dashboard = dashboard.NewModule(m.Finance.Finance(), m.Stock.Stock(), m.Sales.Service(), m.Purchasing.Service(),
		m.SalesReturn.SalesReturns(), m.Payment.Payments())
	m.Reporting = reporting.NewModule(m.Finance.Finance(), m.Sales.Service(), m.Purchasing.Service(), m.Stock.Stock(), m.Payment.Payments())
	// Audit list decoration (F3b-8): actor and branch names resolved through
	// the user/branch contracts, never by joining their tables.
	m.Audit.SetNameResolvers(
		func(ctx context.Context, ids []int64) map[int64]string {
			out := map[int64]string{}
			for _, id := range ids {
				if u, err := m.User.Users().GetByID(ctx, id); err == nil && u != nil {
					out[id] = u.FullName
				}
			}
			return out
		},
		func(ctx context.Context, ids []int64) map[int64]string {
			out := map[int64]string{}
			if bs, err := m.Branch.Branches().GetByIDs(ctx, ids); err == nil {
				for _, b := range bs {
					out[b.ID] = b.Name
				}
			}
			return out
		})
	// L9 assistant: WhatsApp bot + tooling over provider contracts.
	m.Assistant = assistant.NewModule(db, o.SessionDir, m.Branch.Branches(), m.Product.Products(),
		m.Purchasing.PurchaseOrders(), m.Sales.SalesOrders(), m.Stock.Stock(),
		m.Finance.Finance(), o.Log, m.Audit.Audit())
	// Console/simulate runs are bounded to the operator's own branches (AS-01).
	m.Assistant.Service().SetUserAccess(m.User.Users())
	if err := m.Verify(); err != nil {
		return nil, err
	}
	return m, nil
}

// branchAccess adapts the user contract to the id list finance reports need.
func branchAccess(u *user.Module) func(ctx context.Context, userID int64) ([]int64, error) {
	return func(ctx context.Context, userID int64) ([]int64, error) {
		access, err := u.Users().GetBranchAccess(ctx, userID)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(access))
		for _, a := range access {
			ids = append(ids, a.BranchID)
		}
		return ids, nil
	}
}

// Verify asserts that every module exists and that every late-bound
// dependency was connected. It is cheap and side-effect free: Wire runs it
// (so the server and every test fail fast on a forgotten setter) and tests
// may call it again.
func (m *Modules) Verify() error {
	var errs []error
	if m.AuthzHub == nil {
		errs = append(errs, errors.New("authz hub missing"))
	} else if err := m.AuthzHub.Verify(); err != nil {
		errs = append(errs, err)
	}
	checks := []struct {
		name string
		nil_ bool
		fn   func() error
	}{
		{"audit", m.Audit == nil, func() error { return m.Audit.Verify() }},
		{"user", m.User == nil, nil},
		{"branch", m.Branch == nil, nil},
		{"company", m.Company == nil, nil},
		{"auth", m.Auth == nil, nil},
		{"media", m.Media == nil, nil},
		{"product", m.Product == nil, func() error { return m.Product.Verify() }},
		{"party", m.Party == nil, nil},
		{"stock", m.Stock == nil, func() error { return m.Stock.Verify() }},
		{"purchasing", m.Purchasing == nil, func() error { return m.Purchasing.Verify() }},
		{"sales", m.Sales == nil, func() error { return m.Sales.Verify() }},
		{"goodsreceipt", m.GoodsReceipt == nil, func() error { return m.GoodsReceipt.Verify() }},
		{"delivery", m.Delivery == nil, func() error { return m.Delivery.Verify() }},
		{"salesreturn", m.SalesReturn == nil, func() error { return m.SalesReturn.Verify() }},
		{"purchasereturn", m.PurchaseReturn == nil, func() error { return m.PurchaseReturn.Verify() }},
		{"payment", m.Payment == nil, func() error { return m.Payment.Verify() }},
		{"finance", m.Finance == nil, func() error { return m.Finance.Verify() }},
		{"dashboard", m.Dashboard == nil, func() error { return m.Dashboard.Verify() }},
		{"reporting", m.Reporting == nil, func() error { return m.Reporting.Verify() }},
		{"assistant", m.Assistant == nil, nil},
	}
	for _, c := range checks {
		if c.nil_ {
			errs = append(errs, errors.New(c.name+": module not constructed"))
			continue
		}
		if c.fn != nil {
			if err := c.fn(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
