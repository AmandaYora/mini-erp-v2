package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	"mini-erp/internal/modules/dashboard/domain"
	"mini-erp/internal/modules/dashboard/infrastructure"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/health"
	"mini-erp/internal/shared/integrity"
)

// --- data-health fakes ------------------------------------------------------

type fakeHealthSrc struct {
	mu        sync.Mutex
	seed      []usercontracts.SeedPasswordAccount
	seedReady bool
	cost      health.Result
	costErr   error
	opening   health.Result
	periods   []int
	negative  health.Result
	minPrice  health.Result
	dupProd   health.Result
	dupCust   health.Result
	unsigned  health.Result
	unshipped []salescontracts.OrderTotal
	paid      map[int64]int64
	issues    []integrity.Issue
	runs      atomic.Int64
}

func (f *fakeHealthSrc) SeedPasswordAccounts(context.Context) ([]usercontracts.SeedPasswordAccount, bool, error) {
	f.runs.Add(1)
	return f.seed, f.seedReady, nil
}
func (f *fakeHealthSrc) NamesByIDs(_ context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	for _, id := range ids {
		out[id] = "Rina"
	}
	return out, nil
}
func (f *fakeHealthSrc) CostAbovePrice(context.Context, int64, float64, int) (health.Result, error) {
	return f.cost, f.costErr
}
func (f *fakeHealthSrc) CostAbovePriceItems(context.Context, int64, float64, int, int) ([]health.Item, int64, error) {
	return []health.Item{{Key: "1"}}, 1, nil
}
func (f *fakeHealthSrc) OpeningUnverified(context.Context, int64, int) (health.Result, error) {
	return f.opening, nil
}
func (f *fakeHealthSrc) OpeningUnverifiedItems(context.Context, int64, int, int) ([]health.Item, int64, error) {
	return nil, 0, nil
}
func (f *fakeHealthSrc) UnclosedPastPeriods(context.Context, int64, int) ([]int, error) {
	return f.periods, nil
}
func (f *fakeHealthSrc) UnbalancedJournals(context.Context, int64, string, int) (health.Result, error) {
	return health.Result{}, nil
}
func (f *fakeHealthSrc) NegativeSummary(context.Context, int64, int) (health.Result, error) {
	return f.negative, nil
}
func (f *fakeHealthSrc) MinPriceAboveSelling(context.Context, int) (health.Result, error) {
	return f.minPrice, nil
}
func (f *fakeHealthSrc) DuplicateNames(context.Context, int) (health.Result, error) { return f.dupProd, nil }
func (f *fakeHealthSrc) UnsignedSummary(context.Context, int64, int) (health.Result, error) {
	return f.unsigned, nil
}
func (f *fakeHealthSrc) UnshippedOrderTotals(_ context.Context, _ int64, after int64, _ int) ([]salescontracts.OrderTotal, error) {
	if after > 0 {
		return nil, nil
	}
	return f.unshipped, nil
}
func (f *fakeHealthSrc) PaidForOrders(_ context.Context, _ string, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	for _, id := range ids {
		out[id] = f.paid[id]
	}
	return out, nil
}

type fakePartyHealth struct{ f *fakeHealthSrc }

func (p fakePartyHealth) DuplicateNames(context.Context, string, int) (health.Result, error) {
	return p.f.dupCust, nil
}

type fakeSnoozes struct {
	mu   sync.Mutex
	rows map[string]domain.Snooze
	next int64
}

func (s *fakeSnoozes) ListSnoozes(context.Context, int64) ([]domain.Snooze, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Snooze
	for _, r := range s.rows {
		out = append(out, r)
	}
	return out, nil
}

func (s *fakeSnoozes) UpsertSnooze(ctx context.Context, in domain.Snooze, actorID int64, now time.Time, hook infrastructure.TxHook) (*domain.Snooze, *domain.Snooze, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.rows[in.AlertID]
	in.UpdatedAt, in.UpdatedBy = now, actorID
	in.CreatedAt = now
	if had {
		in.ID, in.CreatedAt = prev.ID, prev.CreatedAt
	} else {
		s.next++
		in.ID = s.next
	}
	var previous *domain.Snooze
	if had {
		previous = &prev
	}
	if err := hook(ctx, nil, &in, previous); err != nil {
		return nil, nil, err
	}
	s.rows[in.AlertID] = in
	if had {
		return &in, &prev, nil
	}
	return &in, nil, nil
}

func (s *fakeSnoozes) DeleteSnooze(ctx context.Context, _ int64, alertID string, hook infrastructure.TxHook) (*domain.Snooze, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.rows[alertID]
	if !ok {
		return nil, nil
	}
	if err := hook(ctx, nil, &old, nil); err != nil {
		return nil, err
	}
	delete(s.rows, alertID)
	return &old, nil
}

type fakeAudit struct {
	mu      sync.Mutex
	entries []auditcontracts.Entry
}

func (a *fakeAudit) Log(context.Context, auditcontracts.Entry) error { return nil }
func (a *fakeAudit) LogTx(_ context.Context, _ auditcontracts.AuditTx, e auditcontracts.Entry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, e)
	return nil
}
func (a *fakeAudit) List(context.Context, int64, string, string, int, int) ([]auditcontracts.Record, int64, error) {
	return nil, 0, nil
}

// fixedNow is 2026-10-23 10:00 WIB.
var fixedNow = time.Date(2026, 10, 23, 3, 0, 0, 0, time.UTC)

func newHealthFixture(t *testing.T) (*Health, *fakeHealthSrc, *fakeSnoozes, *fakeAudit) {
	t.Helper()
	src := &fakeHealthSrc{seedReady: true, paid: map[int64]int64{}}
	st := &fakeStock{}
	svc := newService(&fakeFinance{}, st, &fakeSales{}, &fakePurchasing{})
	svc.SetSources(nil, fakePaid{})
	store := &fakeSnoozes{rows: map[string]domain.Snooze{}}
	audit := &fakeAudit{}
	h := NewHealth(svc, HealthSources{
		Users: src, UserNames: src, Finance: src, Stock: src, Products: src, Parties: fakePartyHealth{src},
		Delivery: src, Sales: src, Payments: src,
		Integrity: func(context.Context, int64) ([]integrity.Issue, error) { return src.issues, nil },
	}, store, audit, nil)
	clock := func() time.Time { return fixedNow }
	h.now, h.cache.now = clock, clock
	return h, src, store, audit
}

func ids(list []dashboardcontracts.Alert) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.ID)
	}
	return out
}

func find(list []dashboardcontracts.Alert, id string) *dashboardcontracts.Alert {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

var owner = Viewer{UserID: 7, RoleCode: "owner", Security: true}
var admin = Viewer{UserID: 8, RoleCode: "admin"}

// --- tests --------------------------------------------------------------------

// TestHealthCardOrderAndSeverity locks the priority rule (severity, keamanan
// first, rupiah desc with none last, count desc, id), the cut to limit, that
// info alerts stay off the card, and the severity thresholds by age.
func TestHealthCardOrderAndSeverity(t *testing.T) {
	h, src, _, _ := newHealthFixture(t)
	src.seed = []usercontracts.SeedPasswordAccount{{UserID: 1, Username: "superadmin"}}
	src.cost = health.Result{Count: 64, AmountRp: health.Amount(14_200_431_681)}
	src.opening = health.Result{Count: 2154, AmountRp: health.Amount(232_720_912_863)}
	src.periods = []int{202607, 202608, 202609}
	src.minPrice = health.Result{Count: 38}
	src.dupProd = health.Result{Count: 102}
	src.dupCust = health.Result{Count: 22}
	src.unsigned = health.Result{Count: 10, AmountRp: health.Amount(32_506_002), OldestAt: "2026-10-01"}
	src.unshipped = []salescontracts.OrderTotal{{ID: 1, Number: "PJ/08/00042", GrandTotal: 9_000_000, OrderDate: "2026-08-10"},
		{ID: 2, Number: "PJ/10/02478", GrandTotal: 10_000_000, OrderDate: "2026-10-22"}, {ID: 3, Number: "unpaid", OrderDate: "2026-08-01"}}
	src.paid = map[int64]int64{1: 23_327_000, 2: 6_300_000}

	sum, err := h.Alerts(context.Background(), 1, owner, 5)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{AlertSeedPassword, AlertCostAbovePrice, AlertOpeningUnverified, AlertDeliveryUnsigned, AlertPaidUnshipped}
	if got := ids(sum.Items); len(got) != 5 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] || got[4] != want[4] {
		t.Fatalf("card order = %v, want %v", got, want)
	}
	// critical: A1, A2; warning: A7, A11, A12, A8, A5 (+ A15: fake stock has 2 critical).
	if sum.BySeverity.Critical != 2 || sum.BySeverity.Info != 2 || sum.TotalActive != sum.BySeverity.Critical+sum.BySeverity.Warning {
		t.Fatalf("bySeverity = %+v totalActive = %d", sum.BySeverity, sum.TotalActive)
	}
	if sum.Pending != true {
		t.Fatalf("heavy tier never computed yet: pending must be true")
	}
	a12 := find(sum.Items, AlertPaidUnshipped)
	if a12.Count != 2 || *a12.AmountRp != 29_627_000 || *a12.OldestAt != "2026-08-10" || a12.Severity != dashboardcontracts.SeverityWarning {
		t.Fatalf("A12 = %+v", a12)
	}

	// A11/A12 fall to info when nothing is old enough; A5 is info while only
	// last month is open in the first half of this month.
	src.unsigned.OldestAt = "2026-10-21"
	src.periods = []int{202609}
	h.cache.refresh(1)
	page, err := h.DataHealth(context.Background(), 1, owner, false)
	if err != nil {
		t.Fatal(err)
	}
	var all []dashboardcontracts.Alert
	for _, a := range page.Areas {
		all = append(all, a.Items...)
	}
	if a := find(all, AlertDeliveryUnsigned); a == nil || a.Severity != dashboardcontracts.SeverityInfo {
		t.Fatalf("A11 confirmed 2 days ago must be info: %+v", a)
	}
	if a := find(all, AlertPeriodUnclosed); a == nil || a.Severity != dashboardcontracts.SeverityWarning {
		t.Fatalf("A5 on the 23rd stays warning: %+v", a)
	}
	if a := find(all, AlertSeedPassword); a.Action != nil || a.Hint == "" || a.Snooze.Snoozable {
		t.Fatalf("A1 for an owner: hint without button, never snoozable: %+v", a)
	}
}

// TestHealthSecurityFilteredForAdmin: without dashboard.alerts.security the
// keamanan area does not exist — not in the card, the page, items or snooze.
func TestHealthSecurityFilteredForAdmin(t *testing.T) {
	h, src, _, _ := newHealthFixture(t)
	src.seed = []usercontracts.SeedPasswordAccount{{UserID: 1, Username: "superadmin"}}
	sum, err := h.Alerts(context.Background(), 1, admin, 5)
	if err != nil {
		t.Fatal(err)
	}
	if find(sum.Items, AlertSeedPassword) != nil {
		t.Fatal("admin must not see A1")
	}
	page, _ := h.DataHealth(context.Background(), 1, admin, false)
	for _, a := range page.Areas {
		if a.Area == dashboardcontracts.AreaSecurity {
			t.Fatal("admin must not get the keamanan area")
		}
	}
	for _, c := range page.Checks {
		if c.ID == AlertSeedPassword {
			t.Fatal("admin must not see the A1 check")
		}
	}
	var ae *apperror.AppError
	if _, _, err := h.Items(context.Background(), 1, admin, AlertSeedPassword, 1, 20); !errors.As(err, &ae) || ae.Code != apperror.CodeNotFound {
		t.Fatalf("items A1 for admin = %v, want 404", err)
	}
	if _, err := h.Snooze(context.Background(), 1, admin, AlertSeedPassword, SnoozeInput{Days: 1, Reason: "nanti saja"}); !errors.As(err, &ae) || ae.Code != apperror.CodeNotFound {
		t.Fatalf("snooze A1 for admin = %v, want 404", err)
	}
	// The superadmin itself gets the button to its own account.
	page, _ = h.DataHealth(context.Background(), 1, Viewer{UserID: 1, RoleCode: "superadmin", Security: true}, false)
	for _, a := range page.Areas {
		if a.Area == dashboardcontracts.AreaSecurity {
			if len(a.Items) != 1 || a.Items[0].Action == nil || a.Items[0].Action.To != "/users?search=superadmin" {
				t.Fatalf("superadmin A1 action = %+v", a.Items)
			}
		}
	}
}

// TestHealthSnoozeRules: validation, critical cap, keamanan refusal, resolved
// alerts, hiding while active, breaking through when the problem grows, and
// the audit rows of snooze/unsnooze.
func TestHealthSnoozeRules(t *testing.T) {
	h, src, store, audit := newHealthFixture(t)
	src.seed = []usercontracts.SeedPasswordAccount{{UserID: 1, Username: "superadmin"}}
	src.cost = health.Result{Count: 64, AmountRp: health.Amount(1000)}
	src.dupProd = health.Result{Count: 3}
	ctx := context.Background()
	var ae *apperror.AppError

	if _, err := h.Snooze(ctx, 1, owner, AlertSeedPassword, SnoozeInput{Days: 1, Reason: "nanti saja"}); !errors.As(err, &ae) || ae.Code != apperror.CodeConflict {
		t.Fatalf("keamanan snooze = %v, want conflict", err)
	}
	if _, err := h.Snooze(ctx, 1, owner, AlertCostAbovePrice, SnoozeInput{Days: 14, Reason: "menunggu akuntan"}); !errors.As(err, &ae) || ae.Fields[0].Field != "days" {
		t.Fatalf("critical 14 days = %v, want days field error", err)
	}
	if _, err := h.Snooze(ctx, 1, owner, AlertCostAbovePrice, SnoozeInput{Days: 5, Reason: "menunggu akuntan"}); !errors.As(err, &ae) || ae.Fields[0].Message != "pilih 1, 3, 7, atau 14 hari" {
		t.Fatalf("5 days = %v", err)
	}
	if _, err := h.Snooze(ctx, 1, owner, AlertCostAbovePrice, SnoozeInput{Days: 7, Reason: " ok "}); !errors.As(err, &ae) || ae.Fields[0].Message != "Alasan minimal 5 karakter" {
		t.Fatalf("short reason = %v", err)
	}
	if _, err := h.Snooze(ctx, 1, owner, AlertNegativeStock, SnoozeInput{Days: 1, Reason: "tidak ada masalah"}); !errors.As(err, &ae) || ae.Code != apperror.CodeConflict {
		t.Fatalf("resolved alert = %v, want conflict", err)
	}
	if _, err := h.Snooze(ctx, 1, owner, "no.such", SnoozeInput{Days: 1, Reason: "tidak ada masalah"}); !errors.As(err, &ae) || ae.Message != "Alert tidak dikenal" {
		t.Fatalf("unknown alert = %v", err)
	}

	got, err := h.Snooze(ctx, 1, owner, AlertDuplicateProduct, SnoozeInput{Days: 14, Reason: "Sedang dikerjakan"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Snooze.CountAtSnooze == nil || *got.Snooze.CountAtSnooze != 3 || got.Snooze.ByName == nil || *got.Snooze.ByName != "Rina" {
		t.Fatalf("snoozed alert = %+v", got.Snooze)
	}
	page, _ := h.DataHealth(ctx, 1, owner, false)
	if len(page.Snoozed) != 1 || page.Snoozed[0].ID != AlertDuplicateProduct {
		t.Fatalf("snoozed = %v", ids(page.Snoozed))
	}
	// Re-snooze replaces the row; the audit says so.
	if _, err := h.Snooze(ctx, 1, owner, AlertDuplicateProduct, SnoozeInput{Days: 3, Reason: "Menunggu pemasok"}); err != nil {
		t.Fatal(err)
	}
	if len(store.rows) != 1 || store.rows[AlertDuplicateProduct].Days != 3 {
		t.Fatalf("rows = %+v", store.rows)
	}

	// The problem grows: the snooze lapses and the alert is back, flagged.
	src.dupProd = health.Result{Count: 4}
	h.cache.refresh(1)
	page, _ = h.DataHealth(ctx, 1, owner, false)
	if len(page.Snoozed) != 0 {
		t.Fatal("a grown problem must break through its snooze")
	}
	for _, a := range page.Areas {
		if x := find(a.Items, AlertDuplicateProduct); x != nil && !x.Snooze.Reactivated {
			t.Fatalf("reactivated flag missing: %+v", x.Snooze)
		}
	}

	existed, err := h.Unsnooze(ctx, 1, owner, AlertDuplicateProduct)
	if err != nil || !existed {
		t.Fatalf("unsnooze = %v %v", existed, err)
	}
	existed, err = h.Unsnooze(ctx, 1, owner, AlertDuplicateProduct)
	if err != nil || existed {
		t.Fatalf("second unsnooze must be an idempotent no-op: %v %v", existed, err)
	}
	if len(audit.entries) != 3 || audit.entries[0].Action != "data_health_snoozes.create" ||
		audit.entries[2].Action != "data_health_snoozes.delete" {
		t.Fatalf("audit = %+v", audit.entries)
	}
	if m := audit.entries[1].Metadata.(map[string]any); m["replaced"] != true {
		t.Fatalf("re-snooze audit must say replaced: %+v", m)
	}
}

// TestHealthCacheSingleFlightAndStale: N concurrent readers trigger one
// computation; a stale entry is served while one refresh runs; "Periksa
// ulang" is rate-limited per branch.
func TestHealthCacheSingleFlightAndStale(t *testing.T) {
	now := fixedNow
	var mu sync.Mutex
	c := newHealthCache(func() time.Time { mu.Lock(); defer mu.Unlock(); return now })
	var computes atomic.Int64
	release := make(chan struct{})
	compute := func(context.Context) tierResult {
		computes.Add(1)
		<-release
		return tierResult{outcomes: map[string]checkOutcome{"x": {status: dashboardcontracts.CheckOK}}, computedAt: now}
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.get(context.Background(), 1, TierFast, true, compute)
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if computes.Load() != 1 {
		t.Fatalf("computations = %d, want 1", computes.Load())
	}
	// Two minutes later (stale, < 3× TTL): served at once, one refresh starts.
	mu.Lock()
	now = now.Add(2 * time.Minute)
	mu.Unlock()
	if _, ok := c.get(context.Background(), 1, TierFast, true, compute); !ok {
		t.Fatal("stale entry must be served")
	}
	for i := 0; i < 100 && computes.Load() < 2; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	if computes.Load() != 2 {
		t.Fatalf("stale read must start exactly one refresh, computations = %d", computes.Load())
	}
	// Heavy tier is never waited for.
	if _, ok := c.get(context.Background(), 1, TierHeavy, false, func(context.Context) tierResult {
		time.Sleep(20 * time.Millisecond)
		return tierResult{computedAt: now}
	}); ok {
		t.Fatal("a missing heavy tier must not block the request")
	}
	if !c.refresh(1) || c.refresh(1) {
		t.Fatal("refresh: first allowed, second within 30 s refused")
	}
}

// TestHealthCheckErrorIsolated: one failing check is "error" with a safe
// message; the others still answer.
func TestHealthCheckErrorIsolated(t *testing.T) {
	h, src, _, _ := newHealthFixture(t)
	src.costErr = errors.New("Error 1146: table missing")
	src.minPrice = health.Result{Count: 2}
	page, err := h.DataHealth(context.Background(), 1, owner, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.FailedChecks) != 1 || page.FailedChecks[0].ID != AlertCostAbovePrice || page.FailedChecks[0].Message != failedMessage {
		t.Fatalf("failedChecks = %+v", page.FailedChecks)
	}
	found := false
	for _, a := range page.Areas {
		if find(a.Items, AlertMinPrice) != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("a failing check must not hide the others")
	}
}

// TestHealthSeedPasswordPending: while the user module still compares hashes
// the check is pending and the alert is not shown.
func TestHealthSeedPasswordPending(t *testing.T) {
	h, src, _, _ := newHealthFixture(t)
	src.seedReady = false
	page, _ := h.DataHealth(context.Background(), 1, owner, false)
	for _, c := range page.Checks {
		if c.ID == AlertSeedPassword && c.Status != dashboardcontracts.CheckPending {
			t.Fatalf("A1 status = %s, want pending", c.Status)
		}
	}
	if !page.Pending {
		t.Fatal("page must report pending")
	}
}

func TestDomainSnoozeGrowth(t *testing.T) {
	amt := int64(1000)
	s := domain.Snooze{CountAt: 10, AmountAt: &amt, Until: fixedNow.Add(time.Hour)}
	if s.Grew(10, health.Amount(1100)) || !s.Grew(10, health.Amount(1101)) || !s.Grew(11, nil) || s.Grew(9, nil) {
		t.Fatal("growth: count up or rupiah > 110 %")
	}
	if !s.Active(fixedNow) || s.Active(fixedNow.Add(2*time.Hour)) {
		t.Fatal("active until snoozed_until")
	}
	if domain.MaxDays(dashboardcontracts.SeverityCritical) != 7 || domain.MaxDays(dashboardcontracts.SeverityInfo) != 14 {
		t.Fatal("max days")
	}
	if domain.ReasonProblem("") != "Alasan wajib diisi" || domain.ReasonProblem("abcde") != "" {
		t.Fatal("reason rules")
	}
}
