package sales

import (
	"mini-erp/internal/modules/sales/contracts"
)

// SalesOps exposes the grouped operational reads (status counts, top
// products) to the dashboard. The dashboard depends on the narrow
// contracts.SalesOpsClient, never on application internals (Rule A).
func (m *Module) SalesOps() contracts.SalesOpsClient {
	return m.svc
}
