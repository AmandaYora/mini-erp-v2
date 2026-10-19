package application

import "mini-erp/internal/shared/wiring"

// Verify reports late-bound dependencies the composition root did not set
// (called through the module's Verify by bootstrap.Wire).
func (s *Service) Verify() error {
	return wiring.Check("goodsreceipt",
		wiring.Dep{Name: "payments", Set: s.payments != nil})
}
