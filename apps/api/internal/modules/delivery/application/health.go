package application

import (
	"context"

	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/health"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
)

// UnsignedSummary is data-health alert A11 (delivery.unsigned, ADR-0025):
// confirmed regular notes without a recipient-signature status. AmountRp is
// the grand total of the distinct linked orders (one summaries read);
// OldestAt is the oldest confirmation day (WIB).
func (s *Service) UnsignedSummary(ctx context.Context, branchID int64, sample int) (health.Result, error) {
	rows, err := s.repo.UnsignedConfirmed(ctx, branchID)
	if err != nil {
		return health.Result{}, apperror.Internal(err)
	}
	res := health.Result{Count: int64(len(rows))}
	if len(rows) == 0 {
		return res, nil
	}
	seen := map[int64]bool{}
	var ids []int64
	for _, r := range rows {
		if r.SalesOrderID > 0 && !seen[r.SalesOrderID] {
			seen[r.SalesOrderID] = true
			ids = append(ids, r.SalesOrderID)
		}
	}
	orders, err := s.sales.SummariesByIDs(ctx, ids)
	if err != nil {
		return health.Result{}, apperror.Internal(err)
	}
	var total int64
	for _, id := range ids {
		if o := orders[id]; o != nil {
			total += o.GrandTotal
		}
	}
	res.AmountRp = health.Amount(total)
	if first := rows[0].ConfirmedAt; first.Valid {
		res.OldestAt = first.Time.In(timeutil.Jakarta).Format("2006-01-02")
	}
	for _, r := range rows[:min(sample, len(rows))] {
		value := "terkirim"
		if r.ConfirmedAt.Valid {
			value = "terkirim " + r.ConfirmedAt.Time.In(timeutil.Jakarta).Format("02-01-2006")
		}
		if o := orders[r.SalesOrderID]; o != nil {
			value += " · " + money.Rupiah(o.GrandTotal).String()
		}
		res.Sample = append(res.Sample, health.Sample{Label: r.Number, Value: value})
	}
	return res, nil
}
