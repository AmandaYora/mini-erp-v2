// Command variantfix restores the product-variant columns the first ETL runs
// dropped (temuan 2, 2026-10-23): is_hidden, sort_order and the "inactive"
// status (ETL mapped live inactive variants to "archived"). Without is_hidden
// every hidden "Standar" default became a visible, sellable variant, so the
// POS and the order form offered it instead of the real colours.
//
// Values come from the frozen legacy COPY (etl_copy_marker required) and are
// matched to the revamp rows through etl_id_map (entity product_variants), so
// the script is independent of the revamp ids. A row is only changed while it
// still carries the ETL default (is_hidden 0, sort_order 0, status archived):
// anything a user edited since go-live is left alone and reported. Running it
// twice changes nothing the second time.
//
//	variantfix --legacy-dsn L --target-dsn T                 dry run: report only
//	variantfix --legacy-dsn L --target-dsn T --apply         update in one transaction
//	variantfix --legacy-dsn L --emit-sql fix.sql             standalone SQL for the server
//	                                                         (+ fix.sql.dry-run.sql that rolls back)
//
// Variants still holding stock are never hidden (they would become unsellable);
// they are listed for a manual decision. updated_at is not touched.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"mini-erp/internal/etl/guard"
)

// legacyVariant is the slice of a legacy variant the fix needs.
type legacyVariant struct {
	ID       int64
	Hidden   bool
	Sort     int
	Inactive bool // status inactive and not archived (live, switched off by a user)
}

// targetVariant is the current revamp row.
type targetVariant struct {
	ID        int64
	ProductID int64
	Code      string
	Hidden    bool
	Sort      int
	Status    string
	OnHand    float64 // Σ |on_hand| over the branch positions
}

// change is one planned update.
type change struct {
	VariantID int64
	ProductID int64
	Code      string
	Hide      bool
	Sort      *int
	Inactive  bool
}

// report summarises a plan.
type report struct {
	Legacy, Mapped, Unmapped int
	Hide, Sort, Inactive     int
	Products                 map[int64]bool
	SkippedStock             []targetVariant // would hide a variant holding stock
	SkippedEdited            int             // value differs from both ETL default and legacy
	AlreadyDone              int
}

// plan compares legacy values with the current rows (pure; unit-tested).
//
// A hidden default that holds stock is still hidden when it is the product's
// only sellable variant (a product without real variants: the web falls back
// to it). It is skipped only when another active variant of the product stays
// visible — hiding it then would strand its stock outside the POS.
func plan(legacy []legacyVariant, idMap map[int64]int64, rows map[int64]targetVariant) ([]change, report) {
	r := report{Legacy: len(legacy), Products: map[int64]bool{}}
	var out []change
	willHide := map[int64]bool{}
	for _, lv := range legacy {
		if cur, ok := rows[idMap[lv.ID]]; ok && lv.Hidden && !cur.Hidden {
			willHide[cur.ID] = true
		}
	}
	otherVisible := func(v targetVariant) bool {
		for _, o := range rows {
			if o.ProductID == v.ProductID && o.ID != v.ID && o.Status == "active" && !o.Hidden && !willHide[o.ID] {
				return true
			}
		}
		return false
	}
	for _, lv := range legacy {
		newID, ok := idMap[lv.ID]
		if !ok {
			r.Unmapped++
			continue
		}
		cur, ok := rows[newID]
		if !ok {
			r.Unmapped++
			continue
		}
		r.Mapped++
		c := change{VariantID: cur.ID, ProductID: cur.ProductID, Code: cur.Code}
		done := true
		if lv.Hidden && !cur.Hidden {
			done = false
			if cur.OnHand != 0 && otherVisible(cur) {
				r.SkippedStock = append(r.SkippedStock, cur)
			} else {
				c.Hide = true
			}
		}
		if lv.Sort != 0 && cur.Sort != lv.Sort {
			done = false
			if cur.Sort == 0 {
				s := lv.Sort
				c.Sort = &s
			} else {
				r.SkippedEdited++
			}
		}
		if lv.Inactive && cur.Status != "inactive" {
			done = false
			if cur.Status == "archived" {
				c.Inactive = true
			} else {
				r.SkippedEdited++
			}
		}
		if done {
			r.AlreadyDone++
		}
		if c.Hide || c.Sort != nil || c.Inactive {
			out = append(out, c)
			r.Products[c.ProductID] = true
			if c.Hide {
				r.Hide++
			}
			if c.Sort != nil {
				r.Sort++
			}
			if c.Inactive {
				r.Inactive++
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VariantID < out[j].VariantID })
	return out, r
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("variantfix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	legacyDSN := fs.String("legacy-dsn", os.Getenv("ETL_LEGACY_DSN"), "salinan legacy (wajib punya etl_copy_marker)")
	targetDSN := fs.String("target-dsn", os.Getenv("ETL_TARGET_DSN"), "database revamp (dry run / --apply)")
	apply := fs.Bool("apply", false, "tulis perubahan (bawaan: dry run)")
	allowDB := fs.String("allow-target-db", "", "izinkan --apply ke database ini (selain *_test / *_rehearsal)")
	emit := fs.String("emit-sql", "", "tulis skrip SQL mandiri ke berkas ini (tanpa koneksi target)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *legacyDSN == "" || (*targetDSN == "" && *emit == "") {
		fmt.Fprintln(stderr, "usage: variantfix --legacy-dsn L (--target-dsn T [--apply] | --emit-sql FILE)")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	legacy, err := readLegacy(ctx, *legacyDSN)
	if err != nil {
		fmt.Fprintln(stderr, "variantfix:", err)
		return 1
	}
	if *emit != "" {
		if err := emitSQL(*emit, legacy); err != nil {
			fmt.Fprintln(stderr, "variantfix:", err)
			return 1
		}
		fmt.Fprintf(stdout, "variantfix: %d varian legacy -> %s (+ .dry-run.sql)\n", len(legacy), *emit)
		if *targetDSN == "" {
			return 0
		}
	}
	if err := runTarget(ctx, *targetDSN, *allowDB, *apply, legacy, stdout); err != nil {
		fmt.Fprintln(stderr, "variantfix:", err)
		return 1
	}
	return 0
}

func readLegacy(ctx context.Context, dsn string) ([]legacyVariant, error) {
	if err := guard.CheckLegacyHost(dsn, false); err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	if err := guard.LegacyMarker(ctx, db, "etl_copy_marker"); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id_product_variant, is_hidden, sort_order, status = 'inactive' AND archived_at IS NULL
		 FROM product_variants ORDER BY id_product_variant`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []legacyVariant
	for rows.Next() {
		var v legacyVariant
		if err := rows.Scan(&v.ID, &v.Hidden, &v.Sort, &v.Inactive); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func runTarget(ctx context.Context, dsn, allow string, apply bool, legacy []legacyVariant, stdout io.Writer) error {
	name, err := guard.DBName(dsn)
	if err != nil {
		return err
	}
	if apply {
		if err := guard.CheckTargetName(name, allow); err != nil {
			return err
		}
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	idMap, rows, err := readTarget(ctx, db)
	if err != nil {
		return err
	}
	changes, rep := plan(legacy, idMap, rows)
	printReport(stdout, name, apply, rep, changes)
	if !apply || len(changes) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var hidden, sorted, inactive int64
	for _, c := range changes {
		if c.Hide {
			n, err := exec(ctx, tx, `UPDATE product_variants SET is_hidden = 1 WHERE id = ? AND is_hidden = 0`, c.VariantID)
			if err != nil {
				return err
			}
			hidden += n
		}
		if c.Sort != nil {
			n, err := exec(ctx, tx, `UPDATE product_variants SET sort_order = ? WHERE id = ? AND sort_order = 0`, *c.Sort, c.VariantID)
			if err != nil {
				return err
			}
			sorted += n
		}
		if c.Inactive {
			n, err := exec(ctx, tx, `UPDATE product_variants SET status = 'inactive' WHERE id = ? AND status = 'archived'`, c.VariantID)
			if err != nil {
				return err
			}
			inactive += n
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "DITERAPKAN: is_hidden %d, sort_order %d, status inactive %d\n", hidden, sorted, inactive)
	return nil
}

func exec(ctx context.Context, tx *sql.Tx, q string, args ...any) (int64, error) {
	res, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func readTarget(ctx context.Context, db *sql.DB) (map[int64]int64, map[int64]targetVariant, error) {
	idMap := map[int64]int64{}
	rows, err := db.QueryContext(ctx, `SELECT legacy_id, new_id FROM etl_id_map WHERE entity = 'product_variants'`)
	if err != nil {
		return nil, nil, fmt.Errorf("baca etl_id_map: %w", err)
	}
	for rows.Next() {
		var l, n int64
		if err := rows.Scan(&l, &n); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		idMap[l] = n
	}
	if err := closeRows(rows); err != nil {
		return nil, nil, err
	}
	if len(idMap) == 0 {
		return nil, nil, errors.New("etl_id_map tanpa product_variants: database ini bukan hasil ETL")
	}
	out := map[int64]targetVariant{}
	rows, err = db.QueryContext(ctx,
		`SELECT v.id, v.product_id, v.code, v.is_hidden, v.sort_order, v.status,
		        COALESCE((SELECT SUM(ABS(b.on_hand)) FROM stock_balances b WHERE b.variant_id = v.id), 0)
		 FROM product_variants v`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var v targetVariant
		var onHand string
		if err := rows.Scan(&v.ID, &v.ProductID, &v.Code, &v.Hidden, &v.Sort, &v.Status, &onHand); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		_, _ = fmt.Sscan(onHand, &v.OnHand)
		out[v.ID] = v
	}
	return idMap, out, closeRows(rows)
}

func closeRows(rows *sql.Rows) error {
	err := rows.Err()
	if cerr := rows.Close(); err == nil {
		err = cerr
	}
	return err
}

func printReport(w io.Writer, db string, apply bool, r report, changes []change) {
	mode := "DRY RUN (tidak ada yang ditulis)"
	if apply {
		mode = "APPLY"
	}
	fmt.Fprintf(w, "variantfix %s pada %s\n", mode, db)
	fmt.Fprintf(w, "  varian legacy %d, terpetakan %d, tanpa pasangan %d, sudah sesuai %d\n", r.Legacy, r.Mapped, r.Unmapped, r.AlreadyDone)
	fmt.Fprintf(w, "  rencana: is_hidden=1 %d, sort_order %d, status inactive %d (produk terdampak %d)\n",
		r.Hide, r.Sort, r.Inactive, len(r.Products))
	fmt.Fprintf(w, "  dilewati: disunting pengguna %d, masih punya stok (tidak disembunyikan) %d\n", r.SkippedEdited, len(r.SkippedStock))
	for i, v := range r.SkippedStock {
		if i == 20 {
			fmt.Fprintf(w, "    … %d lainnya\n", len(r.SkippedStock)-20)
			break
		}
		fmt.Fprintf(w, "    varian %d %s (produk %d) stok %g\n", v.ID, v.Code, v.ProductID, v.OnHand)
	}
	for i, c := range changes {
		if i == 5 {
			break
		}
		fmt.Fprintf(w, "  contoh: varian %d %s produk %d hide=%v sort=%v inactive=%v\n", c.VariantID, c.Code, c.ProductID, c.Hide, derefOr(c.Sort), c.Inactive)
	}
}

func derefOr(p *int) any {
	if p == nil {
		return "-"
	}
	return *p
}

// emitSQL writes a self-contained script for the server's mysql client. It
// joins etl_id_map there (no revamp ids baked in) and applies the same guards
// as --apply. The .dry-run.sql twin reports what would change and rolls back.
func emitSQL(path string, legacy []legacyVariant) error {
	var b strings.Builder
	b.WriteString("-- variantfix: pulihkan is_hidden / sort_order / status inactive varian dari salinan legacy (temuan 2, 2026-10-23).\n")
	b.WriteString("-- Idempoten: hanya baris yang masih bernilai bawaan ETL yang berubah; varian bersaldo stok tidak disembunyikan.\n")
	b.WriteString("SET NAMES utf8mb4;\nSTART TRANSACTION;\n")
	b.WriteString("CREATE TEMPORARY TABLE variantfix_src (legacy_id BIGINT NOT NULL PRIMARY KEY, hide TINYINT NOT NULL, sort_order INT NOT NULL, inactive TINYINT NOT NULL);\n")
	for i := 0; i < len(legacy); i += 500 {
		end := min(i+500, len(legacy))
		b.WriteString("INSERT INTO variantfix_src VALUES ")
		for j, v := range legacy[i:end] {
			if j > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, "(%d,%d,%d,%d)", v.ID, b2i(v.Hidden), v.Sort, b2i(v.Inactive))
		}
		b.WriteString(";\n")
	}
	b.WriteString(`CREATE TEMPORARY TABLE variantfix_map AS
  SELECT v.id AS variant_id, v.product_id, s.hide, s.sort_order, s.inactive,
         (s.hide = 1 AND v.is_hidden = 0) AS will_hide,
         COALESCE((SELECT SUM(ABS(b.on_hand)) FROM stock_balances b WHERE b.variant_id = v.id), 0) AS stock
  FROM product_variants v
  JOIN (SELECT legacy_id, MAX(new_id) AS new_id FROM etl_id_map WHERE entity = 'product_variants' GROUP BY legacy_id) m ON m.new_id = v.id
  JOIN variantfix_src s ON s.legacy_id = m.legacy_id;
-- Produk yang tetap punya varian aktif terlihat: varian bersaldo di produk ini tidak disembunyikan.
CREATE TEMPORARY TABLE variantfix_keep AS
  SELECT DISTINCT v.product_id FROM product_variants v LEFT JOIN variantfix_map m ON m.variant_id = v.id
  WHERE v.status = 'active' AND v.is_hidden = 0 AND COALESCE(m.will_hide, 0) = 0;
SELECT 'rencana' AS tahap,
  SUM(m.will_hide = 1 AND (m.stock = 0 OR k.product_id IS NULL)) AS sembunyikan,
  SUM(m.will_hide = 1 AND m.stock <> 0 AND k.product_id IS NOT NULL) AS dilewati_bersaldo,
  SUM(m.sort_order <> 0 AND v.sort_order = 0) AS urutan,
  SUM(m.inactive = 1 AND v.status = 'archived') AS nonaktif
FROM variantfix_map m JOIN product_variants v ON v.id = m.variant_id LEFT JOIN variantfix_keep k ON k.product_id = m.product_id;
UPDATE product_variants v JOIN variantfix_map m ON m.variant_id = v.id LEFT JOIN variantfix_keep k ON k.product_id = m.product_id
  SET v.is_hidden = 1 WHERE m.will_hide = 1 AND v.is_hidden = 0 AND (m.stock = 0 OR k.product_id IS NULL);
UPDATE product_variants v JOIN variantfix_map m ON m.variant_id = v.id
  SET v.sort_order = m.sort_order WHERE m.sort_order <> 0 AND v.sort_order = 0;
UPDATE product_variants v JOIN variantfix_map m ON m.variant_id = v.id
  SET v.status = 'inactive' WHERE m.inactive = 1 AND v.status = 'archived';
SELECT 'sesudah' AS tahap,
  SUM(m.hide = 1 AND v.is_hidden = 0) AS belum_tersembunyi,
  SUM(m.sort_order <> 0 AND v.sort_order <> m.sort_order) AS urutan_beda,
  SUM(m.inactive = 1 AND v.status <> 'inactive') AS nonaktif_beda
FROM variantfix_map m JOIN product_variants v ON v.id = m.variant_id;
`)
	body := b.String()
	if err := os.WriteFile(path, []byte(body+"COMMIT;\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(path+".dry-run.sql", []byte(strings.Replace(body, "START TRANSACTION;", "-- DRY RUN: semua perubahan dibatalkan di akhir.\nSTART TRANSACTION;", 1)+"ROLLBACK;\n"), 0o644)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
