package presentation

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"mini-erp/internal/modules/audit/application"
	"mini-erp/internal/modules/audit/contracts"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
	"mini-erp/internal/shared/timeutil"
	"mini-erp/internal/shared/wiring"
)

// NameResolver turns ids into display names (batch). Injected by the
// composition root so audit never imports user/branch internals.
type NameResolver func(ctx context.Context, ids []int64) map[int64]string

// Handler serves the audit module HTTP surface (read-only; writes arrive via
// the AuditClient from every mutating module).
type Handler struct {
	svc      *application.Service
	users    NameResolver
	branches NameResolver
}

// NewHandler wires audit endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

// SetResolvers installs actor/branch name lookups (optional: names are
// omitted when unset).
func (h *Handler) SetResolvers(users, branches NameResolver) {
	h.users, h.branches = users, branches
}

func rawJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

func recordView(rec contracts.Record, actors, branches map[int64]string) map[string]any {
	actorType := rec.ActorType
	if actorType == "" {
		actorType = "user"
	}
	v := map[string]any{
		"id": rec.ID, "action": rec.Action, "entity": rec.Entity,
		"entityId": rec.EntityID, "branchId": rec.BranchID, "actorId": rec.ActorID,
		"actorType": actorType,
		"note":      rec.Note, "createdAt": timeutil.FormatWIB(rec.CreatedAt),
		"before": rawJSON(rec.Before), "after": rawJSON(rec.After), "metadata": rawJSON(rec.Metadata),
	}
	if n, ok := actors[rec.ActorID]; ok {
		v["actorName"] = n
	}
	if n, ok := branches[rec.BranchID]; ok {
		v["branchName"] = n
	}
	return v
}

func distinct(recs []contracts.Record, pick func(contracts.Record) int64) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, r := range recs {
		if id := pick(r); id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ListLogs handles GET /api/v1/audit-logs.
//
// Filters: branchId (id or "all": any audit.view holder may look across
// branches, as in the legacy system; absent = the session branch plus global
// events), entity, entityId, action (prefix), actorId, from, to (WIB days).
func (h *Handler) ListLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	f := contracts.ListFilter{
		BranchID: authcontracts.BranchID(r), Action: strings.TrimSpace(q.Get("action")),
		Entity: strings.TrimSpace(q.Get("entity")), From: q.Get("from"), To: q.Get("to"),
		Page: page, Limit: limit,
	}
	if raw := strings.TrimSpace(q.Get("branchId")); raw != "" {
		if strings.EqualFold(raw, "all") {
			f.AllBranches = true
		} else if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			f.BranchID = id
		}
	}
	if id, err := strconv.ParseInt(q.Get("actorId"), 10, 64); err == nil && id > 0 {
		f.ActorID = id
	}
	if id, err := strconv.ParseInt(q.Get("entityId"), 10, 64); err == nil && id > 0 {
		f.EntityID = id
	}
	res, total, err := h.svc.Query(r.Context(), f)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	var actors, branches map[int64]string
	if h.users != nil {
		actors = h.users(r.Context(), distinct(res, func(r contracts.Record) int64 { return r.ActorID }))
	}
	if h.branches != nil {
		branches = h.branches(r.Context(), distinct(res, func(r contracts.Record) int64 { return r.BranchID }))
	}
	items := make([]any, 0, len(res))
	for _, rec := range res {
		items = append(items, recordView(rec, actors, branches))
	}
	response.Paginated(w, "Data audit", items, response.Meta{
		Page: page, Limit: limit, Total: total,
		TotalPages: pagination.TotalPages(total, limit),
	})
}

// RegisterRoutes mounts audit endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/audit-logs", perm("audit.view", authcontracts.Guard(h.ListLogs, branchGuard)))
}

// Verify fails when the name resolvers were never installed.
func (h *Handler) Verify() error {
	return wiring.Check("audit",
		wiring.Dep{Name: "actor name resolver", Set: h.users != nil},
		wiring.Dep{Name: "branch name resolver", Set: h.branches != nil})
}
