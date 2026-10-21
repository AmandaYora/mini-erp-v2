package transform

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

// OutDir / ReportDir are the canonical output folders inside the work dir.
func OutDir(work string) string    { return filepath.Join(work, "out") }
func ReportDir(work string) string { return filepath.Join(work, "report") }

// FileInfo is one manifest entry of the transform output.
type FileInfo struct {
	Rows   int64  `json:"rows"`
	SHA256 string `json:"sha256"`
}

// Manifest lists every output file with its row count and hash; comparing two
// manifests proves two transform runs produced identical output.
type Manifest struct {
	Files    map[string]FileInfo `json:"files"`
	Blockers int                 `json:"blockers"`
}

func writeJSONL(path string, rows []row.Row) (FileInfo, error) {
	f, err := os.Create(path)
	if err != nil {
		return FileInfo{}, err
	}
	h := sha256.New()
	bw := bufio.NewWriterSize(io.MultiWriter(f, h), 1<<20)
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
	return FileInfo{Rows: int64(len(rows)), SHA256: hex.EncodeToString(h.Sum(nil))}, f.Close()
}

func writeAny(path string, v any) (FileInfo, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return FileInfo{}, err
	}
	sum := sha256.Sum256(raw)
	return FileInfo{Rows: 1, SHA256: hex.EncodeToString(sum[:])}, os.WriteFile(path, raw, 0o644)
}

// Write stores the output under work/out and the CSV reports under work/report.
func (o *Output) Write(work string) (*Manifest, error) {
	dir := OutDir(work)
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	man := &Manifest{Files: map[string]FileInfo{}, Blockers: o.Log.Blockers()}
	for _, t := range report.SortedKeys(o.Tables) {
		fi, err := writeJSONL(filepath.Join(dir, t+".jsonl"), o.Tables[t])
		if err != nil {
			return nil, err
		}
		man.Files[t+".jsonl"] = fi
	}
	idm := make([]row.Row, 0, len(o.IDMap))
	for _, e := range o.IDMap {
		idm = append(idm, row.Row{"entity": e.Entity, "legacy_id": e.LegacyID, "legacy_sub": e.Sub, "new_id": e.NewID})
	}
	fi, err := writeJSONL(filepath.Join(dir, "id_map.jsonl"), idm)
	if err != nil {
		return nil, err
	}
	man.Files["id_map.jsonl"] = fi
	marks := make([]row.Row, 0, len(o.Marks))
	for _, m := range o.Marks {
		journals := make([]any, 0, len(m.LegacyJournals))
		for _, j := range m.LegacyJournals {
			journals = append(journals, j)
		}
		marks = append(marks, row.Row{"doc_type": m.DocType, "doc_id": m.DocID, "branch_id": m.BranchID, "state": m.State, "reason": m.Reason, "legacy_journals": journals})
	}
	if fi, err = writeJSONL(filepath.Join(dir, "posting_marks.jsonl"), marks); err != nil {
		return nil, err
	}
	man.Files["posting_marks.jsonl"] = fi
	cuts := make([]row.Row, 0, len(o.Cutovers))
	for _, c := range o.Cutovers {
		cuts = append(cuts, row.Row{"branch_id": c.BranchID, "doc_type": c.DocType, "through": c.Through})
	}
	if fi, err = writeJSONL(filepath.Join(dir, "cutovers.jsonl"), cuts); err != nil {
		return nil, err
	}
	man.Files["cutovers.jsonl"] = fi
	if fi, err = writeAny(filepath.Join(dir, "explain.json"), o.Explain); err != nil {
		return nil, err
	}
	man.Files["explain.json"] = fi
	if err := o.Log.WriteCSV(ReportDir(work)); err != nil {
		return nil, err
	}
	// Every migration correction journal line, for the accountant (ADR-0018 D11).
	cols := []string{"kind", "branch_id", "journal_number", "entry_date", "ref", "product_code", "variant_id", "stock_movement_id", "legacy_id", "amount", "debit_account", "credit_account", "reason"}
	corr := [][]string{cols}
	for _, r := range o.Tables["migration_corrections"] {
		line := make([]string, len(cols))
		for i, c := range cols {
			line[i] = r.Str(c)
		}
		corr = append(corr, line)
	}
	if err := report.WriteTable(filepath.Join(ReportDir(work), "migration_corrections.csv"), corr); err != nil {
		return nil, err
	}
	if _, err := writeAny(filepath.Join(dir, "manifest.json"), man); err != nil {
		return nil, err
	}
	return man, nil
}

// ReadManifest loads out/manifest.json.
func ReadManifest(work string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(OutDir(work), "manifest.json"))
	if err != nil {
		return nil, err
	}
	m := &Manifest{}
	return m, json.Unmarshal(raw, m)
}

// ReadRows reads one output file (without extension).
func ReadRows(work, name string) ([]row.Row, error) {
	f, err := os.Open(filepath.Join(OutDir(work), name+".jsonl"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	dec := json.NewDecoder(bufio.NewReaderSize(f, 1<<20))
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

// ReadExplain loads out/explain.json.
func ReadExplain(work string) (map[string]int64, error) {
	raw, err := os.ReadFile(filepath.Join(OutDir(work), "explain.json"))
	if err != nil {
		return nil, err
	}
	m := map[string]int64{}
	return m, json.Unmarshal(raw, &m)
}

// Tables lists the tables present in the output (sorted).
func Tables(work string) ([]string, error) {
	m, err := ReadManifest(work)
	if err != nil {
		return nil, err
	}
	var out []string
	for f := range m.Files {
		if len(f) > 6 && f[len(f)-6:] == ".jsonl" {
			out = append(out, f[:len(f)-6])
		}
	}
	sort.Strings(out)
	return out, nil
}
