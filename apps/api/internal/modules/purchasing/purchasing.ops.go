package purchasing

import (
	"mini-erp/internal/modules/purchasing/contracts"
)

// PurchasingOps exposes the grouped operational reads (status counts) to the
// dashboard. The dashboard depends on the narrow
// contracts.PurchasingOpsClient, never on application internals (Rule A).
func (m *Module) PurchasingOps() contracts.PurchasingOpsClient {
	return m.svc
}
