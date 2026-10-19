package bootstrap

import (
	"log/slog"
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
)

// RegisterRoutes mounts every module's HTTP routes the way cmd/server does.
// It exists so TestRouteGuards (internal/integration) exercises the real
// route table. cmd/server/main.go keeps its own explicit list; the test
// TestRouteTableMatchesMain fails when the two drift apart, so add a module
// in BOTH places.
func RegisterRoutes(m *Modules, mux *http.ServeMux, log *slog.Logger) {
	m.Auth.RegisterRoutes(mux)
	perm := m.Auth.Permission
	branchGuard := authcontracts.RequireBranch
	authn := func(next func(http.ResponseWriter, *http.Request)) http.Handler {
		return authcontracts.Authenticate(log, m.Auth.Sessions(), http.HandlerFunc(next))
	}
	m.User.RegisterRoutes(mux, perm)
	m.Branch.RegisterRoutes(mux, perm, authn)
	m.Company.RegisterRoutes(mux, perm, authn)
	m.Product.RegisterRoutes(mux, m.Media.Media(), perm)
	m.Party.RegisterRoutes(mux, perm)
	m.Stock.RegisterRoutes(mux, perm)
	m.Purchasing.RegisterRoutes(mux, perm, branchGuard)
	m.Sales.RegisterRoutes(mux, perm, branchGuard)
	m.GoodsReceipt.RegisterRoutes(mux, perm, branchGuard)
	m.Delivery.RegisterRoutes(mux, m.Media.Media(), perm, branchGuard)
	m.SalesReturn.RegisterRoutes(mux, perm, branchGuard)
	m.PurchaseReturn.RegisterRoutes(mux, perm, branchGuard)
	m.Payment.RegisterRoutes(mux, perm, branchGuard)
	m.Finance.RegisterRoutes(mux, perm, branchGuard)
	m.Audit.RegisterRoutes(mux, perm, branchGuard)
	m.Dashboard.RegisterRoutes(mux, perm, branchGuard)
	m.Reporting.RegisterRoutes(mux, perm, branchGuard)
	m.Assistant.RegisterRoutes(mux, perm, branchGuard)
	m.RegisterOps(mux, perm, branchGuard)
}
