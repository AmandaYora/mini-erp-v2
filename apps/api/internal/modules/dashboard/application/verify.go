package application

import "mini-erp/internal/shared/wiring"

// Verify reports late-bound dependencies the composition root did not set
// (called through the module's Verify by bootstrap.Wire).
func (s *Service) Verify() error {
	return wiring.Check("dashboard",
		wiring.Dep{Name: "returns", Set: s.returns != nil},
		wiring.Dep{Name: "payments", Set: s.payments != nil})
}

// Verify reports data-health sources the composition root did not pass.
func (h *Health) Verify() error {
	return wiring.Check("dashboard health",
		wiring.Dep{Name: "users", Set: h.src.Users != nil},
		wiring.Dep{Name: "userNames", Set: h.src.UserNames != nil},
		wiring.Dep{Name: "finance", Set: h.src.Finance != nil},
		wiring.Dep{Name: "stock", Set: h.src.Stock != nil},
		wiring.Dep{Name: "products", Set: h.src.Products != nil},
		wiring.Dep{Name: "parties", Set: h.src.Parties != nil},
		wiring.Dep{Name: "delivery", Set: h.src.Delivery != nil},
		wiring.Dep{Name: "sales", Set: h.src.Sales != nil},
		wiring.Dep{Name: "payments", Set: h.src.Payments != nil},
		wiring.Dep{Name: "integrity", Set: h.src.Integrity != nil},
		wiring.Dep{Name: "audit", Set: h.audit != nil},
		wiring.Dep{Name: "store", Set: h.store != nil})
}
