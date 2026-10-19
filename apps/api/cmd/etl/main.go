// Command etl is the legacy -> revamp data migration tool (Fase 6 / F5).
//
//	etl <phase> --company-id N --opening-date YYYY-MM-DD --workdir DIR [flags]
//
// Phases: preflight extract transform media load sequences archive verify all.
// Connection strings come from ETL_LEGACY_DSN / ETL_TARGET_DSN (or the
// --legacy-dsn / --target-dsn flags); they are never printed.
//
// Safety: --company-id and --opening-date have no defaults; the legacy
// connection must be a local COPY carrying the etl_copy_marker table; the
// target database must be named *_test or *_rehearsal (or be allowed
// explicitly with --allow-target-db) and must be empty unless --resume.
//
// Run the database tests of this tool through the shared lock:
//
//	node scripts/db-lock.mjs -- go test ./internal/etl/... -count=1 -timeout=30m
//
// F5-14 operations: rehearse (preflight..verify on a *_rehearsal target with a
// timing/report file), daily-verify --date D (day aggregates legacy vs target
// and a delta report), export-since --since T (rows changed after a timestamp
// or id cursor, for the final delta sync). verify --self-test corrupts a
// transaction-scoped copy of the target and proves the verifier notices.
//
// Exit codes: 0 clean, 1 blocker findings or failed verification, 2 usage.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/pipeline"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, "usage: etl <preflight|extract|transform|media|load|sequences|archive|verify|all|rehearse|daily-verify|export-since> --company-id N --opening-date YYYY-MM-DD --workdir DIR [flags]")
		return 2
	}
	phase := args[0]
	c, err := cfg.Parse(args[1:], os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := pipeline.Run(ctx, phase, c, log); err != nil {
		fmt.Fprintln(os.Stderr, "etl:", err)
		if errors.Is(err, cfg.ErrUsage) {
			return 2
		}
		return 1
	}
	return 0
}
