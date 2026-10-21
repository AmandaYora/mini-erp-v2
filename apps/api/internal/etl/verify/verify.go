// Package verify reconciles the migrated target with the legacy source (K10,
// F5-11). Tolerance is zero (D-ETL-7): counts, money and quantities must match
// exactly; the only allowed differences are the "explained" ones the transform
// computed itself (split items, synthetic documents, merged movements,
// logged balance adjustments). A failing check makes the CLI exit 1.
package verify

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/legacydb"
	"mini-erp/internal/etl/mediacopy"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/etl/sequences"
	"mini-erp/internal/etl/transform"
)

// Check is one reconciliation line.
type Check struct {
	ID        string `json:"id"`
	Desc      string `json:"desc"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	Diff      string `json:"diff"`
	Explained string `json:"explained"`
	Status    string `json:"status"` // LULUS | GAGAL
}

// Report is the whole verification.
type Report struct {
	Checks []Check `json:"checks"`
	Pass   bool    `json:"pass"`
}

// Failed lists the ids of failing checks.
func (r *Report) Failed() []string {
	var out []string
	for _, c := range r.Checks {
		if c.Status != "LULUS" {
			out = append(out, c.ID)
		}
	}
	return out
}

// Querier is the read surface of a target connection: a *sql.DB, or a
// *sql.Tx when the self-test runs the verifier inside a transaction it rolls
// back (the corruption never reaches the database).
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type verifier struct {
	ctx     context.Context
	l       *legacydb.DB
	t       Querier
	cfg     *cfg.Config
	work    string
	explain map[string]int64
	checks  []Check
}

// Run executes every check.
func Run(ctx context.Context, ldb *legacydb.DB, target Querier, c *cfg.Config, work string) (*Report, error) {
	ex, err := transform.ReadExplain(work)
	if err != nil {
		return nil, fmt.Errorf("hasil transform tidak ada: %w", err)
	}
	v := &verifier{ctx: ctx, l: ldb, t: target, cfg: c, work: work, explain: ex}
	for _, f := range []func(){
		v.counts, v.orderTotals, v.orderInvariants, v.stock, v.payments, v.trialBalance, v.journalShape,
		v.costLedger, v.migrationCorrections, v.receivableSigns, v.media, v.numbers, v.nextNumbers, v.orphans, v.postingMarks, v.periods, v.archive, v.accessSets,
		v.returns, v.balances, v.permissions, v.settings,
	} {
		f()
	}
	rep := &Report{Checks: v.checks, Pass: true}
	for _, ch := range rep.Checks {
		if ch.Status != "LULUS" {
			rep.Pass = false
		}
	}
	return rep, nil
}

// ---------------------------------------------------------------- helpers

func norm(s string) string {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return s
	}
	if r.IsInt() {
		return r.Num().String()
	}
	out := r.FloatString(12)
	out = strings.TrimRight(out, "0")
	return strings.TrimSuffix(out, ".")
}

func (v *verifier) add(id, desc, src, tgt, diff, expl string, ok bool) {
	st := "LULUS"
	if !ok {
		st = "GAGAL"
	}
	v.checks = append(v.checks, Check{id, desc, src, tgt, diff, expl, st})
}

func (v *verifier) fail(id, desc string, err error) {
	v.checks = append(v.checks, Check{ID: id, Desc: desc, Status: "GAGAL", Diff: "error: " + err.Error()})
}

func (v *verifier) one(db Querier, q string) (string, error) {
	var s sql.NullString
	if err := db.QueryRowContext(v.ctx, q).Scan(&s); err != nil {
		return "", err
	}
	if !s.Valid {
		return "0", nil
	}
	return norm(s.String), nil
}

func (v *verifier) lq(q string) string {
	return v.l.Q(strings.ReplaceAll(q, ":open", "'"+v.cfg.OpeningDate+"'"))
}

// scalar compares one number: source + explained must equal target.
func (v *verifier) scalar(id, desc, srcSQL, tgtSQL string, explained int64) {
	s, err := v.one(v.l.SQL, v.lq(srcSQL))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	t, err := v.one(v.t, tgtSQL)
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	sr, _ := new(big.Rat).SetString(s)
	want := new(big.Rat).Add(sr, new(big.Rat).SetInt64(explained))
	tr, _ := new(big.Rat).SetString(t)
	diff := new(big.Rat).Sub(tr, want)
	v.add(id, desc, s, t, norm(diff.FloatString(6)), strconv.FormatInt(explained, 10), diff.Sign() == 0)
}

func queryMap(ctx context.Context, db Querier, q string) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	cols, _ := rows.Columns()
	out := map[string]string{}
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		parts := make([]string, 0, len(cols)-1)
		for _, c := range vals[:len(cols)-1] {
			parts = append(parts, c.String)
		}
		last := "0"
		if vals[len(cols)-1].Valid {
			last = norm(vals[len(cols)-1].String)
		}
		out[strings.Join(parts, "|")] = last
	}
	return out, rows.Err()
}

func diffMaps(src, tgt map[string]string) []string {
	keys := map[string]bool{}
	for k := range src {
		keys[k] = true
	}
	for k := range tgt {
		keys[k] = true
	}
	var out []string
	for k := range keys {
		a, b := src[k], tgt[k]
		if a == "" {
			a = "0"
		}
		if b == "" {
			b = "0"
		}
		if a != b {
			out = append(out, fmt.Sprintf("%s: sumber=%s target=%s", k, a, b))
		}
	}
	sort.Strings(out)
	return out
}

// grouped compares two keyed result sets; adjust may modify the source map
// (explained differences) before comparison.
func (v *verifier) grouped(id, desc, srcSQL, tgtSQL string, adjust func(map[string]string)) {
	s, err := queryMap(v.ctx, v.l.SQL, v.lq(srcSQL))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	t, err := queryMap(v.ctx, v.t, tgtSQL)
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	if adjust != nil {
		adjust(s)
	}
	d := diffMaps(s, t)
	show := d
	if len(show) > 5 {
		show = show[:5]
	}
	v.add(id, desc, strconv.Itoa(len(s))+" kelompok", strconv.Itoa(len(t))+" kelompok", strings.Join(show, "; "), "", len(d) == 0)
}

func (v *verifier) targetZero(id, desc, q string) {
	t, err := v.one(v.t, q)
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	v.add(id, desc, "0", t, t, "", t == "0")
}

// ---------------------------------------------------------------- V-01

const byBranch = "id_branch IN (SELECT id_branch FROM {branches} WHERE {co} = :c)"

func (v *verifier) counts() {
	e := v.explain
	type cc struct {
		id, desc, src, tgt string
		expl               int64
	}
	items := []cc{
		{"V-01a", "cabang", "SELECT COUNT(*) FROM {branches} WHERE {co} = :c", "SELECT COUNT(*) FROM branches", 0},
		{"V-01b", "pengguna (tanpa superadmin)", "SELECT COUNT(*) FROM {users} WHERE {co} = :c AND username <> 'superadmin'", "SELECT COUNT(*) FROM users", 0},
		{"V-01c", "tipe member", "SELECT COUNT(*) FROM {member_types} WHERE {co} = :c", "SELECT COUNT(*) FROM member_types", 0},
		{"V-01d", "kategori produk", "SELECT COUNT(*) FROM {product_categories} WHERE {co} = :c", "SELECT COUNT(*) FROM product_categories", 0},
		{"V-01e", "produk", "SELECT COUNT(*) FROM {products} WHERE {co} = :c", "SELECT COUNT(*) FROM products", 0},
		{"V-01f", "varian", "SELECT COUNT(*) FROM {product_variants} WHERE {co} = :c", "SELECT COUNT(*) FROM product_variants", 0},
		{"V-01g", "pihak (customer/supplier)", "SELECT COUNT(*) FROM {business_parties} WHERE {co} = :c AND party_type IN ('customer','supplier')", "SELECT COUNT(*) FROM parties", 0},
		{"V-01h", "alamat kirim", "SELECT COUNT(*) FROM {business_party_delivery_addresses} WHERE {co} = :c", "SELECT COUNT(*) FROM party_delivery_addresses", 0},
		{"V-01i", "lokasi stok (lokasi rusak digabung)",
			"SELECT COUNT(*) - (SUM(status='damaged') - COUNT(DISTINCT CASE WHEN status='damaged' THEN id_branch END)) FROM {stock_locations} WHERE " + byBranch,
			"SELECT COUNT(*) FROM stock_locations", 0},
		{"V-01j", "order penjualan", "SELECT COUNT(*) FROM {orders} WHERE order_kind='sales' AND " + byBranch, "SELECT COUNT(*) FROM sales_orders", 0},
		{"V-01k", "order pembelian", "SELECT COUNT(*) FROM {orders} WHERE order_kind='purchase' AND " + byBranch, "SELECT COUNT(*) FROM purchase_orders", 0},
		{"V-01l", "item order penjualan", "SELECT COUNT(*) FROM {order_items} i JOIN {orders} o ON o.id_order=i.id_order WHERE o.order_kind='sales' AND o." + byBranch, "SELECT COUNT(*) FROM sales_order_items", 0},
		{"V-01m", "item order pembelian", "SELECT COUNT(*) FROM {order_items} i JOIN {orders} o ON o.id_order=i.id_order WHERE o.order_kind='purchase' AND o." + byBranch, "SELECT COUNT(*) FROM purchase_order_items", 0},
		{"V-01n", "surat jalan (+SJ sintetis)", "SELECT COUNT(*) FROM {delivery_notes} WHERE " + byBranch, "SELECT COUNT(*) FROM delivery_notes", e["synthetic_delivery_notes"]},
		{"V-01o", "item SJ (pecah per lokasi, +sintetis)", "SELECT COUNT(*) FROM {delivery_note_items} i JOIN {delivery_notes} d ON d.id_delivery_note=i.id_delivery_note WHERE d." + byBranch,
			"SELECT COUNT(*) FROM delivery_note_items", e["delivery_items_split_extra"] + e["synthetic_delivery_items"]},
		{"V-01p", "penerimaan barang", "SELECT COUNT(*) FROM {goods_receipts} WHERE {co} = :c", "SELECT COUNT(*) FROM goods_receipts", 0},
		{"V-01q", "item penerimaan", "SELECT COUNT(*) FROM {goods_receipt_items} i JOIN {goods_receipts} g ON g.id_goods_receipt=i.id_goods_receipt WHERE g.{co} = :c", "SELECT COUNT(*) FROM goods_receipt_items", 0},
		{"V-01r", "pembayaran", "SELECT COUNT(*) FROM {payments} WHERE " + byBranch, "SELECT COUNT(*) FROM payments WHERE number NOT LIKE 'PAY-%/MIG/%'", 0},
		{"V-01s", "alokasi pembayaran (+sintetis)", "SELECT COUNT(*) FROM {payment_allocations} a JOIN {payments} p ON p.id_payment=a.id_payment WHERE a.archived_at IS NULL AND p." + byBranch,
			"SELECT COUNT(*) FROM payment_allocations a JOIN payments p ON p.id=a.payment_id WHERE p.number NOT LIKE 'PAY-%/MIG/%'", e["synthetic_allocations"]},
		{"V-01t", "transfer stok (+kelompok pasangan lokasi)", "SELECT COUNT(*) FROM {stock_transfers} WHERE {co} = :c", "SELECT COUNT(*) FROM stock_transfers", e["transfer_groups_extra"]},
		{"V-01u", "item transfer", "SELECT COUNT(*) FROM {stock_transfer_items} i JOIN {stock_transfers} t ON t.id_stock_transfer=i.id_stock_transfer WHERE t.{co} = :c", "SELECT COUNT(*) FROM stock_transfer_items", e["synthetic_transfer_items"]},
		{"V-01v", "mutasi stok (-digabung +koreksi)", "SELECT COUNT(*) FROM {inventory_movements} WHERE " + byBranch, "SELECT COUNT(*) FROM stock_movements", e["correction_movements"] - e["movements_merged_extra"]},
		{"V-01w", "baris biaya HPP (+estimasi -tak termigrasi -digabung +saldo awal dibagi per mutasi +baris biaya-saja)", "SELECT COUNT(*) FROM {finance_inventory_cost_movements} WHERE {co} = :c",
			"SELECT COUNT(*) FROM finance_inventory_cost_movements", e["estimated_cost_rows"] - e["cost_rows_unmigrated"] - e["cost_rows_merged_extra"] + e["opening_cost_split_extra"] + e["baseline_cost_rows"]},
		{"V-01x", "jurnal tahun berjalan + saldo awal + koreksi migrasi",
			"SELECT COUNT(*) FROM {finance_journal_entries} WHERE {co} = :c AND status IN ('posted','reversed') AND DATE(DATE_ADD(journal_date, INTERVAL 7 HOUR)) >= :open AND EXISTS (SELECT 1 FROM {finance_journal_lines} zl WHERE zl.id_finance_journal_entry={finance_journal_entries}.id_finance_journal_entry AND (ROUND(zl.debit_amount) <> 0 OR ROUND(zl.credit_amount) <> 0))",
			"SELECT COUNT(*) FROM finance_journal_entries", e["opening_entries"] + e["migration_correction_entries"]},
		{"V-01y", "baris jurnal tahun berjalan",
			"SELECT COUNT(*) FROM {finance_journal_lines} l JOIN {finance_journal_entries} e ON e.id_finance_journal_entry=l.id_finance_journal_entry WHERE e.{co} = :c AND e.status IN ('posted','reversed') AND DATE(DATE_ADD(e.journal_date, INTERVAL 7 HOUR)) >= :open AND (ROUND(l.debit_amount) <> 0 OR ROUND(l.credit_amount) <> 0)",
			"SELECT COUNT(*) FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id=l.entry_id WHERE e.source_doc_type = 'legacy_je'", 0},
		{"V-01aa", "retur penjualan", "SELECT COUNT(*) FROM {sales_returns} WHERE {co} = :c", "SELECT COUNT(*) FROM sales_returns", 0},
		{"V-01ab", "item retur penjualan (dikembalikan)", "SELECT COUNT(*) FROM {sales_return_items} i JOIN {sales_returns} r ON r.id_sales_return=i.id_sales_return WHERE r.{co} = :c AND i.line_type='returned'", "SELECT COUNT(*) FROM sales_return_items", 0},
		{"V-01ac", "item barang pengganti retur", "SELECT COUNT(*) FROM {sales_return_items} i JOIN {sales_returns} r ON r.id_sales_return=i.id_sales_return WHERE r.{co} = :c AND i.line_type='replacement'", "SELECT COUNT(*) FROM sales_return_replacement_items", 0},
		{"V-01ad", "penyelesaian retur penjualan (+offset otomatis)", "SELECT COUNT(*) FROM {sales_return_settlements} s JOIN {sales_returns} r ON r.id_sales_return=s.id_sales_return WHERE r.{co} = :c AND s.archived_at IS NULL",
			"SELECT COUNT(*) FROM sales_return_settlements", e["synthetic_return_settlements"]},
		{"V-01ae", "retur pembelian", "SELECT COUNT(*) FROM {purchase_returns} WHERE {co} = :c", "SELECT COUNT(*) FROM purchase_returns", 0},
		{"V-01af", "item retur pembelian", "SELECT COUNT(*) FROM {purchase_return_items} i JOIN {purchase_returns} r ON r.id_purchase_return=i.id_purchase_return WHERE r.{co} = :c", "SELECT COUNT(*) FROM purchase_return_items", 0},
		{"V-01ag", "penyelesaian retur pembelian", "SELECT COUNT(*) FROM {purchase_return_settlements} s JOIN {purchase_returns} r ON r.id_purchase_return=s.id_purchase_return WHERE r.{co} = :c AND s.archived_at IS NULL", "SELECT COUNT(*) FROM purchase_return_settlements", 0},
		{"V-01ah", "audit log tahun berjalan", "SELECT COUNT(*) FROM {audit_logs} WHERE {co} = :c AND DATE(DATE_ADD(happened_at, INTERVAL 7 HOUR)) >= :open", "SELECT COUNT(*) FROM audit_logs", 0},
		{"V-01z", "biaya usaha", "SELECT COUNT(*) FROM {business_expenses} WHERE {co} = :c", "SELECT COUNT(*) FROM business_expenses", 0},
	}
	for _, it := range items {
		v.scalar(it.id, it.desc, it.src, it.tgt, it.expl)
	}
	// seeded-capable tables are matched by natural key
	codes, err := v.col(v.l.SQL, v.lq("SELECT code FROM {roles} WHERE ({co} = :c OR {co} IS NULL) AND code <> 'superadmin'"))
	if err != nil {
		v.fail("V-16a", "peran per kode", err)
	} else {
		v.inTarget("V-16a", "peran legacy ada di target (per kode)", "roles", "code", codes)
	}
	acodes, err := v.col(v.l.SQL, v.lq("SELECT account_code FROM {finance_accounts} WHERE {co} = :c"))
	if err != nil {
		v.fail("V-16b", "akun per kode", err)
	} else {
		v.inTarget("V-16b", "akun legacy ada di target (per kode)", "finance_accounts", "code", acodes)
	}
	v.scalar("V-16c", "relasi user-peran", "SELECT COUNT(*) FROM {user_roles} ur JOIN {users} u ON u.id_user=ur.id_user JOIN {roles} r ON r.id_role=ur.id_role WHERE u.{co} = :c AND u.username <> 'superadmin' AND r.code <> 'superadmin'", "SELECT COUNT(*) FROM user_roles", 0)
}

func (v *verifier) col(db Querier, q string) ([]string, error) {
	rows, err := db.QueryContext(v.ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s sql.NullString
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s.String)
	}
	return out, rows.Err()
}

func (v *verifier) inTarget(id, desc, table, col string, vals []string) {
	have, err := v.col(v.t, "SELECT `"+col+"` FROM `"+table+"`")
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	var missing []string
	for _, x := range vals {
		if !set[x] {
			missing = append(missing, x)
		}
	}
	sort.Strings(missing)
	v.add(id, desc, strconv.Itoa(len(vals)), strconv.Itoa(len(vals)-len(missing)), strings.Join(missing, ","), "", len(missing) == 0)
}

// ---------------------------------------------------------------- V-02/03/04

func (v *verifier) orderTotals() {
	v.grouped("V-02", "total, pajak, diskon order per cabang x jenis x status",
		`SELECT b.code, o.order_kind,
		   CASE WHEN o.archived_at IS NOT NULL THEN 'cancelled' WHEN d.status_group='pending' THEN 'draft' WHEN d.status_group='active' THEN 'confirmed'
		        WHEN d.status_group='completed' THEN 'completed' ELSE 'cancelled' END st, 'n', COUNT(*) FROM {orders} o
		 JOIN {branches} b ON b.id_branch=o.id_branch JOIN {order_status_definitions} d ON d.id_order_status_definition=o.id_current_status
		 WHERE o.`+byBranch+` GROUP BY 1,2,3
		 UNION ALL
		 SELECT b.code, o.order_kind,
		   CASE WHEN o.archived_at IS NOT NULL THEN 'cancelled' WHEN d.status_group='pending' THEN 'draft' WHEN d.status_group='active' THEN 'confirmed'
		        WHEN d.status_group='completed' THEN 'completed' ELSE 'cancelled' END st, 'grand', SUM(ROUND(o.total_amount)) FROM {orders} o
		 JOIN {branches} b ON b.id_branch=o.id_branch JOIN {order_status_definitions} d ON d.id_order_status_definition=o.id_current_status
		 WHERE o.`+byBranch+` GROUP BY 1,2,3
		 UNION ALL
		 SELECT b.code, o.order_kind,
		   CASE WHEN o.archived_at IS NOT NULL THEN 'cancelled' WHEN d.status_group='pending' THEN 'draft' WHEN d.status_group='active' THEN 'confirmed'
		        WHEN d.status_group='completed' THEN 'completed' ELSE 'cancelled' END st, 'tax', SUM(CASE WHEN o.taxable=0 THEN 0 ELSE ROUND(o.tax_amount) END) FROM {orders} o
		 JOIN {branches} b ON b.id_branch=o.id_branch JOIN {order_status_definitions} d ON d.id_order_status_definition=o.id_current_status
		 WHERE o.`+byBranch+` GROUP BY 1,2,3
		 UNION ALL
		 SELECT b.code, o.order_kind,
		   CASE WHEN o.archived_at IS NOT NULL THEN 'cancelled' WHEN d.status_group='pending' THEN 'draft' WHEN d.status_group='active' THEN 'confirmed'
		        WHEN d.status_group='completed' THEN 'completed' ELSE 'cancelled' END st, 'disc', SUM(ROUND(o.discount_amount)) FROM {orders} o
		 JOIN {branches} b ON b.id_branch=o.id_branch JOIN {order_status_definitions} d ON d.id_order_status_definition=o.id_current_status
		 WHERE o.`+byBranch+` GROUP BY 1,2,3`,
		`SELECT b.code, k, st, m, v FROM (
		   SELECT branch_id, 'sales' k, status st, 'n' m, COUNT(*) v FROM sales_orders GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'sales', status, 'grand', SUM(grand_total) FROM sales_orders GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'sales', status, 'tax', SUM(tax_total) FROM sales_orders GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'sales', status, 'disc', SUM(discount_total) FROM sales_orders GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'purchase', status, 'n', COUNT(*) FROM purchase_orders GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'purchase', status, 'grand', SUM(grand_total) FROM purchase_orders GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'purchase', status, 'tax', SUM(tax_total) FROM purchase_orders GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'purchase', status, 'disc', SUM(discount_total) FROM purchase_orders GROUP BY 1,3) x
		 JOIN branches b ON b.id = x.branch_id`, v.reclassOrders)
}

// reclassOrders moves the orders the transform completed (fully delivered,
// legacy still in progress: ORDER_COMPLETED_DELIVERED) from the legacy
// status group to the target one before V-02 compares them.
func (v *verifier) reclassOrders(src map[string]string) {
	_, codes, err := v.corrections()
	if err != nil {
		return
	}
	moved, err := transform.ReadRows(v.work, "order_reclass")
	if err != nil {
		return
	}
	add := func(key string, n int64) {
		cur, ok := new(big.Rat).SetString(src[key])
		if !ok {
			cur = new(big.Rat)
		}
		src[key] = norm(new(big.Rat).Add(cur, big.NewRat(n, 1)).RatString())
		if src[key] == "0" {
			delete(src, key)
		}
	}
	for _, r := range moved {
		pre := codes[r.Str("branch_id")] + "|" + r.Str("kind") + "|"
		for m, col := range map[string]string{"n": "", "grand": "grand_total", "tax": "tax_total", "disc": "discount_total"} {
			n := int64(1)
			if col != "" {
				n = r.Int(col)
			}
			add(pre+r.Str("from")+"|"+m, -n)
			add(pre+r.Str("to")+"|"+m, n)
		}
	}
}

func (v *verifier) orderInvariants() {
	for _, k := range []string{"sales", "purchase"} {
		v.targetZero("V-04-"+k, "invarian "+k+": Σ baris = total; total = subtotal (+pajak bila eksklusif)",
			fmt.Sprintf(`SELECT (SELECT COUNT(*) FROM (SELECT o.id FROM %[1]s_orders o JOIN %[1]s_order_items i ON i.order_id=o.id GROUP BY o.id, o.grand_total HAVING SUM(i.line_total) <> o.grand_total) a)
			  + (SELECT COUNT(*) FROM %[1]s_orders o WHERE (o.tax_type='exclude' AND o.grand_total <> o.subtotal + o.tax_total) OR (o.tax_type<>'exclude' AND o.grand_total <> o.subtotal))`, k))
	}
}

// ---------------------------------------------------------------- V-05/06

func (v *verifier) stock() {
	v.grouped("V-05", "saldo stok per cabang x produk x varian x lokasi",
		`SELECT b.code, p.product_code,
		        CASE WHEN pv.id_product_variant = (SELECT w.id_product_variant FROM {product_variants} w WHERE w.id_product=pv.id_product AND w.variant_code=pv.variant_code
		             ORDER BY (w.archived_at IS NOT NULL), CASE WHEN w.archived_at IS NULL THEN w.id_product_variant ELSE -w.id_product_variant END LIMIT 1)
		             THEN pv.variant_code ELSE CONCAT(LEFT(pv.variant_code, 50-CHAR_LENGTH(CONCAT('~',pv.id_product_variant))), '~', pv.id_product_variant) END,
		        CASE WHEN l.status='damaged' THEN 'RUSAK' WHEN l.is_default=1 THEN 'GDG' ELSE l.code END, SUM(ib.on_hand_qty)
		   FROM {inventory_balances} ib JOIN {branches} b ON b.id_branch=ib.id_branch JOIN {products} p ON p.id_product=ib.id_product
		   JOIN {product_variants} pv ON pv.id_product_variant=ib.id_product_variant JOIN {stock_locations} l ON l.id_stock_location=ib.id_stock_location
		  WHERE ib.`+byBranch+` GROUP BY 1,2,3,4`,
		`SELECT b.code, p.code, pv.code, l.code, SUM(sb.on_hand) FROM stock_balances sb JOIN branches b ON b.id=sb.branch_id JOIN products p ON p.id=sb.product_id
		   JOIN product_variants pv ON pv.id=sb.variant_id JOIN stock_locations l ON l.id=sb.location_id GROUP BY 1,2,3,4`, nil)
	v.targetZero("V-06", "Σ mutasi bertanda = saldo (target)",
		`SELECT COUNT(*) FROM stock_balances b LEFT JOIN (SELECT branch_id, product_id, variant_id, location_id,
		   SUM(CASE direction WHEN 'in' THEN qty_base ELSE -qty_base END) net FROM stock_movements GROUP BY 1,2,3,4) m
		   ON m.branch_id=b.branch_id AND m.product_id=b.product_id AND m.variant_id=b.variant_id AND m.location_id=b.location_id
		  WHERE b.on_hand <> COALESCE(m.net, 0)`)
	v.targetZero("V-06b", "mutasi tanpa saldo",
		`SELECT COUNT(*) FROM (SELECT m.branch_id FROM stock_movements m LEFT JOIN stock_balances b ON b.branch_id=m.branch_id AND b.product_id=m.product_id
		   AND b.variant_id=m.variant_id AND b.location_id=m.location_id WHERE b.id IS NULL GROUP BY m.branch_id, m.product_id, m.variant_id, m.location_id) x`)
}

// ---------------------------------------------------------------- V-07

func (v *verifier) payments() {
	v.grouped("V-07", "pembayaran per cabang x arah x metode x status (jumlah & nominal)",
		`SELECT b.code, COALESCE(p.financial_side, CASE o.order_kind WHEN 'sales' THEN 'receivable' ELSE 'payable' END) side,
		        CASE p.payment_method WHEN 'bank_transfer' THEN 'transfer' ELSE p.payment_method END, CASE WHEN p.archived_at IS NULL THEN 'active' ELSE 'cancelled' END, 'n', COUNT(*)
		   FROM {payments} p JOIN {branches} b ON b.id_branch=p.id_branch LEFT JOIN {orders} o ON o.id_order=p.id_order WHERE p.`+byBranch+` GROUP BY 1,2,3,4
		 UNION ALL
		 SELECT b.code, COALESCE(p.financial_side, CASE o.order_kind WHEN 'sales' THEN 'receivable' ELSE 'payable' END) side,
		        CASE p.payment_method WHEN 'bank_transfer' THEN 'transfer' ELSE p.payment_method END, CASE WHEN p.archived_at IS NULL THEN 'active' ELSE 'cancelled' END, 'sum', SUM(ROUND(p.amount))
		   FROM {payments} p JOIN {branches} b ON b.id_branch=p.id_branch LEFT JOIN {orders} o ON o.id_order=p.id_order WHERE p.`+byBranch+` GROUP BY 1,2,3,4`,
		`SELECT b.code, CASE p.direction WHEN 'in' THEN 'receivable' ELSE 'payable' END, p.method, p.status, m, v FROM (
		   SELECT branch_id, direction, method, status, 'n' m, COUNT(*) v FROM payments WHERE number NOT LIKE 'PAY-%/MIG/%' GROUP BY 1,2,3,4
		   UNION ALL SELECT branch_id, direction, method, status, 'sum', SUM(amount) FROM payments WHERE number NOT LIKE 'PAY-%/MIG/%' GROUP BY 1,2,3,4) p JOIN branches b ON b.id = p.branch_id`, nil)
	v.targetZero("V-07b", "Σ alokasi = jumlah pembayaran",
		`SELECT COUNT(*) FROM payments p LEFT JOIN (SELECT payment_id, SUM(amount) s FROM payment_allocations GROUP BY payment_id) a ON a.payment_id=p.id WHERE p.amount <> COALESCE(a.s, 0)`)
}

// ---------------------------------------------------------------- V-11 trial balance

var adjRe = regexp.MustCompile(`acct=(\d+);branch=(\d+);side=(\w+);prior=(\d)`)

func (v *verifier) trialBalance() {
	const id, desc = "V-11", "neraca saldo per cabang x akun (s.d. freeze; laba rugi tahun lalu ditutup ke ekuitas; tanpa koreksi migrasi)"
	head, err := v.one(v.l.SQL, v.lq("SELECT code FROM {branches} WHERE {co} = :c AND is_default = 1 ORDER BY id_branch LIMIT 1"))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	// legacy rounded sums per (branch, account, prior?)
	rows, err := v.l.SQL.QueryContext(v.ctx, v.lq(`SELECT COALESCE(b.code, (SELECT code FROM {branches} WHERE {co} = :c AND is_default = 1 ORDER BY id_branch LIMIT 1)), a.account_code, a.account_type, a.system_key,
		   (DATE(DATE_ADD(e.journal_date, INTERVAL 7 HOUR)) < :open) prior, SUM(ROUND(l.debit_amount)) - SUM(ROUND(l.credit_amount)), a.id_finance_account
		 FROM {finance_journal_lines} l JOIN {finance_journal_entries} e ON e.id_finance_journal_entry=l.id_finance_journal_entry
		 JOIN {finance_accounts} a ON a.id_finance_account=l.id_finance_account LEFT JOIN {branches} b ON b.id_branch=e.id_branch
		 WHERE e.{co} = :c AND e.status IN ('posted','reversed') GROUP BY 1,2,3,4,5,7`))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	type k struct{ branch, code string }
	type meta struct {
		typ, sys string
		legacyID int64
	}
	curNet := map[k]*big.Int{}
	priorNet := map[k]*big.Int{}
	metaOf := map[string]meta{}
	acctByLegacy := map[int64]string{}
	add := func(m map[k]*big.Int, key k, n int64) {
		if m[key] == nil {
			m[key] = new(big.Int)
		}
		m[key].Add(m[key], big.NewInt(n))
	}
	for rows.Next() {
		var br, code, typ string
		var sys sql.NullString
		var prior int
		var net, lid int64
		if err := rows.Scan(&br, &code, &typ, &sys, &prior, &net, &lid); err != nil {
			_ = rows.Close()
			v.fail(id, desc, err)
			return
		}
		metaOf[code] = meta{typ, sys.String, lid}
		acctByLegacy[lid] = code
		if prior == 1 {
			add(priorNet, k{br, code}, net)
		} else {
			add(curNet, k{br, code}, net)
		}
	}
	_ = rows.Close()
	// target branch id -> code (for the logged adjustments)
	bcode := map[int64]string{}
	br, err := v.t.QueryContext(v.ctx, "SELECT id, code FROM branches")
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	for br.Next() {
		var i int64
		var c string
		_ = br.Scan(&i, &c)
		bcode[i] = c
	}
	_ = br.Close()
	// explained: the balancing adjustments the transform logged
	if f, err := os.ReadFile(filepath.Join(transform.ReportDir(v.work), "adjustments.csv")); err == nil {
		for _, line := range strings.Split(string(f), "\n") {
			if !strings.Contains(line, "journal_balance") {
				continue
			}
			fields := strings.Split(strings.TrimSpace(line), ",")
			if len(fields) < 6 {
				continue
			}
			m := adjRe.FindStringSubmatch(fields[2])
			if m == nil {
				continue
			}
			lid, _ := strconv.ParseInt(m[1], 10, 64)
			bid, _ := strconv.ParseInt(m[2], 10, 64)
			before, _ := strconv.ParseInt(fields[3], 10, 64)
			after, _ := strconv.ParseInt(fields[4], 10, 64)
			delta := after - before
			if m[3] == "credit" {
				delta = -delta
			}
			key := k{bcode[bid], acctByLegacy[lid]}
			if m[4] == "1" {
				add(priorNet, key, delta)
			} else {
				add(curNet, key, delta)
			}
		}
	}
	// expected target nets
	expected := map[string]string{}
	branchesSeen := map[string]bool{}
	for key := range priorNet {
		branchesSeen[key.branch] = true
	}
	for key := range curNet {
		branchesSeen[key.branch] = true
	}
	for br := range branchesSeen {
		var priorPL = new(big.Int)
		for key, n := range priorNet {
			if key.branch == br && (metaOf[key.code].typ == "revenue" || metaOf[key.code].typ == "expense") {
				priorPL.Add(priorPL, n)
			}
		}
		codes := map[string]bool{}
		for key := range priorNet {
			if key.branch == br {
				codes[key.code] = true
			}
		}
		for key := range curNet {
			if key.branch == br {
				codes[key.code] = true
			}
		}
		for code := range codes {
			m := metaOf[code]
			total := new(big.Int)
			if n := curNet[k{br, code}]; n != nil {
				total.Add(total, n)
			}
			bs := m.typ == "asset" || m.typ == "liability" || m.typ == "equity"
			if bs {
				if n := priorNet[k{br, code}]; n != nil {
					total.Add(total, n)
				}
			}
			if m.sys == "opening_equity" {
				total.Add(total, priorPL)
			}
			expected[br+"|"+code] = total.String()
		}
	}
	for key, val := range expected {
		if val == "0" {
			delete(expected, key)
		}
	}
	_ = head
	// Migration corrections (ADR-0018 D11) are not legacy balances; V-26 checks them.
	got, err := queryMap(v.ctx, v.t, `SELECT b.code, a.code, SUM(l.debit) - SUM(l.credit) FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id=l.entry_id
		JOIN finance_accounts a ON a.id=l.account_id JOIN branches b ON b.id=e.branch_id
		WHERE COALESCE(e.source_doc_type, '') <> 'migration_correction' GROUP BY 1,2`)
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	d := diffMaps(expected, got)
	show := d
	if len(show) > 5 {
		show = show[:5]
	}
	v.add(id, desc, strconv.Itoa(len(expected))+" saldo", strconv.Itoa(len(got))+" saldo", strings.Join(show, "; "), "penyesuaian pembulatan jurnal dari adjustments.csv", len(d) == 0)
}

func (v *verifier) journalShape() {
	v.targetZero("V-13a", "setiap jurnal seimbang",
		`SELECT COUNT(*) FROM (SELECT entry_id FROM finance_journal_lines GROUP BY entry_id HAVING SUM(debit) <> SUM(credit)) x`)
	v.targetZero("V-13b", "jurnal tanpa baris",
		`SELECT COUNT(*) FROM finance_journal_entries e WHERE NOT EXISTS (SELECT 1 FROM finance_journal_lines l WHERE l.entry_id = e.id)`)
	v.targetZero("V-13c", "lebih dari satu jurnal saldo awal per cabang",
		`SELECT COUNT(*) FROM (SELECT branch_id FROM finance_journal_entries WHERE source_doc_type='opening' GROUP BY branch_id HAVING COUNT(*) > 1) x`)
	v.scalar("V-12", "jumlah jurnal tahun berjalan per bulan",
		`SELECT COUNT(*) FROM {finance_journal_entries} WHERE {co} = :c AND status IN ('posted','reversed') AND DATE(DATE_ADD(journal_date, INTERVAL 7 HOUR)) >= :open AND EXISTS (SELECT 1 FROM {finance_journal_lines} zl WHERE zl.id_finance_journal_entry={finance_journal_entries}.id_finance_journal_entry AND (ROUND(zl.debit_amount) <> 0 OR ROUND(zl.credit_amount) <> 0))`,
		"SELECT COUNT(*) FROM finance_journal_entries WHERE source_doc_type = 'legacy_je'", 0)
}

// ---------------------------------------------------------------- V-14

func (v *verifier) costLedger() {
	// Opening rows without a movement count too: the transform spreads their
	// value over the product's opening_stock movements (Q-HPP-2), so per
	// (branch, product) the sums must still agree.
	v.grouped("V-14", "Σ biaya HPP bertanda per cabang x produk (baris asli + saldo awal, tanpa estimasi)",
		`SELECT b.code, p.product_code, SUM(CASE WHEN c.movement_type IN ('in','opening') THEN ROUND(c.total_cost_amount) ELSE -ROUND(c.total_cost_amount) END)
		   FROM {finance_inventory_cost_movements} c JOIN {branches} b ON b.id_branch=c.id_branch JOIN {products} p ON p.id_product=c.id_product
		  WHERE c.{co} = :c AND (c.id_inventory_movement IS NOT NULL OR c.movement_type = 'opening') GROUP BY 1,2`,
		`SELECT b.code, p.code, SUM(c.total_cost) FROM finance_inventory_cost_movements c
		   JOIN branches b ON b.id=c.branch_id JOIN products p ON p.id=c.product_id WHERE c.is_estimated = 0 AND c.ref_type <> 'opening_cost' GROUP BY 1,2`, nil)
	// Legacy's authority is its cost STATE (= its Persediaan account); value it
	// moved without cost rows arrives as cost-only rows, so per (branch,
	// product) the revamp's non-estimated ledger must equal the state.
	v.grouped("V-14d", "nilai ledger HPP per cabang x produk = keadaan biaya legacy (finance_inventory_cost_states) ± koreksi tanpa qty/tanpa nilai",
		`SELECT b.code, p.product_code, ROUND(s.inventory_value)
		   FROM {finance_inventory_cost_states} s JOIN {branches} b ON b.id_branch=s.id_branch JOIN {products} p ON p.id_product=s.id_product
		  WHERE s.{co} = :c`,
		`SELECT b.code, p.code, SUM(c.total_cost) FROM finance_inventory_cost_movements c
		   JOIN branches b ON b.id=c.branch_id JOIN products p ON p.id=c.product_id WHERE c.is_estimated = 0 GROUP BY 1,2`, v.lessWriteDowns)
	v.targetZero("V-14b", "mutasi di bawah kursor HPP tanpa baris biaya",
		`SELECT COUNT(*) FROM stock_movements m JOIN products p ON p.id=m.product_id AND p.stock_tracked=1
		   JOIN (SELECT branch_id, MAX(stock_movement_id) mx FROM finance_inventory_cost_movements WHERE stock_movement_id < 4611686018427387904 GROUP BY branch_id) c ON c.branch_id=m.branch_id
		  WHERE m.id < c.mx AND NOT EXISTS (SELECT 1 FROM finance_inventory_cost_movements x WHERE x.stock_movement_id = m.id)`)
	// Posting matches every SJ line to its SO line (or, for a replacement, to
	// the return's replacement line) by (product, variant, unit); a line
	// without its partner can never be journalled after cutover.
	v.targetZero("V-28", "item SJ terkonfirmasi punya baris SO / barang pengganti retur dengan produk, varian, satuan sama (dapat diposting)",
		`SELECT COUNT(*) FROM delivery_note_items i JOIN delivery_notes d ON d.id = i.delivery_id WHERE d.status = 'confirmed' AND (
		   (d.document_kind = 'order' AND NOT EXISTS (SELECT 1 FROM sales_order_items l WHERE l.order_id = d.sales_order_id AND l.product_id = i.product_id AND l.variant_id = i.variant_id AND l.uom = i.uom))
		OR (d.document_kind = 'replacement' AND NOT EXISTS (SELECT 1 FROM sales_return_replacement_items r WHERE r.return_id = d.sales_return_id AND r.product_id = i.product_id AND r.variant_id = i.variant_id AND r.uom = i.uom)))`)
	// The revamp sums total_cost as-is (InsertCostRow signs an out negative):
	// an unsigned out would ADD its value to the position.
	v.targetZero("V-14c", "tanda total biaya sesuai arah (keluar <= 0, masuk >= 0)",
		`SELECT COUNT(*) FROM finance_inventory_cost_movements WHERE (direction = 'out' AND total_cost > 0) OR (direction = 'in' AND total_cost < 0)`)
}

// ---------------------------------------------------------------- V-15

func (v *verifier) media() {
	const id, desc = "V-15", "lampiran: baris media sesuai berkas (ada, ukuran sama)"
	if cands, _ := transform.ReadRows(v.work, "media_candidates"); len(cands) == 0 {
		v.add(id, desc, "0", "0", "", "tidak ada lampiran", true)
		return
	}
	rep, err := mediacopy.ReadReport(v.work)
	if err != nil {
		v.add(id, desc, "-", "-", "fase media belum dijalankan", "", false)
		return
	}
	rows, err := v.t.QueryContext(v.ctx, "SELECT file_key, size_bytes, owner_type, owner_id FROM media_files WHERE origin = 'legacy'")
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	type mrow struct {
		key, ot   string
		size, oid int64
	}
	var all []mrow
	for rows.Next() {
		var m mrow
		if err := rows.Scan(&m.key, &m.size, &m.ot, &m.oid); err != nil {
			_ = rows.Close()
			v.fail(id, desc, err)
			return
		}
		all = append(all, m)
	}
	_ = rows.Close()
	n, bad := 0, []string{}
	for _, m := range all {
		key, ot, size, oid := m.key, m.ot, m.size, m.oid
		n++
		fi, ok := rep.Files[key]
		if !ok && !v.cfg.KeepMissingMedia {
			bad = append(bad, key+": tidak ada di laporan salin")
			continue
		}
		if ok && fi.Size != size {
			bad = append(bad, key+": ukuran berbeda")
		}
		if v.cfg.MediaDst != "" {
			st, err := os.Stat(filepath.Join(v.cfg.MediaDst, filepath.FromSlash(key)))
			if ok && (err != nil || st.Size() != size) {
				bad = append(bad, key+": berkas tujuan hilang/ukuran berbeda")
			}
		}
		// the owner must exist (design: owner_id goes through the id map)
		var table string
		switch ot {
		case "product":
			table = "products"
		case "delivery_proof", "delivery_document":
			table = "delivery_notes"
		case "payment_proof":
			table = "payments"
		}
		if table != "" {
			var c int
			if err := v.t.QueryRowContext(v.ctx, "SELECT COUNT(*) FROM `"+table+"` WHERE id = ?", oid).Scan(&c); err != nil || c == 0 {
				bad = append(bad, fmt.Sprintf("%s: pemilik %s/%d tidak ada", key, ot, oid))
			}
		}
	}
	if len(bad) > 5 {
		bad = bad[:5]
	}
	v.add(id, desc, strconv.Itoa(rep.Copied+rep.Skipped)+" berkas", strconv.Itoa(n)+" baris", strings.Join(bad, "; "), strconv.Itoa(rep.Missing)+" berkas hilang di sumber (missing_media.csv)", len(bad) == 0 && n == rep.Copied+rep.Skipped)
}

// ---------------------------------------------------------------- V-17/18

func (v *verifier) numbers() {
	type pair struct{ id, desc, src, tgt string }
	for _, p := range []pair{
		{"V-17a", "nomor order", "SELECT order_number FROM {orders} WHERE " + byBranch, "SELECT number FROM sales_orders UNION ALL SELECT number FROM purchase_orders"},
		{"V-17b", "nomor pembayaran", "SELECT payment_number FROM {payments} WHERE " + byBranch, "SELECT number FROM payments"},
		{"V-17c", "nomor surat jalan", "SELECT sj_number FROM {delivery_notes} WHERE " + byBranch, "SELECT number FROM delivery_notes"},
		{"V-17d", "nomor transfer", "SELECT transfer_number FROM {stock_transfers} WHERE {co} = :c", "SELECT number FROM stock_transfers"},
		{"V-17f", "nomor retur penjualan", "SELECT return_number FROM {sales_returns} WHERE {co} = :c", "SELECT number FROM sales_returns"},
		{"V-17g", "nomor retur pembelian", "SELECT return_number FROM {purchase_returns} WHERE {co} = :c", "SELECT number FROM purchase_returns"},
		{"V-17e", "nomor jurnal tahun berjalan", "SELECT journal_number FROM {finance_journal_entries} WHERE {co} = :c AND status IN ('posted','reversed') AND DATE(DATE_ADD(journal_date, INTERVAL 7 HOUR)) >= :open AND EXISTS (SELECT 1 FROM {finance_journal_lines} zl WHERE zl.id_finance_journal_entry={finance_journal_entries}.id_finance_journal_entry AND (ROUND(zl.debit_amount) <> 0 OR ROUND(zl.credit_amount) <> 0))", "SELECT number FROM finance_journal_entries"},
	} {
		src, err := v.col(v.l.SQL, v.lq(p.src))
		if err != nil {
			v.fail(p.id, p.desc, err)
			continue
		}
		tgt, err := v.col(v.t, p.tgt)
		if err != nil {
			v.fail(p.id, p.desc, err)
			continue
		}
		have := map[string]int{}
		for _, n := range tgt {
			have[n]++
		}
		var problems []string
		for _, n := range src {
			if have[n] == 0 {
				problems = append(problems, "hilang: "+n)
			}
		}
		for n, c := range have {
			if c > 1 {
				problems = append(problems, "ganda: "+n)
			}
		}
		sort.Strings(problems)
		if len(problems) > 5 {
			problems = problems[:5]
		}
		v.add(p.id, p.desc+" ada persis dan tidak ganda", strconv.Itoa(len(src)), strconv.Itoa(len(tgt)), strings.Join(problems, "; "), "", len(problems) == 0)
	}
}

func (v *verifier) nextNumbers() {
	const id, desc = "V-18", "nomor berikutnya tiap (cabang, jenis) tidak menabrak nomor yang ada"
	existing := map[string]bool{}
	for _, q := range []string{"SELECT number FROM sales_orders", "SELECT number FROM purchase_orders", "SELECT number FROM payments", "SELECT number FROM delivery_notes",
		"SELECT number FROM goods_receipts", "SELECT number FROM stock_transfers", "SELECT number FROM sales_returns", "SELECT number FROM purchase_returns"} {
		list, err := v.col(v.t, q)
		if err != nil {
			v.fail(id, desc, err)
			return
		}
		for _, n := range list {
			existing[n] = true
		}
	}
	rows, err := v.t.QueryContext(v.ctx, "SELECT b.code, s.doc_kind, s.current_value FROM branch_document_sequences s JOIN branches b ON b.id=s.branch_id")
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	defer func() { _ = rows.Close() }()
	var clash []string
	n := 0
	now := time.Now().UTC()
	years := []time.Time{now, v.cfg.OpeningTime()}
	for rows.Next() {
		var code, kind string
		var val int64
		if err := rows.Scan(&code, &kind, &val); err != nil {
			v.fail(id, desc, err)
			return
		}
		for _, y := range years {
			if num, ok := sequences.NextNumber(code, kind, val, y); ok {
				n++
				if existing[num] {
					clash = append(clash, num)
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		v.fail(id, desc, err)
		return
	}
	// Finance counters (JV journals, EXP expenses): the next number must be
	// new too, including after a legacy series the counter continues.
	for _, q := range []string{"SELECT number FROM finance_journal_entries", "SELECT number FROM business_expenses"} {
		list, err := v.col(v.t, q)
		if err != nil {
			v.fail(id, desc, err)
			return
		}
		for _, x := range list {
			existing[x] = true
		}
	}
	fin, err := queryMap(v.ctx, v.t, "SELECT b.code, s.seq_key, s.current_value FROM finance_document_sequences s JOIN branches b ON b.id=s.branch_id")
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	for k, val := range fin {
		code, key, _ := strings.Cut(k, "|")
		cur, _ := strconv.ParseInt(val, 10, 64)
		if num, ok := sequences.FinanceNextNumber(code, key, cur); ok {
			n++
			if existing[num] {
				clash = append(clash, num)
			}
		}
	}
	sort.Strings(clash)
	v.add(id, desc, strconv.Itoa(n)+" simulasi", strconv.Itoa(len(clash))+" bentrok", strings.Join(clash, ","), "", len(clash) == 0)
}

// ---------------------------------------------------------------- V-19

func (v *verifier) orphans() {
	checks := map[string]string{
		"item penjualan tanpa header":     "SELECT COUNT(*) FROM sales_order_items i LEFT JOIN sales_orders o ON o.id=i.order_id WHERE o.id IS NULL",
		"item pembelian tanpa header":     "SELECT COUNT(*) FROM purchase_order_items i LEFT JOIN purchase_orders o ON o.id=i.order_id WHERE o.id IS NULL",
		"alokasi ke order yang tidak ada": "SELECT COUNT(*) FROM payment_allocations a WHERE (a.order_type='sales' AND a.order_id NOT IN (SELECT id FROM sales_orders)) OR (a.order_type='purchase' AND a.order_id NOT IN (SELECT id FROM purchase_orders)) OR (a.order_type='sales_return' AND a.order_id NOT IN (SELECT id FROM sales_returns)) OR (a.order_type='purchase_return' AND a.order_id NOT IN (SELECT id FROM purchase_returns))",
		"SJ tanpa SO":                     "SELECT COUNT(*) FROM delivery_notes d LEFT JOIN sales_orders o ON o.id=d.sales_order_id WHERE o.id IS NULL",
		"penerimaan tanpa PO":             "SELECT COUNT(*) FROM goods_receipts g LEFT JOIN purchase_orders o ON o.id=g.purchase_order_id WHERE o.id IS NULL",
		"mutasi tanpa dokumen":            "SELECT COUNT(*) FROM stock_movements m WHERE m.ref_id IS NOT NULL AND ((m.ref_type='delivery' AND m.ref_id NOT IN (SELECT id FROM delivery_notes)) OR (m.ref_type='goods_receipt' AND m.ref_id NOT IN (SELECT id FROM goods_receipts)) OR (m.ref_type='sales_return' AND m.ref_id NOT IN (SELECT id FROM sales_returns)) OR (m.ref_type='purchase_return' AND m.ref_id NOT IN (SELECT id FROM purchase_returns)) OR (m.ref_type='transfer' AND m.ref_id NOT IN (SELECT id FROM stock_transfers)) OR (m.ref_type='opening' AND m.ref_id NOT IN (SELECT id FROM branches)))",
		"baris jurnal ke akun hilang":     "SELECT COUNT(*) FROM finance_journal_lines l LEFT JOIN finance_accounts a ON a.id=l.account_id WHERE a.id IS NULL",
	}
	keys := make([]string, 0, len(checks))
	for k := range checks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		v.targetZero(fmt.Sprintf("V-19%c", 'a'+i), "orphan: "+k, checks[k])
	}
}

// ---------------------------------------------------------------- V-20/21

func (v *verifier) postingMarks() {
	const id, desc = "V-20", "antrean posting: dokumen di atas batas cutover = jumlah penanda pending (per cabang x jenis)"
	marks, err := transform.ReadRows(v.work, "posting_marks")
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	want := map[string]string{}
	for _, m := range marks {
		if m.Str("state") == "pending" {
			key := m.Str("branch_id") + "|" + m.Str("doc_type")
			n, _ := strconv.Atoi(want[key])
			want[key] = strconv.Itoa(n + 1)
		}
	}
	got, err := queryMap(v.ctx, v.t, `SELECT x.branch_id, x.t, COUNT(*) FROM (
		SELECT d.branch_id, 'delivery' t, d.id FROM delivery_notes d WHERE d.status='confirmed' AND d.id > COALESCE((SELECT covered_through_id FROM finance_cutover c WHERE c.branch_id=d.branch_id AND c.doc_type='delivery'),0)
		UNION ALL SELECT g.branch_id, 'goods_receipt', g.id FROM goods_receipts g WHERE g.status='posted' AND g.id > COALESCE((SELECT covered_through_id FROM finance_cutover c WHERE c.branch_id=g.branch_id AND c.doc_type='goods_receipt'),0)
		UNION ALL SELECT p.branch_id, 'payment', p.id FROM payments p WHERE p.status='active' AND p.method <> 'offset' AND p.id > COALESCE((SELECT covered_through_id FROM finance_cutover c WHERE c.branch_id=p.branch_id AND c.doc_type='payment'),0)
		UNION ALL SELECT r.branch_id, 'sales_return', r.id FROM sales_returns r WHERE r.status='confirmed' AND r.id > COALESCE((SELECT covered_through_id FROM finance_cutover c WHERE c.branch_id=r.branch_id AND c.doc_type='sales_return'),0)
		UNION ALL SELECT r.branch_id, 'purchase_return', r.id FROM purchase_returns r WHERE r.status='confirmed' AND r.id > COALESCE((SELECT covered_through_id FROM finance_cutover c WHERE c.branch_id=r.branch_id AND c.doc_type='purchase_return'),0)
		UNION ALL SELECT r.branch_id, 'sales_return_settlement', s.id FROM sales_return_settlements s JOIN sales_returns r ON r.id=s.return_id WHERE r.status='confirmed' AND s.settlement_type='customer_credit' AND s.id > COALESCE((SELECT covered_through_id FROM finance_cutover c WHERE c.branch_id=r.branch_id AND c.doc_type='sales_return_settlement'),0)
		UNION ALL SELECT r.branch_id, 'purchase_return_settlement', s.id FROM purchase_return_settlements s JOIN purchase_returns r ON r.id=s.return_id WHERE r.status='confirmed' AND s.settlement_type='supplier_credit' AND s.id > COALESCE((SELECT covered_through_id FROM finance_cutover c WHERE c.branch_id=r.branch_id AND c.doc_type='purchase_return_settlement'),0)
		) x GROUP BY 1,2`)
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	// A non-pending document sitting above the boundary would be queued again
	// (double posting): the counts must agree exactly.
	d := diffMaps(want, got)
	if len(d) > 5 {
		d = d[:5]
	}
	v.add(id, desc, strconv.Itoa(len(want))+" kelompok pending", strconv.Itoa(len(got))+" kelompok di antrean", strings.Join(d, "; "), "", len(d) == 0)
}

func (v *verifier) periods() {
	const id, desc = "V-21", "periode yang ditutup legacy juga tertutup di setiap cabang"
	rows, err := v.l.SQL.QueryContext(v.ctx, v.lq("SELECT date_from, date_to FROM {finance_periods} WHERE {co} = :c AND status = 'closed'"))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	months := map[string]bool{}
	for rows.Next() {
		var a, b string
		_ = rows.Scan(&a, &b)
		from, e1 := time.Parse("2006-01-02", a[:10])
		to, e2 := time.Parse("2006-01-02", b[:10])
		if e1 != nil || e2 != nil {
			continue
		}
		for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC); !m.After(to); m = m.AddDate(0, 1, 0) {
			months[m.Format("2006-01")] = true
		}
	}
	_ = rows.Close()
	branches, err := v.one(v.t, "SELECT COUNT(*) FROM branches")
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	nb, _ := strconv.Atoi(branches)
	closed, err := v.col(v.t, "SELECT CONCAT(year, '-', LPAD(month, 2, '0')) FROM finance_periods WHERE status = 'closed'")
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	have := map[string]int{}
	for _, c := range closed {
		have[c]++
	}
	var missing []string
	for m := range months {
		if have[m] != nb {
			missing = append(missing, m)
		}
	}
	sort.Strings(missing)
	v.add(id, desc, strconv.Itoa(len(months))+" bulan x "+branches+" cabang", strconv.Itoa(len(closed))+" baris", strings.Join(missing, ","), "", len(missing) == 0)
}

// ---------------------------------------------------------------- V-24, access

func (v *verifier) archive() {
	const id, desc = "V-24", "arsip data tak termigrasi lengkap (jumlah baris = sumber)"
	raw, err := os.ReadFile(filepath.Join(v.work, "archive", "manifest.json"))
	if err != nil {
		v.add(id, desc, "-", "-", "fase archive belum dijalankan", "", false)
		return
	}
	var man struct {
		Files map[string]struct {
			Rows int64 `json:"rows"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &man); err != nil {
		v.fail(id, desc, err)
		return
	}
	src, err := os.ReadFile(filepath.Join(extract.RawDir(v.work), "manifest.json"))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	var rm extract.Manifest
	_ = json.Unmarshal(src, &rm)
	var bad []string
	for name, f := range man.Files {
		if strings.HasPrefix(name, "raw/") {
			t := strings.TrimSuffix(strings.TrimPrefix(name, "raw/"), ".jsonl.gz")
			if rm.Tables[t].Rows != f.Rows {
				bad = append(bad, fmt.Sprintf("%s: arsip %d != sumber %d", t, f.Rows, rm.Tables[t].Rows))
			}
		}
	}
	sort.Strings(bad)
	v.add(id, desc, strconv.Itoa(len(man.Files))+" berkas", strconv.Itoa(len(man.Files)), strings.Join(bad, "; "), "", len(bad) == 0)
}

func (v *verifier) accessSets() {
	// user -> role set must be identical by name (permission-level identity belongs to F3b).
	v.grouped("V-16d", "peran per pengguna",
		`SELECT u.username, GROUP_CONCAT(r.code ORDER BY r.code SEPARATOR ','), COUNT(*) FROM {user_roles} ur JOIN {users} u ON u.id_user=ur.id_user JOIN {roles} r ON r.id_role=ur.id_role
		  WHERE u.{co} = :c AND u.username <> 'superadmin' AND r.code <> 'superadmin' GROUP BY 1`,
		`SELECT u.username, GROUP_CONCAT(r.code ORDER BY r.code SEPARATOR ','), COUNT(*) FROM user_roles ur JOIN users u ON u.id=ur.user_id JOIN roles r ON r.id=ur.role_id GROUP BY 1`, nil)
}

// WriteReport stores verify.json and verify.md under report/.
func WriteReport(work string, r *Report) error {
	dir := transform.ReportDir(work)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "verify.json"), raw, 0o644); err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("# Hasil verifikasi\n\n| id | deskripsi | sumber | target | selisih | terjelaskan | status |\n|---|---|---|---|---|---|---|\n")
	for _, c := range r.Checks {
		fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s | %s | %s |\n", c.ID, c.Desc, c.Source, c.Target, strings.ReplaceAll(c.Diff, "|", "/"), c.Explained, c.Status)
	}
	if r.Pass {
		sb.WriteString("\n**LULUS**\n")
	} else {
		sb.WriteString("\n**GAGAL**: " + strings.Join(r.Failed(), ", ") + "\n")
	}
	return os.WriteFile(filepath.Join(dir, "verify.md"), []byte(sb.String()), 0o644)
}

var _ = row.Row{}

// ---------------------------------------------------------------- V-26/V-27 migration corrections

// corrections reads out/migration_corrections.jsonl and the target branch codes.
func (v *verifier) corrections() ([]row.Row, map[string]string, error) {
	rows, err := transform.ReadRows(v.work, "migration_corrections")
	if err != nil {
		return nil, nil, err
	}
	codes := map[string]string{}
	q, err := v.t.QueryContext(v.ctx, "SELECT id, code FROM branches")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = q.Close() }()
	for q.Next() {
		var id, code string
		if err := q.Scan(&id, &code); err != nil {
			return nil, nil, err
		}
		codes[id] = code
	}
	return rows, codes, q.Err()
}

// lessWriteDowns applies the state corrections (written down: no qty;
// written up: no value) to the legacy state before V-14d compares it with
// the ledger.
func (v *verifier) lessWriteDowns(src map[string]string) {
	rows, codes, err := v.corrections()
	if err != nil {
		return
	}
	for _, r := range rows {
		var sign int64
		switch r.Str("kind") {
		case "state_no_qty": // written down to zero
			sign = -1
		case "state_no_value", "cost_revalue": // written up (a revaluation's amount is signed)
			sign = 1
		default:
			continue
		}
		key := codes[r.Str("branch_id")] + "|" + r.Str("product_code")
		cur, _ := new(big.Rat).SetString(src[key])
		if cur == nil {
			cur = new(big.Rat)
		}
		src[key] = norm(new(big.Rat).Add(cur, new(big.Rat).Mul(big.NewRat(sign, 1), r.Rat("amount"))).RatString())
	}
}

func (v *verifier) migrationCorrections() {
	const id22, desc22 = "V-26", "jurnal koreksi migrasi = daftar koreksi (per jurnal) = Σ temuan DN_CANCEL_COGS, COST_STATE_NO_QTY/NO_VALUE, COGS_UNPOSTED, STOCK_UNPOSTED, REVENUE_UNBOOKED(_CANCELLED), COST_REVALUED"
	rows, codes, err := v.corrections()
	if err != nil {
		v.fail(id22, desc22, err)
		return
	}
	want := map[string]string{}
	var listed int64
	for _, r := range rows {
		n, _ := strconv.ParseInt(want[r.Str("journal_number")], 10, 64)
		a := r.Int("amount")
		if a < 0 {
			a = -a
		}
		want[r.Str("journal_number")] = strconv.FormatInt(n+a, 10)
		listed += r.Int("amount")
	}
	got, err := queryMap(v.ctx, v.t, `SELECT e.number, SUM(l.debit) FROM finance_journal_entries e JOIN finance_journal_lines l ON l.entry_id = e.id
		WHERE e.source_doc_type = 'migration_correction' AND e.status = 'posted' GROUP BY 1`)
	if err != nil {
		v.fail(id22, desc22, err)
		return
	}
	// The findings carry the amounts independently of the correction table.
	var found int64
	if f, err := os.ReadFile(filepath.Join(transform.ReportDir(v.work), "findings.csv")); err == nil {
		re := regexp.MustCompile(`Rp(-?\d+)`)
		for _, line := range strings.Split(string(f), "\n") {
			if !strings.Contains(line, ",DN_CANCEL_COGS,") && !strings.Contains(line, ",COST_STATE_NO_QTY,") && !strings.Contains(line, ",REVENUE_UNBOOKED,") && !strings.Contains(line, ",REVENUE_UNBOOKED_CANCELLED,") && !strings.Contains(line, ",COST_REVALUED,") &&
				!strings.Contains(line, ",COGS_UNPOSTED,") && !strings.Contains(line, ",STOCK_UNPOSTED,") && !strings.Contains(line, ",COST_STATE_NO_VALUE,") {
				continue
			}
			if m := re.FindStringSubmatch(line); m != nil {
				n, _ := strconv.ParseInt(m[1], 10, 64)
				found += n
			}
		}
	}
	d := diffMaps(want, got)
	if found != listed {
		d = append(d, fmt.Sprintf("Σ temuan %d != Σ daftar koreksi %d", found, listed))
	}
	if len(d) > 5 {
		d = d[:5]
	}
	v.add(id22, desc22, strconv.Itoa(len(want))+" jurnal / Rp"+strconv.FormatInt(found, 10), strconv.Itoa(len(got))+" jurnal / Rp"+strconv.FormatInt(listed, 10),
		strings.Join(d, "; "), "", len(d) == 0)

	// V-27: after the corrections the books' Persediaan and the cost ledger
	// differ by exactly what they differed in legacy (legacy books − legacy
	// cost state, both in rupiah; production: rounding only), leaving out
	// estimated rows of documents legacy never journalled (the posting queue
	// books them later). Any shift made by the migration fails the rehearsal.
	const id23, desc23 = "V-27", "buku Persediaan - ledger biaya HPP setelah koreksi = selisih bawaan legacy (buku - keadaan biaya)"
	var book, ledger, corrected, legacyBook, legacyState int64
	q := func(db Querier, dst *int64, sqlText string) error {
		var s sql.NullString
		if err := db.QueryRowContext(v.ctx, sqlText).Scan(&s); err != nil {
			return err
		}
		*dst = transform.RoundHalfAway(row.ParseRat(norm(s.String)))
		return nil
	}
	steps := []struct {
		db  Querier
		dst *int64
		q   string
	}{
		{v.t, &book, `SELECT COALESCE(SUM(l.debit) - SUM(l.credit), 0) FROM finance_journal_lines l JOIN finance_account_mappings m ON m.account_id = l.account_id AND m.mapping_key = 'inventory'`},
		{v.t, &ledger, `SELECT COALESCE(SUM(total_cost), 0) FROM finance_inventory_cost_movements WHERE is_estimated = 0`},
		{v.l.SQL, &legacyBook, v.lq(`SELECT COALESCE(SUM(ROUND(l.debit_amount)) - SUM(ROUND(l.credit_amount)), 0) FROM {finance_journal_lines} l
			JOIN {finance_journal_entries} e ON e.id_finance_journal_entry = l.id_finance_journal_entry JOIN {finance_accounts} a ON a.id_finance_account = l.id_finance_account
			WHERE e.{co} = :c AND e.status IN ('posted','reversed') AND a.system_key = 'inventory'`)},
		{v.l.SQL, &legacyState, v.lq(`SELECT COALESCE(SUM(ROUND(inventory_value)), 0) FROM {finance_inventory_cost_states} WHERE {co} = :c`)},
	}
	var ids []string
	for _, r := range rows {
		if r.Has("stock_movement_id") {
			ids = append(ids, r.Str("stock_movement_id"))
		}
	}
	if len(ids) > 0 {
		steps = append(steps, struct {
			db  Querier
			dst *int64
			q   string
		}{v.t, &corrected, "SELECT COALESCE(SUM(total_cost), 0) FROM finance_inventory_cost_movements WHERE is_estimated = 1 AND stock_movement_id IN (" + strings.Join(ids, ",") + ")"})
	}
	for _, st := range steps {
		if err := q(st.db, st.dst, st.q); err != nil {
			v.fail(id23, desc23, err)
			return
		}
	}
	_ = codes
	gap, legacyGap := book-(ledger+corrected), legacyBook-legacyState
	// Journal balancing (K4) may move a rupiah or few onto an inventory line.
	tol := v.explain["balance_entry_adjustments"] * v.cfg.MaxBalanceDelta
	shift := gap - legacyGap
	if shift < 0 {
		shift = -shift
	}
	v.add(id23, desc23, "Rp"+strconv.FormatInt(legacyGap, 10), "Rp"+strconv.FormatInt(gap, 10),
		fmt.Sprintf("buku Rp%d, ledger Rp%d (+ estimasi terkoreksi Rp%d)", book, ledger, corrected),
		fmt.Sprintf("selisih bawaan legacy: buku Rp%d - keadaan biaya Rp%d; toleransi penyeimbang jurnal Rp%d", legacyBook, legacyState, tol), shift <= tol)
}

// ---------------------------------------------------------------- V-29 receivable sign

// receivableTolerance absorbs the rupiah rounding between the pro-rated SJ
// value estimated here and the revamp's own posting, per customer.
const receivableTolerance = 100

// receivableSQL projects each customer's receivable in the books (Piutang
// lines with the party, every journal including the migration corrections)
// to after the cutover-day posting queue: plus the confirmed order SJs above
// the delivery boundary not yet journalled, minus the incoming payments above
// the payment boundary not yet journalled. A negative projection must be
// explained by money the customer paid ahead: payments on orders beyond what
// was delivered (drafts included), open return credit, and exchange
// replacements not yet shipped. Anything else is revenue the books never got
// (REVENUE_UNBOOKED) or a payment booked twice.
const receivableSQL = `WITH bk AS (
  SELECT l.party_id p, SUM(l.debit) - SUM(l.credit) bal FROM finance_journal_lines l
    JOIN finance_account_mappings m ON m.account_id = l.account_id AND m.mapping_key = 'receivable'
    JOIN finance_journal_entries e ON e.id = l.entry_id AND e.status IN ('posted','reversed')
   WHERE l.party_id IS NOT NULL GROUP BY 1),
dv AS (
  SELECT d.id dn, d.sales_order_id so, d.branch_id br, SUM(i.qty / NULLIF(l.qty, 0) * l.line_total) v
    FROM delivery_notes d JOIN delivery_note_items i ON i.delivery_id = d.id
    JOIN sales_order_items l ON l.id = (SELECT MIN(x.id) FROM sales_order_items x WHERE x.order_id = d.sales_order_id AND x.product_id = i.product_id AND x.variant_id = i.variant_id AND x.uom = i.uom)
   WHERE d.status = 'confirmed' AND d.document_kind = 'order' GROUP BY 1,2,3),
pd AS (
  SELECT o.party_id p, SUM(dv.v) v FROM dv JOIN sales_orders o ON o.id = dv.so
    LEFT JOIN finance_cutover c ON c.branch_id = dv.br AND c.doc_type = 'delivery'
   WHERE dv.dn > COALESCE(c.covered_through_id, 0)
     AND NOT EXISTS (SELECT 1 FROM finance_journal_entries e WHERE e.source_doc_type = 'delivery' AND e.source_doc_id = dv.dn)
   GROUP BY 1),
pp AS (
  SELECT p.party_id p, SUM(p.amount) v FROM payments p
    LEFT JOIN finance_cutover c ON c.branch_id = p.branch_id AND c.doc_type = 'payment'
   WHERE p.direction = 'in' AND p.status = 'active' AND p.method <> 'offset' AND p.id > COALESCE(c.covered_through_id, 0)
     AND NOT EXISTS (SELECT 1 FROM finance_journal_entries e WHERE e.source_doc_type = 'payment' AND e.source_doc_id = p.id)
   GROUP BY 1),
od AS (
  SELECT o.party_id p, GREATEST(COALESCE((SELECT SUM(a.amount) FROM payment_allocations a JOIN payments y ON y.id = a.payment_id
      WHERE a.order_type = 'sales' AND a.order_id = o.id AND y.status = 'active' AND y.method <> 'offset' AND y.direction = 'in'), 0)
    - COALESCE((SELECT SUM(dv.v) FROM dv WHERE dv.so = o.id), 0), 0) ahead
    FROM sales_orders o WHERE o.status <> 'cancelled'),
rc AS (
  SELECT o.party_id p, SUM(CASE WHEN r.return_mode = 'exchange' THEN GREATEST(-r.difference_amount, 0) ELSE r.total END
      - COALESCE((SELECT SUM(a.amount) FROM payment_allocations a JOIN payments y ON y.id = a.payment_id
          WHERE a.order_type = 'sales_return' AND a.order_id = r.id AND y.status = 'active'), 0)
      + CASE WHEN r.return_mode = 'exchange' AND r.replacement_delivery_status = 'pending' THEN r.replacement_total ELSE 0 END) v
    FROM sales_returns r JOIN sales_orders o ON o.id = r.sales_order_id WHERE r.status = 'confirmed' GROUP BY 1),
ex AS (SELECT p, SUM(ahead) v FROM od GROUP BY 1)
SELECT pt.code, bk.bal + COALESCE(pd.v, 0) - COALESCE(pp.v, 0) projected, COALESCE(ex.v, 0) + GREATEST(COALESCE(rc.v, 0), 0) explained
  FROM bk JOIN parties pt ON pt.id = bk.p
  LEFT JOIN pd ON pd.p = bk.p LEFT JOIN pp ON pp.p = bk.p LEFT JOIN ex ON ex.p = bk.p LEFT JOIN rc ON rc.p = bk.p
 WHERE bk.bal + COALESCE(pd.v, 0) - COALESCE(pp.v, 0) < 0
 ORDER BY pt.code`

func (v *verifier) receivableSigns() {
	const id, desc = "V-29", "tidak ada piutang pelanggan negatif (sesudah antrean posting hari-H) tanpa uang muka/kredit retur yang menjelaskannya"
	rows, err := v.t.QueryContext(v.ctx, receivableSQL)
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	defer func() { _ = rows.Close() }()
	var bad []string
	neg, explained := 0, 0
	for rows.Next() {
		var code, projected, ex string
		if err := rows.Scan(&code, &projected, &ex); err != nil {
			v.fail(id, desc, err)
			return
		}
		neg++
		p, e := row.ParseRat(projected), row.ParseRat(ex)
		short := new(big.Rat).Add(p, e) // < 0: more credit than explained
		if short.Cmp(big.NewRat(-receivableTolerance, 1)) < 0 {
			bad = append(bad, fmt.Sprintf("%s: piutang Rp%s, terjelaskan Rp%s", code, strconv.FormatInt(transform.RoundHalfAway(p), 10), strconv.FormatInt(transform.RoundHalfAway(e), 10)))
			continue
		}
		explained++
	}
	if err := rows.Err(); err != nil {
		v.fail(id, desc, err)
		return
	}
	n := len(bad)
	if len(bad) > 5 {
		bad = bad[:5]
	}
	v.add(id, desc, strconv.Itoa(neg)+" pelanggan negatif", strconv.Itoa(n)+" tak terjelaskan", strings.Join(bad, "; "),
		fmt.Sprintf("%d terjelaskan uang muka/kredit retur; toleransi Rp%d per pelanggan", explained, receivableTolerance), n == 0)
}
