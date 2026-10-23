package bootstrap

import (
	"context"
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/integrity"
	"mini-erp/internal/shared/response"
)

// IntegrityReport collects the in-between states of failed multi-step flows
// (draft shipments with stock out, pending receipts, returns whose stock effect
// disagrees with their status). Read-only: the operator repeats or undoes each
// action; nothing heals behind the user's back.
func (m *Modules) IntegrityReport(r *http.Request, branchID int64) ([]integrity.Issue, error) {
	return m.IntegrityIssues(r.Context(), branchID)
}

// IntegrityIssues is IntegrityReport for callers without a request: the
// data-health alert A13 reads it on its own timer (ADR-0025).
func (m *Modules) IntegrityIssues(ctx context.Context, branchID int64) ([]integrity.Issue, error) {
	out := []integrity.Issue{}
	for _, collect := range []func() ([]integrity.Issue, error){
		func() ([]integrity.Issue, error) { return m.Delivery.Service().IntegrityIssues(ctx, branchID) },
		func() ([]integrity.Issue, error) { return m.GoodsReceipt.Service().IntegrityIssues(ctx, branchID) },
		func() ([]integrity.Issue, error) { return m.SalesReturn.Service().IntegrityIssues(ctx, branchID) },
		func() ([]integrity.Issue, error) { return m.PurchaseReturn.Service().IntegrityIssues(ctx, branchID) },
	} {
		issues, err := collect()
		if err != nil {
			return nil, err
		}
		out = append(out, issues...)
	}
	return out, nil
}

// RegisterOps mounts the operator console endpoints.
func (m *Modules) RegisterOps(mux *http.ServeMux, perm func(string, func(http.ResponseWriter, *http.Request)) http.Handler, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/ops/integrity", perm("stock.update", func(w http.ResponseWriter, r *http.Request) {
		branchGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess := authcontracts.SessionFromContext(r.Context())
			if sess == nil {
				response.Fail(w, apperror.Forbidden())
				return
			}
			issues, err := m.IntegrityReport(r, sess.BranchID)
			if err != nil {
				response.FailErr(w, err)
				return
			}
			response.OK(w, "Laporan konsistensi", map[string]any{"issues": issues, "count": len(issues)})
		})).ServeHTTP(w, r)
	}))
}
