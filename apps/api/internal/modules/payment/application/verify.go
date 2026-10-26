package application

import "mini-erp/internal/shared/wiring"

// Verify reports late-bound dependencies the composition root did not set
// (called through the module's Verify by bootstrap.Wire).
func (s *Service) Verify() error {
	return wiring.Check("payment",
		wiring.Dep{Name: "reverser", Set: s.reverser != nil},
		wiring.Dep{Name: "banks", Set: s.banks != nil})
}
