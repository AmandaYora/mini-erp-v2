// Package report collects what the ETL decided: every changed money value
// (adjustments.csv, D-ETL-8) and every finding (findings.csv).
package report

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// Severity of a finding.
const (
	Blocker = "blocker"
	Warn    = "warn"
	Info    = "info"
)

// Adjustment is one value that changed during conversion.
type Adjustment struct {
	Entity, Field, Before, After, Rule string
	LegacyID                           int64
}

// Finding is one thing the owner must know or decide.
type Finding struct {
	Severity, Code, Entity, Detail string
	LegacyID                       int64
}

// Log accumulates adjustments and findings in deterministic order.
type Log struct {
	Adjustments []Adjustment
	Findings    []Finding
}

// Adjust records a changed value.
func (l *Log) Adjust(entity string, id int64, field, before, after, rule string) {
	l.Adjustments = append(l.Adjustments, Adjustment{entity, field, before, after, rule, id})
}

// Find records a finding.
func (l *Log) Find(sev, code, entity string, id int64, detail string) {
	l.Findings = append(l.Findings, Finding{sev, code, entity, detail, id})
}

// Blockers counts blocker findings.
func (l *Log) Blockers() int {
	n := 0
	for _, f := range l.Findings {
		if f.Severity == Blocker {
			n++
		}
	}
	return n
}

// CountByCode returns findings per code (for reconciliation "explained" sums).
func (l *Log) CountByCode() map[string]int {
	m := map[string]int{}
	for _, f := range l.Findings {
		m[f.Code]++
	}
	return m
}

// WriteCSV writes adjustments.csv and findings.csv under dir.
func (l *Log) WriteCSV(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	adj := [][]string{{"entity", "legacy_id", "field", "before", "after", "rule"}}
	for _, a := range l.Adjustments {
		adj = append(adj, []string{a.Entity, strconv.FormatInt(a.LegacyID, 10), a.Field, a.Before, a.After, a.Rule})
	}
	fnd := [][]string{{"severity", "code", "entity", "legacy_id", "detail"}}
	for _, f := range l.Findings {
		fnd = append(fnd, []string{f.Severity, f.Code, f.Entity, strconv.FormatInt(f.LegacyID, 10), f.Detail})
	}
	if err := writeCSV(filepath.Join(dir, "adjustments.csv"), adj); err != nil {
		return err
	}
	return writeCSV(filepath.Join(dir, "findings.csv"), fnd)
}

// WriteTable writes one extra CSV report (header row first).
func WriteTable(path string, rows [][]string) error { return writeCSV(path, rows) }

func writeCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// SortedKeys is a small helper for deterministic map iteration.
func SortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
