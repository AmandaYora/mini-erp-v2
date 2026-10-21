package transform

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

// Return documents (design F5-5.E). Revamp semantics reproduced here:
//   - condition per line (normal | damaged | not_returned), reason on the header;
//   - value follows the source order economics (tax_base = line_total - tax so
//     total = subtotal + tax_total for every tax type);
//   - exchange totals: replacement_* and difference_amount = replacement - return;
//   - the part of a return that legacy took off the open bill
//     (receivable/payable reduction) becomes receivable_reduction /
//     payable_reduction plus an offset payment pair (method "offset"), exactly
//     what the revamp's autoOffset writes at confirmation; refunds/collections
//     become payments allocated to the return, customer/supplier credit stays
//     as the open balance of the return;
//   - return numbers are kept verbatim.
const (
	docSalesReturn    = "sales_return"
	docPurchaseReturn = "purchase_return"
	// Credit settlements (K-04a) are journalled on their own (source_doc_id =
	// settlement row id), so each kind has its own cutover boundary.
	docSalesReturnSettlement    = "sales_return_settlement"
	docPurchaseReturnSettlement = "purchase_return_settlement"
)

// retMeta is what later steps need to know about one migrated return.
type retMeta struct {
	legacy, id, branch, order, party int64
	date                             time.Time
	confirmed                        bool
	st, reason                       string
	journals                         []int64
	number                           string
}

// synthRepl marks a POS-style exchange whose replacement left the counter
// without a delivery note: the revamp needs a (confirmed) replacement SJ.
type synthRepl struct{ ret int64 }

// retPay is one payment the returns step asks the payments step to write.
type retPay struct {
	idx         int64
	branch      int64
	party       int64
	ptype, dir  string
	amount      int64
	method      string
	date        time.Time
	notes       string
	number      string
	by          any
	reference   any
	orderType   string
	orderID     int64
	clientRef   any
	st, reason  string
	journals    []int64
	band        int
	pairWith    int // 1-based idx of the other leg, 0 = none
	newID       int64
	row         row.Row
	retDocLabel string
}

func (s *state) returns() error {
	s.prepareDocs()
	s.srNew, s.prNew, s.replSJ = map[int64]int64{}, map[int64]int64{}, map[int64]int64{}
	s.srSynth, s.srReplDN, s.srItemLoc = map[int64]*synthRepl{}, map[int64]int64{}, map[int64]int64{}
	s.srRepl, s.srMeta = map[int64][]row.Row{}, map[int64]*retMeta{}
	s.salesReturns()
	s.purchaseReturns()
	return nil
}

// ---------------------------------------------------------------- helpers

func (s *state) payMethod(entity string, id int64, m string) string {
	switch m {
	case "cash", "check", "transfer":
		return m
	case "bank_transfer":
		return "transfer"
	case "":
		return "cash"
	}
	s.find(report.Warn, "PAY_METHOD", entity, id, "metode %q dipetakan ke transfer", m)
	return "transfer"
}

// retLine converts one legacy return line. grossCol is the unit price column
// before discount (sales) or the unit price (purchase).
func (s *state) retLine(entity string, lid int64, it row.Row, grossCol, totalCol string) (unit, nominal, total, tax, base int64) {
	ilid := it.Int("id_" + entity + "_item")
	unit = s.money.R(entity+"_items", ilid, "unit_price", it, grossCol)
	gross := RoundHalfAway(new(big.Rat).Mul(it.Rat("quantity"), it.Rat(grossCol)))
	before := s.money.Round(entity+"_items", ilid, "line_total_before_tax", it.Rat("line_total_before_tax"))
	nominal = gross - before
	if nominal < 0 {
		s.log.Adjust(entity+"_items", ilid, "discount_nominal", fmt.Sprint(nominal), "0", "negative_discount_clamped")
		nominal = 0
	}
	total = s.money.R(entity+"_items", ilid, "line_total", it, totalCol)
	tax = s.money.R(entity+"_items", ilid, "tax_amount", it, "tax_amount")
	base = total - tax
	if tb := s.money.Round(entity+"_items", ilid, "tax_base", it.Rat("tax_base_amount")); tb != base {
		s.log.Adjust(entity+"_items", ilid, "tax_base", fmt.Sprint(tb), fmt.Sprint(base), "base_from_total_minus_tax")
	}
	return
}

// absorbLines forces the line totals to add up to the header total by moving
// the difference onto the largest line (logged), like orders do.
func (s *state) absorbLines(entity string, id int64, rows []row.Row, want int64) {
	var sum int64
	for _, r := range rows {
		sum += r.Int("line_total")
	}
	if len(rows) == 0 || sum == want {
		return
	}
	best := 0
	for i, r := range rows {
		if abs64(r.Int("line_total")) > abs64(rows[best].Int("line_total")) {
			best = i
		}
	}
	d := want - sum
	rows[best]["line_total"] = rows[best].Int("line_total") + d
	rows[best]["tax_base"] = rows[best].Int("tax_base") + d
	s.log.Adjust(entity, id, "line_total", fmt.Sprint(sum), fmt.Sprint(want), "absorb_into_largest_line")
}

func sumCol(rows []row.Row, col string) int64 {
	var n int64
	for _, r := range rows {
		n += r.Int(col)
	}
	return n
}

func (s *state) partyOfOrder(o row.Row) int64 {
	if p, ok := o.NInt("id_related_party"); ok {
		id, _ := s.id("parties", p)
		return id
	}
	return 0
}

func (s *state) locFor(entity string, ilid int64, it row.Row, branch int64, cond string) int64 {
	loc, has := it.NInt("id_stock_location")
	tracked := it.Bool("stock_tracked_snapshot")
	switch {
	case cond == "not_returned":
		return 0
	case has:
		id, ok := s.id("stock_locations", loc)
		if !ok {
			s.find(report.Blocker, "RET_LOCATION", entity, ilid, "lokasi %d tidak termigrasi", loc)
		}
		return id
	case cond == "damaged":
		return s.rusak[branch]
	case tracked:
		s.find(report.Warn, "RET_NO_LOCATION", entity, ilid, "item tanpa lokasi: dipindah ke GDG cabang")
		return s.gdg[branch]
	}
	return 0
}

// ---------------------------------------------------------------- sales returns

type retDraft struct {
	*docDraft
	r row.Row
}

func (s *state) salesReturns() {
	rets := sortedBy(s.raw["sales_returns"], "id_sales_return")
	if len(rets) == 0 {
		return
	}
	returned := map[int64][]row.Row{}
	for _, it := range sortedBy(s.raw["sales_return_items"], "id_sales_return_item") {
		s.srRepl[it.Int("id_sales_return")] = s.srRepl[it.Int("id_sales_return")]
		returned[it.Int("id_sales_return")] = append(returned[it.Int("id_sales_return")], it)
	}
	setts := map[int64][]row.Row{}
	for _, st := range sortedBy(s.raw["sales_return_settlements"], "id_sales_return_settlement") {
		if !st.Has("archived_at") {
			setts[st.Int("id_sales_return")] = append(setts[st.Int("id_sales_return")], st)
		}
	}
	for _, d := range sortedBy(s.raw["delivery_notes"], "id_delivery_note") {
		if d.Str("document_kind") == "replacement" && d.Has("id_sales_return") && !d.Has("archived_at") {
			if _, dup := s.srReplDN[d.Int("id_sales_return")]; !dup {
				s.srReplDN[d.Int("id_sales_return")] = d.Int("id_delivery_note")
			}
		}
	}
	var drafts []*retDraft
	for _, r := range rets {
		lid := r.Int("id_sales_return")
		branch, ok := s.id("branches", r.Int("id_branch"))
		if !ok {
			s.find(report.Blocker, "SR_BRANCH", "sales_returns", lid, "cabang retur tidak dikenal")
			continue
		}
		ord := r.Int("id_original_order")
		if s.orderKind[ord] != "sales" {
			s.find(report.Blocker, "SR_ORDER", "sales_returns", lid, "order asal %d bukan order penjualan termigrasi", ord)
			continue
		}
		switch r.Str("return_mode") {
		case "return_only", "exchange":
		default:
			s.find(report.Blocker, "SR_MODE", "sales_returns", lid, "return_mode %q tidak dikenal", r.Str("return_mode"))
			continue
		}
		var status string
		switch {
		case r.Has("archived_at"), r.Str("status") == "cancelled":
			status = "cancelled"
		case r.Str("status") == "completed":
			status = "confirmed"
		default:
			s.find(report.Blocker, "SR_STATUS", "sales_returns", lid, "status %q tidak dikenal", r.Str("status"))
			continue
		}
		switch r.Str("replacement_delivery_status") {
		case "not_required", "pending", "dispatched", "delivered":
		default:
			s.find(report.Blocker, "SR_REPL_STATUS", "sales_returns", lid, "replacement_delivery_status %q tidak dikenal", r.Str("replacement_delivery_status"))
			continue
		}
		date, _ := r.Time("return_date")
		dr := &docDraft{date: date, legacy: lid, branch: branch}
		if status == "cancelled" {
			dr.st, dr.reason = stateIgnored, "cancelled"
			if r.Has("archived_at") {
				dr.reason = "archived"
			}
		} else {
			reqs := []postKey{{"sales_return", lid, "sales_return_completed"}}
			for _, m := range s.movesByRef["sales_return:"+itoa(lid)] {
				reqs = append(reqs, postKey{"inventory_movement", m.Int("id_inventory_movement"), "sales_return_stock_in"})
			}
			for _, st := range setts[lid] {
				if ev := map[string]string{"refund": "sales_return_refund", "customer_credit": "sales_return_customer_credit", "collect_payment": "sales_return_payment"}[st.Str("settlement_type")]; ev != "" {
					reqs = append(reqs, postKey{"sales_return_settlement", st.Int("id_sales_return_settlement"), ev})
				}
			}
			dr.st, dr.reason, dr.journals = s.postState("sales_returns", lid, date, reqs)
		}
		dr.band = band(dr.st)
		drafts = append(drafts, &retDraft{dr, r})
	}
	sort.SliceStable(drafts, func(i, j int) bool { return drafts[i].docDraft.less(drafts[j].docDraft) })
	through, settThrough := int64(0), int64(0)
	for _, dd := range drafts {
		r, lid := dd.r, dd.legacy
		id := s.alloc("sales_returns", lid)
		s.srNew[lid] = id
		ord := r.Int("id_original_order")
		orow := s.orderRow[ord]
		soID, _ := s.id("sales_orders", ord)
		status := "confirmed"
		if dd.st == stateIgnored && (dd.reason == "cancelled" || dd.reason == "archived") {
			status = "cancelled"
		}
		meta := &retMeta{legacy: lid, id: id, branch: dd.branch, order: soID, party: s.partyOfOrder(orow), date: dd.date,
			confirmed: status == "confirmed", st: dd.st, reason: dd.reason, journals: dd.journals, number: r.Str("return_number")}
		s.srMeta[lid] = meta

		var retRows, replRows []row.Row
		var replLegacy []row.Row
		for _, it := range returned[lid] {
			ilid := it.Int("id_sales_return_item")
			pid, ok1 := s.id("products", it.Int("id_product"))
			vid, ok2 := s.id("product_variants", it.Int("id_product_variant"))
			if !ok1 || !ok2 {
				s.find(report.Blocker, "SRI_PRODUCT", "sales_return_items", ilid, "produk/varian tidak termigrasi")
				continue
			}
			unit, nominal, total, tax, base := s.retLine("sales_return", lid, it, "unit_price_before_discount", "line_total_after_tax")
			switch it.Str("line_type") {
			case "returned":
				cond := it.Str("return_condition")
				if cond == "" {
					cond = "normal"
				}
				switch cond {
				case "normal", "damaged", "not_returned":
				default:
					s.find(report.Blocker, "SRI_CONDITION", "sales_return_items", ilid, "return_condition %q tidak dikenal", cond)
					continue
				}
				loc := s.locFor("sales_return_items", ilid, it, dd.branch, cond)
				s.srItemLoc[ilid] = loc
				retRows = append(retRows, row.Row{
					"id": s.nextID("sales_return_items"), "return_id": id, "product_id": pid, "variant_id": vid, "location_id": loc,
					"uom": it.Str("transaction_uom_snapshot"), "uom_factor": it.Str("uom_to_base_factor"), "qty": it.Str("quantity"), "qty_base": it.Str("quantity_in_base_uom"),
					"unit_price": unit, "discount_pct": "0.00", "discount_nominal": nominal, "tax_base": base, "tax_amount": tax, "line_total": total, "return_condition": cond,
				})
			case "replacement":
				loc := s.locFor("sales_return_items", ilid, it, dd.branch, "normal")
				s.srItemLoc[ilid] = loc
				replLegacy = append(replLegacy, it)
				replRows = append(replRows, row.Row{
					"id": s.nextID("sales_return_replacement_items"), "return_id": id, "product_id": pid, "variant_id": vid, "location_id": loc,
					"uom": it.Str("transaction_uom_snapshot"), "uom_factor": it.Str("uom_to_base_factor"), "qty": it.Str("quantity"), "qty_base": it.Str("quantity_in_base_uom"),
					"unit_price": unit, "line_total": total, "tax_base": base, "tax_amount": tax,
				})
			default:
				s.find(report.Blocker, "SRI_LINE_TYPE", "sales_return_items", ilid, "line_type %q tidak dikenal", it.Str("line_type"))
			}
		}
		s.srRepl[lid] = replLegacy
		hdrTotal := s.money.R("sales_returns", lid, "total", r, "returned_total_amount")
		replTotal := s.money.R("sales_returns", lid, "replacement_total", r, "replacement_total_amount")
		s.absorbLines("sales_return_items", lid, retRows, hdrTotal)
		s.absorbLines("sales_return_replacement_items", lid, replRows, replTotal)
		diff := s.money.R("sales_returns", lid, "difference_amount", r, "difference_amount")
		mode := r.Str("return_mode")
		if mode == "exchange" && diff != replTotal-hdrTotal {
			s.find(report.Warn, "SR_DIFF", "sales_returns", lid, "difference_amount %d != pengganti %d - retur %d: nilai legacy dipakai", diff, replTotal, hdrTotal)
		}

		// settlements (legacy rows) + the offset part
		var ext int64
		var settRows []row.Row
		for _, st := range setts[lid] {
			slid := st.Int("id_sales_return_settlement")
			typ := st.Str("settlement_type")
			switch typ {
			case "refund", "customer_credit", "collect_payment":
			default:
				s.find(report.Blocker, "SRS_TYPE", "sales_return_settlements", slid, "settlement_type %q tidak dikenal", typ)
				continue
			}
			amount := s.money.R("sales_return_settlements", slid, "amount", st, "amount")
			sdate, _ := st.Time("settlement_date")
			if sdate.IsZero() {
				sdate = dd.date
			}
			var method any
			if m, ok := st.NStr("payment_method"); ok && m != "" {
				method = s.payMethod("sales_return_settlements", slid, m)
			}
			settRows = append(settRows, row.Row{
				"id": s.nextID("sales_return_settlements"), "return_id": id, "settlement_type": typ, "settlement_date": fmtWIB(sdate), "amount": amount,
				"payment_method": method, "reference_number": s.optStr("sales_return_settlements", slid, "reference_number", st, "reference_number", 100),
				"notes": s.optStr("sales_return_settlements", slid, "notes", st, "notes", 255), "created_at": tsLocalOr(st, "created_at", sdate),
				"created_by": s.userID(st.Int("id_created_by")), "auto": 0,
			})
			if typ != "collect_payment" {
				ext += amount
			}
			if meta.confirmed {
				pm := s.payMethod("sales_return_settlements", slid, st.Str("payment_method"))
				switch typ {
				case "refund":
					s.addRetPay(&retPay{branch: dd.branch, party: meta.party, ptype: "customer", dir: "out", amount: amount, method: pm, date: sdate,
						notes: "Refund retur " + meta.number, number: fmt.Sprintf("PAY-%s/MIG/RTR%05d-S%d", s.branchCode[dd.branch], lid, slid), by: s.userID(st.Int("id_created_by")),
						reference: s.optStr("sales_return_settlements", slid, "reference_number", st, "reference_number", 100),
						orderType: "sales_return", orderID: id}, "sales_return_settlement", slid, "sales_return_refund", dd.date)
				case "collect_payment":
					s.addRetPay(&retPay{branch: dd.branch, party: meta.party, ptype: "customer", dir: "in", amount: amount, method: pm, date: sdate,
						notes: "Tambah bayar tukar " + meta.number, number: fmt.Sprintf("PAY-%s/MIG/RTR%05d-S%d", s.branchCode[dd.branch], lid, slid), by: s.userID(st.Int("id_created_by")),
						reference: s.optStr("sales_return_settlements", slid, "reference_number", st, "reference_number", 100),
						orderType: "sales_return", orderID: id}, "sales_return_settlement", slid, "sales_return_payment", dd.date)
				}
			}
		}
		var reduction int64
		if meta.confirmed && diff < 0 {
			if reduction = -diff - ext; reduction < 0 {
				reduction = 0
			}
		}
		if reduction > 0 {
			settRows = append(settRows, row.Row{
				"id": s.nextID("sales_return_settlements"), "return_id": id, "settlement_type": "reduce_receivable", "settlement_date": fmtWIB(dd.date), "amount": reduction,
				"payment_method": nil, "reference_number": nil, "notes": "Offset otomatis ke tagihan order", "created_at": fmtUTC(dd.date),
				"created_by": s.userID(r.Int("id_created_by")), "auto": 1,
			})
			s.out.Explain["synthetic_return_settlements"]++
			s.addOffsetPair("sales", meta, reduction, dd.date)
			if meta.party == 0 {
				s.find(report.Warn, "SR_OFFSET_NOPARTY", "sales_returns", lid, "pengurangan piutang %d pada order tanpa pihak", reduction)
			}
		}
		for _, sr := range settRows {
			s.emit("sales_return_settlements", sr)
		}
		s.creditSettlementMarks(docSalesReturnSettlement, "customer_credit", dd.docDraft, meta.confirmed, settRows, &settThrough)

		// replacement delivery status (+ synthetic SJ for counter exchanges)
		repl := r.Str("replacement_delivery_status")
		if repl == "delivered" {
			repl = "confirmed"
		}
		if mode == "exchange" && status == "confirmed" && repl == "not_required" && s.srReplDN[lid] == 0 && len(s.movesByRef["sales_return_exchange:"+itoa(lid)]) > 0 {
			s.srSynth[lid] = &synthRepl{ret: lid}
			repl = "confirmed"
		}
		storedDiff := diff // the revamp keeps a signed difference only for exchanges
		if mode != "exchange" {
			storedDiff = 0
		}
		notes := any(nil)
		if c := r.Str("cancel_reason"); c != "" {
			notes = s.trunc("sales_returns", lid, "notes", "[Dibatalkan] "+c, 255)
		}
		var rsub, rtax, ssub, stax, disc int64
		rsub, rtax, disc = sumCol(retRows, "tax_base"), sumCol(retRows, "tax_amount"), sumCol(retRows, "discount_nominal")
		ssub, stax = sumCol(replRows, "tax_base"), sumCol(replRows, "tax_amount")
		s.emit("sales_returns", row.Row{
			"id": id, "number": s.trunc("sales_returns", lid, "number", r.Str("return_number"), 100), "branch_id": dd.branch, "sales_order_id": soID,
			"return_date": fmtWIB(dd.date), "total": hdrTotal, "subtotal": rsub, "discount_total": disc, "tax_total": rtax,
			"tax_type": taxType(orow), "tax_rate": s.rate2("sales_returns", lid, orow.Rat("tax_rate_snapshot")),
			"return_mode": mode, "replacement_delivery_status": repl, "status": status, "notes": notes,
			"created_at": tsLocalOr(r, "created_at", dd.date), "updated_at": tsLocalOr(r, "updated_at", dd.date), "created_by": s.userID(r.Int("id_created_by")),
			"reason":               s.trunc("sales_returns", lid, "reason", r.Str("reason_text"), 255),
			"replacement_subtotal": ssub, "replacement_tax_total": stax, "replacement_total": replTotal, "difference_amount": storedDiff,
			"receivable_reduction": reduction,
		})
		for _, x := range retRows {
			s.emit("sales_return_items", x)
		}
		for _, x := range replRows {
			s.emit("sales_return_replacement_items", x)
		}
		s.out.Marks = append(s.out.Marks, Mark{DocType: docSalesReturn, DocID: id, BranchID: dd.branch, State: dd.st, Reason: dd.reason, LegacyJournals: dd.journals})
		if dd.band == 0 {
			through = id
		}
	}
	s.cutoverFor(docSalesReturn, through)
	s.cutoverFor(docSalesReturnSettlement, settThrough)
}

// creditSettlementMarks classifies the credit-type settlement rows of one
// migrated return (K-04a). The customer/supplier-balance journal is booked per
// settlement row, so a credit row of a confirmed return enters the posting
// queue unless it sits at or below the cutover boundary. A settlement is
// journalled in legacy together with its return, so it follows the return's
// posting state (pending / posted_legacy / opening_covered / ignored): rows of
// returns already booked elsewhere raise the boundary, rows of pending
// returns stay above it and become queue items with a pending mark. Ids are
// allocated in the same band order as the returns, so one boundary per kind
// is exact.
func (s *state) creditSettlementMarks(docType, creditType string, dd *docDraft, confirmed bool, rows []row.Row, through *int64) {
	for _, sr := range rows {
		sid, _ := sr["id"].(int64)
		if dd.band == 0 && sid > *through {
			*through = sid
		}
		if confirmed && sr["settlement_type"] == creditType {
			s.out.Marks = append(s.out.Marks, Mark{DocType: docType, DocID: sid, BranchID: dd.branch, State: dd.st, Reason: dd.reason})
			s.out.Explain["credit_settlement_marks"]++
		}
	}
}

// ---------------------------------------------------------------- purchase returns

func (s *state) purchaseReturns() {
	rets := sortedBy(s.raw["purchase_returns"], "id_purchase_return")
	if len(rets) == 0 {
		return
	}
	items := map[int64][]row.Row{}
	for _, it := range sortedBy(s.raw["purchase_return_items"], "id_purchase_return_item") {
		items[it.Int("id_purchase_return")] = append(items[it.Int("id_purchase_return")], it)
	}
	setts := map[int64][]row.Row{}
	for _, st := range sortedBy(s.raw["purchase_return_settlements"], "id_purchase_return_settlement") {
		if !st.Has("archived_at") {
			setts[st.Int("id_purchase_return")] = append(setts[st.Int("id_purchase_return")], st)
		}
	}
	var drafts []*retDraft
	for _, r := range rets {
		lid := r.Int("id_purchase_return")
		branch, ok := s.id("branches", r.Int("id_branch"))
		if !ok {
			s.find(report.Blocker, "PR_BRANCH", "purchase_returns", lid, "cabang retur tidak dikenal")
			continue
		}
		ord := r.Int("id_original_order")
		if s.orderKind[ord] != "purchase" {
			s.find(report.Blocker, "PR_ORDER", "purchase_returns", lid, "order asal %d bukan order pembelian termigrasi", ord)
			continue
		}
		var status string
		switch {
		case r.Has("archived_at"), r.Str("status") == "cancelled":
			status = "cancelled"
		case r.Str("status") == "completed":
			status = "confirmed"
		default:
			s.find(report.Blocker, "PR_STATUS", "purchase_returns", lid, "status %q tidak dikenal", r.Str("status"))
			continue
		}
		date, _ := r.Time("return_date")
		dr := &docDraft{date: date, legacy: lid, branch: branch}
		if status == "cancelled" {
			dr.st, dr.reason = stateIgnored, "cancelled"
			if r.Has("archived_at") {
				dr.reason = "archived"
			}
		} else {
			reqs := []postKey{{"purchase_return", lid, "purchase_return_completed"}}
			for _, m := range s.movesByRef["purchase_return:"+itoa(lid)] {
				reqs = append(reqs, postKey{"inventory_movement", m.Int("id_inventory_movement"), "purchase_return_stock_out"})
			}
			for _, st := range setts[lid] {
				if ev := map[string]string{"refund": "purchase_return_refund", "supplier_credit": "purchase_return_supplier_credit"}[st.Str("settlement_type")]; ev != "" {
					reqs = append(reqs, postKey{"purchase_return_settlement", st.Int("id_purchase_return_settlement"), ev})
				}
			}
			dr.st, dr.reason, dr.journals = s.postState("purchase_returns", lid, date, reqs)
		}
		dr.band = band(dr.st)
		drafts = append(drafts, &retDraft{dr, r})
	}
	sort.SliceStable(drafts, func(i, j int) bool { return drafts[i].docDraft.less(drafts[j].docDraft) })
	through, settThrough := int64(0), int64(0)
	for _, dd := range drafts {
		r, lid := dd.r, dd.legacy
		id := s.alloc("purchase_returns", lid)
		s.prNew[lid] = id
		ord := r.Int("id_original_order")
		orow := s.orderRow[ord]
		poID, _ := s.id("purchase_orders", ord)
		status := "confirmed"
		if dd.st == stateIgnored && (dd.reason == "cancelled" || dd.reason == "archived") {
			status = "cancelled"
		}
		meta := &retMeta{legacy: lid, id: id, branch: dd.branch, order: poID, party: s.partyOfOrder(orow), date: dd.date,
			confirmed: status == "confirmed", st: dd.st, reason: dd.reason, journals: dd.journals, number: r.Str("return_number")}
		var rows []row.Row
		for _, it := range items[lid] {
			ilid := it.Int("id_purchase_return_item")
			pid, ok1 := s.id("products", it.Int("id_product"))
			vid, ok2 := s.id("product_variants", it.Int("id_product_variant"))
			if !ok1 || !ok2 {
				s.find(report.Blocker, "PRI_PRODUCT", "purchase_return_items", ilid, "produk/varian tidak termigrasi")
				continue
			}
			unit, nominal, total, tax, base := s.retLine("purchase_return", lid, it, "unit_price", "line_total")
			loc := s.locFor("purchase_return_items", ilid, it, dd.branch, "normal")
			rows = append(rows, row.Row{
				"id": s.nextID("purchase_return_items"), "return_id": id, "product_id": pid, "variant_id": vid, "location_id": loc,
				"uom": it.Str("transaction_uom_snapshot"), "uom_factor": it.Str("uom_to_base_factor"), "qty": it.Str("quantity"), "qty_base": it.Str("quantity_in_base_uom"),
				"unit_price": unit, "discount_pct": "0.00", "discount_nominal": nominal, "tax_base": base, "tax_amount": tax, "line_total": total,
			})
		}
		hdrTotal := s.money.R("purchase_returns", lid, "total", r, "returned_total_amount")
		s.absorbLines("purchase_return_items", lid, rows, hdrTotal)

		var ext int64
		var purchSett []row.Row
		for _, st := range setts[lid] {
			slid := st.Int("id_purchase_return_settlement")
			typ := st.Str("settlement_type")
			switch typ {
			case "refund", "supplier_credit":
			default:
				s.find(report.Blocker, "PRS_TYPE", "purchase_return_settlements", slid, "settlement_type %q tidak dikenal", typ)
				continue
			}
			amount := s.money.R("purchase_return_settlements", slid, "amount", st, "amount")
			sdate, _ := st.Time("settlement_date")
			if sdate.IsZero() {
				sdate = dd.date
			}
			var method any
			if m, ok := st.NStr("payment_method"); ok && m != "" {
				method = s.payMethod("purchase_return_settlements", slid, m)
			}
			prow := row.Row{
				"id": s.nextID("purchase_return_settlements"), "return_id": id, "settlement_type": typ, "settlement_date": fmtWIB(sdate), "amount": amount,
				"payment_method": method, "reference_number": s.optStr("purchase_return_settlements", slid, "reference_number", st, "reference_number", 100),
				"notes": s.optStr("purchase_return_settlements", slid, "notes", st, "notes", 255), "created_at": tsLocalOr(st, "created_at", sdate),
				"created_by": s.userID(st.Int("id_created_by")),
			}
			s.emit("purchase_return_settlements", prow)
			purchSett = append(purchSett, prow)
			ext += amount
			if meta.confirmed && typ == "refund" {
				s.addRetPay(&retPay{branch: dd.branch, party: meta.party, ptype: "supplier", dir: "in", amount: amount,
					method: s.payMethod("purchase_return_settlements", slid, st.Str("payment_method")), date: sdate,
					notes: "Refund retur pembelian " + meta.number, number: fmt.Sprintf("PAY-%s/MIG/RTB%05d-S%d", s.branchCode[dd.branch], lid, slid),
					by: s.userID(st.Int("id_created_by")), reference: s.optStr("purchase_return_settlements", slid, "reference_number", st, "reference_number", 100),
					orderType: "purchase_return", orderID: id}, "purchase_return_settlement", slid, "purchase_return_refund", dd.date)
			}
		}
		var reduction int64
		if meta.confirmed {
			if reduction = hdrTotal - ext; reduction < 0 {
				reduction = 0
			}
		}
		if reduction > 0 {
			s.addOffsetPair("purchase", meta, reduction, dd.date)
		}
		notes := any(nil)
		s.emit("purchase_returns", row.Row{
			"id": id, "number": s.trunc("purchase_returns", lid, "number", r.Str("return_number"), 100), "branch_id": dd.branch, "purchase_order_id": poID,
			"return_date": fmtWIB(dd.date), "total": hdrTotal, "subtotal": sumCol(rows, "tax_base"), "discount_total": sumCol(rows, "discount_nominal"),
			"tax_total": sumCol(rows, "tax_amount"), "tax_type": taxType(orow), "tax_rate": s.rate2("purchase_returns", lid, orow.Rat("tax_rate_snapshot")),
			"status": status, "notes": notes, "created_at": tsLocalOr(r, "created_at", dd.date), "updated_at": tsLocalOr(r, "updated_at", dd.date),
			"created_by": s.userID(r.Int("id_created_by")), "reason": s.trunc("purchase_returns", lid, "reason", r.Str("reason_text"), 255),
			"payable_reduction": reduction,
		})
		for _, x := range rows {
			s.emit("purchase_return_items", x)
		}
		s.srMeta[-lid] = meta // negative key: purchase side
		s.creditSettlementMarks(docPurchaseReturnSettlement, "supplier_credit", dd.docDraft, meta.confirmed, purchSett, &settThrough)
		s.out.Marks = append(s.out.Marks, Mark{DocType: docPurchaseReturn, DocID: id, BranchID: dd.branch, State: dd.st, Reason: dd.reason, LegacyJournals: dd.journals})
		if dd.band == 0 {
			through = id
		}
	}
	s.cutoverFor(docPurchaseReturn, through)
	s.cutoverFor(docPurchaseReturnSettlement, settThrough)
}

// ---------------------------------------------------------------- payments asked of the payments step

// addRetPay queues one settlement payment; its posting state follows the legacy
// settlement event (and the return date against the opening date).
func (s *state) addRetPay(p *retPay, srcType string, srcID int64, event string, retDate time.Time) {
	p.st, p.reason, p.journals = s.postState("payments", p.idx, p.date, []postKey{{srcType, srcID, event}})
	if wibDate(retDate) < s.open {
		p.st, p.reason, p.journals = stateOpeningCover, "", nil
	}
	p.band = band(p.st)
	s.retPays = append(s.retPays, p)
	p.idx = int64(len(s.retPays))
}

// addOffsetPair queues the two contra legs of an offset (no cash moves): the
// refund leg settles the return, the bill leg pays the order's open bill.
func (s *state) addOffsetPair(side string, m *retMeta, amount int64, date time.Time) {
	tag, ptype := "RTR", "customer"
	refundDir, billDir, retType, billType, ref := "out", "in", "sales_return", "sales", "offset:sr:"
	if side == "purchase" {
		tag, ptype = "RTB", "supplier"
		refundDir, billDir, retType, billType, ref = "in", "out", "purchase_return", "purchase", "offset:pr:"
	}
	code := s.branchCode[m.branch]
	base := fmt.Sprintf("PAY-%s/MIG/%s%05d", code, tag, m.legacy)
	a := &retPay{branch: m.branch, party: m.party, ptype: ptype, dir: refundDir, amount: amount, method: "offset", date: date,
		notes: "Offset otomatis retur " + m.number, number: base + "-A", orderType: retType, orderID: m.id, st: stateIgnored, reason: "offset", band: 0}
	b := &retPay{branch: m.branch, party: m.party, ptype: ptype, dir: billDir, amount: amount, method: "offset", date: date,
		notes: "Offset otomatis retur " + m.number, number: base + "-B", orderType: billType, orderID: m.order, st: stateIgnored, reason: "offset", band: 0,
		clientRef: ref + itoa(m.id)}
	s.retPays = append(s.retPays, a, b)
	a.idx = int64(len(s.retPays) - 1)
	b.idx = int64(len(s.retPays))
	a.pairWith, b.pairWith = int(b.idx), int(a.idx)
	s.out.Explain["synthetic_return_offsets"]++
}

// emitRetPayment writes one queued payment (called from the payments step so
// ids and the cutover boundary stay in one sequence).
func (s *state) emitRetPayment(p *retPay, id int64) {
	p.newID = id
	r := row.Row{
		"id": id, "number": p.number, "branch_id": p.branch, "party_id": p.party, "party_type": p.ptype, "direction": p.dir, "amount": p.amount,
		"method": p.method, "paid_at": fmtWIB(p.date), "notes": p.notes, "status": "active", "created_at": fmtUTC(p.date), "created_by": p.by,
		"client_ref": p.clientRef, "amount_tendered": nil, "reference_number": p.reference, "pair_id": nil,
	}
	p.row = r
	s.emit("payments", r)
	s.emit("payment_allocations", row.Row{"id": s.nextID("payment_allocations"), "payment_id": id, "order_type": p.orderType, "order_id": p.orderID, "amount": p.amount})
}

// linkPairs fills pair_id of the offset legs once both ids exist.
func (s *state) linkPairs() {
	for _, p := range s.retPays {
		if p.pairWith > 0 && p.row != nil {
			if o := s.retPays[p.pairWith-1]; o.newID != 0 {
				p.row["pair_id"] = o.newID
			}
		}
	}
}
