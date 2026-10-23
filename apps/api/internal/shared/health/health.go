// Package health carries the read-only data-health facts that modules report
// to the dashboard's alert composer (ADR-0025). Pure data, like
// shared/integrity: each owning module counts its own rows; the dashboard
// alone decides severity, wording and actions.
package health

// Result is one check's aggregate: how many rows are affected, an optional
// rupiah figure, extra per-check numbers, and up to five display rows.
type Result struct {
	Count int64
	// AmountRp is nil when no rupiah figure is meaningful (never "Rp0").
	AmountRp *int64
	Meta     map[string]int64
	Sample   []Sample
	// OldestAt is the age anchor of the problem (YYYY-MM-DD, WIB), "" when the
	// data carries none.
	OldestAt string
}

// Sample is one ready-to-display example row.
type Sample struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Item is one row of a check's full, paginated list.
type Item struct {
	Key     string         `json:"key"`
	Label   string         `json:"label"`
	Columns map[string]any `json:"columns"`
	Links   []Link         `json:"links"`
}

// Link is a navigation target; the frontend hides it when !can(Perm).
type Link struct {
	Label string `json:"label"`
	To    string `json:"to"`
	Perm  string `json:"perm"`
}

// Amount boxes a rupiah total for Result.AmountRp.
func Amount(v int64) *int64 { return &v }
