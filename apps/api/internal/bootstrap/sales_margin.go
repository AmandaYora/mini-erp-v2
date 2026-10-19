package bootstrap

import (
	"context"

	financecontracts "mini-erp/internal/modules/finance/contracts"
)

// salesMargin feeds the sales order export summary with the books' gross
// margin. Finance owns the figure and already depends on sales, so the edge is
// closed here (sales declares the interface it needs, finance satisfies it).
type salesMargin struct {
	fin financecontracts.FinanceClient
}

// GrossMargin sums the per-product realised margin of a range (no cap: the
// summary needs the total, not a ranking).
func (m salesMargin) GrossMargin(ctx context.Context, branchID int64, from, to string) (int64, bool, error) {
	rows, err := m.fin.MarginByProduct(ctx, branchID, from, to, 1<<30)
	if err != nil {
		return 0, false, err
	}
	var gross int64
	incomplete := false
	for _, r := range rows {
		gross += r.Gross
		incomplete = incomplete || r.CostIncomplete
	}
	return gross, incomplete, nil
}
