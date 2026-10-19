// Package pipeline wires the ETL phases together behind the safety guards. The
// command (cmd/etl) and the fixture tests both go through it, so what the tests
// exercise is what the operator runs.
//
// Phase order of `all`: preflight, extract, transform, media, load, sequences,
// archive, verify. Each phase can also be run on its own and is re-runnable.
package pipeline

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"mini-erp/internal/etl/archive"
	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/guard"
	"mini-erp/internal/etl/legacydb"
	"mini-erp/internal/etl/load"
	"mini-erp/internal/etl/mediacopy"
	"mini-erp/internal/etl/preflight"
	"mini-erp/internal/etl/sequences"
	"mini-erp/internal/etl/transform"
	"mini-erp/internal/etl/verify"
)

// ErrBlockers means a phase finished but found blocking problems (exit 1).
var ErrBlockers = errors.New("ada temuan blocker")

// Phases in execution order of `all`.
var Phases = []string{"preflight", "extract", "transform", "media", "load", "sequences", "archive", "verify"}

// Env holds the opened connections.
type Env struct {
	Legacy *legacydb.DB
	Target *sql.DB
	Cfg    *cfg.Config
	Log    *slog.Logger
}

// Close releases both pools.
func (e *Env) Close() {
	if e.Legacy != nil {
		_ = e.Legacy.Close()
	}
	if e.Target != nil {
		_ = e.Target.Close()
	}
}

func openTarget(dsn string) (*sql.DB, error) {
	c, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return nil, errors.New("DSN target tidak terbaca")
	}
	c.ParseTime = true
	c.Loc = time.UTC
	c.Params = map[string]string{"time_zone": "'+00:00'"}
	conn, err := mysqldriver.NewConnector(c)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(conn)
	db.SetMaxOpenConns(10)
	return db, nil
}

// Open applies every guard that needs connections (design F5-1):
// legacy host/marker, target name whitelist, and "legacy != target".
func Open(ctx context.Context, c *cfg.Config, log *slog.Logger, needLegacy, needTarget bool) (*Env, error) {
	e := &Env{Cfg: c, Log: log}
	if err := guard.CheckIdent(c.LegacyPrefix); err != nil {
		return nil, err
	}
	if err := guard.CheckIdent(c.StagePrefix); err != nil {
		return nil, err
	}
	if needLegacy {
		if c.LegacyDSN == "" {
			return nil, fmt.Errorf("%w: --legacy-dsn (atau ETL_LEGACY_DSN) wajib", cfg.ErrUsage)
		}
		if err := guard.CheckLegacyHost(c.LegacyDSN, c.AllowRemoteLegacy); err != nil {
			return nil, err
		}
		l, err := legacydb.Open(c.LegacyDSN, c.LegacyPrefix, c.CompanyID)
		if err != nil {
			return nil, err
		}
		e.Legacy = l
		log.Info("legacy", "dsn", guard.Redact(c.LegacyDSN), "prefix", c.LegacyPrefix)
		if err := guard.LegacyMarker(ctx, l.SQL, c.LegacyPrefix+"etl_copy_marker"); err != nil {
			e.Close()
			return nil, err
		}
	}
	if needTarget {
		if c.TargetDSN == "" {
			e.Close()
			return nil, fmt.Errorf("%w: --target-dsn (atau ETL_TARGET_DSN) wajib", cfg.ErrUsage)
		}
		name, err := guard.DBName(c.TargetDSN)
		if err != nil {
			e.Close()
			return nil, err
		}
		if err := guard.CheckTargetName(name, c.AllowTargetDB); err != nil {
			e.Close()
			return nil, err
		}
		if needLegacy && c.LegacyDSN != "" {
			if ln, err := guard.DBName(c.LegacyDSN); err == nil && ln == name && c.LegacyPrefix == "" {
				e.Close()
				return nil, errors.New("legacy dan target menunjuk database yang sama tanpa --legacy-prefix: ditolak")
			}
		}
		t, err := openTarget(c.TargetDSN)
		if err != nil {
			e.Close()
			return nil, err
		}
		e.Target = t
		log.Info("target", "dsn", guard.Redact(c.TargetDSN))
		if err := t.PingContext(ctx); err != nil {
			e.Close()
			return nil, fmt.Errorf("target tidak terjangkau: %w", err)
		}
	}
	return e, nil
}

// Run executes one phase (or "all").
func Run(ctx context.Context, phase string, c *cfg.Config, log *slog.Logger) error {
	if phase == "all" {
		for _, p := range Phases {
			if p == "media" && c.MediaSrc == "" {
				log.Info("fase media dilewati: --media-src kosong")
				continue
			}
			if err := Run(ctx, p, c, log); err != nil {
				return fmt.Errorf("fase %s: %w", p, err)
			}
		}
		return nil
	}
	switch phase {
	case "preflight":
		e, err := Open(ctx, c, log, true, false)
		if err != nil {
			return err
		}
		defer e.Close()
		rep, err := preflight.Run(ctx, e.Legacy, c, c.WorkDir)
		if err != nil {
			return err
		}
		log.Info("preflight", "blockers", rep.Blockers, "facts", len(rep.Facts))
		if rep.Blockers > 0 {
			return fmt.Errorf("%w: preflight %d (lihat report/facts.md)", ErrBlockers, rep.Blockers)
		}
	case "extract":
		e, err := Open(ctx, c, log, true, false)
		if err != nil {
			return err
		}
		defer e.Close()
		_, err = extract.Run(ctx, e.Legacy, c.WorkDir, c.Resume, log)
		return err
	case "transform":
		raw, err := extract.Load(c.WorkDir)
		if err != nil {
			return err
		}
		out, err := transform.Run(transform.Input{Cfg: c, Raw: raw})
		if err != nil {
			return err
		}
		man, err := out.Write(c.WorkDir)
		if err != nil {
			return err
		}
		log.Info("transform", "files", len(man.Files), "blockers", man.Blockers)
		if man.Blockers > 0 {
			return fmt.Errorf("%w: transform %d (lihat report/findings.csv)", ErrBlockers, man.Blockers)
		}
	case "media":
		rep, err := mediacopy.Run(ctx, c, c.WorkDir)
		if err != nil {
			return err
		}
		log.Info("media", "copied", rep.Copied, "identical", rep.Skipped, "missing", rep.Missing, "bytes", rep.Bytes)
	case "load":
		e, err := Open(ctx, c, log, false, true)
		if err != nil {
			return err
		}
		defer e.Close()
		res, err := load.Run(ctx, e.Target, c, c.WorkDir, log)
		if err != nil {
			return err
		}
		for _, w := range res.Warnings {
			log.Warn("load", "peringatan", w)
		}
		log.Info("load selesai", "inserted", res.Inserted())
	case "sequences":
		e, err := Open(ctx, c, log, false, true)
		if err != nil {
			return err
		}
		defer e.Close()
		rep, err := sequences.Rebuild(ctx, e.Target, c.WorkDir)
		if err != nil {
			return err
		}
		log.Info("sequences", "branch", len(rep.Branch), "finance", len(rep.Finance))
	case "archive":
		m, err := archive.Run(c, c.WorkDir)
		if err != nil {
			return err
		}
		log.Info("archive", "files", len(m.Files))
	case "verify":
		e, err := Open(ctx, c, log, true, true)
		if err != nil {
			return err
		}
		defer e.Close()
		if c.SelfTest {
			st, err := verify.SelfTest(ctx, e.Legacy, e.Target, c, c.WorkDir)
			if st != nil {
				if werr := verify.WriteSelfTest(c.WorkDir, st); werr != nil {
					return werr
				}
			}
			if err != nil {
				return fmt.Errorf("%w: %v", ErrBlockers, err)
			}
			if !st.Pass {
				return fmt.Errorf("%w: self-test: verifier tidak menangkap %s", ErrBlockers, strings.Join(st.Missed(), ", "))
			}
			log.Info("verify --self-test LULUS", "cases", len(st.Cases))
			return nil
		}
		rep, err := verify.Run(ctx, e.Legacy, e.Target, c, c.WorkDir)
		if err != nil {
			return err
		}
		if err := verify.WriteReport(c.WorkDir, rep); err != nil {
			return err
		}
		if !rep.Pass {
			return fmt.Errorf("%w: verify GAGAL pada %s", ErrBlockers, strings.Join(rep.Failed(), ", "))
		}
		log.Info("verify LULUS", "checks", len(rep.Checks))
	case "rehearse":
		rep, err := Rehearse(ctx, c, log)
		if rep != nil {
			log.Info("rehearse", "pass", rep.Pass, "seconds", rep.TotalSeconds, "report", "report/rehearsal.md")
		}
		return err
	case "export-since":
		e, err := Open(ctx, c, log, true, false)
		if err != nil {
			return err
		}
		defer e.Close()
		man, err := ExportSince(ctx, e, log)
		if err != nil {
			return err
		}
		log.Info("export-since selesai", "tables", len(man.Tables), "skipped", len(man.Skipped), "dir", "delta/")
	case "daily-verify":
		e, err := Open(ctx, c, log, true, true)
		if err != nil {
			return err
		}
		defer e.Close()
		rep, err := verify.Daily(ctx, e.Legacy, e.Target, c, c.WorkDir, c.FullVerify)
		if err != nil {
			return err
		}
		md, err := verify.WriteDaily(c.WorkDir, rep)
		if err != nil {
			return err
		}
		fmt.Print(md)
		if !rep.Pass {
			return fmt.Errorf("%w: daily-verify %s: %d selisih tak terjelaskan", ErrBlockers, rep.Date, rep.Unexplained)
		}
	default:
		return fmt.Errorf("%w: fase %q tidak dikenal", cfg.ErrUsage, phase)
	}
	return nil
}
