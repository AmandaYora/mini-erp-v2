package infrastructure

import "context"

type branchSetKey struct{}

// WithBranchSet makes the footing readers (accounts, days, products, parties)
// cover several branches at once: "all branches" reports. The caller has
// already narrowed the ids to what the user may see.
func WithBranchSet(ctx context.Context, ids []int64) context.Context {
	return context.WithValue(ctx, branchSetKey{}, ids)
}

// BranchSetFrom returns the branch set carried by ctx, if any.
func BranchSetFrom(ctx context.Context) ([]int64, bool) {
	v, ok := ctx.Value(branchSetKey{}).([]int64)
	return v, ok && len(v) > 0
}

// BranchCond is the SQL condition `col IN (...)` for the branch set a report
// runs over (the ctx set, else the single session branch), with its args.
func BranchCond(ctx context.Context, branchID int64, col string) (string, []any) {
	ids := branches(ctx, branchID)
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	cond := col + " IN (?"
	for i := 1; i < len(ids); i++ {
		cond += ",?"
	}
	return cond + ")", args
}

// branches is the set a reader runs over: the ctx set, else the single branch.
func branches(ctx context.Context, branchID int64) []int64 {
	if ids, ok := BranchSetFrom(ctx); ok {
		return ids
	}
	return []int64{branchID}
}
