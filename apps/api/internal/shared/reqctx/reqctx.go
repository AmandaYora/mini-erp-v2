package reqctx

import "context"

// Info carries per-request identity resolved by middleware. It is a mutable
// pointer so auth can fill UserID/BranchID after Verify without replacing the
// context value.
type Info struct {
	ID       string
	IP       string
	UserID   int64
	BranchID int64
}

type ctxKey struct{}

// WithInfo stores info in ctx.
func WithInfo(ctx context.Context, info *Info) context.Context {
	return context.WithValue(ctx, ctxKey{}, info)
}

// From returns the request info, or nil when outside HTTP middleware.
func From(ctx context.Context) *Info {
	if v, ok := ctx.Value(ctxKey{}).(*Info); ok {
		return v
	}
	return nil
}
