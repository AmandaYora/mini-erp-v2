package application

import "mini-erp/internal/shared/wiring"

// Verify reports late-bound dependencies the composition root did not set
// (called through the module's Verify by bootstrap.Wire).
func (s *Service) Verify() error {
	return wiring.Check("sales",
		wiring.Dep{Name: "dependents.deliveries", Set: s.dep.deliveries != nil},
		wiring.Dep{Name: "dependents.payments", Set: s.dep.payments != nil},
		wiring.Dep{Name: "dependents.returns", Set: s.dep.returns != nil},
		wiring.Dep{Name: "banks", Set: s.banks != nil})
}
