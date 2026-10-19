package verify

import (
	"sort"
	"strconv"
	"strings"

	"mini-erp/internal/bootstrap"
)

const srStatus = `CASE WHEN r.archived_at IS NOT NULL OR r.status = 'cancelled' THEN 'cancelled' ELSE 'confirmed' END`

// returns reconciles the return documents (V-23): counts, values per branch x
// status, the synthetic payments the migration derived from the settlements
// and the internal invariants of the target (line sums, offset pairs).
func (v *verifier) returns() {
	v.grouped("V-23a", "retur penjualan per cabang x status (jumlah, total, pajak, pengganti, selisih tukar)",
		`SELECT b.code, `+srStatus+` st, 'n', COUNT(*) FROM {sales_returns} r JOIN {branches} b ON b.id_branch=r.id_branch WHERE r.{co} = :c GROUP BY 1,2
		 UNION ALL SELECT b.code, `+srStatus+`, 'total', SUM(ROUND(r.returned_total_amount)) FROM {sales_returns} r JOIN {branches} b ON b.id_branch=r.id_branch WHERE r.{co} = :c GROUP BY 1,2
		 UNION ALL SELECT b.code, `+srStatus+`, 'tax', SUM(ROUND(r.returned_tax_amount)) FROM {sales_returns} r JOIN {branches} b ON b.id_branch=r.id_branch WHERE r.{co} = :c GROUP BY 1,2
		 UNION ALL SELECT b.code, `+srStatus+`, 'repl', SUM(ROUND(r.replacement_total_amount)) FROM {sales_returns} r JOIN {branches} b ON b.id_branch=r.id_branch WHERE r.{co} = :c GROUP BY 1,2
		 UNION ALL SELECT b.code, `+srStatus+`, 'diff', SUM(CASE WHEN r.return_mode='exchange' THEN ROUND(r.difference_amount) ELSE 0 END) FROM {sales_returns} r JOIN {branches} b ON b.id_branch=r.id_branch WHERE r.{co} = :c GROUP BY 1,2`,
		`SELECT b.code, x.st, x.m, x.v FROM (
		   SELECT branch_id, status st, 'n' m, COUNT(*) v FROM sales_returns GROUP BY 1,2
		   UNION ALL SELECT branch_id, status, 'total', SUM(total) FROM sales_returns GROUP BY 1,2
		   UNION ALL SELECT branch_id, status, 'tax', SUM(tax_total) FROM sales_returns GROUP BY 1,2
		   UNION ALL SELECT branch_id, status, 'repl', SUM(replacement_total) FROM sales_returns GROUP BY 1,2
		   UNION ALL SELECT branch_id, status, 'diff', SUM(difference_amount) FROM sales_returns GROUP BY 1,2) x JOIN branches b ON b.id = x.branch_id`, nil)
	v.grouped("V-23b", "retur pembelian per cabang x status (jumlah, total, pajak)",
		`SELECT b.code, `+srStatus+` st, 'n', COUNT(*) FROM {purchase_returns} r JOIN {branches} b ON b.id_branch=r.id_branch WHERE r.{co} = :c GROUP BY 1,2
		 UNION ALL SELECT b.code, `+srStatus+`, 'total', SUM(ROUND(r.returned_total_amount)) FROM {purchase_returns} r JOIN {branches} b ON b.id_branch=r.id_branch WHERE r.{co} = :c GROUP BY 1,2
		 UNION ALL SELECT b.code, `+srStatus+`, 'tax', SUM(ROUND(r.returned_tax_amount)) FROM {purchase_returns} r JOIN {branches} b ON b.id_branch=r.id_branch WHERE r.{co} = :c GROUP BY 1,2`,
		`SELECT b.code, x.st, x.m, x.v FROM (
		   SELECT branch_id, status st, 'n' m, COUNT(*) v FROM purchase_returns GROUP BY 1,2
		   UNION ALL SELECT branch_id, status, 'total', SUM(total) FROM purchase_returns GROUP BY 1,2
		   UNION ALL SELECT branch_id, status, 'tax', SUM(tax_total) FROM purchase_returns GROUP BY 1,2) x JOIN branches b ON b.id = x.branch_id`, nil)
	v.grouped("V-23c", "penyelesaian retur per jenis (nilai), tanpa offset otomatis",
		`SELECT 'sales', s.settlement_type, SUM(ROUND(s.amount)) FROM {sales_return_settlements} s JOIN {sales_returns} r ON r.id_sales_return=s.id_sales_return WHERE r.{co} = :c AND s.archived_at IS NULL GROUP BY 1,2
		 UNION ALL SELECT 'purchase', s.settlement_type, SUM(ROUND(s.amount)) FROM {purchase_return_settlements} s JOIN {purchase_returns} r ON r.id_purchase_return=s.id_purchase_return WHERE r.{co} = :c AND s.archived_at IS NULL GROUP BY 1,2`,
		`SELECT 'sales', settlement_type, SUM(amount) FROM sales_return_settlements WHERE auto = 0 GROUP BY 1,2
		 UNION ALL SELECT 'purchase', settlement_type, SUM(amount) FROM purchase_return_settlements GROUP BY 1,2`, nil)
	// refunds / collections of completed returns become payments allocated to the return
	v.grouped("V-23d", "pembayaran sintetis refund/tambah-bayar retur = penyelesaian legacy retur selesai",
		`SELECT 'sales', s.settlement_type, SUM(ROUND(s.amount)) FROM {sales_return_settlements} s JOIN {sales_returns} r ON r.id_sales_return=s.id_sales_return
		   WHERE r.{co} = :c AND s.archived_at IS NULL AND r.archived_at IS NULL AND r.status='completed' AND s.settlement_type IN ('refund','collect_payment') GROUP BY 1,2
		 UNION ALL SELECT 'purchase', s.settlement_type, SUM(ROUND(s.amount)) FROM {purchase_return_settlements} s JOIN {purchase_returns} r ON r.id_purchase_return=s.id_purchase_return
		   WHERE r.{co} = :c AND s.archived_at IS NULL AND r.archived_at IS NULL AND r.status='completed' AND s.settlement_type = 'refund' GROUP BY 1,2`,
		`SELECT CASE WHEN a.order_type='sales_return' THEN 'sales' ELSE 'purchase' END, CASE WHEN p.direction = 'out' AND a.order_type='sales_return' THEN 'refund'
		        WHEN a.order_type='sales_return' THEN 'collect_payment' ELSE 'refund' END, SUM(a.amount)
		   FROM payments p JOIN payment_allocations a ON a.payment_id = p.id
		  WHERE p.number LIKE 'PAY-%/MIG/%' AND p.method <> 'offset' AND a.order_type IN ('sales_return','purchase_return') GROUP BY 1,2`, nil)
	v.targetZero("V-23e", "pengurangan piutang/hutang retur = Σ kaki tagihan offset ke order",
		`SELECT (SELECT COUNT(*) FROM sales_returns r WHERE r.status='confirmed' AND r.receivable_reduction <> COALESCE((SELECT SUM(a.amount) FROM payment_allocations a JOIN payments p ON p.id=a.payment_id
		     WHERE a.order_type='sales' AND p.method='offset' AND p.status='active' AND p.client_ref = CONCAT('offset:sr:', r.id)),0))
		 + (SELECT COUNT(*) FROM purchase_returns r WHERE r.status='confirmed' AND r.payable_reduction <> COALESCE((SELECT SUM(a.amount) FROM payment_allocations a JOIN payments p ON p.id=a.payment_id
		     WHERE a.order_type='purchase' AND p.method='offset' AND p.status='active' AND p.client_ref = CONCAT('offset:pr:', r.id)),0))`)
	v.targetZero("V-23f", "kaki offset berpasangan, sama nominal, satu cabang",
		`SELECT COUNT(*) FROM payments p LEFT JOIN payments q ON q.id = p.pair_id WHERE p.method = 'offset' AND (q.id IS NULL OR q.pair_id <> p.id OR q.amount <> p.amount OR q.branch_id <> p.branch_id OR q.direction = p.direction)`)
	v.targetZero("V-23g", "invarian retur: Σ item = total; total = subtotal + pajak; pengganti sama",
		`SELECT (SELECT COUNT(*) FROM sales_returns r WHERE r.total <> COALESCE((SELECT SUM(line_total) FROM sales_return_items WHERE return_id=r.id),0) OR r.total <> r.subtotal + r.tax_total
		          OR r.replacement_total <> COALESCE((SELECT SUM(line_total) FROM sales_return_replacement_items WHERE return_id=r.id),0))
		      + (SELECT COUNT(*) FROM purchase_returns r WHERE r.total <> COALESCE((SELECT SUM(line_total) FROM purchase_return_items WHERE return_id=r.id),0) OR r.total <> r.subtotal + r.tax_total)`)
	v.targetZero("V-23h", "SJ pengganti menunjuk retur penjualan dan order yang sama",
		`SELECT COUNT(*) FROM delivery_notes d LEFT JOIN sales_returns r ON r.id = d.sales_return_id WHERE d.document_kind = 'replacement' AND (r.id IS NULL OR r.sales_order_id <> d.sales_order_id)`)
}

// ---------------------------------------------------------------- V-08 / V-09

// legacyOutstanding is the legacy per-order open amount (buildPaymentSummary):
// total + return adjustments - effective paid, never below 0. kind is
// 'sales' or 'purchase'; the legacy purchase side takes the returned value off
// the total and the refunded/credited part off the paid amount.
func legacyOutstanding(kind string) string {
	adj, settle := "", ""
	if kind == "sales" {
		adj = `(SELECT COALESCE(SUM(ROUND(r.difference_amount)),0) FROM {sales_returns} r WHERE r.id_original_order=o.id_order AND r.status='completed' AND r.archived_at IS NULL)`
		settle = `(SELECT COALESCE(SUM(CASE s.settlement_type WHEN 'collect_payment' THEN ROUND(s.amount) WHEN 'refund' THEN -ROUND(s.amount) WHEN 'customer_credit' THEN -ROUND(s.amount) ELSE 0 END),0)
		   FROM {sales_return_settlements} s JOIN {sales_returns} r ON r.id_sales_return=s.id_sales_return WHERE r.id_original_order=o.id_order AND r.status='completed' AND r.archived_at IS NULL AND s.archived_at IS NULL)`
	} else {
		adj = `(SELECT -COALESCE(SUM(ROUND(r.returned_total_amount)),0) FROM {purchase_returns} r WHERE r.id_original_order=o.id_order AND r.status='completed' AND r.archived_at IS NULL)`
		settle = `(SELECT -COALESCE(SUM(CASE WHEN s.settlement_type IN ('refund','supplier_credit') THEN ROUND(s.amount) ELSE 0 END),0)
		   FROM {purchase_return_settlements} s JOIN {purchase_returns} r ON r.id_purchase_return=s.id_purchase_return WHERE r.id_original_order=o.id_order AND r.status='completed' AND r.archived_at IS NULL AND s.archived_at IS NULL)`
	}
	paid := `(SELECT COALESCE(SUM(z.amt),0) FROM (
		 SELECT p.id_order, ROUND(p.amount) amt FROM {payments} p WHERE p.id_order IS NOT NULL AND p.archived_at IS NULL
		 UNION ALL SELECT a.id_order, ROUND(a.allocated_amount) FROM {payment_allocations} a JOIN {payments} p ON p.id_payment=a.id_payment WHERE a.archived_at IS NULL AND p.archived_at IS NULL) z
		 WHERE z.id_order=o.id_order)`
	return `GREATEST(0, ROUND(o.total_amount) + ` + adj + ` - GREATEST(0, ` + paid + ` + ` + settle + `))`
}

const legacyOpenOrder = `o.order_kind = '%s' AND o.archived_at IS NULL AND o.` + byBranch + ` AND o.id_current_status IN (SELECT id_order_status_definition FROM {order_status_definitions} WHERE status_group IN ('active','completed'))`

// balances: per-order open amounts (V-08) and per-party receivable/payable and
// open return credit (V-09), legacy formula against the revamp's allocation
// based balances. An exchange the customer still owes (charge) is part of the
// order position in legacy and of the return's open amount in the revamp, so
// the target adds it back to the order.
func (v *verifier) balances() {
	allocOf := func(t string) string {
		return `(SELECT a.order_id, SUM(a.amount) paid FROM payment_allocations a JOIN payments p ON p.id=a.payment_id WHERE a.order_type='` + t + `' AND p.status='active' GROUP BY a.order_id)`
	}
	targetSales := `GREATEST(0, o.grand_total - COALESCE(pa.paid,0) + COALESCE(ch.unpaid,0))`
	chargeJoin := `LEFT JOIN (SELECT r.sales_order_id order_id, SUM(r.difference_amount - COALESCE(x.paid,0)) unpaid FROM sales_returns r LEFT JOIN ` + allocOf("sales_return") + ` x ON x.order_id=r.id
		 WHERE r.status='confirmed' AND r.return_mode='exchange' AND r.difference_amount > 0 GROUP BY r.sales_order_id) ch ON ch.order_id=o.id`
	for _, k := range []string{"sales", "purchase"} {
		src := `SELECT b.code, o.order_number, ` + legacyOutstanding(k) + ` FROM {orders} o JOIN {branches} b ON b.id_branch=o.id_branch WHERE ` + strings.Replace(legacyOpenOrder, "%s", k, 1)
		var tgt string
		if k == "sales" {
			tgt = `SELECT b.code, o.number, ` + targetSales + ` FROM sales_orders o JOIN branches b ON b.id=o.branch_id LEFT JOIN ` + allocOf("sales") + ` pa ON pa.order_id=o.id ` + chargeJoin + ` WHERE o.status IN ('confirmed','completed')`
		} else {
			tgt = `SELECT b.code, o.number, GREATEST(0, o.grand_total - COALESCE(pa.paid,0)) FROM purchase_orders o JOIN branches b ON b.id=o.branch_id LEFT JOIN ` + allocOf("purchase") + ` pa ON pa.order_id=o.id WHERE o.status IN ('confirmed','completed')`
		}
		v.grouped("V-08-"+k, "sisa tagihan per order "+k+" (rumus legacy buildPaymentSummary vs alokasi revamp)", src, tgt, dropZeros)
	}
	// V-09a: per party and side
	srcParty := func(k, side string) string {
		return `SELECT '` + side + `', bp.code, SUM(q.v) FROM (SELECT o.id_related_party pid, ` + legacyOutstanding(k) + ` v FROM {orders} o WHERE ` + strings.Replace(legacyOpenOrder, "%s", k, 1) + `) q
		  JOIN {business_parties} bp ON bp.id_business_party = q.pid GROUP BY 1,2`
	}
	tgtSales := `SELECT 'receivable', p.code, SUM(` + targetSales + `) FROM sales_orders o JOIN parties p ON p.id=o.party_id LEFT JOIN ` + allocOf("sales") + ` pa ON pa.order_id=o.id ` + chargeJoin + ` WHERE o.status IN ('confirmed','completed') GROUP BY 1,2`
	tgtPurch := `SELECT 'payable', p.code, SUM(GREATEST(0, o.grand_total - COALESCE(pa.paid,0))) FROM purchase_orders o JOIN parties p ON p.id=o.party_id LEFT JOIN ` + allocOf("purchase") + ` pa ON pa.order_id=o.id WHERE o.status IN ('confirmed','completed') GROUP BY 1,2`
	v.grouped("V-09a", "piutang/hutang per pihak (legacy listPartyBalances vs Σ sisa order revamp)",
		srcParty("sales", "receivable")+" UNION ALL "+srcParty("purchase", "payable"), tgtSales+" UNION ALL "+tgtPurch, dropZeros)
	// V-09b: open credit of returns per party (what GetPartyBalance subtracts as returned - refunded)
	v.grouped("V-09b", "saldo kredit retur per pihak (customer_credit/supplier_credit legacy vs sisa retur revamp)",
		`SELECT 'receivable', bp.code, SUM(ROUND(s.amount)) FROM {sales_return_settlements} s JOIN {sales_returns} r ON r.id_sales_return=s.id_sales_return JOIN {orders} o ON o.id_order=r.id_original_order
		   JOIN {business_parties} bp ON bp.id_business_party=o.id_related_party WHERE r.{co} = :c AND r.status='completed' AND r.archived_at IS NULL AND s.archived_at IS NULL AND s.settlement_type='customer_credit' GROUP BY 1,2
		 UNION ALL SELECT 'payable', bp.code, SUM(ROUND(s.amount)) FROM {purchase_return_settlements} s JOIN {purchase_returns} r ON r.id_purchase_return=s.id_purchase_return JOIN {orders} o ON o.id_order=r.id_original_order
		   JOIN {business_parties} bp ON bp.id_business_party=o.id_related_party WHERE r.{co} = :c AND r.status='completed' AND r.archived_at IS NULL AND s.archived_at IS NULL AND s.settlement_type='supplier_credit' GROUP BY 1,2`,
		`SELECT 'receivable', p.code, SUM(CASE WHEN r.return_mode='exchange' THEN -r.difference_amount ELSE r.total END - COALESCE(x.paid,0))
		   FROM sales_returns r JOIN sales_orders o ON o.id=r.sales_order_id JOIN parties p ON p.id=o.party_id LEFT JOIN `+allocOf("sales_return")+` x ON x.order_id=r.id
		  WHERE r.status='confirmed' AND (r.return_mode='return_only' OR r.difference_amount < 0) GROUP BY 1,2
		 UNION ALL SELECT 'payable', p.code, SUM(r.total - COALESCE(x.paid,0))
		   FROM purchase_returns r JOIN purchase_orders o ON o.id=r.purchase_order_id JOIN parties p ON p.id=o.party_id LEFT JOIN `+allocOf("purchase_return")+` x ON x.order_id=r.id
		  WHERE r.status='confirmed' GROUP BY 1,2`, dropZeros)
}

func dropZeros(m map[string]string) {
	for k, val := range m {
		if val == "0" || val == "" {
			delete(m, k)
		}
	}
}

// ---------------------------------------------------------------- V-16e permissions

// permissions proves the effective permission set of every migrated role: the
// legacy codes mapped with the bootstrap legacy map must all be present, and a
// role the migration created must not hold anything more (no widening).
// System roles that already existed in the target keep their seeded extras.
func (v *verifier) permissions() {
	const id, desc = "V-16e", "izin efektif per peran = peta legacy (tidak melebar, tidak hilang)"
	rows, err := v.l.SQL.QueryContext(v.ctx, v.lq(`SELECT r.code, p.permission_code FROM {roles} r JOIN {role_permissions} rp ON rp.id_role=r.id_role
		JOIN {permissions} p ON p.id_permission=rp.id_permission WHERE (r.{co} = :c OR r.{co} IS NULL) AND r.code <> 'superadmin' ORDER BY 1,2`))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	legacy := map[string][]string{}
	for rows.Next() {
		var code, perm string
		if err := rows.Scan(&code, &perm); err != nil {
			_ = rows.Close()
			v.fail(id, desc, err)
			return
		}
		legacy[code] = append(legacy[code], perm)
	}
	_ = rows.Close()
	roles, err := v.col(v.l.SQL, v.lq("SELECT code FROM {roles} WHERE ({co} = :c OR {co} IS NULL) AND code <> 'superadmin'"))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	got := map[string]map[string]bool{}
	trows, err := v.t.QueryContext(v.ctx, `SELECT r.code, p.code FROM roles r JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id`)
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	for trows.Next() {
		var rc, pc string
		if err := trows.Scan(&rc, &pc); err != nil {
			_ = trows.Close()
			v.fail(id, desc, err)
			return
		}
		if got[rc] == nil {
			got[rc] = map[string]bool{}
		}
		got[rc][pc] = true
	}
	_ = trows.Close()
	system := map[string]bool{}
	for _, d := range bootstrap.SystemRoles() {
		system[d.Code] = true
	}
	sort.Strings(roles)
	var bad []string
	for _, rc := range roles {
		want := bootstrap.MapLegacyCodes(legacy[rc])
		wantSet := map[string]bool{}
		for _, w := range want {
			wantSet[w] = true
			if !got[rc][w] {
				bad = append(bad, rc+": hilang "+w)
			}
		}
		if !system[rc] {
			for g := range got[rc] {
				if !wantSet[g] {
					bad = append(bad, rc+": melebar "+g)
				}
			}
		}
	}
	sort.Strings(bad)
	if len(bad) > 5 {
		bad = bad[:5]
	}
	v.add(id, desc, strconv.Itoa(len(roles))+" peran", strconv.Itoa(len(got))+" peran berizin", strings.Join(bad, "; "), "", len(bad) == 0)
}

// ---------------------------------------------------------------- V-16f settings

func (v *verifier) settings() {
	const id, desc = "V-16f", "pengaturan perusahaan: kunci terdaftar terpetakan dari company_settings legacy"
	n, err := v.one(v.l.SQL, v.lq("SELECT COUNT(*) FROM {company_settings} WHERE {co} = :c"))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	if n == "0" {
		v.add(id, desc, "0", "0", "", "tidak ada company_settings legacy", true)
		return
	}
	wantMode, err := v.one(v.l.SQL, v.lq(`SELECT CASE WHEN COALESCE(JSON_UNQUOTE(JSON_EXTRACT(operational_rules_json,'$.stockAdjustmentApprovalMode')), JSON_UNQUOTE(JSON_EXTRACT(operational_rules_json,'$.stock_adjustment_approval_mode'))) = 'strict' THEN 'strict' ELSE 'simple' END FROM {company_settings} WHERE {co} = :c`))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	hasProfile, err := v.one(v.l.SQL, v.lq(`SELECT CASE WHEN JSON_TYPE(JSON_EXTRACT(ui_preferences_json,'$.documentProfile')) = 'OBJECT' AND JSON_LENGTH(JSON_EXTRACT(ui_preferences_json,'$.documentProfile')) > 0 THEN 1 ELSE 0 END FROM {company_settings} WHERE {co} = :c`))
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	got := map[string]string{}
	rows, err := v.t.QueryContext(v.ctx, "SELECT setting_key, COALESCE(setting_value,'') FROM company_settings")
	if err != nil {
		v.fail(id, desc, err)
		return
	}
	for rows.Next() {
		var k, val string
		if err := rows.Scan(&k, &val); err != nil {
			_ = rows.Close()
			v.fail(id, desc, err)
			return
		}
		got[k] = val
	}
	_ = rows.Close()
	var bad []string
	check := func(k, want string) {
		if got[k] != want {
			bad = append(bad, k+": legacy="+want+" target="+got[k])
		}
	}
	check("stock_adjust_approval_mode", wantMode)
	check("stock_adjust_always_approve", "true")
	check("delivery_require_evidence", "true")
	if hasProfile == "1" && strings.TrimSpace(got["document_profile"]) == "" {
		bad = append(bad, "document_profile: hilang")
	}
	if hasProfile != "1" && got["document_profile"] != "" {
		bad = append(bad, "document_profile: tidak ada di legacy")
	}
	v.add(id, desc, "5 kunci", strconv.Itoa(len(got))+" kunci", strings.Join(bad, "; "), "", len(bad) == 0)
}
