package load

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/guard"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/etl/transform"
)

// BatchSize is the number of rows per INSERT / transaction (design F5-8).
const BatchSize = 500

// TableResult is what happened to one table.
type TableResult struct {
	Table    string `json:"table"`
	Rows     int    `json:"rows"`
	Inserted int    `json:"inserted"`
	Existing int    `json:"existing"`
	Skipped  bool   `json:"stage_skipped"`
}

// Result is the load report.
type Result struct {
	Tables   []TableResult `json:"tables"`
	Warnings []string      `json:"warnings"`
}

// Inserted returns the total number of rows written by this run.
func (r *Result) Inserted() int {
	n := 0
	for _, t := range r.Tables {
		n += t.Inserted
	}
	return n
}

// Loader carries the state of one load run.
type Loader struct {
	DB    *sql.DB
	Cfg   *cfg.Config
	Work  string
	Log   *slog.Logger
	man   *transform.Manifest
	remap map[string]map[int64]int64
	res   *Result
}

func (l *Loader) t(name string) string { return "`" + l.Cfg.StagePrefix + name + "`" }

// EnsureStaging creates the ETL bookkeeping tables inside the target database.
func EnsureStaging(ctx context.Context, db *sql.DB, prefix string) error {
	stmts := []string{
		"CREATE TABLE IF NOT EXISTS `" + prefix + "run` (run_id VARCHAR(100) NOT NULL PRIMARY KEY, company BIGINT NOT NULL, opening_date VARCHAR(10) NOT NULL, started_at DATETIME(6) NOT NULL) ENGINE=InnoDB",
		"CREATE TABLE IF NOT EXISTS `" + prefix + "id_map` (run_id VARCHAR(100) NOT NULL, entity VARCHAR(60) NOT NULL, legacy_id BIGINT NOT NULL, legacy_sub BIGINT NOT NULL, new_id BIGINT NOT NULL, PRIMARY KEY (run_id, entity, legacy_id, legacy_sub)) ENGINE=InnoDB",
		"CREATE TABLE IF NOT EXISTS `" + prefix + "stage` (run_id VARCHAR(100) NOT NULL, stage VARCHAR(80) NOT NULL, status VARCHAR(20) NOT NULL, rows_loaded BIGINT NOT NULL, checksum CHAR(64) NOT NULL, finished_at DATETIME(6) NOT NULL, PRIMARY KEY (run_id, stage)) ENGINE=InnoDB",
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// Run loads every stage. It is safe to run again: finished stages with an
// unchanged checksum are skipped, partly loaded ones continue by primary key.
func Run(ctx context.Context, db *sql.DB, c *cfg.Config, work string, log *slog.Logger) (*Result, error) {
	l := &Loader{DB: db, Cfg: c, Work: work, Log: log, remap: map[string]map[int64]int64{}, res: &Result{}}
	if err := guard.CheckIdent(c.StagePrefix); err != nil {
		return nil, err
	}
	man, err := transform.ReadManifest(work)
	if err != nil {
		return nil, fmt.Errorf("hasil transform tidak ada (jalankan transform dulu): %w", err)
	}
	l.man = man
	if man.Blockers > 0 {
		return nil, fmt.Errorf("transform menghasilkan %d temuan blocker (lihat report/findings.csv): load ditolak", man.Blockers)
	}
	name, err := guard.DBName(c.TargetDSN)
	if err == nil {
		if err := guard.CheckTargetName(name, c.AllowTargetDB); err != nil {
			return nil, err
		}
	}
	if err := EnsureStaging(ctx, db, c.StagePrefix); err != nil {
		return nil, fmt.Errorf("staging: %w", err)
	}
	if err := guard.TargetEmpty(ctx, db, c.StagePrefix, c.RunID, Tables(), c.Resume, c.AllowNonEmptyTarget); err != nil {
		return nil, err
	}
	if fp := c.ExpectFingerprint; fp != "" {
		got, err := guard.Fingerprint(ctx, db, Tables())
		if err != nil {
			return nil, err
		}
		if got != fp {
			return nil, fmt.Errorf("sidik jari schema target berbeda (%s != %s)", got, fp)
		}
	}
	if _, err := db.ExecContext(ctx, "INSERT IGNORE INTO "+l.t("run")+" (run_id, company, opening_date, started_at) VALUES (?, ?, ?, UTC_TIMESTAMP(6))",
		c.RunID, c.CompanyID, c.OpeningDate); err != nil {
		return nil, err
	}
	if err := l.idMap(ctx); err != nil {
		return nil, err
	}
	if err := l.resolveRoles(ctx); err != nil {
		return nil, fmt.Errorf("roles: %w", err)
	}
	if err := l.resolveAccounts(ctx); err != nil {
		return nil, fmt.Errorf("finance_accounts: %w", err)
	}
	for _, s := range Stages {
		if s.Table == "media_files" {
			if err := l.cutovers(ctx); err != nil {
				return nil, err
			}
			if err := l.rollup(ctx); err != nil {
				return nil, err
			}
			if _, err := os.Stat(filepath.Join(transform.OutDir(work), "media_files_final.jsonl")); err != nil {
				l.res.Warnings = append(l.res.Warnings, "media_files dilewati: jalankan fase media dulu")
				continue
			}
		}
		if err := l.table(ctx, s); err != nil {
			return nil, fmt.Errorf("%s: %w", s.Table, err)
		}
		switch s.Table {
		case "user_branch_access":
			if err := l.rolePermissions(ctx); err != nil {
				return nil, fmt.Errorf("role_permissions: %w", err)
			}
			if err := l.companyProfile(ctx); err != nil {
				return nil, fmt.Errorf("company_profile: %w", err)
			}
		}
	}
	return l.res, nil
}

// ---------------------------------------------------------------- id map

func (l *Loader) idMap(ctx context.Context) error {
	rows, err := transform.ReadRows(l.Work, "id_map")
	if err != nil {
		return err
	}
	for i := 0; i < len(rows); i += 2000 {
		end := min(i+2000, len(rows))
		tx, err := l.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		var sb strings.Builder
		sb.WriteString("INSERT INTO " + l.t("id_map") + " (run_id, entity, legacy_id, legacy_sub, new_id) VALUES ")
		var args []any
		for j, r := range rows[i:end] {
			if j > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?)")
			args = append(args, l.Cfg.RunID, r.Str("entity"), r.Int("legacy_id"), r.Int("legacy_sub"), r.Int("new_id"))
		}
		sb.WriteString(" ON DUPLICATE KEY UPDATE new_id = VALUES(new_id)")
		if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------- resolvers

func (l *Loader) setRemap(table string, tmp, real int64) {
	if l.remap[table] == nil {
		l.remap[table] = map[int64]int64{}
	}
	l.remap[table][tmp] = real
}

// resolveRoles matches roles by code (the target may already hold seeded
// roles) and creates the rest; dependent tables use the resulting ids.
func (l *Loader) resolveRoles(ctx context.Context) error {
	rows, err := transform.ReadRows(l.Work, "roles")
	if err != nil {
		return err
	}
	res := TableResult{Table: "roles", Rows: len(rows)}
	for _, r := range rows {
		var id int64
		err := l.DB.QueryRowContext(ctx, "SELECT id FROM roles WHERE code = ?", r.Str("code")).Scan(&id)
		switch {
		case err == nil:
			res.Existing++
		case errors.Is(err, sql.ErrNoRows):
			out, err := l.DB.ExecContext(ctx, "INSERT INTO roles (code, name, description, is_system, created_at, updated_at) VALUES (?,?,?,?,?,?)",
				r.Str("code"), r.Str("name"), arg(r["description"]), r.Int("is_system"), r.Str("created_at"), r.Str("updated_at"))
			if err != nil {
				return err
			}
			id, _ = out.LastInsertId()
			res.Inserted++
		default:
			return err
		}
		l.setRemap("roles", r.Int("id"), id)
	}
	l.res.Tables = append(l.res.Tables, res)
	return nil
}

func (l *Loader) resolveAccounts(ctx context.Context) error {
	rows, err := transform.ReadRows(l.Work, "finance_accounts")
	if err != nil {
		return err
	}
	res := TableResult{Table: "finance_accounts", Rows: len(rows)}
	for _, r := range rows {
		var id int64
		var typ string
		err := l.DB.QueryRowContext(ctx, "SELECT id, type FROM finance_accounts WHERE code = ?", r.Str("code")).Scan(&id, &typ)
		switch {
		case err == nil:
			res.Existing++
			if typ != r.Str("type") {
				l.res.Warnings = append(l.res.Warnings, fmt.Sprintf("akun %s sudah ada dengan tipe %s (legacy: %s)", r.Str("code"), typ, r.Str("type")))
			}
		case errors.Is(err, sql.ErrNoRows):
			out, err := l.DB.ExecContext(ctx, "INSERT INTO finance_accounts (code, name, type, is_cash, status, created_at, updated_at) VALUES (?,?,?,?,?,?,?)",
				r.Str("code"), r.Str("name"), r.Str("type"), r.Int("is_cash"), r.Str("status"), r.Str("created_at"), r.Str("updated_at"))
			if err != nil {
				return err
			}
			id, _ = out.LastInsertId()
			res.Inserted++
		default:
			return err
		}
		l.setRemap("finance_accounts", r.Int("id"), id)
	}
	l.res.Tables = append(l.res.Tables, res)
	return nil
}

func (l *Loader) rolePermissions(ctx context.Context) error {
	rows, err := transform.ReadRows(l.Work, "role_permissions")
	if err != nil {
		return err
	}
	res := TableResult{Table: "role_permissions", Rows: len(rows)}
	unknown := map[string]bool{}
	for _, r := range rows {
		var pid int64
		err := l.DB.QueryRowContext(ctx, "SELECT id FROM permissions WHERE code = ?", r.Str("permission_code")).Scan(&pid)
		if errors.Is(err, sql.ErrNoRows) {
			unknown[r.Str("permission_code")] = true
			continue
		}
		if err != nil {
			return err
		}
		role := l.remap["roles"][r.Int("role_id")]
		out, err := l.DB.ExecContext(ctx, "INSERT IGNORE INTO role_permissions (role_id, permission_id) VALUES (?, ?)", role, pid)
		if err != nil {
			return err
		}
		if n, _ := out.RowsAffected(); n > 0 {
			res.Inserted++
		} else {
			res.Existing++
		}
	}
	if len(unknown) > 0 {
		var codes []string
		for c := range unknown {
			codes = append(codes, c)
		}
		sort.Strings(codes)
		l.res.Warnings = append(l.res.Warnings, "izin tidak ada di katalog target (dilewati): "+strings.Join(codes, ", "))
	}
	l.res.Tables = append(l.res.Tables, res)
	return nil
}

func (l *Loader) companyProfile(ctx context.Context) error {
	rows, err := transform.ReadRows(l.Work, "company_profile")
	if err != nil || len(rows) == 0 {
		return err
	}
	var n int
	if err := l.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM company_profile").Scan(&n); err != nil {
		return err
	}
	res := TableResult{Table: "company_profile", Rows: len(rows)}
	if n == 0 {
		r := rows[0]
		if _, err := l.DB.ExecContext(ctx, "INSERT INTO company_profile (name, legal_name, created_at, updated_at) VALUES (?,?,?,?)",
			r.Str("name"), arg(r["legal_name"]), r.Str("created_at"), r.Str("updated_at")); err != nil {
			return err
		}
		res.Inserted = 1
	} else {
		res.Existing = 1
	}
	l.res.Tables = append(l.res.Tables, res)
	return nil
}

func (l *Loader) cutovers(ctx context.Context) error {
	rows, err := transform.ReadRows(l.Work, "cutovers")
	if err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := l.DB.ExecContext(ctx,
			`INSERT INTO finance_cutover (branch_id, doc_type, covered_through_id) VALUES (?,?,?)
			 ON DUPLICATE KEY UPDATE covered_through_id = VALUES(covered_through_id)`,
			r.Int("branch_id"), r.Str("doc_type"), r.Int("through")); err != nil {
			return err
		}
	}
	l.res.Tables = append(l.res.Tables, TableResult{Table: "finance_cutover", Rows: len(rows), Inserted: 0, Existing: len(rows)})
	return nil
}

// rollup rebuilds the per-day account totals the reports read (migration 47).
func (l *Loader) rollup(ctx context.Context) error {
	_, err := l.DB.ExecContext(ctx,
		`INSERT INTO finance_account_daily (branch_id, account_id, entry_date, scope, debit, credit)
		 SELECT e.branch_id, l.account_id, DATE(e.entry_date), e.scope, SUM(l.debit), SUM(l.credit)
		   FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id
		  GROUP BY e.branch_id, l.account_id, DATE(e.entry_date), e.scope
		 ON DUPLICATE KEY UPDATE debit = VALUES(debit), credit = VALUES(credit)`)
	return err
}

// ---------------------------------------------------------------- generic tables

type liveCol struct {
	name     string
	maxLen   sql.NullInt64
	nullable bool
	hasDef   bool
	extra    string
}

func (l *Loader) liveColumns(ctx context.Context, table string) (map[string]liveCol, error) {
	rows, err := l.DB.QueryContext(ctx,
		`SELECT column_name, character_maximum_length, is_nullable, column_default IS NOT NULL, extra
		   FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ?`, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]liveCol{}
	for rows.Next() {
		var c liveCol
		var nl string
		if err := rows.Scan(&c.name, &c.maxLen, &nl, &c.hasDef, &c.extra); err != nil {
			return nil, err
		}
		c.nullable = nl == "YES"
		out[c.name] = c
	}
	return out, rows.Err()
}

func arg(v any) any {
	switch x := v.(type) {
	case json.Number:
		return x.String()
	default:
		return v
	}
}

func rowKey(r row.Row, pk []string) string {
	parts := make([]string, len(pk))
	for i, c := range pk {
		parts[i] = r.Str(c)
	}
	return strings.Join(parts, "\x1f")
}

// orderParentsFirst sorts rows so a self-referencing parent precedes its children.
func orderParentsFirst(rows []row.Row, ref string) []row.Row {
	placed := map[string]bool{}
	pending := rows
	var out []row.Row
	for len(pending) > 0 {
		var next []row.Row
		for _, r := range pending {
			if !r.Has(ref) || placed[r.Str(ref)] {
				out = append(out, r)
				placed[r.Str("id")] = true
			} else {
				next = append(next, r)
			}
		}
		if len(next) == len(pending) { // cycle or dangling: keep original order, DB will complain
			return append(out, next...)
		}
		pending = next
	}
	return out
}

func (l *Loader) stageDone(ctx context.Context, stage string) (string, bool, error) {
	var sum string
	err := l.DB.QueryRowContext(ctx, "SELECT checksum FROM "+l.t("stage")+" WHERE run_id = ? AND stage = ? AND status = 'done'", l.Cfg.RunID, stage).Scan(&sum)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return sum, err == nil, err
}

func (l *Loader) table(ctx context.Context, s Spec) error {
	src := s.Source
	if src == "" {
		src = s.Table
	}
	rows, err := transform.ReadRows(l.Work, src)
	if err != nil {
		return err
	}
	var checksum string
	if fi, ok := l.man.Files[src+".jsonl"]; ok {
		checksum = fi.SHA256
	} else {
		checksum = fmt.Sprintf("%064d", len(rows)) // files produced after transform (media)
		if h, err := fileSum(filepath.Join(transform.OutDir(l.Work), src+".jsonl")); err == nil {
			checksum = h
		}
	}
	res := TableResult{Table: s.Table, Rows: len(rows)}
	if prev, done, err := l.stageDone(ctx, s.Table); err != nil {
		return err
	} else if done {
		if prev != checksum {
			return fmt.Errorf("tahap %s sudah selesai dengan data berbeda (staging berubah); mulai dari target kosong", s.Table)
		}
		res.Skipped, res.Existing = true, len(rows)
		l.res.Tables = append(l.res.Tables, res)
		l.Log.Info("load: dilewati (sudah selesai)", "table", s.Table)
		return nil
	}
	live, err := l.liveColumns(ctx, s.Table)
	if err != nil {
		return err
	}
	if len(live) == 0 {
		return fmt.Errorf("tabel target %s tidak ada", s.Table)
	}
	var cols []string
	inSpec := map[string]bool{}
	for _, c := range s.Cols {
		inSpec[c] = true
		if _, ok := live[c]; !ok {
			if s.Optional[c] {
				l.res.Warnings = append(l.res.Warnings, fmt.Sprintf("%s.%s tidak ada di target: dilewati", s.Table, c))
				continue
			}
			return fmt.Errorf("kolom %s.%s tidak ada di schema target (drift)", s.Table, c)
		}
		cols = append(cols, c)
	}
	// drift in the other direction: a new mandatory column nobody maps yet.
	var unmapped []string
	for name, c := range live {
		if inSpec[name] || c.nullable || c.hasDef || strings.Contains(c.extra, "auto_increment") || strings.Contains(strings.ToUpper(c.extra), "GENERATED") {
			continue
		}
		if s.Table == "media_files" && name == "id" {
			continue
		}
		unmapped = append(unmapped, name)
	}
	if len(unmapped) > 0 {
		sort.Strings(unmapped)
		return fmt.Errorf("kolom wajib baru di %s belum dipetakan ETL: %s", s.Table, strings.Join(unmapped, ", "))
	}
	// no silent truncation: every text value must fit the live column
	var tooLong []string
	for _, r := range rows {
		for _, c := range cols {
			lc := live[c]
			if !lc.maxLen.Valid {
				continue
			}
			if v, ok := r.NStr(c); ok && int64(utf8.RuneCountInString(v)) > lc.maxLen.Int64 {
				tooLong = append(tooLong, fmt.Sprintf("%s.%s=%q(%d>%d)", s.Table, c, trimShow(v), utf8.RuneCountInString(v), lc.maxLen.Int64))
			}
		}
		if len(tooLong) >= 20 {
			break
		}
	}
	if len(tooLong) > 0 {
		return fmt.Errorf("data lebih panjang dari kolom target (tidak ada pemotongan diam-diam): %s", strings.Join(tooLong, "; "))
	}
	// remap temporary ids of resolver tables
	for col, from := range s.Remap {
		for _, r := range rows {
			if !r.Has(col) {
				continue
			}
			real, ok := l.remap[from][r.Int(col)]
			if !ok {
				return fmt.Errorf("%s.%s: id %d tidak ada di hasil resolusi %s", s.Table, col, r.Int(col), from)
			}
			r[col] = real
		}
	}
	if s.SelfRef != "" {
		rows = orderParentsFirst(rows, s.SelfRef)
	}
	for i := 0; i < len(rows); i += BatchSize {
		end := min(i+BatchSize, len(rows))
		ins, ex, err := l.batch(ctx, s, cols, rows[i:end])
		if err != nil {
			return fmt.Errorf("baris %d..%d: %w", i, end, err)
		}
		res.Inserted += ins
		res.Existing += ex
	}
	if _, err := l.DB.ExecContext(ctx,
		"INSERT INTO "+l.t("stage")+" (run_id, stage, status, rows_loaded, checksum, finished_at) VALUES (?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE status = VALUES(status), rows_loaded = VALUES(rows_loaded), checksum = VALUES(checksum), finished_at = VALUES(finished_at)",
		l.Cfg.RunID, s.Table, "done", len(rows), checksum); err != nil {
		return err
	}
	l.res.Tables = append(l.res.Tables, res)
	l.Log.Info("load", "table", s.Table, "rows", len(rows), "inserted", res.Inserted, "existing", res.Existing)
	return nil
}

func trimShow(v string) string {
	if utf8.RuneCountInString(v) > 30 {
		return string([]rune(v)[:30]) + "..."
	}
	return v
}

func (l *Loader) batch(ctx context.Context, s Spec, cols []string, rows []row.Row) (inserted, existing int, err error) {
	if len(rows) == 0 {
		return 0, 0, nil
	}
	todo := rows
	if !s.Upsert {
		have, err := l.existing(ctx, s, rows)
		if err != nil {
			return 0, 0, err
		}
		todo = todo[:0:0]
		for _, r := range rows {
			if have[rowKey(r, s.PK)] {
				existing++
			} else {
				todo = append(todo, r)
			}
		}
	}
	if len(todo) == 0 {
		return 0, existing, nil
	}
	var sb strings.Builder
	sb.WriteString("INSERT INTO `" + s.Table + "` (`" + strings.Join(cols, "`, `") + "`) VALUES ")
	ph := "(" + strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",") + ")"
	args := make([]any, 0, len(todo)*len(cols))
	for i, r := range todo {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(ph)
		for _, c := range cols {
			args = append(args, arg(r[c]))
		}
	}
	if s.Upsert {
		var sets []string
		for _, c := range cols {
			isPK := false
			for _, p := range s.PK {
				if p == c {
					isPK = true
				}
			}
			if !isPK {
				sets = append(sets, "`"+c+"` = VALUES(`"+c+"`)")
			}
		}
		sb.WriteString(" ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", "))
	}
	tx, err := l.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	if s.Upsert {
		return 0, len(todo), nil
	}
	return len(todo), existing, nil
}

// existing returns the keys of the batch rows already present in the target.
func (l *Loader) existing(ctx context.Context, s Spec, rows []row.Row) (map[string]bool, error) {
	var q strings.Builder
	var args []any
	pk := "`" + strings.Join(s.PK, "`, `") + "`"
	q.WriteString("SELECT " + pk + " FROM `" + s.Table + "` WHERE ")
	if len(s.PK) == 1 {
		q.WriteString("`" + s.PK[0] + "` IN (" + strings.TrimSuffix(strings.Repeat("?,", len(rows)), ",") + ")")
		for _, r := range rows {
			args = append(args, arg(r[s.PK[0]]))
		}
	} else {
		one := "(" + strings.TrimSuffix(strings.Repeat("?,", len(s.PK)), ",") + ")"
		q.WriteString("(" + pk + ") IN (")
		for i, r := range rows {
			if i > 0 {
				q.WriteString(",")
			}
			q.WriteString(one)
			for _, c := range s.PK {
				args = append(args, arg(r[c]))
			}
		}
		q.WriteString(")")
	}
	rs, err := l.DB.QueryContext(ctx, q.String(), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rs.Close() }()
	out := map[string]bool{}
	vals := make([]sql.NullString, len(s.PK))
	ptrs := make([]any, len(s.PK))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = v.String
		}
		out[strings.Join(parts, "\x1f")] = true
	}
	return out, rs.Err()
}

func fileSum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
