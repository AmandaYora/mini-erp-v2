// Package extract streams the legacy tables into gzip-compressed JSONL files
// (F5-3). One REPEATABLE READ / READ ONLY snapshot covers every table, keyset
// pagination keeps memory at one batch, and a manifest with sha256 per file is
// the basis of the determinism and idempotency checks.
package extract

import (
	"bufio"
	"compress/gzip"
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
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/etl/legacydb"
	"mini-erp/internal/etl/registry"
	"mini-erp/internal/etl/row"
)

// PageSize is the keyset batch size.
const PageSize = 5000

// TableInfo is one manifest entry.
type TableInfo struct {
	Rows    int64    `json:"rows"`
	MinPK   string   `json:"min_pk"`
	MaxPK   string   `json:"max_pk"`
	SHA256  string   `json:"sha256"`
	Columns []string `json:"columns"`
}

// Manifest describes a finished extraction.
type Manifest struct {
	SnapshotAt string               `json:"snapshot_at"`
	Version    string               `json:"mysql_version"`
	SQLMode    string               `json:"sql_mode"`
	Tables     map[string]TableInfo `json:"tables"`
}

// RawDir is where the extracted tables live inside the work dir.
func RawDir(work string) string { return filepath.Join(work, "raw") }

// Run extracts every registered table. With resume, tables whose file still
// matches the manifest are skipped (a partially written table restarts).
func Run(ctx context.Context, ldb *legacydb.DB, work string, resume bool, log *slog.Logger) (*Manifest, error) {
	if err := os.MkdirAll(RawDir(work), 0o755); err != nil {
		return nil, err
	}
	mpath := filepath.Join(RawDir(work), "manifest.json")
	man := &Manifest{Tables: map[string]TableInfo{}}
	if resume {
		if raw, err := os.ReadFile(mpath); err == nil {
			_ = json.Unmarshal(raw, man)
			if man.Tables == nil {
				man.Tables = map[string]TableInfo{}
			}
		}
	}
	conn, err := ldb.SQL.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	for _, s := range []string{"SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ"} {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return nil, err
		}
	}
	if _, err := conn.ExecContext(ctx, "START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY"); err != nil {
		return nil, fmt.Errorf("snapshot legacy: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK") }()
	if err := conn.QueryRowContext(ctx, "SELECT @@version, @@sql_mode").Scan(&man.Version, &man.SQLMode); err != nil {
		return nil, err
	}
	if man.SnapshotAt == "" || !resume {
		man.SnapshotAt = time.Now().UTC().Format(time.RFC3339)
	}
	for _, ent := range registry.All {
		path := tablePath(work, ent.Name)
		if info, ok := man.Tables[ent.Name]; ok && resume {
			if sum, err := fileSHA(path); err == nil && sum == info.SHA256 {
				log.Info("extract: dilewati (sudah selesai)", "table", ent.Name)
				continue
			}
		}
		info, err := extractTable(ctx, conn, ldb, ent, path)
		if err != nil {
			return nil, fmt.Errorf("extract %s: %w", ent.Name, err)
		}
		man.Tables[ent.Name] = info
		log.Info("extract", "table", ent.Name, "rows", info.Rows)
		if err := writeManifest(mpath, man); err != nil {
			return nil, err
		}
	}
	return man, writeManifest(mpath, man)
}

func writeManifest(path string, m *Manifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func tablePath(work, name string) string { return filepath.Join(RawDir(work), name+".jsonl.gz") }

func fileSHA(path string) (string, error) {
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

func extractTable(ctx context.Context, conn *sql.Conn, ldb *legacydb.DB, ent registry.Entity, path string) (TableInfo, error) {
	var info TableInfo
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return info, err
	}
	h := sha256.New()
	gz, _ := gzip.NewWriterLevel(io.MultiWriter(f, h), gzip.BestSpeed)
	bw := bufio.NewWriterSize(gz, 1<<20)
	fail := func(e error) (TableInfo, error) {
		_ = f.Close()
		_ = os.Remove(tmp)
		return info, e
	}
	where := ldb.Q(ent.Tenant)
	table := ldb.Table(ent.Name)
	keyset := len(ent.PK) == 1
	var last int64
	for {
		var q string
		var args []any
		if keyset {
			q = fmt.Sprintf("SELECT * FROM %s WHERE (%s) AND `%s` > ? ORDER BY `%s` LIMIT %d", table, where, ent.PK[0], ent.PK[0], PageSize)
			args = []any{last}
		} else {
			order := "`" + strings.Join(ent.PK, "`, `") + "`"
			q = fmt.Sprintf("SELECT * FROM %s WHERE (%s) ORDER BY %s", table, where, order)
		}
		n, lastPK, cols, err := streamPage(ctx, conn, q, args, bw, &info, ent.PK[0])
		if err != nil {
			return fail(err)
		}
		if len(info.Columns) == 0 {
			info.Columns = cols
		}
		if !keyset || n < PageSize {
			break
		}
		last = lastPK
	}
	if err := bw.Flush(); err != nil {
		return fail(err)
	}
	if err := gz.Close(); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return info, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return info, err
	}
	info.SHA256 = hex.EncodeToString(h.Sum(nil))
	return info, nil
}

// streamPage runs one query and appends its rows; rows are fully read and
// closed before returning (a snapshot connection serves one cursor at a time).
func streamPage(ctx context.Context, conn *sql.Conn, q string, args []any, w io.Writer, info *TableInfo, pkCol string) (int, int64, []string, error) {
	rows, err := conn.QueryContext(ctx, q, args...)
	if err != nil {
		return 0, 0, nil, err
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return 0, 0, nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return 0, 0, nil, err
	}
	vals := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	n := 0
	var lastPK int64
	enc := json.NewEncoder(w)
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return n, lastPK, cols, err
		}
		r := make(row.Row, len(cols))
		for i, c := range cols {
			r[c] = convert(types[i].DatabaseTypeName(), vals[i])
		}
		if err := enc.Encode(r); err != nil {
			return n, lastPK, cols, err
		}
		pk := r.Str(pkCol)
		if info.Rows == 0 {
			info.MinPK = pk
		}
		info.MaxPK = pk
		lastPK = r.Int(pkCol)
		info.Rows++
		n++
	}
	return n, lastPK, cols, rows.Err()
}

// convert maps a MySQL text value to its JSON scalar (design F5-3 point 3).
func convert(dbType string, v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	t := strings.ToUpper(dbType)
	switch {
	case strings.Contains(t, "INT") || t == "YEAR":
		return json.Number(v.String)
	case t == "DATETIME" || t == "TIMESTAMP":
		s := v.String
		if len(s) >= 19 {
			frac := "000000"
			if i := strings.IndexByte(s, '.'); i >= 0 {
				frac = (s[i+1:] + "000000")[:6]
			}
			return s[:10] + "T" + s[11:19] + "." + frac + "Z"
		}
		return s
	default:
		return v.String
	}
}

// Load reads every extracted table back (UseNumber keeps integers exact and
// decimals as text).
func Load(work string) (map[string][]row.Row, error) {
	out := map[string][]row.Row{}
	for _, ent := range registry.All {
		rows, err := ReadTable(work, ent.Name)
		if err != nil {
			return nil, err
		}
		out[ent.Name] = rows
	}
	return out, nil
}

// ReadTable reads one extracted table; a missing file is an error.
func ReadTable(work, name string) ([]row.Row, error) {
	f, err := os.Open(tablePath(work, name))
	if err != nil {
		return nil, fmt.Errorf("tabel %s belum diextract: %w", name, err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	dec := json.NewDecoder(bufio.NewReaderSize(gz, 1<<20))
	dec.UseNumber()
	var rows []row.Row
	for {
		var r row.Row
		if err := dec.Decode(&r); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// ParseInt is a tiny helper kept here for tests.
func ParseInt(s string) int64 { n, _ := strconv.ParseInt(s, 10, 64); return n }

// Table extracts one (possibly further restricted) entity into path with the
// same canonical format as Run; the delta export uses it.
func Table(ctx context.Context, conn *sql.Conn, ldb *legacydb.DB, ent registry.Entity, path string) (TableInfo, error) {
	return extractTable(ctx, conn, ldb, ent, path)
}
