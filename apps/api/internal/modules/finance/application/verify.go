package application

import "mini-erp/internal/shared/wiring"

// Verify reports late-bound dependencies the composition root did not set
// (called through the module's Verify by bootstrap.Wire).
func (s *Service) Verify() error {
	return wiring.Check("finance",
		wiring.Dep{Name: "salesOps", Set: s.salesOps != nil},
		wiring.Dep{Name: "purchOps", Set: s.purchOps != nil})
}
