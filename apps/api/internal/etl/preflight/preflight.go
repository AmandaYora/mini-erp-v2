// Package preflight measures the legacy copy before anything is extracted
// (F5-2): it runs the SQL fact-checkers of the design (facts/*.sql, one file
// per check with a small header) and decides which findings are blockers. It
// also verifies that every legacy table the ETL reads exists and answers the
// tenant query, which catches schema drift in the legacy dump early.
package preflight

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/legacydb"
	"mini-erp/internal/etl/registry"
	"mini-erp/internal/etl/transform"
)

//go:embed facts/*.sql
var factFS embed.FS

// Fact is one parsed fact-checker file.
type Fact struct {
	ID, Severity, Rule, Action string
	Stmts                      []string
}

// Result is the outcome of one fact.
type Result struct {
	ID       string     `json:"id"`
	Severity string     `json:"severity"`
	Action   string     `json:"action"`
	Rows     int        `json:"rows"`
	Columns  [][]string `json:"columns"`
	Sample   [][]string `json:"sample"`
	Blocker  bool       `json:"blocker"`
	Error    string     `json:"error,omitempty"`
}

// Report is the whole preflight.
type Report struct {
	Facts    []Result         `json:"facts"`
	Tables   map[string]int64 `json:"table_rows"`
	Missing  []string         `json:"drift"`
	Blockers int              `json:"blockers"`
}

// Facts loads and parses the embedded fact files (sorted by id).
func Facts() ([]Fact, error) {
	entries, err := factFS.ReadDir("facts")
	if err != nil {
		return nil, err
	}
	var out []Fact
	for _, e := range entries {
		raw, err := factFS.ReadFile("facts/" + e.Name())
		if err != nil {
			return nil, err
		}
		f := Fact{}
		var body []string
		for _, line := range strings.Split(string(raw), "\n") {
			switch {
			case strings.HasPrefix(line, "-- id: "):
				f.ID = strings.TrimSpace(strings.TrimPrefix(line, "-- id: "))
			case strings.HasPrefix(line, "-- severity: "):
				f.Severity = strings.TrimSpace(strings.TrimPrefix(line, "-- severity: "))
			case strings.HasPrefix(line, "-- rule: "):
				f.Rule = strings.TrimSpace(strings.TrimPrefix(line, "-- rule: "))
			case strings.HasPrefix(line, "-- action: "):
				f.Action = strings.TrimSpace(strings.TrimPrefix(line, "-- action: "))
			default:
				body = append(body, line)
			}
		}
		for _, s := range strings.Split(strings.Join(body, "\n"), ";\n") {
			if s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), ";")); s != "" {
				f.Stmts = append(f.Stmts, s)
			}
		}
		if f.ID == "" || len(f.Stmts) == 0 {
			return nil, fmt.Errorf("fakta %s tidak valid", e.Name())
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

type stmtResult struct {
	cols []string
	rows [][]string // NULL rendered as "NULL"
}

func runStmt(ctx context.Context, conn *sql.Conn, q string) (*stmtResult, error) {
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	cols, _ := rows.Columns()
	res := &stmtResult{cols: cols}
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		r := make([]string, len(cols))
		for i, v := range vals {
			if v.Valid {
				r[i] = v.String
			} else {
				r[i] = "NULL"
			}
		}
		res.rows = append(res.rows, r)
	}
	return res, rows.Err()
}

func colIndex(cols []string, name string) int {
	for i, c := range cols {
		if strings.EqualFold(c, name) {
			return i
		}
	}
	return -1
}

func num(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// evaluate applies the rule(s) of a fact to its statement results.
func evaluate(rule string, rs []*stmtResult) bool {
	if rule == "" || rule == "none" {
		return false
	}
	for _, r := range strings.Split(rule, ";") {
		kind, arg, _ := strings.Cut(r, ":")
		switch {
		case r == "rows>0":
			for _, s := range rs {
				if len(s.rows) > 0 {
					return true
				}
			}
		case r == "first>0":
			if len(rs) > 0 && len(rs[0].rows) > 0 && num(rs[0].rows[0][0]) > 0 {
				return true
			}
		case kind == "stmtrows":
			n, _, _ := strings.Cut(arg, ">")
			i, _ := strconv.Atoi(n)
			if i >= 1 && i <= len(rs) && len(rs[i-1].rows) > 0 {
				return true
			}
		case kind == "stmtfirst":
			n, _, _ := strings.Cut(arg, ">")
			i, _ := strconv.Atoi(n)
			if i >= 1 && i <= len(rs) && len(rs[i-1].rows) > 0 && num(rs[i-1].rows[0][0]) > 0 {
				return true
			}
		case kind == "col" || kind == "colmax":
			name, lim, _ := strings.Cut(arg, ">")
			limit := num(lim)
			for _, s := range rs {
				ci := colIndex(s.cols, name)
				if ci < 0 {
					continue
				}
				sum, max := 0.0, -1e18
				for _, row := range s.rows {
					v := num(row[ci])
					sum += v
					if v > max {
						max = v
					}
				}
				if (kind == "col" && sum > limit) || (kind == "colmax" && max > limit) {
					return true
				}
			}
		case kind == "cell":
			name, val, _ := strings.Cut(arg, "=")
			for _, s := range rs {
				if ci := colIndex(s.cols, name); ci >= 0 {
					for _, row := range s.rows {
						if row[ci] == val {
							return true
						}
					}
				}
			}
		case kind == "notin":
			name, list, _ := strings.Cut(arg, "=")
			allowed := map[string]bool{}
			for _, a := range strings.Split(list, ",") {
				allowed[a] = true
			}
			for _, s := range rs {
				if ci := colIndex(s.cols, name); ci >= 0 {
					for _, row := range s.rows {
						if !allowed[row[ci]] {
							return true
						}
					}
				}
			}
		case kind == "absdiff":
			cols, lim, _ := strings.Cut(arg, ">")
			a, b, _ := strings.Cut(cols, ",")
			limit := num(lim)
			for _, s := range rs {
				ia, ib := colIndex(s.cols, a), colIndex(s.cols, b)
				if ia < 0 || ib < 0 {
					continue
				}
				for _, row := range s.rows {
					d := num(row[ia]) - num(row[ib])
					if d < 0 {
						d = -d
					}
					if d > limit {
						return true
					}
				}
			}
		}
	}
	return false
}

// Run executes every fact plus the table/drift checks and writes
// report/facts.json and facts.md. A failing query is itself a blocker: the
// legacy copy does not look the way the ETL expects.
func Run(ctx context.Context, ldb *legacydb.DB, c *cfg.Config, work string) (*Report, error) {
	facts, err := Facts()
	if err != nil {
		return nil, err
	}
	conn, err := ldb.SQL.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY"); err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "SET @c = ?, @open = ?", c.CompanyID, c.OpeningDate); err != nil {
		return nil, err
	}
	rep := &Report{Tables: map[string]int64{}}
	// table presence + tenant predicate (drift check)
	for _, ent := range registry.All {
		var n int64
		q := ldb.Q("SELECT COUNT(*) FROM {" + ent.Name + "} WHERE (" + ent.Tenant + ")")
		if err := conn.QueryRowContext(ctx, q).Scan(&n); err != nil {
			rep.Missing = append(rep.Missing, ent.Name+": "+err.Error())
			continue
		}
		rep.Tables[ent.Name] = n
	}
	for _, f := range facts {
		res := Result{ID: f.ID, Severity: f.Severity, Action: f.Action}
		var all []*stmtResult
		for _, st := range f.Stmts {
			sr, err := runStmt(ctx, conn, ldb.Q(st))
			if err != nil {
				res.Error = err.Error()
				res.Blocker = true
				break
			}
			all = append(all, sr)
			res.Rows += len(sr.rows)
			res.Columns = append(res.Columns, sr.cols)
			for i, r := range sr.rows {
				if i >= 5 {
					break
				}
				res.Sample = append(res.Sample, r)
			}
		}
		if res.Error == "" && f.Severity == "blocker" {
			res.Blocker = evaluate(f.Rule, all)
		}
		if res.Blocker {
			rep.Blockers++
		}
		rep.Facts = append(rep.Facts, res)
	}
	rep.Blockers += len(rep.Missing)
	return rep, write(work, rep)
}

func write(work string, rep *Report) error {
	dir := transform.ReportDir(work)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "facts.json"), raw, 0o644); err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("# Preflight\n\n")
	fmt.Fprintf(&sb, "Blocker: **%d**\n\n| id | severity | baris | blocker | tindakan |\n|---|---|---|---|---|\n", rep.Blockers)
	for _, f := range rep.Facts {
		b := ""
		if f.Blocker {
			b = "YA"
		}
		if f.Error != "" {
			b = "ERROR: " + f.Error
		}
		fmt.Fprintf(&sb, "| %s | %s | %d | %s | %s |\n", f.ID, f.Severity, f.Rows, strings.ReplaceAll(b, "|", "/"), f.Action)
	}
	if len(rep.Missing) > 0 {
		sb.WriteString("\n## Drift tabel legacy\n\n")
		for _, m := range rep.Missing {
			sb.WriteString("- " + m + "\n")
		}
	}
	return os.WriteFile(filepath.Join(dir, "facts.md"), []byte(sb.String()), 0o644)
}
