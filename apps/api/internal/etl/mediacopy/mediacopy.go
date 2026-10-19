// Package mediacopy copies the attachment files to the revamp media layout and
// produces the final media_files rows (F5-9). A row is only produced for a
// file that exists and was copied intact: a dead link in the UI is worse than a
// reported gap. The destination must be a different folder (or bucket) than the
// source, because the revamp deletes objects it owns.
package mediacopy

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/etl/transform"
)

// FileInfo is one copied file.
type FileInfo struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Report summarises a run.
type Report struct {
	Copied  int                 `json:"copied"`
	Skipped int                 `json:"skipped_identical"`
	Missing int                 `json:"missing"`
	Bytes   int64               `json:"bytes"`
	Files   map[string]FileInfo `json:"files"`
}

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

func hashFile(path string) (FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileInfo{}, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return FileInfo{Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}, err
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// Run copies the files of out/media_candidates.jsonl.
func Run(ctx context.Context, c *cfg.Config, work string) (*Report, error) {
	if c.MediaSrc == "" || c.MediaDst == "" {
		return nil, errors.New("--media-src dan --media-dst wajib untuk fase media")
	}
	srcRoot, _ := filepath.Abs(c.MediaSrc)
	dstRoot, _ := filepath.Abs(c.MediaDst)
	if srcRoot == dstRoot {
		return nil, errors.New("tujuan media tidak boleh sama dengan sumber")
	}
	cands, err := transform.ReadRows(work, "media_candidates")
	if err != nil {
		return nil, err
	}
	rep := &Report{Files: map[string]FileInfo{}}
	var final []row.Row
	var missing [][]string
	missing = append(missing, []string{"legacy_id", "owner_type", "owner_id", "src_key", "reason"})
	seenKey := map[string]bool{}
	for _, cand := range cands {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := cand.Str("file_key")
		if seenKey[key] {
			missing = append(missing, []string{cand.Str("legacy_id"), cand.Str("owner_type"), cand.Str("owner_id"), cand.Str("src_key"), "duplicate_file_key"})
			continue
		}
		src := filepath.Join(srcRoot, filepath.FromSlash(cand.Str("src_key")))
		dst := filepath.Join(dstRoot, filepath.FromSlash(key))
		if !within(srcRoot, src) || !within(dstRoot, dst) {
			return nil, fmt.Errorf("kunci berkas di luar folder: %q", cand.Str("src_key"))
		}
		si, err := hashFile(src)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			rep.Missing++
			missing = append(missing, []string{cand.Str("legacy_id"), cand.Str("owner_type"), cand.Str("owner_id"), cand.Str("src_key"), "missing_source"})
			if !c.KeepMissingMedia {
				continue
			}
			si = FileInfo{Size: cand.Int("size_bytes")}
		} else {
			if di, err := hashFile(dst); err == nil {
				if di.SHA256 != si.SHA256 {
					return nil, fmt.Errorf("berkas tujuan %s sudah ada dengan isi berbeda", key)
				}
				rep.Skipped++
			} else {
				if err := copyFile(src, dst); err != nil {
					return nil, err
				}
				di, err := hashFile(dst)
				if err != nil || di.SHA256 != si.SHA256 || di.Size != si.Size {
					return nil, fmt.Errorf("verifikasi salinan %s gagal", key)
				}
				rep.Copied++
				rep.Bytes += si.Size
			}
			if want := cand.Int("size_bytes"); want > 0 && want != si.Size {
				missing = append(missing, []string{cand.Str("legacy_id"), cand.Str("owner_type"), cand.Str("owner_id"), cand.Str("src_key"),
					"size_mismatch legacy=" + strconv.FormatInt(want, 10) + " actual=" + strconv.FormatInt(si.Size, 10)})
			}
			rep.Files[key] = si
		}
		seenKey[key] = true
		final = append(final, row.Row{"owner_type": cand["owner_type"], "owner_id": cand["owner_id"], "file_key": key, "original_name": cand["original_name"],
			"mime": cand["mime"], "size_bytes": si.Size, "is_primary": cand["is_primary"], "created_by": nil, "origin": "legacy"})
	}
	if err := writeFinal(work, final); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(transform.ReportDir(work), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(transform.ReportDir(work), "missing_media.csv"))
	if err != nil {
		return nil, err
	}
	w := csv.NewWriter(f)
	_ = w.WriteAll(missing)
	_ = f.Close()
	raw, _ := json.MarshalIndent(rep, "", "  ")
	return rep, os.WriteFile(filepath.Join(transform.OutDir(work), "media_report.json"), raw, 0o644)
}

func writeFinal(work string, rows []row.Row) error {
	f, err := os.Create(filepath.Join(transform.OutDir(work), "media_files_final.jsonl"))
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			_ = f.Close()
			return err
		}
	}
	return f.Close()
}

// ReadReport loads out/media_report.json.
func ReadReport(work string) (*Report, error) {
	raw, err := os.ReadFile(filepath.Join(transform.OutDir(work), "media_report.json"))
	if err != nil {
		return nil, err
	}
	r := &Report{}
	return r, json.Unmarshal(raw, r)
}
