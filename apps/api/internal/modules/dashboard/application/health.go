package application

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	"mini-erp/internal/modules/dashboard/domain"
	"mini-erp/internal/modules/dashboard/infrastructure"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/health"
	"mini-erp/internal/shared/integrity"
	"mini-erp/internal/shared/timeutil"
)

// IntegritySource is the composition root's in-between-state collector
// (GET /ops/integrity), handed in as a port: the dashboard imports no module
// beyond contracts/.
type IntegritySource func(ctx context.Context, branchID int64) ([]integrity.Issue, error)

// UserNames names snooze authors.
type UserNames interface {
	NamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
}

// HealthSources are the owning modules' read-only checks (ADR-0025). Each
// counts its own rows; the dashboard only composes.
type HealthSources struct {
	Users     usercontracts.HealthClient
	UserNames UserNames
	Finance   financecontracts.HealthClient
	Stock     stockcontracts.HealthClient
	Products  productcontracts.HealthClient
	Parties   partycontracts.HealthClient
	Delivery  deliverycontracts.HealthClient
	Sales     salescontracts.HealthClient
	Payments  Payments
	Integrity IntegritySource
}

// SnoozeStore keeps the per-branch alert snoozes.
type SnoozeStore interface {
	ListSnoozes(ctx context.Context, branchID int64) ([]domain.Snooze, error)
	UpsertSnooze(ctx context.Context, s domain.Snooze, actorID int64, now time.Time, hook infrastructure.TxHook) (stored, previous *domain.Snooze, err error)
	DeleteSnooze(ctx context.Context, branchID int64, alertID string, hook infrastructure.TxHook) (*domain.Snooze, error)
}

// Health composes the data-health alerts: runs the catalog's checks through
// the two-tier cache, applies the viewer's permissions and the branch's
// snoozes, and orders the result.
type Health struct {
	svc   *Service
	src   HealthSources
	store SnoozeStore
	audit auditcontracts.AuditClient
	log   *slog.Logger
	cache *healthCache
	defs  []checkDef
	now   func() time.Time
}

// NewHealth wires the data-health composer. svc supplies the operational
// widgets A14/A15 reuse (priority orders, critical stock).
func NewHealth(svc *Service, src HealthSources, store SnoozeStore, audit auditcontracts.AuditClient, log *slog.Logger) *Health {
	if log == nil {
		log = slog.Default()
	}
	return &Health{svc: svc, src: src, store: store, audit: audit, log: log,
		cache: newHealthCache(timeutil.NowUTC), defs: catalog(), now: timeutil.NowUTC}
}

// Viewer is the reading session as the data-health rules need it.
type Viewer struct {
	UserID   int64
	RoleCode string
	// Security is dashboard.alerts.security: the keamanan area is invisible
	// without it (filtered here, never merely hidden by the frontend).
	Security bool
}

func (v Viewer) internal() viewer {
	return viewer{userID: v.UserID, roleCode: v.RoleCode, security: v.Security}
}

// failedMessage is what a person sees for a check that could not run; the
// cause goes to the log only.
const failedMessage = "Pemeriksaan tidak dapat dijalankan, coba Periksa ulang"

// computeTier runs one tier's checks in sequence (to spare the database);
// a failing check is isolated as status "error".
func (h *Health) computeTier(ctx context.Context, branchID int64, tier string) tierResult {
	now := h.now()
	out := tierResult{outcomes: map[string]checkOutcome{}, computedAt: now}
	for _, d := range h.defs {
		if d.tier != tier {
			continue
		}
		start := h.now()
		res, ready, err := d.run(ctx, h, branchID, now)
		o := checkOutcome{result: res, computedAt: h.now()}
		switch {
		case err != nil:
			o.status, o.err = dashboardcontracts.CheckError, err
			h.log.Error("data-health check failed", "check", d.id, "branch", branchID, "error", err)
		case !ready:
			o.status = dashboardcontracts.CheckPending
			out.pending = true
		case res.Count > 0:
			o.status = dashboardcontracts.CheckAlert
		default:
			o.status = dashboardcontracts.CheckOK
		}
		h.log.Debug("data-health check", "check", d.id, "branch", branchID, "ms", h.now().Sub(start).Milliseconds())
		out.outcomes[d.id] = o
	}
	return out
}

// snapshot reads both tiers: the fast one is waited for (bounded), the heavy
// one never — before its first result its checks read "pending".
func (h *Health) snapshot(ctx context.Context, branchID int64) (map[string]checkOutcome, time.Time) {
	fast, okFast := h.cache.get(ctx, branchID, TierFast, true, func(c context.Context) tierResult {
		return h.computeTier(c, branchID, TierFast)
	})
	heavy, okHeavy := h.cache.get(ctx, branchID, TierHeavy, false, func(c context.Context) tierResult {
		return h.computeTier(c, branchID, TierHeavy)
	})
	out := map[string]checkOutcome{}
	for _, d := range h.defs {
		var tr tierResult
		ok := okFast
		if d.tier == TierHeavy {
			tr, ok = heavy, okHeavy
		} else {
			tr = fast
		}
		if o, found := tr.outcomes[d.id]; ok && found {
			out[d.id] = o
			continue
		}
		out[d.id] = checkOutcome{status: dashboardcontracts.CheckPending}
	}
	at := fast.computedAt
	if !okFast {
		at = h.now()
	}
	return out, at
}

// view is everything one request knows about the branch's alerts.
type view struct {
	active, snoozed []dashboardcontracts.Alert
	checks          []dashboardcontracts.CheckStatus
	failed          []dashboardcontracts.FailedCheck
	pending         bool
	computedAt      time.Time
}

func (h *Health) visible(d checkDef, v viewer) bool {
	return d.area != dashboardcontracts.AreaSecurity || v.security
}

// build composes the visible alerts with their snooze state.
func (h *Health) build(ctx context.Context, branchID int64, v viewer) (*view, error) {
	outcomes, computedAt := h.snapshot(ctx, branchID)
	snoozes, err := h.store.ListSnoozes(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byAlert := map[string]domain.Snooze{}
	var authors []int64
	for _, s := range snoozes {
		byAlert[s.AlertID] = s
		if s.UpdatedBy > 0 {
			authors = append(authors, s.UpdatedBy)
		}
	}
	names := map[int64]string{}
	if len(authors) > 0 && h.src.UserNames != nil {
		if names, err = h.src.UserNames.NamesByIDs(ctx, authors); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	now := h.now()
	out := &view{computedAt: computedAt}
	for _, d := range h.defs {
		if !h.visible(d, v) {
			continue
		}
		o := outcomes[d.id]
		at := ""
		if !o.computedAt.IsZero() {
			at = o.computedAt.UTC().Format(time.RFC3339)
		}
		out.checks = append(out.checks, dashboardcontracts.CheckStatus{ID: d.id, Label: d.label, Area: d.area,
			Status: o.status, Tier: d.tier, ComputedAt: at})
		switch o.status {
		case dashboardcontracts.CheckError:
			out.failed = append(out.failed, dashboardcontracts.FailedCheck{ID: d.id, Label: d.label, Message: failedMessage})
			continue
		case dashboardcontracts.CheckPending:
			out.pending = true
			continue
		case dashboardcontracts.CheckOK:
			continue
		}
		alert := h.alert(d, o, v, now)
		s, has := byAlert[d.id]
		if !has || !d.snoozable || d.area == dashboardcontracts.AreaSecurity {
			out.active = append(out.active, alert)
			continue
		}
		fillSnooze(&alert, s, names)
		switch {
		case s.Active(now) && !s.Grew(alert.Count, alert.AmountRp):
			out.snoozed = append(out.snoozed, alert)
		case s.Active(now):
			alert.Snooze.Reactivated = true
			out.active = append(out.active, alert)
		case now.Sub(s.Until) <= 7*24*time.Hour:
			// Recently expired: keep the trail so the row can say so once.
			out.active = append(out.active, alert)
		default:
			out.active = append(out.active, h.alert(d, o, v, now))
		}
	}
	sortAlerts(out.active)
	sortAlerts(out.snoozed)
	return out, nil
}

// alert turns one check outcome into the alert the viewer reads.
func (h *Health) alert(d checkDef, o checkOutcome, v viewer, now time.Time) dashboardcontracts.Alert {
	r := o.result
	sev := d.severity(r, now)
	a := dashboardcontracts.Alert{
		ID: d.id, Area: d.area, AreaLabel: areaLabels[d.area], Severity: sev, Title: d.title, Impact: d.impact,
		Count: r.Count, CountUnit: d.countUnit, AmountRp: r.AmountRp, AmountLabel: d.amountLabel,
		Meta: r.Meta, Sample: r.Sample, Tier: d.tier, ComputedAt: o.computedAt.UTC().Format(time.RFC3339),
		Snooze: dashboardcontracts.AlertSnooze{Snoozable: d.snoozable && d.area != dashboardcontracts.AreaSecurity},
	}
	if a.Meta == nil {
		a.Meta = map[string]int64{}
	}
	if a.Sample == nil {
		a.Sample = []health.Sample{}
	}
	if a.Snooze.Snoozable {
		a.Snooze.MaxDays = domain.MaxDays(sev)
	}
	if r.OldestAt != "" {
		oldest := r.OldestAt
		a.OldestAt = &oldest
	}
	a.Action, a.Hint = d.action(r, v)
	return a
}

func fillSnooze(a *dashboardcontracts.Alert, s domain.Snooze, names map[int64]string) {
	at := s.UpdatedAt.UTC().Format(time.RFC3339)
	until := s.Until.UTC().Format(time.RFC3339)
	reason := s.Reason
	count := s.CountAt
	a.Snooze.SnoozedAt, a.Snooze.SnoozedUntil, a.Snooze.Reason, a.Snooze.CountAtSnooze = &at, &until, &reason, &count
	a.Snooze.AmountAtSnooze = s.AmountAt
	if name, ok := names[s.UpdatedBy]; ok {
		a.Snooze.ByName = &name
	}
}

var severityRank = map[string]int{
	dashboardcontracts.SeverityCritical: 0, dashboardcontracts.SeverityWarning: 1, dashboardcontracts.SeverityInfo: 2,
}

// sortAlerts orders by severity, keamanan first, rupiah desc (none last),
// count desc, then id.
func sortAlerts(list []dashboardcontracts.Alert) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if severityRank[a.Severity] != severityRank[b.Severity] {
			return severityRank[a.Severity] < severityRank[b.Severity]
		}
		as, bs := a.Area == dashboardcontracts.AreaSecurity, b.Area == dashboardcontracts.AreaSecurity
		if as != bs {
			return as
		}
		if (a.AmountRp == nil) != (b.AmountRp == nil) {
			return a.AmountRp != nil
		}
		if a.AmountRp != nil && *a.AmountRp != *b.AmountRp {
			return *a.AmountRp > *b.AmountRp
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.ID < b.ID
	})
}

func countSeverity(list []dashboardcontracts.Alert) dashboardcontracts.BySeverity {
	var out dashboardcontracts.BySeverity
	for _, a := range list {
		switch a.Severity {
		case dashboardcontracts.SeverityCritical:
			out.Critical++
		case dashboardcontracts.SeverityWarning:
			out.Warning++
		default:
			out.Info++
		}
	}
	return out
}

// Alerts is the dashboard card: the top critical/warning alerts that are not
// snoozed, cut to limit (default 5, max 20). Info alerts only count in
// bySeverity.info (for the page link).
func (h *Health) Alerts(ctx context.Context, branchID int64, v Viewer, limit int) (*dashboardcontracts.AlertSummary, error) {
	limit = clampWidgetLimit(limit)
	vw, err := h.build(ctx, branchID, v.internal())
	if err != nil {
		return nil, err
	}
	items := []dashboardcontracts.Alert{}
	for _, a := range vw.active {
		if a.Severity != dashboardcontracts.SeverityInfo {
			items = append(items, a)
		}
	}
	total := len(items)
	if len(items) > limit {
		items = items[:limit]
	}
	failed := vw.failed
	if failed == nil {
		failed = []dashboardcontracts.FailedCheck{}
	}
	return &dashboardcontracts.AlertSummary{
		Items: items, TotalActive: total, BySeverity: countSeverity(vw.active), SnoozedCount: len(vw.snoozed),
		FailedChecks: failed, Pending: vw.pending, ComputedAt: vw.computedAt.UTC().Format(time.RFC3339),
	}, nil
}

// DataHealth is the full page: every visible alert per area (info included),
// the snoozed ones, and each check's status. refresh drops the branch's cached
// results first (rate-limited; a refused refresh is silent).
func (h *Health) DataHealth(ctx context.Context, branchID int64, v Viewer, refresh bool) (*dashboardcontracts.DataHealth, error) {
	if refresh {
		h.cache.refresh(branchID)
	}
	iv := v.internal()
	vw, err := h.build(ctx, branchID, iv)
	if err != nil {
		return nil, err
	}
	byArea := map[string][]dashboardcontracts.Alert{}
	for _, a := range vw.active {
		byArea[a.Area] = append(byArea[a.Area], a)
	}
	out := &dashboardcontracts.DataHealth{
		Areas: []dashboardcontracts.AreaAlerts{}, Snoozed: vw.snoozed, Checks: vw.checks, FailedChecks: vw.failed,
		BySeverity: countSeverity(vw.active), Pending: vw.pending, ComputedAt: vw.computedAt.UTC().Format(time.RFC3339),
	}
	for _, area := range areaOrder {
		if area == dashboardcontracts.AreaSecurity && !iv.security {
			continue
		}
		items := byArea[area]
		if items == nil {
			items = []dashboardcontracts.Alert{}
		}
		out.Areas = append(out.Areas, dashboardcontracts.AreaAlerts{Area: area, AreaLabel: areaLabels[area], Items: items})
	}
	if out.Snoozed == nil {
		out.Snoozed = []dashboardcontracts.Alert{}
	}
	if out.Checks == nil {
		out.Checks = []dashboardcontracts.CheckStatus{}
	}
	if out.FailedChecks == nil {
		out.FailedChecks = []dashboardcontracts.FailedCheck{}
	}
	return out, nil
}

// errUnknownAlert is the 404 for an id the viewer cannot see or that does not exist.
func errUnknownAlert() *apperror.AppError {
	return &apperror.AppError{Code: apperror.CodeNotFound, Message: "Alert tidak dikenal"}
}

// lookup finds a visible check by id.
func (h *Health) lookup(alertID string, v viewer) (checkDef, *apperror.AppError) {
	for _, d := range h.defs {
		if d.id == alertID {
			if !h.visible(d, v) {
				return checkDef{}, errUnknownAlert()
			}
			return d, nil
		}
	}
	return checkDef{}, errUnknownAlert()
}

// Items is the full, paginated list behind an alert (A2, A7: the alerts whose
// action is this page). Not cached: opened rarely, one at a time.
func (h *Health) Items(ctx context.Context, branchID int64, v Viewer, alertID string, page, limit int) ([]health.Item, int64, error) {
	d, aerr := h.lookup(alertID, v.internal())
	if aerr != nil {
		return nil, 0, aerr
	}
	if d.items == nil {
		return nil, 0, &apperror.AppError{Code: apperror.CodeNotFound, Message: "Alert ini tidak punya daftar rinci"}
	}
	items, total, err := d.items(ctx, h, branchID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []health.Item{}
	}
	return items, total, nil
}

// SnoozeInput is a snooze request.
type SnoozeInput struct {
	Days   int
	Reason string
}

// Snooze pauses an active alert for the whole branch, with a reason, for 1, 3,
// 7 or 14 days (critical: at most 7). The audit row co-commits (ADR-0011).
func (h *Health) Snooze(ctx context.Context, branchID int64, v Viewer, alertID string, in SnoozeInput) (*dashboardcontracts.Alert, error) {
	iv := v.internal()
	d, aerr := h.lookup(alertID, iv)
	if aerr != nil {
		return nil, aerr
	}
	if !d.snoozable || d.area == dashboardcontracts.AreaSecurity {
		return nil, apperror.Conflict("Alert keamanan tidak bisa ditunda")
	}
	outcomes, _ := h.snapshot(ctx, branchID)
	o := outcomes[d.id]
	switch o.status {
	case dashboardcontracts.CheckOK:
		return nil, apperror.Conflict("Alert ini sudah selesai, tidak perlu ditunda")
	case dashboardcontracts.CheckPending, dashboardcontracts.CheckError:
		return nil, apperror.Conflict("Pemeriksaan alert ini belum selesai, coba lagi sebentar")
	}
	now := h.now()
	current := h.alert(d, o, iv, now)
	var fields []apperror.FieldError
	switch {
	case !domain.ValidDays(in.Days):
		fields = append(fields, apperror.FieldError{Field: "days", Message: "pilih 1, 3, 7, atau 14 hari"})
	case in.Days > domain.MaxDays(current.Severity):
		fields = append(fields, apperror.FieldError{Field: "days", Message: "alert kritis paling lama ditunda 7 hari"})
	}
	if msg := domain.ReasonProblem(in.Reason); msg != "" {
		fields = append(fields, apperror.FieldError{Field: "reason", Message: msg})
	}
	if len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	reason := strings.TrimSpace(in.Reason)
	until := now.Add(time.Duration(in.Days) * 24 * time.Hour)
	snooze := domain.Snooze{BranchID: branchID, AlertID: d.id, Reason: reason, Days: in.Days, Until: until,
		CountAt: current.Count, AmountAt: current.AmountRp}
	stored, _, err := h.store.UpsertSnooze(ctx, snooze, v.UserID, now, func(ctx context.Context, tx auditcontracts.AuditTx, row, previous *domain.Snooze) error {
		return h.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "data_health_snoozes.create", Entity: "data_health_snooze", EntityID: row.ID,
			BranchID: branchID, ActorID: v.UserID, Note: reason,
			Metadata: map[string]any{"alertId": d.id, "days": in.Days, "snoozedUntil": until.UTC().Format(time.RFC3339),
				"countAtSnooze": current.Count, "amountAtSnooze": current.AmountRp, "replaced": previous != nil},
		})
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	names := map[int64]string{}
	if h.src.UserNames != nil {
		if n, err := h.src.UserNames.NamesByIDs(ctx, []int64{v.UserID}); err == nil {
			names = n
		}
	}
	fillSnooze(&current, *stored, names)
	return &current, nil
}

// Unsnooze lifts a branch's snooze of an alert. Idempotent: without a snooze
// it reports existed=false and writes nothing.
func (h *Health) Unsnooze(ctx context.Context, branchID int64, v Viewer, alertID string) (existed bool, err error) {
	if _, aerr := h.lookup(alertID, v.internal()); aerr != nil {
		return false, aerr
	}
	old, err := h.store.DeleteSnooze(ctx, branchID, alertID, func(ctx context.Context, tx auditcontracts.AuditTx, row, _ *domain.Snooze) error {
		return h.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "data_health_snoozes.delete", Entity: "data_health_snooze", EntityID: row.ID,
			BranchID: branchID, ActorID: v.UserID, Note: row.Reason,
			Before: map[string]any{"alertId": row.AlertID, "reason": row.Reason, "days": row.Days,
				"snoozedUntil": row.Until.UTC().Format(time.RFC3339), "countAtSnooze": row.CountAt, "amountAtSnooze": row.AmountAt},
		})
	})
	if err != nil {
		var ae *apperror.AppError
		if errors.As(err, &ae) {
			return false, ae
		}
		return false, apperror.Internal(err)
	}
	return old != nil, nil
}
