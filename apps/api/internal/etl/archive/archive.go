// Package archive exports what the migration does not carry into the revamp
// (F5-12, design appendix C): whole legacy tables without a target, the columns
// dropped from migrated tables (keyed by legacy id) and the journals of the
// years before the opening date. Nothing is deleted from the source; the
// archive is the owner's proof that no data was lost, with a manifest.
package archive

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/registry"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/shared/timeutil"
)

// WholeTables are archived as they were extracted (design appendix C).
// inventory_movements keeps legacy's own balance_before/after (the revamp
// recomputes the stock card in date order); media_files keeps the archived
// attachments the media phase does not carry.
var WholeTables = []string{
	"audit_logs", "order_status_definitions", "stock_reservations", "finance_posting_sources", "finance_cash_accounts",
	"company_settings", "permissions", "inventory_movements", "media_files",
}

// DroppedColumns are the legacy columns without a revamp column.
var DroppedColumns = map[string][]string{
	"orders": {"id_assigned_user", "credit_approved", "credit_approved_by", "credit_approved_at", "payment_term_days", "goods_received_at", "goods_delivered_at",
		"metadata_json", "subtotal_amount", "subtotal_before_tax", "subtotal_after_tax"},
	"order_items":           {"pricing_source", "unit_price", "line_discount_amount", "line_discount_percent", "line_total_before_tax", "line_total_after_tax", "tax_rate_snapshot", "notes"},
	"products":              {"uom", "has_variants", "attributes_json", "summary_text"},
	"product_variants":      {"attributes_json", "is_hidden", "sort_order"},
	"business_parties":      {"metadata_json"},
	"users":                 {"last_login_at", "display_title", "phone_e164"},
	"payment_allocations":   {"allocation_method", "allocated_at"},
	"sales_returns":         {"source", "cancelled_by", "id_related_party", "updated_at"},
	"purchase_returns":      {"cancels_order", "returned_subtotal_before_tax", "updated_at"},
	"sales_return_items":    {"product_name_snapshot", "product_code_snapshot", "unit_price", "line_discount_amount", "line_discount_percent", "line_total_before_tax", "tax_base_amount", "id_inventory_movement", "notes"},
	"purchase_return_items": {"product_name_snapshot", "product_code_snapshot", "line_total_before_tax", "tax_base_amount", "id_inventory_movement", "notes"},
}

// FileInfo is one archive file.
type FileInfo struct {
	Rows   int64  `json:"rows"`
	SHA256 string `json:"sha256"`
}

// Manifest lists the archive.
type Manifest struct {
	Files map[string]FileInfo `json:"files"`
}

// Dir is the archive folder inside the work dir.
func Dir(work string) string { return filepath.Join(work, "archive") }

func writeGz(path string, rows []row.Row) (FileInfo, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return FileInfo{}, err
	}
	f, err := os.Create(path)
	if err != nil {
		return FileInfo{}, err
	}
	h := sha256.New()
	gz, _ := gzip.NewWriterLevel(io.MultiWriter(f, h), gzip.BestSpeed)
	bw := bufio.NewWriter(gz)
	enc := json.NewEncoder(bw)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			_ = f.Close()
			return FileInfo{}, err
		}
	}
	if err := bw.Flush(); err != nil {
		_ = f.Close()
		return FileInfo{}, err
	}
	if err := gz.Close(); err != nil {
		_ = f.Close()
		return FileInfo{}, err
	}
	return FileInfo{Rows: int64(len(rows)), SHA256: hex.EncodeToString(h.Sum(nil))}, f.Close()
}

// Run writes the archive and its manifest.
func Run(c *cfg.Config, work string) (*Manifest, error) {
	dir := Dir(work)
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	man := &Manifest{Files: map[string]FileInfo{}}
	for _, t := range WholeTables {
		rows, err := extract.ReadTable(work, t)
		if err != nil {
			return nil, err
		}
		fi, err := writeGz(filepath.Join(dir, "raw", t+".jsonl.gz"), rows)
		if err != nil {
			return nil, err
		}
		man.Files["raw/"+t+".jsonl.gz"] = fi
	}
	for t, cols := range DroppedColumns {
		rows, err := extract.ReadTable(work, t)
		if err != nil {
			return nil, err
		}
		ent, _ := registry.ByName(t)
		pk := ent.PK[0]
		var out []row.Row
		for _, r := range rows {
			kept := row.Row{"legacy_id": r[pk]}
			for _, col := range cols {
				if v, ok := r[col]; ok {
					kept[col] = v
				}
			}
			out = append(out, kept)
		}
		fi, err := writeGz(filepath.Join(dir, "legacy_columns", t+".jsonl.gz"), out)
		if err != nil {
			return nil, err
		}
		man.Files["legacy_columns/"+t+".jsonl.gz"] = fi
	}
	// journals before the opening date: the opening balance replaces their detail
	entries, err := extract.ReadTable(work, "finance_journal_entries")
	if err != nil {
		return nil, err
	}
	lines, err := extract.ReadTable(work, "finance_journal_lines")
	if err != nil {
		return nil, err
	}
	prior := map[int64]bool{}
	var pe, pl []row.Row
	for _, e := range entries {
		t, _ := e.Time("journal_date")
		if t.In(timeutil.Jakarta).Format("2006-01-02") < c.OpeningDate {
			prior[e.Int("id_finance_journal_entry")] = true
			pe = append(pe, e)
		}
	}
	for _, l := range lines {
		if prior[l.Int("id_finance_journal_entry")] {
			pl = append(pl, l)
		}
	}
	for name, rows := range map[string][]row.Row{"journals_prior/entries.jsonl.gz": pe, "journals_prior/lines.jsonl.gz": pl} {
		fi, err := writeGz(filepath.Join(dir, name), rows)
		if err != nil {
			return nil, err
		}
		man.Files[name] = fi
	}
	raw, _ := json.MarshalIndent(struct {
		Files   map[string]FileInfo `json:"files"`
		Created string              `json:"created"`
	}{man.Files, time.Now().UTC().Format(time.RFC3339)}, "", "  ")
	return man, os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644)
}
