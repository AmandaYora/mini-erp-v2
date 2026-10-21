package transform

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/shared/timeutil"
)

// IDEntry is one legacy -> revamp id pair (the id_map).
type IDEntry struct {
	Entity   string `json:"entity"`
	LegacyID int64  `json:"legacy_id"`
	Sub      int64  `json:"legacy_sub"`
	NewID    int64  `json:"new_id"`
}

// Mark is the posting state of a migrated source document (design D-ETL-5).
type Mark struct {
	DocType        string  `json:"doc_type"`
	DocID          int64   `json:"doc_id"`
	BranchID       int64   `json:"branch_id"`
	State          string  `json:"state"` // pending | posted_legacy | opening_covered | ignored
	Reason         string  `json:"reason,omitempty"`
	LegacyJournals []int64 `json:"legacy_journals,omitempty"`
}

// Cutover is one finance_cutover boundary.
type Cutover struct {
	BranchID int64  `json:"branch_id"`
	DocType  string `json:"doc_type"`
	Through  int64  `json:"through"`
}

// Output is everything the load phase needs.
type Output struct {
	// Tables are keyed by target table; rows carry explicit ids.
	Tables   map[string][]row.Row
	IDMap    []IDEntry
	Marks    []Mark
	Cutovers []Cutover
	Log      *report.Log
	// Explain holds the pre-computed row-count factors verify needs
	// (split items, synthetic documents, merged movements ...).
	Explain map[string]int64
}

// Input is the extracted legacy data.
type Input struct {
	Cfg *cfg.Config
	Raw map[string][]row.Row
}

// Target date formats.
const (
	dtFmt   = "2006-01-02 15:04:05.000000"
	dateFmt = "2006-01-02"
)

// fmtUTC renders a time as a target DATETIME(6) (UTC).
func fmtUTC(t time.Time) string { return t.UTC().Format(dtFmt) }

// fmtWIB renders a UTC instant as a WIB wall clock (naive DATETIME). Document
// dates the v2 runtime writes as WIB calendar dates (order/due/paid/return/
// settlement dates) take this form, so a migrated row reads like a native one
// and keeps the date the legacy screens showed (D-ETL-3b).
func fmtWIB(t time.Time) string { return t.In(timeutil.Jakarta).Format(dtFmt) }

// wibDate returns the Jakarta calendar date of a UTC instant (D-ETL-3).
func wibDate(t time.Time) string { return t.In(timeutil.Jakarta).Format(dateFmt) }

// naiveDateTime stores a calendar date as midnight, the way manual journals do.
func naiveDateTime(date string) string { return date + " 00:00:00.000000" }

// dateAtWIBMidnight converts a legacy DATE to the UTC instant of 00:00 WIB.
func dateAtWIBMidnight(date string) (time.Time, bool) {
	d, err := time.ParseInLocation(dateFmt, date, timeutil.Jakarta)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// state is the working context shared by the transform steps.
type state struct {
	in    Input
	raw   map[string][]row.Row
	log   *report.Log
	money *Rounder
	open  string // opening date, YYYY-MM-DD (WIB)
	out   *Output

	next map[string]int64
	maps map[string]map[int64]int64

	// lookups filled by the steps, used by later ones
	branchCode map[int64]string // new branch id -> code
	headBranch int64
	gdg        map[int64]int64 // new branch id -> new GDG location id
	rusak      map[int64]int64 // new branch id -> new RUSAK location id
	locBranch  map[int64]int64 // new location id -> new branch id
	productRow map[int64]row.Row
	defVariant map[int64]int64 // new product id -> default variant id
	prodType   map[int64]string
	orderKind  map[int64]string // legacy order id -> sales|purchase
	orderRow   map[int64]row.Row
	userSkip   map[int64]bool

	post           map[postKey]string
	postJ          map[postKey]int64
	journaled      map[[2]string]bool // legacy (source_type, id_source) with a live journal
	movesByRef     map[string][]row.Row
	directSJ       map[int64]int64 // legacy order id -> synthetic SJ id
	dnOfLegacy     map[int64]int64
	grOfLegacy     map[int64]int64
	transferOf     map[transferKey]int64
	transferFirst  map[int64]int64
	orphanTransfer map[int64]int64 // legacy inventory movement id -> synthetic transfer id (pindah-lokasi tanpa dokumen)
	deliveryNew    map[int64]int64
	proofs         []proofRef
	acct           map[int64]*acct
	journalNew     map[int64]int64
	orderLines     map[int64][]row.Row // legacy order items per legacy order
	migCorr        []migCorrection     // legacy errors the ETL corrects with a journal (ADR-0018 D11)
	revCand        []revCandidate      // order SJs whose revenue no journal will ever book (REVENUE_UNBOOKED)
	lastBuy        map[int64]*big.Rat  // product -> last purchase unit cost per base unit (lazy)

	// returns (returns.go)
	srNew     map[int64]int64 // legacy sales return -> new id
	prNew     map[int64]int64 // legacy purchase return -> new id
	replSJ    map[int64]int64 // legacy sales return -> new replacement SJ id (real or synthetic)
	srSynth   map[int64]*synthRepl
	srReplDN  map[int64]int64 // legacy sales return -> legacy replacement delivery note
	srItemLoc map[int64]int64 // legacy return item -> new location id
	srRepl    map[int64][]row.Row
	srMeta    map[int64]*retMeta
	retPays   []*retPay
}

func (s *state) alloc(entity string, legacy int64) int64 {
	return s.allocSub(entity, legacy, 0)
}

func (s *state) allocSub(entity string, legacy, sub int64) int64 {
	s.next[entity]++
	id := s.next[entity]
	if sub == 0 {
		if s.maps[entity] == nil {
			s.maps[entity] = map[int64]int64{}
		}
		s.maps[entity][legacy] = id
	}
	s.out.IDMap = append(s.out.IDMap, IDEntry{entity, legacy, sub, id})
	return id
}

// alias maps another legacy id to an already allocated id (many-to-one).
func (s *state) alias(entity string, legacy, id int64) {
	if s.maps[entity] == nil {
		s.maps[entity] = map[int64]int64{}
	}
	s.maps[entity][legacy] = id
	s.out.IDMap = append(s.out.IDMap, IDEntry{entity, legacy, 0, id})
}

func (s *state) id(entity string, legacy int64) (int64, bool) {
	v, ok := s.maps[entity][legacy]
	return v, ok
}

func (s *state) emit(table string, r row.Row) { s.out.Tables[table] = append(s.out.Tables[table], r) }

func (s *state) find(sev, code, entity string, id int64, format string, args ...any) {
	s.log.Find(sev, code, entity, id, fmt.Sprintf(format, args...))
}

// trunc cuts text to max runes and logs the loss.
func (s *state) trunc(entity string, id int64, field, v string, max int) string {
	if utf8.RuneCountInString(v) <= max {
		return v
	}
	cut := string([]rune(v)[:max])
	s.log.Adjust(entity, id, field, fmt.Sprintf("%d karakter", utf8.RuneCountInString(v)), fmt.Sprintf("%d karakter", max), "truncate")
	return cut
}

// optStr returns nil for NULL/empty text, else the (truncated) value.
func (s *state) optStr(entity string, id int64, field string, r row.Row, col string, max int) any {
	v, ok := r.NStr(col)
	if !ok || v == "" {
		return nil
	}
	return s.trunc(entity, id, field, v, max)
}

// ts renders a nullable legacy UTC timestamp column.
func ts(r row.Row, col string) any {
	t, ok := r.Time(col)
	if !ok {
		return nil
	}
	return fmtUTC(t)
}

func tsOr(r row.Row, col string, def time.Time) string {
	if t, ok := r.Time(col); ok {
		return fmtUTC(t)
	}
	return fmtUTC(def)
}

// timeOr reads a legacy UTC column, or def when it is empty.
func timeOr(r row.Row, col string, def time.Time) time.Time {
	if t, ok := r.Time(col); ok {
		return t
	}
	return def
}

// tsWIB renders a nullable legacy document date (written in UTC by the legacy
// app) as the target's WIB wall clock — see fmtWIB.
func tsWIB(r row.Row, col string) any {
	t, ok := r.Time(col)
	if !ok {
		return nil
	}
	return fmtWIB(t)
}

// tsLocalOr renders a legacy database-default timestamp (created_at /
// updated_at, stored as a WIB wall clock — row.LocalTime) as the target's UTC
// DATETIME, or def when the column is empty.
func tsLocalOr(r row.Row, col string, def time.Time) string {
	if t, ok := r.LocalTime(col); ok {
		return fmtUTC(t)
	}
	return fmtUTC(def)
}

func sortedBy(rows []row.Row, pk string) []row.Row {
	out := append([]row.Row(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Int(pk) < out[j].Int(pk) })
	return out
}

// epoch is the deterministic fallback timestamp for rows without one.
func (s *state) epoch() time.Time { return s.in.Cfg.OpeningTime() }

func lower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Run executes every transform step in dependency order. Blocker findings do
// not abort (the caller decides from Output.Log); only structural errors do.
func Run(in Input) (*Output, error) {
	s := &state{
		in: in, raw: in.Raw, open: in.Cfg.OpeningDate,
		log:  &report.Log{},
		next: map[string]int64{}, maps: map[string]map[int64]int64{},
		out: &Output{Tables: map[string][]row.Row{}, Explain: map[string]int64{}},
	}
	s.out.Log = s.log
	s.money = &Rounder{Log: s.log}
	s.branchCode = map[int64]string{}
	s.gdg, s.rusak, s.locBranch = map[int64]int64{}, map[int64]int64{}, map[int64]int64{}
	s.productRow, s.defVariant, s.prodType = map[int64]row.Row{}, map[int64]int64{}, map[int64]string{}
	s.orderKind, s.orderRow, s.userSkip = map[int64]string{}, map[int64]row.Row{}, map[int64]bool{}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"master", s.master},
		{"orders", s.orders},
		{"returns", s.returns},
		{"documents", s.documents},
		{"stock", s.stock},
		{"finance", s.finance},
		{"settings", func() error { s.settings(); return nil }},
		{"audit", func() error { s.auditLogs(); return nil }},
		{"media", s.media},
	}
	for _, st := range steps {
		if err := st.fn(); err != nil {
			return nil, fmt.Errorf("transform %s: %w", st.name, err)
		}
	}
	return s.out, nil
}
