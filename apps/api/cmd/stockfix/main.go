// Command stockfix neutralizes stock adjustments proven wrong with a counter
// adjustment made through the stock service (the Stok → Koreksi path), so the
// stock card, cost ledger, audit trail and daily-close journal stay consistent.
// No row is edited or deleted. See package fix and RUNBOOK §6e.
//
//	stockfix --file lines.json|lines.csv --branch-id N --actor USER [--approver USER] [--commit] [--json]
//
// Without --commit it is a DRY RUN: every line is checked against its
// referenced movement (same branch, an adjustment, opposite direction, qty not
// above it, matching product/variant/location), against the live balance (never
// negative, never below what holds keep) and against earlier runs (the marker
// "[stockfix:<refMovementId>]" in the reason makes a rerun skip applied lines);
// the balance before → after is printed and nothing is written. --commit
// applies the lines only when none is rejected.
//
// The approver password is read from STOCKFIX_APPROVER_PASSWORD, or prompted
// when that is empty (without echo on a terminal, else one line of standard
// input) — never from a flag (shell history), and never printed. Like costrebuild it runs in the app container
// (`docker compose ... run --rm app ./stockfix ...`) against the host database.
//
// Exit codes: 0 ok (dry run clean, or committed), 1 rejected / failed, 2 usage.
package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"mini-erp/cmd/stockfix/fix"
	"mini-erp/internal/bootstrap"
	"mini-erp/internal/config"
	"mini-erp/internal/database"
	"mini-erp/internal/shared/logger"
	"mini-erp/internal/shared/signedurl"

	"golang.org/x/term"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, stdin io.Reader, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("stockfix", flag.ContinueOnError)
	fs.SetOutput(errOut)
	file := fs.String("file", "", "berkas baris (.json atau .csv) — wajib")
	branchID := fs.Int64("branch-id", 0, "cabang (wajib)")
	actor := fs.String("actor", "", "username pengguna yang tercatat (wajib; aktif, izin stock.adjust)")
	approver := fs.String("approver", "", "username penyetuju (izin stock.approve) bila aturan persetujuan koreksi berlaku")
	commit := fs.Bool("commit", false, "tulis penyeimbang (bawaan: dry run)")
	asJSON := fs.Bool("json", false, "cetak laporan sebagai JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *file == "" || *branchID <= 0 || strings.TrimSpace(*actor) == "" {
		fmt.Fprintln(errOut, "usage: stockfix --file F --branch-id N --actor USER [--approver USER] [--commit] [--json]")
		return 2
	}
	lines, err := readLines(*file)
	if err != nil {
		fmt.Fprintln(errOut, "stockfix:", err)
		return 2
	}
	password := os.Getenv("STOCKFIX_APPROVER_PASSWORD")
	if *commit && *approver != "" && password == "" {
		if password, err = readPassword(stdin, errOut); err != nil {
			fmt.Fprintln(errOut, "stockfix:", err)
			return 2
		}
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(errOut, "stockfix: config:", err)
		return 1
	}
	db, err := database.Open(cfg.DBDSN)
	if err != nil {
		fmt.Fprintln(errOut, "stockfix: connect database:", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	mods, err := bootstrap.Wire(db, bootstrap.Options{
		Log: logger.New(), JWTSecret: cfg.JWTSecret, AccessTTL: cfg.AccessTTL, RefreshTTL: cfg.RefreshTTL,
		Storage: cfg.Storage, MediaDir: cfg.MediaDir, Signer: signedurl.New(cfg.MediaSigningKey), SessionDir: cfg.SessionDir,
	})
	if err != nil {
		fmt.Fprintln(errOut, "stockfix: wire modules:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rep, err := fix.Run(ctx, fix.Deps{DB: db, Stock: mods.Stock.Service(), Products: mods.Product.Products(),
		Users: mods.User.Users(), Roles: mods.User.Roles()},
		fix.Options{BranchID: *branchID, Actor: *actor, Approver: *approver, ApproverPassword: password, Commit: *commit}, lines)
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
		fmt.Fprintln(errOut, "stockfix:", err)
		return 1
	}
	return 0
}

// readPassword prompts for the approver password. On a terminal it is read
// without echo; otherwise (a pipe, a file) it is the first line. The password
// is never written anywhere — not to the prompt, an error, or the report.
func readPassword(stdin io.Reader, prompt io.Writer) (string, error) {
	fmt.Fprint(prompt, "Kata sandi penyetuju: ")
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(prompt)
		if err != nil {
			return "", errors.New("gagal membaca kata sandi penyetuju")
		}
		return nonEmpty(string(b))
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", errors.New("gagal membaca kata sandi penyetuju")
	}
	return nonEmpty(strings.TrimRight(line, "\r\n"))
}

func nonEmpty(password string) (string, error) {
	if password == "" {
		return "", errors.New("kata sandi penyetuju kosong")
	}
	return password, nil
}

// readLines accepts a JSON array (or {"lines": [...]}) or a CSV whose header
// names the JSON fields.
func readLines(path string) ([]fix.Line, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lines []fix.Line
	if strings.EqualFold(filepath.Ext(path), ".csv") {
		lines, err = parseCSV(string(raw))
	} else if err = json.Unmarshal(raw, &lines); err != nil {
		var wrapped struct {
			Lines []fix.Line `json:"lines"`
		}
		if json.Unmarshal(raw, &wrapped) == nil && wrapped.Lines != nil {
			lines, err = wrapped.Lines, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("%s: tidak ada baris", path)
	}
	return lines, nil
}

func parseCSV(s string) ([]fix.Line, error) {
	recs, err := csv.NewReader(strings.NewReader(s)).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) < 2 {
		return nil, errors.New("CSV butuh baris judul dan minimal satu baris")
	}
	col := map[string]int{}
	for i, h := range recs[0] {
		col[strings.TrimSpace(h)] = i
	}
	get := func(rec []string, name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	id := func(rec []string, name string) (int64, error) {
		v := get(rec, name)
		if v == "" {
			return 0, nil
		}
		return strconv.ParseInt(v, 10, 64)
	}
	var lines []fix.Line
	for n, rec := range recs[1:] {
		var ln fix.Line
		var errs []error
		var e error
		ln.ProductCode, ln.Direction, ln.Reason = get(rec, "productCode"), get(rec, "direction"), get(rec, "reason")
		ln.ProductID, e = id(rec, "productId")
		errs = append(errs, e)
		ln.VariantID, e = id(rec, "variantId")
		errs = append(errs, e)
		ln.LocationID, e = id(rec, "locationId")
		errs = append(errs, e)
		ln.RefMovementID, e = id(rec, "refMovementId")
		errs = append(errs, e)
		ln.Qty, e = strconv.ParseFloat(get(rec, "qty"), 64)
		errs = append(errs, e)
		if err := errors.Join(errs...); err != nil {
			return nil, fmt.Errorf("baris CSV %d: %w", n+2, err)
		}
		lines = append(lines, ln)
	}
	return lines, nil
}

func printReport(w io.Writer, r *fix.Report) {
	mode := "DRY-RUN (tidak menulis apa pun)"
	if r.Commit {
		mode = "COMMIT"
	}
	fmt.Fprintf(w, "stockfix %s | cabang %d | actor %s | penyetuju %s | %s\n",
		mode, r.BranchID, r.Actor, orDash(r.Approver), r.At.Format("2006-01-02 15:04:05 MST"))
	for _, l := range r.Lines {
		fmt.Fprintf(w, "  #%-3d ref %-8d [%s] %-16s %-28s v%-6d lok%-4d %-3s %-8s saldo %s -> %s (ditahan %s)  %s\n",
			l.Line, l.RefMovementID, l.RefSummary, l.ProductCode, trunc(l.ProductName, 28), l.VariantID, l.LocationID,
			l.Direction, num(l.Qty), num(l.OnHandBefore), num(l.OnHandAfter), num(l.Reserved), strings.ToUpper(l.Status))
		if l.Problem != "" {
			fmt.Fprintf(w, "        alasan ditolak: %s\n", l.Problem)
		}
	}
	fmt.Fprintf(w, "Ringkasan: rencana %d, diterapkan %d, sudah dinetralkan %d, ditolak %d\n", r.Planned, r.Applied, r.Done, r.Rejected)
	if !r.Commit && r.Rejected == 0 && r.Planned > 0 {
		fmt.Fprintln(w, "Dry-run bersih. Backup DB, lalu jalankan ulang dengan --commit.")
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func trunc(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func num(f float64) string {
	s := strconv.FormatFloat(f, 'f', 4, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}
