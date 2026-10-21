// Command costrebuild is the cost restatement tool (OP-04, legacy `cost:rebuild`).
//
//	costrebuild --branch-id N [--commit] [--date YYYY-MM-DD] [--actor-id N] [--json]
//
// Without --commit it is a DRY RUN: it replays every position's cost chain from
// the ledger, prints the per-product difference and the gates a commit would
// meet, and writes nothing. With --commit it rewrites the stale outflow rows
// and posts ONE balanced correction journal (Persediaan against the account each
// restated outflow was booked to — HPP, selisih persediaan, hapus stok, selisih
// retur beli — source `cost_restatement`) in a single transaction, behind the trial-balance gate,
// and writes an audit entry (`finance.cost_restatement`). Afterwards the K-05
// book-vs-ledger reconciliation is printed.
//
// It is an operator command, not an endpoint: it runs in the app container
// (`docker compose exec app ./costrebuild ...`) against the host database.
// Like the dev seed it drives the application services through the composition
// root, so it uses exactly the production wiring and rules.
//
// Exit codes: 0 ok (dry run or committed), 1 failed / gate refused, 2 usage.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"mini-erp/internal/bootstrap"
	"mini-erp/internal/config"
	"mini-erp/internal/database"
	"mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/logger"
	"mini-erp/internal/shared/signedurl"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("costrebuild", flag.ContinueOnError)
	fs.SetOutput(errOut)
	branchID := fs.Int64("branch-id", 0, "branch to restate (required)")
	commit := fs.Bool("commit", false, "write the correction (default: dry run)")
	date := fs.String("date", "", "date of the correction journal, YYYY-MM-DD (default: today, Asia/Jakarta)")
	actorID := fs.Int64("actor-id", 0, "user id recorded on the journal/audit (default 0 = operator console)")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *branchID <= 0 {
		fmt.Fprintln(errOut, "costrebuild: --branch-id is required")
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(errOut, "costrebuild: config:", err)
		return 1
	}
	db, err := database.Open(cfg.DBDSN)
	if err != nil {
		fmt.Fprintln(errOut, "costrebuild: connect database:", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	mods, err := bootstrap.Wire(db, bootstrap.Options{
		Log: logger.New(), JWTSecret: cfg.JWTSecret, AccessTTL: cfg.AccessTTL, RefreshTTL: cfg.RefreshTTL,
		Storage: cfg.Storage, MediaDir: cfg.MediaDir, Signer: signedurl.New(cfg.MediaSigningKey), SessionDir: cfg.SessionDir,
	})
	if err != nil {
		fmt.Fprintln(errOut, "costrebuild: wire modules:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rep, err := mods.Finance.Service().RestateCost(ctx, application.RestateCostOptions{
		BranchID: *branchID, ActorID: *actorID, Commit: *commit, Date: *date})
	if rep != nil {
		if *asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			_ = enc.Encode(rep)
		} else {
			printReport(out, rep)
		}
	}
	if err != nil {
		fmt.Fprintln(errOut, "costrebuild:", err)
		return 1
	}
	return 0
}

func printReport(w io.Writer, r *application.RestateReport) {
	mode := "DRY-RUN (tidak menulis apa pun)"
	if r.Commit {
		mode = "COMMIT"
	}
	fmt.Fprintf(w, "Mode: %s | cabang %d | tanggal jurnal %s\n", mode, r.BranchID, r.Date)
	fmt.Fprintf(w, "Posisi diperiksa: %d | baris diubah: %d | produk berselisih: %d | dilewati: %d\n",
		r.Positions, r.RowsChanged, len(r.Products), len(r.Skipped))
	for _, p := range r.Products {
		fmt.Fprintf(w, "  %-14s %-32s baris %-3d sebelum %14.2f sesudah %14.2f selisih %12d\n",
			p.ProductCode, trunc(p.ProductName, 32), p.Rows, p.ValueBefore, p.ValueAfter, p.Delta)
	}
	for _, s := range r.Skipped {
		fmt.Fprintf(w, "  DILEWATI produk %d/%d: %s\n", s.ProductID, s.VariantID, s.Reason)
	}
	fmt.Fprintf(w, "Selisih nilai persediaan (jurnal koreksi): Rp %d\n", r.Delta)
	for _, g := range r.Gates {
		state := "LULUS"
		if !g.OK {
			state = "GAGAL"
		}
		fmt.Fprintf(w, "Gerbang %-18s %s  %s\n", g.Key, state, g.Detail)
	}
	if r.Applied {
		if r.EntryID > 0 {
			fmt.Fprintf(w, "Jurnal koreksi: %s (id %d)\n", r.EntryNumber, r.EntryID)
		} else {
			fmt.Fprintln(w, "Baris ledger dikoreksi; selisih di bawah Rp 1 sehingga tanpa jurnal.")
		}
	} else if !r.Commit {
		fmt.Fprintln(w, "Dry-run selesai. Jalankan ulang dengan --commit untuk menulis.")
	}
	for _, x := range []struct {
		name string
		rec  *application.InventoryRecon
	}{{"sebelum", r.ReconBefore}, {"sesudah", r.ReconAfter}} {
		if x.rec != nil {
			fmt.Fprintf(w, "Rekonsiliasi %-8s buku %d, ledger %d, selisih %d, tak terjelaskan %d, konsisten=%v\n",
				x.name, x.rec.Book, x.rec.Ledger, x.rec.Diff, x.rec.Unexplained, x.rec.Consistent)
		}
	}
}

func trunc(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
