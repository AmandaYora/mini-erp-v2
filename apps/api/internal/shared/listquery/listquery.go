// Package listquery builds the WHERE clause shared by a list query and its
// COUNT, and parses/validates the common list filters (search, status CSV,
// date range, party, sort whitelist, pagination). It is a technical utility:
// no table or domain knowledge lives here, callers pass column names that are
// compile-time constants of their own repository (never request input).
package listquery

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/timeutil"
)

// MaxIn caps the number of values a single IN (...) may carry.
const MaxIn = 1000

// Where accumulates AND-ed conditions with their positional arguments. One
// Where feeds both List and Count so the two can never drift apart.
type Where struct {
	conds []string
	args  []any
}

// New starts an empty condition set.
func New() *Where { return &Where{} }

// Raw adds a verbatim condition (column names/placeholders only) with args.
func (w *Where) Raw(cond string, args ...any) *Where {
	if cond == "" {
		return w
	}
	w.conds = append(w.conds, cond)
	w.args = append(w.args, args...)
	return w
}

// Eq adds `col = ?` unless v is a zero value ("", 0, nil).
func (w *Where) Eq(col string, v any) *Where {
	if isZero(v) {
		return w
	}
	return w.Raw(col+" = ?", v)
}

// EqBool adds `col = 1|0` when set is true (tri-state filters: skip when unset).
func (w *Where) EqBool(col string, set bool, v bool) *Where {
	if !set {
		return w
	}
	n := 0
	if v {
		n = 1
	}
	return w.Raw(col+" = ?", n)
}

func isZero(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case int:
		return x == 0
	case int64:
		return x == 0
	case uint64:
		return x == 0
	}
	return false
}

// In adds `col IN (?,?,..)`; skipped when vals is empty. More than MaxIn
// values is truncated to the first MaxIn (callers that must reject use
// CheckInLimit first).
func (w *Where) In(col string, vals ...any) *Where {
	if len(vals) == 0 {
		return w
	}
	if len(vals) > MaxIn {
		vals = vals[:MaxIn]
	}
	return w.Raw(col+" IN ("+placeholders(len(vals))+")", vals...)
}

// InInt64 is In for id lists.
func (w *Where) InInt64(col string, ids []int64) *Where {
	vals := make([]any, len(ids))
	for i, id := range ids {
		vals[i] = id
	}
	return w.In(col, vals...)
}

// CheckInLimit returns a validation error when n exceeds MaxIn.
func CheckInLimit(field string, n int) error {
	if n > MaxIn {
		return apperror.Validation("", []apperror.FieldError{{Field: field, Message: "maksimal " + strconv.Itoa(MaxIn) + " nilai"}})
	}
	return nil
}

// StatusCSV adds `col IN (...)` for "confirmed,completed" (blank items and
// duplicates dropped; empty = no condition).
func (w *Where) StatusCSV(col, csv string) *Where {
	return w.InStrings(col, SplitCSV(csv))
}

// InStrings is In for string lists.
func (w *Where) InStrings(col string, vals []string) *Where {
	if len(vals) == 0 {
		return w
	}
	if len(vals) == 1 {
		return w.Raw(col+" = ?", vals[0])
	}
	args := make([]any, len(vals))
	for i, v := range vals {
		args[i] = v
	}
	return w.In(col, args...)
}

// SplitCSV splits "a, b,,a" into ["a","b"] preserving first-seen order.
func SplitCSV(csv string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ParseDate validates a YYYY-MM-DD calendar date.
func ParseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if !dateRe.MatchString(s) {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	return t, err == nil
}

// DateRange adds the half-open range `col >= from 00:00:00 AND col < to+1d`
// for columns that store the local calendar day (order_date style). Empty or
// malformed bounds are ignored here; the service layer reports them through
// Common.Validate so the user sees a 400 instead of a silently wider list.
func (w *Where) DateRange(col, from, to string) *Where {
	if t, ok := ParseDate(from); ok {
		w.Raw(col+" >= ?", t.Format("2006-01-02")+" 00:00:00")
	}
	if t, ok := ParseDate(to); ok {
		w.Raw(col+" < ?", t.AddDate(0, 0, 1).Format("2006-01-02")+" 00:00:00")
	}
	return w
}

// DateRangeWIB is DateRange for UTC timestamp columns: the WIB calendar day
// boundaries are converted to UTC instants.
func (w *Where) DateRangeWIB(col, from, to string) *Where {
	if t, ok := ParseDate(from); ok {
		w.Raw(col+" >= ?", wibMidnightUTC(t))
	}
	if t, ok := ParseDate(to); ok {
		w.Raw(col+" < ?", wibMidnightUTC(t.AddDate(0, 0, 1)))
	}
	return w
}

func wibMidnightUTC(day time.Time) string {
	local := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, timeutil.Jakarta)
	return local.UTC().Format("2006-01-02 15:04:05")
}

// EscapeLike escapes %, _ and \ so user text is matched literally.
func EscapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// Like adds, per whitespace-separated word, `(c1 LIKE ? OR c2 LIKE ? ...)`
// (words AND-ed, columns OR-ed, wildcard characters escaped).
func (w *Where) Like(term string, cols ...string) *Where {
	return w.LikeOrIn(term, cols, "", nil)
}

// LikeOrIn is Like where the whole term may alternatively match `inCol IN
// (ids)` — the way a document list searches its number OR a party resolved
// through a module client (no cross-module join). ids were resolved by the
// caller from the WHOLE term, so the condition is
//
//	((every word matches some like column) OR inCol IN (ids))
//
// Empty ids = plain Like (never an empty IN ()).
func (w *Where) LikeOrIn(term string, likeCols []string, inCol string, ids []int64) *Where {
	words := strings.Fields(term)
	var wordConds []string
	var wordArgs []any
	for _, word := range words {
		pat := "%" + EscapeLike(word) + "%"
		parts := make([]string, 0, len(likeCols))
		for _, c := range likeCols {
			parts = append(parts, c+" LIKE ?")
			wordArgs = append(wordArgs, pat)
		}
		if len(parts) == 0 {
			continue
		}
		wordConds = append(wordConds, "("+strings.Join(parts, " OR ")+")")
	}
	if inCol == "" || len(ids) == 0 {
		// plain Like: one AND-ed condition per word
		w.conds = append(w.conds, wordConds...)
		w.args = append(w.args, wordArgs...)
		return w
	}
	if len(ids) > MaxIn {
		ids = ids[:MaxIn]
	}
	inSQL := inCol + " IN (" + placeholders(len(ids)) + ")"
	args := append([]any(nil), wordArgs...)
	for _, id := range ids {
		args = append(args, id)
	}
	if len(wordConds) == 0 {
		return w
	}
	return w.Raw("(("+strings.Join(wordConds, " AND ")+") OR "+inSQL+")", args...)
}

// SQL returns the joined conditions ("1=1" when there are none, so callers can
// always write "WHERE "+sql) and the argument slice (a copy).
func (w *Where) SQL() (string, []any) {
	args := append([]any(nil), w.args...)
	if len(w.conds) == 0 {
		return "1=1", args
	}
	return strings.Join(w.conds, " AND "), args
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

// Common is the parsed shape of the filters every list endpoint shares.
type Common struct {
	Search   string
	Statuses []string
	From, To string
	PartyID  int64
	Sort     string // already resolved through the whitelist (SQL expression) or ""
	Desc     bool
	Page     int
	Limit    int
}

// Offset is the SQL offset of the requested page.
func (c Common) Offset() int { return pagination.Offset(c.Page, c.Limit) }

// SortSpec whitelists sortable keys of one list: public key -> SQL ORDER BY
// expression (without direction), plus the default key.
type SortSpec struct {
	Allowed map[string]string
	Default string
	// DefaultDesc is the direction used when the client sends none.
	DefaultDesc bool
}

// ParseCommon reads search, status (CSV), from, to, partyId, sort, dir, page
// and limit from a query string. Malformed dates, a bad partyId, an unknown
// sort key or direction yield a validation error naming the field. spec may
// be nil when the list is not sortable.
func ParseCommon(q url.Values, spec *SortSpec) (Common, error) {
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	c := Common{
		Search:   strings.TrimSpace(q.Get("search")),
		Statuses: SplitCSV(q.Get("status")),
		From:     strings.TrimSpace(q.Get("from")),
		To:       strings.TrimSpace(q.Get("to")),
		Page:     page,
		Limit:    limit,
	}
	var errs []apperror.FieldError
	if c.From != "" {
		if _, ok := ParseDate(c.From); !ok {
			errs = append(errs, apperror.FieldError{Field: "from", Message: "format tanggal harus YYYY-MM-DD"})
		}
	}
	if c.To != "" {
		if _, ok := ParseDate(c.To); !ok {
			errs = append(errs, apperror.FieldError{Field: "to", Message: "format tanggal harus YYYY-MM-DD"})
		}
	}
	if c.From != "" && c.To != "" && len(errs) == 0 && c.From > c.To {
		errs = append(errs, apperror.FieldError{Field: "from", Message: "tanggal awal tidak boleh setelah tanggal akhir"})
	}
	if raw := strings.TrimSpace(q.Get("partyId")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 0 {
			errs = append(errs, apperror.FieldError{Field: "partyId", Message: "harus angka positif"})
		} else {
			c.PartyID = id
		}
	}
	if spec != nil {
		key := strings.TrimSpace(q.Get("sort"))
		if key == "" {
			key = spec.Default
		}
		expr, ok := spec.Allowed[key]
		if !ok {
			errs = append(errs, apperror.FieldError{Field: "sort", Message: "kolom urut tidak dikenal"})
		}
		c.Sort = expr
		c.Desc = spec.DefaultDesc
		switch strings.ToLower(strings.TrimSpace(q.Get("dir"))) {
		case "":
		case "asc":
			c.Desc = false
		case "desc":
			c.Desc = true
		default:
			errs = append(errs, apperror.FieldError{Field: "dir", Message: "harus asc atau desc"})
		}
	}
	if len(errs) > 0 {
		return c, apperror.Validation("", errs)
	}
	return c, nil
}

// OrderBy renders `expr DESC|ASC`, falling back to fallback when Sort is empty.
func (c Common) OrderBy(fallback string) string {
	if c.Sort == "" {
		return fallback
	}
	if c.Desc {
		return c.Sort + " DESC"
	}
	return c.Sort + " ASC"
}

// ValidateEnum checks v (when non-empty) is one of allowed; the error names field.
func ValidateEnum(field, v string, allowed ...string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	return apperror.Validation("", []apperror.FieldError{{Field: field, Message: "nilai harus salah satu dari: " + strings.Join(allowed, ", ")}})
}

// ParseID reads an optional positive id query value; blank = 0, anything else
// that is not a positive integer is a validation error naming the field.
func ParseID(raw, field string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.Validation("", []apperror.FieldError{{Field: field, Message: "harus angka positif"}})
	}
	return id, nil
}
