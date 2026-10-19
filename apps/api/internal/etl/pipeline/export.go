package pipeline

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/registry"
)

// changeColumns are the legacy columns whose value moves forward when a row is
// created or changed (legacy removes nothing: it archives or cancels).
var changeColumns = []string{"updated_at", "created_at", "archived_at", "cancelled_at", "reversed_at", "posted_at", "closed_at", "confirmed_at", "received_at", "happened_at", "moved_at"}

// DeltaManifest describes one export-since run.
type DeltaManifest struct {
	Since      string                       `json:"since,omitempty"`
	SnapshotAt string                       `json:"snapshot_at"`
	CursorIn   map[string]int64             `json:"cursor_in,omitempty"`
	CursorOut  map[string]int64             `json:"cursor_out"`
	Tables     map[string]extract.TableInfo `json:"tables"`
	Predicates map[string]string            `json:"predicates"`
	Skipped    map[string]string            `json:"skipped"`
}

// DeltaDir is where export-since writes (inside the work dir).
func DeltaDir(work string) string { return filepath.Join(work, "delta") }

// ParseSince accepts RFC3339, "YYYY-MM-DD HH:MM:SS[.ffffff]" or a date (UTC).
func ParseSince(v string) (time.Time, error) {
	for _, l := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999", "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(l, strings.TrimSpace(v), time.UTC); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: --since %q tidak terbaca (RFC3339 atau YYYY-MM-DD HH:MM:SS, UTC)", cfg.ErrUsage, v)
}

func readCursor(path string) (map[string]int64, error) {
	out := map[string]int64{}
	if path == "" {
		return out, nil
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("berkas kursor %s rusak: %w", path, err)
	}
	return out, nil
}

// ExportSince extracts only the legacy rows changed after --since and/or
// beyond the id cursor of --cursor-file, from one consistent snapshot, for the
// final delta sync before cutover. A table is selected by any of its change
// columns; tables without one (link tables) can only be followed by a single
// primary-key cursor and are otherwise listed as not trackable. The cursor file
// is rewritten with the highest primary key per table so the next run only
// sees newer rows.
func ExportSince(ctx context.Context, e *Env, log *slog.Logger) (*DeltaManifest, error) {
	c := e.Cfg
	if c.Since == "" && c.CursorFile == "" {
		return nil, fmt.Errorf("%w: export-since butuh --since dan/atau --cursor-file", cfg.ErrUsage)
	}
	var since time.Time
	if c.Since != "" {
		t, err := ParseSince(c.Since)
		if err != nil {
			return nil, err
		}
		since = t
	}
	cursor, err := readCursor(c.CursorFile)
	if err != nil {
		return nil, err
	}
	dir := DeltaDir(c.WorkDir)
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	conn, err := e.Legacy.SQL.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY"); err != nil {
		return nil, fmt.Errorf("snapshot legacy: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK") }()
	man := &DeltaManifest{
		SnapshotAt: time.Now().UTC().Format(time.RFC3339), CursorIn: cursor, CursorOut: map[string]int64{},
		Tables: map[string]extract.TableInfo{}, Predicates: map[string]string{}, Skipped: map[string]string{},
	}
	if !since.IsZero() {
		man.Since = since.Format(time.RFC3339)
	}
	lit := "'" + since.Format("2006-01-02 15:04:05.000000") + "'"
	for _, ent := range registry.All {
		cols, err := tableColumns(ctx, conn, e.Legacy.Prefix+ent.Name)
		if err != nil {
			return nil, err
		}
		var preds []string
		if !since.IsZero() {
			for _, col := range changeColumns {
				if cols[col] {
					preds = append(preds, "`"+col+"` > "+lit)
				}
			}
		}
		single := len(ent.PK) == 1
		if cur, ok := cursor[ent.Name]; ok && single {
			preds = append(preds, fmt.Sprintf("`%s` > %d", ent.PK[0], cur))
		}
		if single {
			var mx sql.NullInt64
			q := fmt.Sprintf("SELECT MAX(`%s`) FROM %s WHERE (%s)", ent.PK[0], e.Legacy.Table(ent.Name), e.Legacy.Q(ent.Tenant))
			if err := conn.QueryRowContext(ctx, q).Scan(&mx); err != nil {
				return nil, fmt.Errorf("kursor %s: %w", ent.Name, err)
			}
			if mx.Valid {
				man.CursorOut[ent.Name] = mx.Int64
			}
		}
		if len(preds) == 0 {
			if !since.IsZero() || len(cursor) > 0 {
				man.Skipped[ent.Name] = "tanpa kolom perubahan dan tanpa kursor id: tidak dapat dilacak, muat penuh dari dump final"
			}
			continue
		}
		sub := ent
		sub.Tenant = "(" + ent.Tenant + ") AND (" + strings.Join(preds, " OR ") + ")"
		man.Predicates[ent.Name] = strings.Join(preds, " OR ")
		info, err := extract.Table(ctx, conn, e.Legacy, sub, filepath.Join(dir, ent.Name+".jsonl.gz"))
		if err != nil {
			return nil, fmt.Errorf("export-since %s: %w", ent.Name, err)
		}
		if info.Rows == 0 {
			_ = os.Remove(filepath.Join(dir, ent.Name+".jsonl.gz"))
			continue
		}
		man.Tables[ent.Name] = info
		log.Info("export-since", "table", ent.Name, "rows", info.Rows)
	}
	raw, _ := json.MarshalIndent(man, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		return nil, err
	}
	if c.CursorFile != "" {
		out := map[string]int64{}
		for k, v := range man.CursorOut {
			out[k] = v
		}
		raw, _ := json.MarshalIndent(out, "", "  ")
		if err := os.WriteFile(c.CursorFile, raw, 0o644); err != nil {
			return nil, err
		}
	}
	return man, nil
}

func tableColumns(ctx context.Context, conn *sql.Conn, table string) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ?`, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[strings.ToLower(n)] = true
	}
	return out, rows.Err()
}

// Rows summarises the exported row counts (tests, logs).
func (m *DeltaManifest) Rows() map[string]int64 {
	out := map[string]int64{}
	for k, v := range m.Tables {
		out[k] = v.Rows
	}
	return out
}

// Names lists exported tables in order.
func (m *DeltaManifest) Names() []string {
	var out []string
	for k := range m.Tables {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var _ = strconv.Itoa
