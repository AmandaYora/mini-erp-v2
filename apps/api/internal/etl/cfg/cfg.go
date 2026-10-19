// Package cfg holds the ETL configuration and its flag parsing (F5-1).
//
// Required flags have NO default on purpose: a cutover tool that guesses the
// company or the opening date will eventually guess wrong. DSNs may come from
// the environment (ETL_LEGACY_DSN / ETL_TARGET_DSN) so that passwords stay out
// of shell history; they are never printed (see guard.Redact).
package cfg

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// ErrUsage marks a wrong invocation; the CLI maps it to exit code 2.
var ErrUsage = errors.New("penggunaan salah")

// Config is everything a phase needs.
type Config struct {
	CompanyID   int64
	OpeningDate string // YYYY-MM-DD, WIB calendar date
	FreezeAt    string // informational: when legacy was frozen (RFC3339), recorded in the run report

	LegacyDSN    string
	TargetDSN    string
	LegacyPrefix string // table-name prefix of the legacy copy (tests: "legacy_")
	StagePrefix  string // prefix of ETL bookkeeping tables in the target DB

	WorkDir  string
	MediaSrc string
	MediaDst string
	RunID    string

	Resume              bool
	AllowTargetDB       string // exact database name explicitly allowed besides *_test / *_rehearsal
	AllowNonEmptyTarget bool
	AllowRemoteLegacy   bool
	KeepMissingMedia    bool
	ExpectFingerprint   string
	MaxBalanceDelta     int64

	// F5-14 operations.
	SelfTest    bool   // verify: prove the verifier catches deliberate corruption
	Date        string // daily-verify: WIB calendar date YYYY-MM-DD
	Since       string // export-since: UTC timestamp (RFC3339, "YYYY-MM-DD HH:MM:SS" or a date)
	CursorFile  string // export-since: JSON {table: last primary key} read and rewritten
	ExplainFile string // daily-verify: CSV "key,reason" of deltas the operator explained
	CompareWith string // rehearse: work dir of an earlier rehearsal (hash comparison, L1 vs L2)
	FullVerify  bool   // daily-verify: also run the full verify checks
	// TODO(owner): --mem-budget (bbolt lookup) is not implemented; transform
	// keeps lookups in memory.
}

// Parse reads flags for one subcommand. Unknown flags are an error.
func Parse(args []string, out io.Writer) (*Config, error) {
	c := &Config{}
	fs := flag.NewFlagSet("etl", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Int64Var(&c.CompanyID, "company-id", 0, "id perusahaan legacy (WAJIB)")
	fs.StringVar(&c.OpeningDate, "opening-date", "", "tanggal saldo awal WIB, YYYY-MM-DD (WAJIB)")
	fs.StringVar(&c.FreezeAt, "freeze-at", "", "waktu freeze legacy (RFC3339), dicatat di laporan")
	fs.StringVar(&c.LegacyDSN, "legacy-dsn", os.Getenv("ETL_LEGACY_DSN"), "DSN salinan legacy (atau env ETL_LEGACY_DSN)")
	fs.StringVar(&c.TargetDSN, "target-dsn", os.Getenv("ETL_TARGET_DSN"), "DSN target revamp (atau env ETL_TARGET_DSN)")
	fs.StringVar(&c.LegacyPrefix, "legacy-prefix", "", "awalan nama tabel legacy")
	fs.StringVar(&c.StagePrefix, "stage-prefix", "etl_", "awalan tabel pembukuan ETL di DB target")
	fs.StringVar(&c.WorkDir, "workdir", "", "folder kerja (raw/, out/, report/) (WAJIB)")
	fs.StringVar(&c.MediaSrc, "media-src", "", "folder berkas media legacy")
	fs.StringVar(&c.MediaDst, "media-dst", "", "folder tujuan media revamp (harus berbeda dari sumber)")
	fs.StringVar(&c.RunID, "run-id", "", "id lari (default: dari opening-date + company-id)")
	fs.BoolVar(&c.Resume, "resume", false, "lanjutkan lari yang sama pada target yang sudah terisi")
	fs.StringVar(&c.AllowTargetDB, "allow-target-db", "", "nama DB target yang diizinkan secara eksplisit")
	fs.BoolVar(&c.AllowNonEmptyTarget, "allow-non-empty-target", false, "izinkan target berisi data (berbahaya)")
	fs.BoolVar(&c.AllowRemoteLegacy, "allow-remote-legacy", false, "izinkan host legacy non-lokal")
	fs.BoolVar(&c.KeepMissingMedia, "keep-missing-media", false, "tetap buat baris media untuk berkas yang hilang")
	fs.StringVar(&c.ExpectFingerprint, "expect-fingerprint", "", "sidik jari schema target yang diharapkan")
	fs.Int64Var(&c.MaxBalanceDelta, "max-balance-delta", 5, "selisih maksimum yang boleh diserap saat menyeimbangkan jurnal (rupiah)")
	fs.BoolVar(&c.SelfTest, "self-test", false, "verify: rusak sengaja salinan target (dalam transaksi yang dibatalkan) dan buktikan verifier menangkapnya")
	fs.StringVar(&c.Date, "date", "", "daily-verify: tanggal WIB YYYY-MM-DD")
	fs.StringVar(&c.Since, "since", "", "export-since: hanya baris berubah sesudah waktu UTC ini")
	fs.StringVar(&c.CursorFile, "cursor-file", "", "export-since: berkas kursor JSON {tabel: pk terakhir}")
	fs.StringVar(&c.ExplainFile, "explain-file", "", "daily-verify: CSV kunci,alasan untuk selisih yang sudah dijelaskan")
	fs.StringVar(&c.CompareWith, "compare-with", "", "rehearse: folder kerja gladi sebelumnya untuk membandingkan hash keluaran")
	fs.BoolVar(&c.FullVerify, "full-verify", false, "daily-verify: jalankan juga seluruh pemeriksaan verify")
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUsage, err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c.RunID == "" {
		c.RunID = fmt.Sprintf("c%d-%s", c.CompanyID, c.OpeningDate)
	}
	return c, nil
}

// Validate enforces the no-default flags.
func (c *Config) Validate() error {
	if c.CompanyID <= 0 {
		return fmt.Errorf("%w: --company-id wajib dan tidak punya default", ErrUsage)
	}
	if c.OpeningDate == "" {
		return fmt.Errorf("%w: --opening-date wajib dan tidak punya default", ErrUsage)
	}
	if _, err := time.Parse("2006-01-02", c.OpeningDate); err != nil {
		return fmt.Errorf("%w: --opening-date harus YYYY-MM-DD", ErrUsage)
	}
	if c.WorkDir == "" {
		return fmt.Errorf("%w: --workdir wajib", ErrUsage)
	}
	if c.MediaSrc != "" && c.MediaSrc == c.MediaDst {
		return fmt.Errorf("%w: --media-dst tidak boleh sama dengan --media-src", ErrUsage)
	}
	return nil
}

// OpeningTime returns the opening date as a midnight UTC time (calendar date).
func (c *Config) OpeningTime() time.Time {
	t, _ := time.Parse("2006-01-02", c.OpeningDate)
	return t
}
