package presentation

import (
	"context"
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/wiring"
)

// BranchAccessFunc returns the ids of the branches a user may read.
type BranchAccessFunc func(ctx context.Context, userID int64) ([]int64, error)

// SetBranchAccess wires the user's branch access (late-bound: user is built
// before finance, but the composition root closes the edge here).
func (h *Handler) SetBranchAccess(f BranchAccessFunc) { h.access = f }

// branchScope resolves the optional ?branch= parameter of a report:
// empty = the session branch; "all" = every branch the user may read; an id =
// that branch, if allowed. A branch outside the user's access is refused.
func (h *Handler) branchScope(r *http.Request) (context.Context, *apperror.AppError) {
	ctx := r.Context()
	raw := r.URL.Query().Get("branch")
	if raw == "" || raw == strconv.FormatInt(authcontracts.BranchID(r), 10) {
		return ctx, nil
	}
	sess := authcontracts.SessionFromContext(ctx)
	if sess == nil || h.access == nil {
		return ctx, apperror.Forbidden()
	}
	allowed, err := h.access(ctx, sess.UserID)
	if err != nil {
		return ctx, apperror.Internal(err)
	}
	if !sess.HasPermission("finance.report.all_branches") {
		return ctx, apperror.Forbidden()
	}
	if raw == "all" {
		if len(allowed) == 0 {
			return ctx, apperror.Forbidden()
		}
		return application.WithBranchScope(ctx, allowed), nil
	}
	id, perr := strconv.ParseInt(raw, 10, 64)
	if perr != nil || id <= 0 {
		return ctx, apperror.Validation("", []apperror.FieldError{{Field: "branch", Message: "harus all atau id cabang"}})
	}
	for _, a := range allowed {
		if a == id {
			return application.WithBranchScope(ctx, []int64{id}), nil
		}
	}
	return ctx, apperror.Forbidden()
}

// Verify fails when the branch-access reader was never supplied.
func (h *Handler) Verify() error {
	return wiring.Check("finance",
		wiring.Dep{Name: "branch access reader", Set: h.access != nil})
}
