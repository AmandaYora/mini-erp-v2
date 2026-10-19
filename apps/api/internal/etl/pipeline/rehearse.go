package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/guard"
	"mini-erp/internal/etl/transform"
)

// PhaseTiming is one line of the rehearsal report.
type PhaseTiming struct {
	Phase   string  `json:"phase"`
	Seconds float64 `json:"seconds"`
	Status  string  `json:"status"` // LULUS | GAGAL | DILEWATI
	Detail  string  `json:"detail,omitempty"`
}

// Rehearsal is the timing/report file of one gladi (design F5-14, L1/L2).
type Rehearsal struct {
	RunID          string            `json:"run_id"`
	Target         string            `json:"target_database"`
	StartedAt      string            `json:"started_at"`
	TotalSeconds   float64           `json:"total_seconds"`
	Phases         []PhaseTiming     `json:"phases"`
	Pass           bool              `json:"pass"`
	Findings       map[string]int    `json:"findings_by_severity,omitempty"`
	OutputRows     map[string]int64  `json:"output_rows,omitempty"`
	ManifestDigest string            `json:"manifest_digest,omitempty"`
	Compare        *RehearsalCompare `json:"compare,omitempty"`
	VerifyFailed   []string          `json:"verify_failed,omitempty"`
}

// RehearsalCompare is the L1-vs-L2 determinism check against an earlier gladi.
type RehearsalCompare struct {
	With      string   `json:"with"`
	Identical bool     `json:"identical"`
	Diffs     []string `json:"diffs,omitempty"`
}

// RehearsalPhases is the order a gladi runs (media only with --media-src).
var RehearsalPhases = []string{"preflight", "extract", "transform", "media", "load", "sequences", "archive", "verify"}

func manifestDigest(m *transform.Manifest) string {
	names := make([]string, 0, len(m.Files))
	for n := range m.Files {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		// the id map holds allocator output only; every file is part of the digest
		fmt.Fprintf(h, "%s=%s\n", n, m.Files[n].SHA256)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Rehearse runs preflight -> extract -> transform -> (media) -> load ->
// sequences -> archive -> verify against a *_rehearsal target and writes
// report/rehearsal.json|md with the time of every phase. The target must be
// empty (the load guard) and must carry the _rehearsal suffix, or be named
// explicitly with --allow-target-db. Nothing is dropped or truncated here: a
// fresh L2 means a fresh database, made by the operator.
func Rehearse(ctx context.Context, c *cfg.Config, log *slog.Logger) (*Rehearsal, error) {
	name, err := guard.DBName(c.TargetDSN)
	if err != nil {
		return nil, fmt.Errorf("%w: --target-dsn wajib untuk rehearse", cfg.ErrUsage)
	}
	if !strings.HasSuffix(name, "_rehearsal") && name != c.AllowTargetDB {
		return nil, fmt.Errorf("%w: rehearse hanya ke database berakhiran _rehearsal (atau --allow-target-db sama persis); target %q ditolak", cfg.ErrUsage, name)
	}
	rep := &Rehearsal{RunID: c.RunID, Target: name, StartedAt: time.Now().UTC().Format(time.RFC3339), Pass: true}
	started := time.Now()
	var runErr error
	for _, p := range RehearsalPhases {
		if p == "media" && c.MediaSrc == "" {
			rep.Phases = append(rep.Phases, PhaseTiming{Phase: p, Status: "DILEWATI", Detail: "--media-src kosong"})
			continue
		}
		t0 := time.Now()
		err := Run(ctx, p, c, log)
		pt := PhaseTiming{Phase: p, Seconds: time.Since(t0).Seconds(), Status: "LULUS"}
		if err != nil {
			pt.Status, pt.Detail = "GAGAL", err.Error()
			rep.Pass = false
			rep.Phases = append(rep.Phases, pt)
			runErr = fmt.Errorf("fase %s: %w", p, err)
			break
		}
		rep.Phases = append(rep.Phases, pt)
	}
	rep.TotalSeconds = time.Since(started).Seconds()
	rep.Findings = findingCounts(c.WorkDir)
	if man, err := transform.ReadManifest(c.WorkDir); err == nil {
		rep.ManifestDigest = manifestDigest(man)
		rep.OutputRows = map[string]int64{}
		for n, f := range man.Files {
			rep.OutputRows[strings.TrimSuffix(n, ".jsonl")] = f.Rows
		}
		if c.CompareWith != "" {
			rep.Compare = compareManifests(c.CompareWith, man)
			if !rep.Compare.Identical {
				rep.Pass = false
				if runErr == nil {
					runErr = fmt.Errorf("%w: keluaran transform berbeda dari gladi sebelumnya (%d berkas)", ErrBlockers, len(rep.Compare.Diffs))
				}
			}
		}
	}
	if raw, err := os.ReadFile(filepath.Join(transform.ReportDir(c.WorkDir), "verify.json")); err == nil {
		var v struct {
			Checks []struct{ ID, Status string } `json:"checks"`
		}
		if json.Unmarshal(raw, &v) == nil {
			for _, ch := range v.Checks {
				if ch.Status != "LULUS" {
					rep.VerifyFailed = append(rep.VerifyFailed, ch.ID)
				}
			}
		}
	}
	if werr := writeRehearsal(c.WorkDir, rep); werr != nil && runErr == nil {
		runErr = werr
	}
	return rep, runErr
}

func compareManifests(other string, mine *transform.Manifest) *RehearsalCompare {
	cmp := &RehearsalCompare{With: other, Identical: true}
	theirs, err := transform.ReadManifest(other)
	if err != nil {
		cmp.Identical = false
		cmp.Diffs = []string{"manifest gladi pembanding tidak terbaca: " + err.Error()}
		return cmp
	}
	names := map[string]bool{}
	for n := range mine.Files {
		names[n] = true
	}
	for n := range theirs.Files {
		names[n] = true
	}
	var all []string
	for n := range names {
		all = append(all, n)
	}
	sort.Strings(all)
	for _, n := range all {
		a, aok := mine.Files[n]
		b, bok := theirs.Files[n]
		switch {
		case !aok:
			cmp.Diffs = append(cmp.Diffs, n+": hanya ada di gladi pembanding")
		case !bok:
			cmp.Diffs = append(cmp.Diffs, n+": hanya ada di gladi ini")
		case a.SHA256 != b.SHA256:
			cmp.Diffs = append(cmp.Diffs, fmt.Sprintf("%s: hash berbeda (baris %d vs %d)", n, a.Rows, b.Rows))
		}
	}
	cmp.Identical = len(cmp.Diffs) == 0
	return cmp
}

func findingCounts(work string) map[string]int {
	f, err := os.Open(filepath.Join(transform.ReportDir(work), "findings.csv"))
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil || len(rows) < 2 {
		return nil
	}
	out := map[string]int{}
	for _, r := range rows[1:] {
		out[r[0]]++
	}
	return out
}

func writeRehearsal(work string, r *Rehearsal) error {
	dir := transform.ReportDir(work)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "rehearsal.json"), raw, 0o644); err != nil {
		return err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Gladi %s\n\nTarget `%s`, mulai %s, total %.1f detik.\n\n| fase | detik | status | catatan |\n|---|---|---|---|\n", r.RunID, r.Target, r.StartedAt, r.TotalSeconds)
	for _, p := range r.Phases {
		fmt.Fprintf(&sb, "| %s | %.2f | %s | %s |\n", p.Phase, p.Seconds, p.Status, strings.ReplaceAll(p.Detail, "|", "/"))
	}
	if len(r.Findings) > 0 {
		fmt.Fprintf(&sb, "\nTemuan: blocker=%d warn=%d info=%d\n", r.Findings["blocker"], r.Findings["warn"], r.Findings["info"])
	}
	if r.ManifestDigest != "" {
		fmt.Fprintf(&sb, "\nDigest keluaran transform: `%s`\n", r.ManifestDigest)
	}
	if r.Compare != nil {
		if r.Compare.Identical {
			fmt.Fprintf(&sb, "\nBanding dengan `%s`: **identik** (L1 = L2).\n", r.Compare.With)
		} else {
			fmt.Fprintf(&sb, "\nBanding dengan `%s`: **BERBEDA**\n", r.Compare.With)
			for _, d := range r.Compare.Diffs {
				sb.WriteString("- " + d + "\n")
			}
		}
	}
	if len(r.VerifyFailed) > 0 {
		sb.WriteString("\nVerify gagal pada: " + strings.Join(r.VerifyFailed, ", ") + "\n")
	}
	if r.Pass {
		sb.WriteString("\n**LULUS**\n")
	} else {
		sb.WriteString("\n**GAGAL**\n")
	}
	return os.WriteFile(filepath.Join(dir, "rehearsal.md"), []byte(sb.String()), 0o644)
}

var _ = errors.New
