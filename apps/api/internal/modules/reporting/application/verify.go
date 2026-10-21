package application

import "mini-erp/internal/shared/wiring"

// Verify reports late-bound dependencies the composition root did not set
// (called through the module's Verify by bootstrap.Wire).
func (s *Service) Verify() error {
	return wiring.Check("reporting",
		wiring.Dep{Name: "sales", Set: s.sales != nil},
		wiring.Dep{Name: "purchasing", Set: s.purchasing != nil},
		wiring.Dep{Name: "stock", Set: s.stock != nil},
		wiring.Dep{Name: "payments", Set: s.payments != nil})
}
