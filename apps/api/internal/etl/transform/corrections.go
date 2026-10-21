package transform

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

// Migration corrections (ADR-0018 D11): legacy errors the books must not carry
// into the revamp. Each becomes a posted journal dated at the freeze, marked
// source_doc_type = migration_correction, and one row of
// report/migration_corrections.csv (out/migration_corrections.jsonl).
const (
	sourceMigrationCorrection = "migration_correction"

	// corrDNCancel: an SJ cancelled in legacy brought its goods back but its
	// journalled COGS was never reversed — Dr Persediaan / Cr HPP.
	corrDNCancel = "dn_cancel_cogs"
	// corrStateNoQty: legacy's cost state values a product it holds no unit
	// of — Dr selisih persediaan / Cr Persediaan, ledger to zero.
	corrStateNoQty = "state_no_qty"
	// corrStateNoValue: legacy's cost state holds units at no value although
	// its own rows price them — Dr Persediaan / Cr selisih persediaan.
	corrStateNoValue = "state_no_value"
	// corrCOGSUnposted: goods an SJ took out (or brought back) that no
	// journal will ever book (legacy posted it only in part, archived it, or
	// it predates the opening) — HPP against Persediaan.
	corrCOGSUnposted = "cogs_unposted"
	// corrStockUnposted: any other movement without a coming journal (drift
	// corrections, adjustments legacy never posted, receipts/returns before
	// the opening) — selisih persediaan against Persediaan.
	corrStockUnposted = "stock_unposted"
	// corrRevenueUnbooked: an order SJ delivered and its COGS journalled in
	// legacy, but its revenue never booked (legacy books it when the order
	// completes, which never happened) and below the delivery cutover, so the
	// revamp's queue will not book it either — Dr Piutang (customer) /
	// Cr Penjualan per product, Cr PPN Keluaran for the tax (taxLeg).
	corrRevenueUnbooked = "revenue_unbooked"
	// corrCancelledShipped: an SJ of an order cancelled in legacy (re-entered
	// as a new order) whose COGS legacy journalled — not the cost of a sale,
	// so Dr selisih persediaan / Cr HPP. Quantities are not touched.
	corrCancelledShipped = "cancelled_order_cogs"
	// corrCostRevalue: a position revalued from legacy's drifted book value
	// to the cost its purchases support (ADR-0018 D11g) — Dr Persediaan /
	// Cr selisih persediaan (reversed for a write-down).
	corrCostRevalue = "cost_revalue"
)

type migCorrection struct {
	kind, refType, reason    string
	ref                      string // document number when the group needs one (revenue: the SO)
	branch, doc              int64
	party                    int64 // receivable leg (revenue)
	product, variant         int64
	movement, legacyMovement int64
	deliveryNote             int64 // revenue: the SJ the amount belongs to
	legacyState              int64
	amount                   int64
	taxLeg                   bool // revenue: the PPN Keluaran part
}

// corrAccounts are the legacy system keys a correction debits and credits
// (before a negative amount swaps them).
func corrAccounts(c migCorrection) (dr, cr string) {
	switch c.kind {
	case corrDNCancel, corrCOGSUnposted:
		return "inventory", "cogs"
	case corrStateNoQty:
		return "stock_difference", "inventory"
	case corrCancelledShipped:
		return "stock_difference", "cogs"
	case corrRevenueUnbooked:
		if c.taxLeg {
			return "accounts_receivable", "output_tax"
		}
		return "accounts_receivable", "sales_revenue"
	}
	return "inventory", "stock_difference" // corrStateNoValue, corrStockUnposted
}

// freezeTime is when legacy stopped: --freeze-at, else the latest record
// creation in the dump (creation clocks, not business dates, which may lie
// in the future).
func (s *state) freezeTime() (time.Time, string) {
	if s.in.Cfg.FreezeAt != "" {
		if t, err := time.Parse(time.RFC3339, s.in.Cfg.FreezeAt); err == nil {
			return t, "--freeze-at"
		}
		s.find(report.Blocker, "FREEZE_AT", "etl", 0, "--freeze-at %q bukan RFC3339", s.in.Cfg.FreezeAt)
	}
	var last time.Time
	for _, t := range []string{"finance_journal_entries", "payments", "orders", "finance_posting_sources"} {
		for _, r := range s.raw[t] {
			if c, ok := r.LocalTime("created_at"); ok && c.After(last) {
				last = c
			}
		}
	}
	if last.IsZero() {
		last = s.epoch()
	}
	return last, "akhir data dump (created_at terakhir)"
}

var reJVNumber = regexp.MustCompile(`^JV-(.+)/(\d{4})/(\d+)$`)

func (s *state) migrationCorrections() {
	if len(s.migCorr) == 0 {
		return
	}
	freeze, why := s.freezeTime()
	day := wibDate(freeze)
	s.find(report.Info, "MIG_CORR_DATE", "etl", 0, "jurnal koreksi migrasi bertanggal %s (%s)", day, why)
	if day < s.open {
		s.find(report.Blocker, "MIG_CORR_DATE", "etl", 0, "tanggal freeze %s sebelum tanggal saldo awal %s", day, s.open)
		return
	}
	for _, p := range s.raw["finance_periods"] {
		from, ok1 := p.Time("date_from")
		to, ok2 := p.Time("date_to")
		if p.Str("status") == "closed" && ok1 && ok2 && day >= from.Format(dateFmt) && day <= to.Format(dateFmt) {
			s.find(report.Blocker, "MIG_CORR_PERIOD", "etl", 0, "periode tanggal freeze %s sudah ditutup legacy: koreksi tidak boleh membuka ulang periode", day)
			return
		}
	}
	bySys := map[string]*acct{}
	for _, a := range s.acct {
		if a.sysKey != "" {
			bySys[a.sysKey] = a
		}
	}
	for _, c := range s.migCorr {
		dr, cr := corrAccounts(c)
		for _, key := range []string{dr, cr} {
			if bySys[key] == nil {
				s.find(report.Blocker, "MIG_CORR_ACCOUNT", "finance_accounts", 0, "akun ber-system_key %q tidak ada: koreksi migrasi %s tidak dapat dijurnal (jangan buat akun baru tanpa keputusan)", key, c.kind)
				return
			}
		}
	}

	// JV numbers continue after any the ETL already used (the opening).
	nextJV := map[string]int64{}
	for _, e := range s.out.Tables["finance_journal_entries"] {
		if m := reJVNumber.FindStringSubmatch(e.Str("number")); m != nil {
			n, _ := strconv.ParseInt(m[3], 10, 64)
			if key := m[1] + "/" + m[2]; n > nextJV[key] {
				nextJV[key] = n
			}
		}
	}
	docNumber := map[string]string{}
	for t, table := range map[string]string{"delivery": "delivery_notes", "goods_receipt": "goods_receipts", "sales_return": "sales_returns", "purchase_return": "purchase_returns"} {
		for _, d := range s.out.Tables[table] {
			docNumber[t+":"+d.Str("id")] = d.Str("number")
		}
	}

	// One journal per cancelled SJ, one per write-down; deterministic order.
	type group struct {
		kind, ref string
		branch    int64
		items     []migCorrection
	}
	groups := map[string]*group{}
	for _, c := range s.migCorr {
		var key, ref string
		switch c.kind {
		case corrDNCancel:
			key, ref = fmt.Sprintf("%s|%d|%d", c.kind, c.branch, c.doc), docNumber["delivery:"+strconv.FormatInt(c.doc, 10)]
		case corrStateNoQty, corrStateNoValue, corrCostRevalue:
			key, ref = fmt.Sprintf("%s|%d|%d", c.kind, c.branch, c.product), s.productRow[c.product].Str("code")
		case corrRevenueUnbooked, corrCancelledShipped: // one journal per sales order
			key, ref = fmt.Sprintf("%s|%d|%d", c.kind, c.branch, c.doc), c.ref
		default:
			// One journal per document; document-less movements of a branch together.
			ref = docNumber[c.refType+":"+strconv.FormatInt(c.doc, 10)]
			if ref == "" {
				ref = "mutasi stok tanpa dokumen"
			}
			key = fmt.Sprintf("%s|%d|%s", c.kind, c.branch, ref)
		}
		if groups[key] == nil {
			groups[key] = &group{kind: c.kind, ref: ref, branch: c.branch}
		}
		groups[key].items = append(groups[key].items, c)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	entryDate := naiveDateTime(day)
	for i, k := range keys {
		g := groups[k]
		sort.SliceStable(g.items, func(a, b int) bool {
			x, y := g.items[a], g.items[b]
			if x.taxLeg != y.taxLeg {
				return !x.taxLeg
			}
			if x.deliveryNote != y.deliveryNote {
				return x.deliveryNote < y.deliveryNote
			}
			if x.product != y.product {
				return x.product < y.product
			}
			if x.variant != y.variant {
				return x.variant < y.variant
			}
			return x.movement < y.movement
		})
		code := s.branchCode[g.branch]
		jvKey := code + "/" + day[:4]
		nextJV[jvKey]++
		number := fmt.Sprintf("JV-%s/%s/%05d", code, day[:4], nextJV[jvKey])
		id := s.allocSub("finance_journal_entries", int64(i+1), 8)
		var memo, desc string
		switch g.kind {
		case corrDNCancel:
			memo = fmt.Sprintf("Koreksi migrasi: SJ %s dibatalkan di legacy, HPP keluarnya tidak pernah dibalik; barang kembali ke stok (Dr Persediaan / Cr HPP)", g.ref)
			desc = "Koreksi migrasi: HPP SJ batal " + g.ref
		case corrStateNoQty:
			memo = fmt.Sprintf("Koreksi migrasi: produk %s bernilai di keadaan biaya legacy tanpa qty; nilai persediaan dinolkan (Dr selisih persediaan / Cr Persediaan)", g.ref)
			desc = "Koreksi migrasi: persediaan tanpa qty " + g.ref
		case corrStateNoValue:
			memo = fmt.Sprintf("Koreksi migrasi: produk %s ber-qty tanpa nilai di keadaan biaya legacy, padahal baris biayanya bernilai; Persediaan ditulis naik (Dr Persediaan / Cr selisih persediaan)", g.ref)
			desc = "Koreksi migrasi: persediaan tanpa nilai " + g.ref
		case corrCOGSUnposted:
			memo = fmt.Sprintf("Koreksi migrasi: barang SJ %s keluar/masuk tanpa jurnal HPP di legacy (%s); Persediaan disesuaikan ke ledger biaya", g.ref, g.items[0].reason)
			desc = "Koreksi migrasi: HPP tanpa jurnal " + g.ref
		case corrRevenueUnbooked:
			memo = fmt.Sprintf("Koreksi migrasi: penjualan %s (%s) sudah dikirim dan HPP-nya dijurnal legacy, tetapi pendapatannya tidak pernah dijurnal (legacy menjurnal saat order selesai) dan SJ-nya di bawah batas cutover; piutang dan penjualan dibukukan (Dr Piutang / Cr Penjualan)", g.ref, g.items[0].reason)
			memo = s.trunc("finance_journal_entries", 0, "memo", memo, maxJournalText)
			desc = "Koreksi migrasi: pendapatan belum dijurnal " + g.ref
		case corrCostRevalue:
			memo = fmt.Sprintf("Koreksi migrasi: produk %s dinilai ulang ke harga modal yang didukung harga beli (nilai buku legacy menyimpang >5%%); Persediaan disesuaikan lawan selisih persediaan", g.ref)
			desc = "Koreksi migrasi: penilaian ulang persediaan " + g.ref
		case corrCancelledShipped:
			memo = fmt.Sprintf("Koreksi migrasi: order %s dibatalkan di legacy dan diganti order baru, tetapi SJ-nya (%s) tetap terkonfirmasi dan HPP-nya dijurnal; bukan penjualan, HPP dipindah ke selisih persediaan (Dr Selisih Persediaan / Cr HPP), qty stok tidak diubah", g.ref, g.items[0].reason)
			memo = s.trunc("finance_journal_entries", 0, "memo", memo, maxJournalText)
			desc = "Koreksi migrasi: HPP SJ order batal " + g.ref
		default:
			memo = fmt.Sprintf("Koreksi migrasi: %s — mutasi persediaan tanpa jurnal di legacy (%s); Persediaan disesuaikan ke ledger biaya", g.ref, g.items[0].reason)
			desc = "Koreksi migrasi: mutasi tanpa jurnal " + g.ref
		}
		s.emit("finance_journal_entries", row.Row{
			"id": id, "number": number, "branch_id": g.branch, "entry_date": entryDate, "memo": memo,
			"source_doc_type": sourceMigrationCorrection, "source_doc_id": id, "status": "posted", "reversed_by": nil,
			"created_at": fmtUTC(freeze), "created_by": nil, "scope": 0, "requeue": 0,
		})
		s.out.Explain["migration_correction_entries"]++
		for _, c := range g.items {
			dk, ck := corrAccounts(c)
			d, cc := bySys[dk], bySys[ck]
			amt := c.amount
			if amt < 0 { // a negative write-down reverses the legs
				d, cc, amt = cc, d, -amt
			}
			var variant any
			if c.variant != 0 {
				variant = c.variant
			}
			for _, leg := range []struct {
				a      *acct
				dr, cr int64
			}{{d, amt, 0}, {cc, 0, amt}} {
				var product, legVariant, party any = c.product, variant, nil
				if c.kind == corrRevenueUnbooked {
					// Dimensions as the revamp's delivery journal: the customer on
					// Piutang, the product on Penjualan, none on PPN Keluaran.
					product, legVariant = nil, nil
					switch {
					case leg.a.sysKey == "accounts_receivable":
						if c.party != 0 {
							party = c.party
						}
					case !c.taxLeg:
						product, legVariant = c.product, variant
					}
				}
				s.emit("finance_journal_lines", row.Row{"id": s.nextID("finance_journal_lines"), "entry_id": id, "account_id": leg.a.newID,
					"product_id": product, "variant_id": legVariant, "party_id": party, "debit": leg.dr, "credit": leg.cr, "description": desc})
			}
			var mv, legacy any = nil, c.legacyState
			switch c.kind {
			case corrStateNoQty, corrStateNoValue, corrCostRevalue:
			case corrRevenueUnbooked:
				legacy = c.legacyMovement // the legacy SJ (or order of a synthetic SJ)
			default:
				mv, legacy = c.movement, c.legacyMovement
			}
			var productID any = c.product
			if c.product == 0 {
				productID = nil
			}
			s.out.Explain["migration_correction_value"] += c.amount
			s.emit("migration_corrections", row.Row{
				"kind": c.kind, "branch_id": c.branch, "entry_id": id, "journal_number": number, "entry_date": day, "ref": g.ref,
				"product_id": productID, "product_code": s.productRow[c.product].Str("code"), "variant_id": variant,
				"stock_movement_id": mv, "legacy_id": legacy, "amount": c.amount, "debit_account": d.code, "credit_account": cc.code, "reason": c.reason,
			})
		}
	}
}
