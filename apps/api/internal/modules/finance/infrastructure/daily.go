package infrastructure

import (
	"context"
)

// DailyFooting is one account's debit/credit footing on one calendar day.
type DailyFooting struct {
	Date      string
	AccountID int64
	Debit     int64
	Credit    int64
}

// DailyFootings sums journal legs per (day, account) for a branch in one
// pass. Callers net the sides per account role — the repository does not know
// which account is revenue and which is COGS.
//
// One query for the whole range: the alternative (a NetProfit per day) is the
// N+1 this method exists to kill. DATE_FORMAT keeps the day a plain string so
// scanning never depends on the driver's parseTime setting.
func (r *Repository) DailyFootings(ctx context.Context, branchID int64, from, to string) ([]DailyFooting, error) {
	var out []DailyFooting
	for _, b := range branches(ctx, branchID) {
		one, err := r.dailyFootingsOne(ctx, b, from, to)
		if err != nil {
			return nil, err
		}
		out = append(out, one...)
	}
	return out, nil
}

func (r *Repository) dailyFootingsOne(ctx context.Context, branchID int64, from, to string) ([]DailyFooting, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DATE_FORMAT(entry_date, '%Y-%m-%d'), account_id, debit, credit
		 FROM finance_account_daily
		 WHERE branch_id = ? AND scope = 0 AND entry_date >= ? AND entry_date <= ?
		 ORDER BY entry_date ASC`,
		branchID, from[:min(10, len(from))], to[:min(10, len(to))])
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []DailyFooting
	for rows.Next() {
		var f DailyFooting
		if err := rows.Scan(&f.Date, &f.AccountID, &f.Debit, &f.Credit); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
