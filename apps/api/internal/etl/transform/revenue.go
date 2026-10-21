package transform

import (
	"math/big"
	"sort"
	"strconv"
	"strings"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/shared/money"
)

// revCandidate is an order SJ that legacy posted only in part because its
// ORDER never reached the books: legacy journals revenue when the order
// completes (event sales_completed) and COGS when the goods leave
// (stock_issue_cogs). Such an SJ sits below the delivery cutover boundary
// (ignored, legacy_partial), so the revamp's posting queue never books its
// revenue either.
type revCandidate struct {
	dn, legacyDoc, legacyOrder, branch int64
}

type soLine struct {
	qty, discPct           float64
	unitPrice, discNominal int64
}

func ratF(r row.Row, k string) float64 {
	f, _ := r.Rat(k).Float64()
	return f
}

func lineKey(product, variant int64, uom string) string {
	return strconv.FormatInt(product, 10) + "/" + strconv.FormatInt(variant, 10) + "/" + strings.TrimSpace(uom)
}

// unbookedRevenue books, as a migration correction (ADR-0018 D11), the
// revenue of every delivered order SJ that neither legacy nor the revamp will
// ever journal: Dr Piutang (customer) / Cr Penjualan (+ Cr PPN Keluaran when
// taxed). The amount is what the revamp's own delivery posting (buildDelivery)
// would book for that SJ: the SO line's tax base and tax pro-rated to the
// SJ's quantity window, so a later SJ of the same order, posted by the queue,
// books exactly the remainder and nothing twice.
func (s *state) unbookedRevenue() {
	if len(s.revCand) == 0 {
		return
	}
	through := map[int64]int64{}
	for _, c := range s.out.Cutovers {
		if c.DocType == docDelivery {
			through[c.BranchID] = c.Through
		}
	}
	orders := map[int64]row.Row{}
	for _, o := range s.out.Tables["sales_orders"] {
		orders[o.Int("id")] = o
	}
	lines := map[int64]map[string]*soLine{}
	for _, it := range s.out.Tables["sales_order_items"] {
		oid := it.Int("order_id")
		if lines[oid] == nil {
			lines[oid] = map[string]*soLine{}
		}
		k := lineKey(it.Int("product_id"), it.Int("variant_id"), it.Str("uom"))
		if lines[oid][k] == nil { // the first SO line of a key, as buildDelivery
			lines[oid][k] = &soLine{qty: ratF(it, "qty"), unitPrice: it.Int("unit_price"), discPct: ratF(it, "discount_pct"), discNominal: it.Int("discount_nominal")}
		}
	}
	notesOf := map[int64][]row.Row{}
	number := map[int64]string{}
	for _, d := range s.out.Tables["delivery_notes"] {
		number[d.Int("id")] = d.Str("number")
		if d.Str("status") == "confirmed" && d.Str("document_kind") == "order" {
			notesOf[d.Int("sales_order_id")] = append(notesOf[d.Int("sales_order_id")], d)
		}
	}
	itemsOf := map[int64][]row.Row{}
	for _, it := range s.out.Tables["delivery_note_items"] {
		itemsOf[it.Int("delivery_id")] = append(itemsOf[it.Int("delivery_id")], it)
	}

	cands := append([]revCandidate(nil), s.revCand...)
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].legacyOrder != cands[j].legacyOrder {
			return cands[i].legacyOrder < cands[j].legacyOrder
		}
		return cands[i].dn < cands[j].dn
	})
	for i := 0; i < len(cands); {
		j := i
		for j < len(cands) && cands[j].legacyOrder == cands[i].legacyOrder {
			j++
		}
		group := cands[i:j]
		i = j
		lo := group[0].legacyOrder
		soID, ok := s.id("sales_orders", lo)
		so := orders[soID]
		if !ok || so == nil {
			s.find(report.Blocker, "REVENUE_UNBOOKED", "orders", lo, "order penjualan SJ tanpa jurnal pendapatan tidak termigrasi")
			continue
		}
		var sjs []string
		for _, c := range group {
			sjs = append(sjs, number[c.dn])
			if c.dn > through[c.branch] {
				// Never: a queued SJ is pending, not legacy_partial. Guarded
				// because booking it here as well would book it twice.
				s.find(report.Blocker, "REVENUE_UNBOOKED_QUEUED", "delivery_notes", c.legacyDoc, "SJ %s di atas batas cutover tetapi ditandai terposting sebagian", number[c.dn])
			}
		}
		sjList := strings.Join(sjs, ", ")
		if so.Str("status") == "cancelled" {
			s.out.Explain["revenue_unbooked_cancelled_orders"]++
			s.find(report.Warn, "REVENUE_UNBOOKED_CANCELLED", "orders", lo,
				"order %s dibatalkan di legacy padahal SJ %s terkonfirmasi tanpa pembatalan: HPP-nya dijurnal legacy, pendapatan tidak; tanpa koreksi otomatis, mohon telaah pemilik/akuntan", so.Str("number"), sjList)
			continue
		}
		taxType, taxRate := so.Str("tax_type"), ratF(so, "tax_rate")
		var revenue, tax int64
		type pv struct{ dn, product, variant int64 }
		revOf := map[pv]int64{}
		var order []pv
		for _, c := range group {
			before := map[string]float64{}
			for _, d := range notesOf[soID] {
				if d.Int("id") >= c.dn {
					continue
				}
				for _, it := range itemsOf[d.Int("id")] {
					before[lineKey(it.Int("product_id"), it.Int("variant_id"), it.Str("uom"))] += ratF(it, "qty")
				}
			}
			for _, it := range itemsOf[c.dn] {
				k := lineKey(it.Int("product_id"), it.Int("variant_id"), it.Str("uom"))
				sl := lines[soID][k]
				if sl == nil {
					s.find(report.Blocker, "REVENUE_UNBOOKED", "delivery_notes", c.legacyDoc, "baris SJ %s tak cocok baris SO %s", number[c.dn], so.Str("number"))
					continue
				}
				_, base, lineTax, _ := money.LineAmounts(sl.qty, sl.unitPrice, sl.discPct, sl.discNominal, taxType, taxRate).Split()
				part, prior := ratF(it, "qty"), before[k]
				rev := money.Prorate(base, prior+part, sl.qty) - money.Prorate(base, prior, sl.qty)
				tax += money.Prorate(lineTax, prior+part, sl.qty) - money.Prorate(lineTax, prior, sl.qty)
				revenue += rev
				key := pv{c.dn, it.Int("product_id"), it.Int("variant_id")}
				if _, seen := revOf[key]; !seen {
					order = append(order, key)
				}
				revOf[key] += rev
			}
		}
		legacyOf := map[int64]int64{}
		for _, c := range group {
			legacyOf[c.dn] = c.legacyDoc
		}
		party := so.Int("party_id")
		for _, k := range order {
			if revOf[k] == 0 {
				continue
			}
			s.migCorr = append(s.migCorr, migCorrection{kind: corrRevenueUnbooked, branch: group[0].branch, doc: soID, ref: so.Str("number"),
				party: party, product: k.product, variant: k.variant, deliveryNote: k.dn, legacyMovement: legacyOf[k.dn], amount: revOf[k], reason: "SJ " + sjList})
		}
		if tax != 0 {
			s.migCorr = append(s.migCorr, migCorrection{kind: corrRevenueUnbooked, taxLeg: true, branch: group[0].branch, doc: soID, ref: so.Str("number"),
				party: party, deliveryNote: group[len(group)-1].dn, legacyMovement: group[len(group)-1].legacyDoc, amount: tax, reason: "SJ " + sjList})
		}
		s.out.Explain["revenue_unbooked_orders"]++
		s.out.Explain["revenue_unbooked_value"] += revenue + tax
		s.find(report.Warn, "REVENUE_UNBOOKED", "orders", lo,
			"pendapatan Rp%d (penjualan Rp%d + PPN Rp%d) order %s atas SJ %s tidak pernah dijurnal: legacy menjurnal pendapatan saat order selesai (belum terjadi) sedangkan HPP SJ-nya sudah dijurnal, dan SJ di bawah batas cutover tidak diantre revamp; jurnal koreksi migrasi Dr Piutang / Cr Penjualan",
			revenue+tax, revenue, tax, so.Str("number"), sjList)
	}
}

// completeDelivered completes a confirmed sales order whose every line is
// fully delivered by confirmed order SJs — what the revamp's own
// completeOrderIfDone does after the last SJ. Legacy left such orders
// in progress (e.g. a COD order waiting for its payment), and the revamp
// would never complete them: no SJ is left to trigger it.
func (s *state) completeDelivered() {
	delivered := map[int64]map[[2]int64]*big.Rat{}
	lastConfirm := map[int64]string{}
	itemsOf := map[int64][]row.Row{}
	for _, it := range s.out.Tables["delivery_note_items"] {
		itemsOf[it.Int("delivery_id")] = append(itemsOf[it.Int("delivery_id")], it)
	}
	for _, d := range s.out.Tables["delivery_notes"] {
		if d.Str("status") != "confirmed" || d.Str("document_kind") != "order" {
			continue
		}
		so := d.Int("sales_order_id")
		if delivered[so] == nil {
			delivered[so] = map[[2]int64]*big.Rat{}
		}
		for _, it := range itemsOf[d.Int("id")] {
			k := [2]int64{it.Int("product_id"), it.Int("variant_id")}
			if delivered[so][k] == nil {
				delivered[so][k] = new(big.Rat)
			}
			delivered[so][k].Add(delivered[so][k], it.Rat("qty_base"))
		}
		if c := d.Str("confirmed_at"); c > lastConfirm[so] {
			lastConfirm[so] = c
		}
	}
	ordered := map[int64]map[[2]int64]*big.Rat{}
	for _, it := range s.out.Tables["sales_order_items"] {
		so := it.Int("order_id")
		if ordered[so] == nil {
			ordered[so] = map[[2]int64]*big.Rat{}
		}
		k := [2]int64{it.Int("product_id"), it.Int("variant_id")}
		if ordered[so][k] == nil {
			ordered[so][k] = new(big.Rat)
		}
		ordered[so][k].Add(ordered[so][k], it.Rat("qty_base"))
	}
	statusCode := map[int64]string{}
	for _, d := range s.raw["order_status_definitions"] {
		statusCode[d.Int("id_order_status_definition")] = d.Str("code")
	}
	legacyOf := map[int64]int64{}
	for lid, id := range s.maps["sales_orders"] {
		legacyOf[id] = lid
	}
	for _, o := range s.out.Tables["sales_orders"] {
		id := o.Int("id")
		if o.Str("status") != "confirmed" || len(ordered[id]) == 0 || lastConfirm[id] == "" {
			continue
		}
		done := true
		for k, q := range ordered[id] {
			if got := delivered[id][k]; got == nil || got.Cmp(q) < 0 {
				done = false
				break
			}
		}
		if !done {
			continue
		}
		o["status"], o["completed_at"] = "completed", lastConfirm[id]
		s.out.Explain["orders_completed_delivered"]++
		s.emit("order_reclass", row.Row{"branch_id": o.Int("branch_id"), "kind": "sales", "number": o.Str("number"), "from": "confirmed", "to": "completed",
			"grand_total": o.Int("grand_total"), "tax_total": o.Int("tax_total"), "discount_total": o.Int("discount_total")})
		lo := legacyOf[id]
		s.find(report.Info, "ORDER_COMPLETED_DELIVERED", "orders", lo,
			"order %s terkirim penuh oleh SJ terkonfirmasi tetapi legacy masih %q: dimigrasi selesai (completed_at = konfirmasi SJ terakhir)",
			o.Str("number"), statusCode[s.orderRow[lo].Int("id_current_status")])
	}
}

// draftOrdersPaid lists draft sales orders that already carry an active
// payment: the money is in the books (Cr Piutang) while the order never
// shipped, so the customer shows a credit until operations confirm (and
// deliver) the order or cancel and refund it.
func (s *state) draftOrdersPaid() {
	active := map[int64]string{}
	for _, p := range s.out.Tables["payments"] {
		if p.Str("status") == "active" {
			active[p.Int("id")] = p.Str("number")
		}
	}
	paid := map[int64]int64{}
	pays := map[int64][]string{}
	for _, a := range s.out.Tables["payment_allocations"] {
		if a.Str("order_type") != "sales" {
			continue
		}
		if n, ok := active[a.Int("payment_id")]; ok {
			paid[a.Int("order_id")] += a.Int("amount")
			pays[a.Int("order_id")] = append(pays[a.Int("order_id")], n)
		}
	}
	legacyOf := map[int64]int64{}
	for lid, id := range s.maps["sales_orders"] {
		legacyOf[id] = lid
	}
	for _, o := range s.out.Tables["sales_orders"] {
		id := o.Int("id")
		if o.Str("status") != "draft" || paid[id] == 0 {
			continue
		}
		s.out.Explain["draft_orders_paid"]++
		s.find(report.Warn, "DRAFT_ORDER_PAID", "orders", legacyOf[id],
			"order %s masih draf tetapi sudah dibayar Rp%d (%s): operasional mengonfirmasi order itu atau membatalkannya dan mengembalikan uangnya",
			o.Str("number"), paid[id], strings.Join(pays[id], ", "))
	}
}
