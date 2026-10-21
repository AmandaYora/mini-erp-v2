package transform

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

// Posting states of a migrated source document (design D-ETL-5).
const (
	statePending       = "pending"
	statePostedLegacy  = "posted_legacy"
	stateOpeningCover  = "opening_covered"
	stateIgnored       = "ignored"
	docDelivery        = "delivery"
	docGoodsReceipt    = "goods_receipt"
	docPayment         = "payment"
	docStockMovement   = "stock_movement"
	legacyPostedStatus = "posted"
)

type postKey struct {
	stype string
	id    int64
	event string
}

func (s *state) indexPosting() {
	s.post = map[postKey]string{}
	s.postJ = map[postKey]int64{}
	for _, r := range s.raw["finance_posting_sources"] {
		k := postKey{r.Str("source_type"), r.Int("id_source"), r.Str("event_key")}
		s.post[k] = r.Str("posting_status")
		if j, ok := r.NInt("id_journal_entry"); ok {
			s.postJ[k] = j
		}
	}
	s.journaled = map[[2]string]bool{}
	for _, j := range s.raw["finance_journal_entries"] {
		// Live journals only: a reversed one and its reversal net to nothing,
		// so posting the document again is right.
		if j.Str("status") == "posted" && !j.Has("id_reversal_of") && j.Has("id_source") {
			s.journaled[[2]string{j.Str("source_type"), j.Str("id_source")}] = true
		}
	}
}

// postState classifies a document. Documents dated before the opening date
// are covered by the opening balance regardless of their legacy status.
func (s *state) postState(entity string, id int64, date time.Time, reqs []postKey) (st, reason string, journals []int64) {
	if wibDate(date) < s.open {
		return stateOpeningCover, "", nil
	}
	posted, other := 0, 0
	ignored := 0
	for _, k := range reqs {
		switch s.post[k] {
		case legacyPostedStatus:
			posted++
			if j, ok := s.postJ[k]; ok {
				journals = append(journals, j)
			}
		case "ignored":
			ignored++
		default:
			other++
		}
	}
	switch {
	case len(reqs) > 0 && posted == len(reqs):
		return statePostedLegacy, "", journals
	case posted > 0:
		s.find(report.Warn, "POST_PARTIAL", entity, id, "terposting sebagian di legacy (%d dari %d peristiwa): ditandai ignored", posted, len(reqs))
		return stateIgnored, "legacy_partial", journals
	case ignored > 0 && other == 0:
		return stateIgnored, "legacy_ignored", nil
	}
	// Pending = queued for posting in the revamp. That is only right when
	// legacy never journalled it: either its poster never picked the document
	// up (no posting-source row yet: created after the last legacy posting
	// run) or it sat in the legacy queue (pending/ready/failed).
	queued := 0
	for _, k := range reqs {
		if s.journaled[[2]string{k.stype, strconv.FormatInt(k.id, 10)}] {
			s.find(report.Blocker, "POST_PENDING_JOURNALED", entity, id, "dokumen akan diantre ulang padahal legacy sudah menjurnal %s %d (%s): dobel posting", k.stype, k.id, k.event)
		}
		if _, ok := s.post[k]; ok {
			queued++
		}
	}
	why := "belum dikandidatkan legacy"
	key := "pending_docs_no_legacy_candidate"
	if queued > 0 {
		why, key = "masih di antrean legacy", "pending_docs_legacy_queue"
	}
	s.out.Explain[key]++
	s.find(report.Info, "POST_PENDING", entity, id, "masuk antrean posting revamp: %s (tanpa jurnal legacy)", why)
	return statePending, "", nil
}

func ratStr(r *big.Rat) string { return r.FloatString(12) }

type docDraft struct {
	band      int
	date      time.Time
	legacy    int64
	synthetic bool
	kind      string // "" | "repl" (real replacement SJ) | "synthrepl" (counter exchange)
	st        string
	reason    string
	journals  []int64
	branch    int64
}

func (d *docDraft) less(o *docDraft) bool {
	if d.band != o.band {
		return d.band < o.band
	}
	if !d.date.Equal(o.date) {
		return d.date.Before(o.date)
	}
	if d.synthetic != o.synthetic {
		return !d.synthetic
	}
	return d.legacy < o.legacy
}

func band(st string) int {
	if st == statePending {
		return 1
	}
	return 0
}

// prepareDocs builds the lookups shared by the returns and documents steps.
func (s *state) prepareDocs() {
	if s.post != nil {
		return
	}
	s.indexPosting()
	s.movesByRef = map[string][]row.Row{}
	for _, m := range sortedBy(s.raw["inventory_movements"], "id_inventory_movement") {
		if m.Has("reference_type") && m.Has("id_reference") {
			k := m.Str("reference_type") + ":" + m.Str("id_reference")
			s.movesByRef[k] = append(s.movesByRef[k], m)
		}
	}
	s.directSJ = map[int64]int64{}
	s.dnOfLegacy = map[int64]int64{}
	s.grOfLegacy = map[int64]int64{}
	s.transferOf = map[transferKey]int64{}
	s.transferFirst = map[int64]int64{}
	s.orphanTransfer = map[int64]int64{}
	s.deliveryNew = map[int64]int64{}
}

func (s *state) documents() error {
	s.prepareDocs()
	if err := s.deliveryNotes(); err != nil {
		return err
	}
	s.unbookedRevenue()
	s.completeDelivered()
	s.goodsReceipts()
	s.payments()
	s.draftOrdersPaid()
	s.transfers()
	s.syntheticTransfers()
	s.inclusiveTax()
	return nil
}

// inclusiveTax makes tax-inclusive orders visible (P2, Q-INC-1/Q-JRN-7):
// legacy booked an inclusive receipt's Persediaan at the gross price while
// the revamp costs the tax base, so a receipt journal legacy already posted
// needs a correction decision before cutover (blocker); any other inclusive
// order is reported so it is never migrated silently.
func (s *state) inclusiveTax() {
	posted := map[int64]bool{}
	for _, g := range sortedBy(s.raw["goods_receipts"], "id_goods_receipt") {
		gid := g.Int("id_goods_receipt")
		o := s.orderRow[g.Int("id_order")]
		if o == nil || !o.Bool("taxable") || !o.Bool("price_includes_tax") {
			continue
		}
		if s.post[postKey{"goods_receipt", gid, "purchase_received"}] == legacyPostedStatus {
			posted[g.Int("id_order")] = true
			s.find(report.Blocker, "INCL_GR_POSTED", "goods_receipts", gid,
				"jurnal penerimaan order inklusif pajak %s sudah diposting legacy dengan Persediaan bruto: perlu keputusan koreksi (P2)", o.Str("order_number"))
		}
	}
	for _, o := range sortedBy(s.raw["orders"], "id_order") {
		if o.Bool("taxable") && o.Bool("price_includes_tax") && !posted[o.Int("id_order")] {
			s.out.Explain["inclusive_tax_orders"]++
			s.find(report.Warn, "INCL_ORDER", "orders", o.Int("id_order"), "order %s %s inklusif pajak: periksa basis pajak & HPP (P2)", o.Str("order_kind"), o.Str("order_number"))
		}
	}
}

type transferKey struct {
	legacy, product, variant, loc int64
	dir                           string
}

func (s *state) cutoverFor(docType string, through int64) {
	var branches []int64
	for b := range s.branchCode {
		branches = append(branches, b)
	}
	sort.Slice(branches, func(i, j int) bool { return branches[i] < branches[j] })
	for _, b := range branches {
		s.out.Cutovers = append(s.out.Cutovers, Cutover{BranchID: b, DocType: docType, Through: through})
	}
}

// ---------------------------------------------------------------- delivery

func (s *state) deliveryNotes() error {
	allocByItem := map[int64][]row.Row{}
	for _, a := range sortedBy(s.raw["stock_issue_allocations"], "id_stock_issue_allocation") {
		if k, ok := a.NInt("id_delivery_note_item"); ok {
			allocByItem[k] = append(allocByItem[k], a)
		}
	}
	itemsByDN := map[int64][]row.Row{}
	for _, it := range sortedBy(s.raw["delivery_note_items"], "id_delivery_note_item") {
		itemsByDN[it.Int("id_delivery_note")] = append(itemsByDN[it.Int("id_delivery_note")], it)
	}
	orderItems := map[int64]row.Row{}
	for _, it := range s.raw["order_items"] {
		orderItems[it.Int("id_order_item")] = it
	}
	dnRows := map[int64]row.Row{}
	var drafts []*docDraft
	for _, d := range sortedBy(s.raw["delivery_notes"], "id_delivery_note") {
		lid := d.Int("id_delivery_note")
		dnRows[lid] = d
		branch, ok := s.id("branches", d.Int("id_branch"))
		if !ok {
			s.find(report.Blocker, "DN_BRANCH", "delivery_notes", lid, "cabang SJ tidak dikenal")
			continue
		}
		isRepl := d.Str("document_kind") == "replacement"
		if isRepl {
			if _, found := s.srNew[d.Int("id_sales_return")]; !found || !d.Has("id_sales_return") {
				s.find(report.Blocker, "DN_REPLACEMENT", "delivery_notes", lid, "SJ pengganti tanpa retur penjualan termigrasi")
				continue
			}
		} else if d.Str("document_kind") != "order" || !d.Has("id_order") {
			s.find(report.Blocker, "DN_KIND", "delivery_notes", lid, "jenis SJ %q tanpa order tidak dikenal", d.Str("document_kind"))
			continue
		}
		date, ok := dateAtWIBMidnight(d.Str("delivery_date"))
		if !ok {
			s.find(report.Blocker, "DN_DATE", "delivery_notes", lid, "tanggal SJ tidak terbaca")
			continue
		}
		dr := &docDraft{date: date, legacy: lid, branch: branch}
		if isRepl {
			dr.kind = "repl"
			srl := d.Int("id_sales_return")
			if d.Has("archived_at") {
				dr.st, dr.reason = stateIgnored, "archived"
			} else {
				reqs := []postKey{{"sales_return", srl, "sales_return_completed"}}
				for _, m := range s.movesByRef["sales_return_exchange:"+strconv.FormatInt(srl, 10)] {
					reqs = append(reqs, postKey{"inventory_movement", m.Int("id_inventory_movement"), "stock_issue_cogs"})
				}
				dr.st, dr.reason, dr.journals = s.postState("delivery_notes", lid, date, reqs)
			}
		} else if d.Has("archived_at") {
			dr.st, dr.reason = stateIgnored, "archived"
		} else {
			reqs := []postKey{{"order", d.Int("id_order"), "sales_completed"}}
			for _, m := range s.movesByRef["delivery_note:"+strconv.FormatInt(lid, 10)] {
				reqs = append(reqs, postKey{"inventory_movement", m.Int("id_inventory_movement"), "stock_issue_cogs"})
			}
			dr.st, dr.reason, dr.journals = s.postState("delivery_notes", lid, date, reqs)
		}
		dr.band = band(dr.st)
		drafts = append(drafts, dr)
	}
	// Direct handover without a delivery note (POS/COD): a synthetic SJ per order.
	var directOrders []int64
	seen := map[int64]bool{}
	for k, ms := range s.movesByRef {
		if !strings.HasPrefix(k, "order:") || ms[0].Str("movement_type") != "out" {
			continue
		}
		oid := ms[0].Int("id_reference")
		if !seen[oid] {
			seen[oid] = true
			directOrders = append(directOrders, oid)
		}
	}
	sort.Slice(directOrders, func(i, j int) bool { return directOrders[i] < directOrders[j] })
	for _, oid := range directOrders {
		o, ok := s.orderRow[oid]
		if !ok || s.orderKind[oid] != "sales" {
			s.find(report.Blocker, "DIRECT_ORDER", "inventory_movements", oid, "mutasi ber-referensi order %d yang bukan penjualan termigrasi", oid)
			continue
		}
		branch, _ := s.id("branches", o.Int("id_branch"))
		ms := s.movesByRef["order:"+strconv.FormatInt(oid, 10)]
		date, ok := o.Time("goods_delivered_at")
		if !ok {
			date, _ = ms[0].Time("moved_at")
		}
		reqs := []postKey{{"order", oid, "sales_completed"}}
		for _, m := range ms {
			reqs = append(reqs, postKey{"inventory_movement", m.Int("id_inventory_movement"), "stock_issue_cogs"})
		}
		dr := &docDraft{date: date, legacy: oid, synthetic: true, branch: branch}
		if o.Has("archived_at") {
			dr.st, dr.reason = stateIgnored, "archived"
		} else {
			dr.st, dr.reason, dr.journals = s.postState("delivery_notes", oid, date, reqs)
		}
		dr.band = band(dr.st)
		drafts = append(drafts, dr)
	}
	// Counter exchanges: the replacement left without a delivery note, the revamp needs one.
	var synthRets []int64
	for lid := range s.srSynth {
		synthRets = append(synthRets, lid)
	}
	sort.Slice(synthRets, func(i, j int) bool { return synthRets[i] < synthRets[j] })
	for _, rl := range synthRets {
		m := s.srMeta[rl]
		ms := s.movesByRef["sales_return_exchange:"+strconv.FormatInt(rl, 10)]
		date, _ := ms[0].Time("moved_at")
		reqs := []postKey{{"sales_return", rl, "sales_return_completed"}}
		for _, mv := range ms {
			reqs = append(reqs, postKey{"inventory_movement", mv.Int("id_inventory_movement"), "stock_issue_cogs"})
		}
		dr := &docDraft{date: date, legacy: rl, synthetic: true, kind: "synthrepl", branch: m.branch}
		dr.st, dr.reason, dr.journals = s.postState("delivery_notes", rl, date, reqs)
		dr.band = band(dr.st)
		drafts = append(drafts, dr)
	}
	sort.SliceStable(drafts, func(i, j int) bool { return drafts[i].less(drafts[j]) })

	through := int64(0)
	for _, dr := range drafts {
		var id int64
		if dr.kind == "repl" {
			id = s.alloc("delivery_notes", dr.legacy)
			s.dnOfLegacy[dr.legacy] = id
			s.replSJ[dnRows[dr.legacy].Int("id_sales_return")] = id
			s.emitReplSJ(id, dr, dnRows[dr.legacy], itemsByDN[dr.legacy])
		} else if dr.kind == "synthrepl" {
			id = s.allocSub("delivery_notes", dr.legacy, 2)
			s.replSJ[dr.legacy] = id
			s.out.Explain["synthetic_delivery_notes"]++
			s.emitSynthReplSJ(id, dr)
		} else if dr.synthetic {
			id = s.allocSub("delivery_notes", dr.legacy, 1) // sub 1: synthetic SJ of order
			s.directSJ[dr.legacy] = id
			s.out.Explain["synthetic_delivery_notes"]++
			s.emitSyntheticSJ(id, dr)
		} else {
			id = s.alloc("delivery_notes", dr.legacy)
			s.dnOfLegacy[dr.legacy] = id
			s.emitSJ(id, dr, dnRows[dr.legacy], itemsByDN[dr.legacy], allocByItem, orderItems)
		}
		s.out.Marks = append(s.out.Marks, Mark{DocType: docDelivery, DocID: id, BranchID: dr.branch, State: dr.st, Reason: dr.reason, LegacyJournals: dr.journals})
		if dr.band == 0 {
			through = id
		}
		// An order SJ legacy posted only in part because the ORDER was never
		// completed (legacy books revenue at completion, COGS at the issue):
		// its revenue would never reach the books (ADR-0018 D11, REVENUE_UNBOOKED).
		if dr.st == stateIgnored && dr.reason == "legacy_partial" && dr.kind == "" {
			oid := dr.legacy
			if !dr.synthetic {
				oid = dnRows[dr.legacy].Int("id_order")
			}
			if st := s.post[postKey{"order", oid, "sales_completed"}]; st != legacyPostedStatus && st != "ignored" &&
				!s.journaled[[2]string{"order", strconv.FormatInt(oid, 10)}] {
				s.revCand = append(s.revCand, revCandidate{dn: id, legacyDoc: dr.legacy, legacyOrder: oid, branch: dr.branch})
			}
		}
	}
	s.cutoverFor(docDelivery, through)
	return nil
}

// confirmation is the confirmed_at/by of a migrated SJ. One dispatched but
// never confirmed in legacy (still on the road at the freeze) migrates
// confirmed: the revamp knows no in-transit state and a confirmed note always
// carries its dispatch stamp as confirmed_at (ConfirmNote), so it takes the
// dispatch time and is listed for operations to collect the signed copy.
func (s *state) confirmation(d row.Row, dr *docDraft, status string) (any, any) {
	at, by := ts(d, "confirmed_at"), s.userID(d.Int("confirmed_by"))
	if status != "confirmed" || at != nil {
		return at, by
	}
	s.out.Explain["dn_in_transit"]++
	s.find(report.Warn, "DN_IN_TRANSIT", "delivery_notes", dr.legacy,
		"SJ %s dikirim %s tetapi belum dikonfirmasi diterima di legacy: dimigrasi terkonfirmasi (confirmed_at = waktu kirim); kumpulkan SJ bertanda tangan",
		d.Str("sj_number"), wibDate(timeOr(d, "dispatched_at", dr.date)))
	return tsOr(d, "dispatched_at", dr.date), s.userID(d.Int("dispatched_by"))
}

func (s *state) emitSJ(id int64, dr *docDraft, d row.Row, items []row.Row, allocByItem map[int64][]row.Row, orderItems map[int64]row.Row) {
	lid := dr.legacy
	oid, _ := s.id("sales_orders", d.Int("id_order"))
	if _, ok := s.id("sales_orders", d.Int("id_order")); !ok {
		s.find(report.Blocker, "DN_ORDER", "delivery_notes", lid, "order penjualan SJ tidak termigrasi")
	}
	status := "confirmed"
	if d.Has("archived_at") {
		status = "cancelled"
	}
	confirmedAt, confirmedBy := s.confirmation(d, dr, status)
	s.emit("delivery_notes", row.Row{
		"id": id, "number": s.trunc("delivery_notes", lid, "number", d.Str("sj_number"), 60), "branch_id": dr.branch, "sales_order_id": oid,
		"delivery_date": deliveryDate(dr.date), "driver_name": s.optStr("delivery_notes", lid, "driver_name", d, "driver_name", 100),
		"vehicle_plate":                      s.optStr("delivery_notes", lid, "vehicle_plate", d, "vehicle_plate", 20),
		"warehouse_staff_name":               s.optStr("delivery_notes", lid, "warehouse_staff_name", d, "warehouse_staff_name", 100),
		"recipient_name":                     s.optStr("delivery_notes", lid, "recipient_name", d, "recipient_name", 150),
		"recipient_signature_status":         s.optStr("delivery_notes", lid, "recipient_signature_status", d, "recipient_signature_status", 30),
		"recipient_signature_missing_reason": emptyNil(d.Str("recipient_signature_missing_reason")),
		"drop_location_note":                 emptyNil(d.Str("drop_location_note")),
		"dispatched_at":                      ts(d, "dispatched_at"), "dispatched_by": s.userID(d.Int("dispatched_by")),
		"confirmed_at": confirmedAt, "confirmed_by": confirmedBy,
		"status": status, "document_kind": "order", "sales_return_id": nil,
		"notes": s.optStr("delivery_notes", lid, "notes", d, "notes", 255), "created_at": tsLocalOr(d, "created_at", dr.date),
		"created_by": s.userID(d.Int("dispatched_by")), "client_ref": nil,
	})
	for _, it := range items {
		ilid := it.Int("id_delivery_note_item")
		oi := orderItems[it.Int("id_order_item")]
		uom, factor := "pcs", "1.000000000000"
		if oi != nil {
			uom, factor = oi.Str("transaction_uom_snapshot"), oi.Str("uom_to_base_factor")
		}
		type bucket struct{ qty, base *big.Rat }
		byLoc := map[int64]*bucket{}
		var locOrder []int64
		var productLegacy int64
		sumBase := new(big.Rat)
		for _, a := range allocByItem[ilid] {
			loc, ok := s.id("stock_locations", a.Int("id_stock_location"))
			if !ok {
				s.find(report.Blocker, "DN_LOCATION", "delivery_note_items", ilid, "lokasi alokasi tidak termigrasi")
				continue
			}
			productLegacy = a.Int("id_product")
			b := byLoc[loc]
			if b == nil {
				b = &bucket{new(big.Rat), new(big.Rat)}
				byLoc[loc] = b
				locOrder = append(locOrder, loc)
			}
			b.qty.Add(b.qty, a.Rat("quantity"))
			b.base.Add(b.base, a.Rat("quantity_in_base_uom"))
			sumBase.Add(sumBase, a.Rat("quantity_in_base_uom"))
		}
		if oi != nil && productLegacy == 0 {
			productLegacy = oi.Int("id_product")
		}
		if len(locOrder) == 0 && status != "cancelled" {
			// derive from the stock movements of this delivery note
			for _, m := range s.movesByRef["delivery_note:"+strconv.FormatInt(lid, 10)] {
				if m.Int("id_product_variant") != it.Int("id_product_variant") {
					continue
				}
				if loc, ok := s.id("stock_locations", m.Int("id_stock_location")); ok {
					if byLoc[loc] == nil {
						byLoc[loc] = &bucket{new(big.Rat), new(big.Rat)}
						locOrder = append(locOrder, loc)
					}
					byLoc[loc].qty.Add(byLoc[loc].qty, m.Rat("quantity"))
					byLoc[loc].base.Add(byLoc[loc].base, m.Rat("quantity"))
					sumBase.Add(sumBase, m.Rat("quantity"))
				}
			}
		}
		if len(locOrder) == 0 {
			loc := s.gdg[dr.branch]
			byLoc[loc] = &bucket{it.Rat("quantity"), it.Rat("quantity_in_base_uom")}
			locOrder = []int64{loc}
			sumBase = it.Rat("quantity_in_base_uom")
			s.find(report.Warn, "DN_NO_ALLOC", "delivery_note_items", ilid, "item SJ tanpa alokasi lokasi: dipindah ke GDG")
		}
		if status != "cancelled" && sumBase.Cmp(it.Rat("quantity_in_base_uom")) != 0 {
			s.find(report.Blocker, "DN_ALLOC_SUM", "delivery_note_items", ilid, "jumlah alokasi %s != qty item %s", sumBase.FloatString(6), it.Rat("quantity_in_base_uom").FloatString(6))
		}
		pid, _ := s.id("products", productLegacy)
		vid, ok := s.id("product_variants", it.Int("id_product_variant"))
		if !ok || pid == 0 {
			s.find(report.Blocker, "DN_PRODUCT", "delivery_note_items", ilid, "produk/varian item SJ tidak termigrasi")
			continue
		}
		for n, loc := range locOrder {
			b := byLoc[loc]
			sub := int64(n + 1)
			nid := s.nextID("delivery_note_items")
			s.out.IDMap = append(s.out.IDMap, IDEntry{"delivery_note_items", ilid, sub, nid})
			if n > 0 {
				s.out.Explain["delivery_items_split_extra"]++
			}
			s.emit("delivery_note_items", row.Row{"id": nid, "delivery_id": id, "product_id": pid, "variant_id": vid, "location_id": loc,
				"uom": uom, "uom_factor": factor, "qty": ratStr(b.qty), "qty_base": ratStr(b.base), "reservation_key": nil})
		}
	}
}

func (s *state) emitSyntheticSJ(id int64, dr *docDraft) {
	oid := dr.legacy
	o := s.orderRow[oid]
	soID, _ := s.id("sales_orders", oid)
	code := s.branchCode[dr.branch]
	s.emit("delivery_notes", row.Row{
		"id": id, "number": fmt.Sprintf("SJ-%s/MIG/%05d", code, oid), "branch_id": dr.branch, "sales_order_id": soID,
		"delivery_date": deliveryDate(dr.date), "driver_name": nil, "vehicle_plate": nil, "warehouse_staff_name": nil, "recipient_name": nil,
		"recipient_signature_status": nil, "recipient_signature_missing_reason": nil, "drop_location_note": nil,
		"dispatched_at": fmtUTC(dr.date), "dispatched_by": s.userID(o.Int("id_created_by")),
		"confirmed_at": fmtUTC(dr.date), "confirmed_by": s.userID(o.Int("id_created_by")),
		"status": "confirmed", "document_kind": "order", "sales_return_id": nil,
		"notes": "SJ sintetis migrasi: serah langsung tanpa surat jalan", "created_at": fmtUTC(dr.date),
		"created_by": s.userID(o.Int("id_created_by")), "client_ref": nil,
	})
	s.find(report.Info, "DN_SYNTHETIC", "orders", oid, "SJ sintetis dibuat untuk serah langsung order %d", oid)
	type key struct{ p, v, l int64 }
	sums := map[key]*big.Rat{}
	var keys []key
	for _, m := range s.movesByRef["order:"+strconv.FormatInt(oid, 10)] {
		p, ok1 := s.id("products", m.Int("id_product"))
		v, ok2 := s.id("product_variants", m.Int("id_product_variant"))
		l, ok3 := s.id("stock_locations", m.Int("id_stock_location"))
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		k := key{p, v, l}
		if sums[k] == nil {
			sums[k] = new(big.Rat)
			keys = append(keys, k)
		}
		sums[k].Add(sums[k], m.Rat("quantity"))
	}
	for n, k := range keys {
		// The order line's own unit: posting matches SJ lines to SO lines by
		// (product, variant, unit), and qty is in that unit.
		uom, factor := s.orderLineUnit(oid, k.p, k.v)
		nid := s.nextID("delivery_note_items")
		s.out.Explain["synthetic_delivery_items"]++
		s.out.IDMap = append(s.out.IDMap, IDEntry{"delivery_note_items", -oid, int64(n + 1), nid})
		s.emit("delivery_note_items", row.Row{"id": nid, "delivery_id": id, "product_id": k.p, "variant_id": k.v, "location_id": k.l,
			"uom": uom, "uom_factor": ratStr(factor), "qty": ratStr(new(big.Rat).Quo(sums[k], factor)), "qty_base": ratStr(sums[k]), "reservation_key": nil})
	}
}

// orderLineUnit is the unit and base factor of the first legacy order line
// of (product, variant) — new ids — on order oid ("pcs", 1 when none).
func (s *state) orderLineUnit(oid, product, variant int64) (string, *big.Rat) {
	if s.orderLines == nil {
		s.orderLines = map[int64][]row.Row{}
		for _, it := range sortedBy(s.raw["order_items"], "id_order_item") {
			s.orderLines[it.Int("id_order")] = append(s.orderLines[it.Int("id_order")], it)
		}
	}
	for _, it := range s.orderLines[oid] {
		p, _ := s.id("products", it.Int("id_product"))
		v, _ := s.id("product_variants", it.Int("id_product_variant"))
		if p == product && v == variant && it.Str("transaction_uom_snapshot") != "" {
			if f := it.Rat("uom_to_base_factor"); f.Sign() > 0 {
				return it.Str("transaction_uom_snapshot"), f
			}
		}
	}
	return "pcs", big.NewRat(1, 1)
}

// ---------------------------------------------------------------- goods receipts

func (s *state) goodsReceipts() {
	items := map[int64][]row.Row{}
	for _, it := range sortedBy(s.raw["goods_receipt_items"], "id_goods_receipt_item") {
		items[it.Int("id_goods_receipt")] = append(items[it.Int("id_goods_receipt")], it)
	}
	orderItems := map[int64]row.Row{}
	for _, it := range s.raw["order_items"] {
		orderItems[it.Int("id_order_item")] = it
	}
	rows := map[int64]row.Row{}
	var drafts []*docDraft
	for _, g := range sortedBy(s.raw["goods_receipts"], "id_goods_receipt") {
		lid := g.Int("id_goods_receipt")
		rows[lid] = g
		branch, ok := s.id("branches", g.Int("id_branch"))
		if !ok {
			s.find(report.Blocker, "GR_BRANCH", "goods_receipts", lid, "cabang penerimaan tidak dikenal")
			continue
		}
		date, _ := g.Time("received_at")
		dr := &docDraft{date: date, legacy: lid, branch: branch}
		if g.Has("archived_at") {
			dr.st, dr.reason = stateIgnored, "archived"
		} else {
			dr.st, dr.reason, dr.journals = s.postState("goods_receipts", lid, date, []postKey{{"goods_receipt", lid, "purchase_received"}})
		}
		dr.band = band(dr.st)
		drafts = append(drafts, dr)
	}
	// numbers follow the chronological order (continuous per branch, revamp format)
	chrono := append([]*docDraft(nil), drafts...)
	sort.SliceStable(chrono, func(i, j int) bool {
		if !chrono[i].date.Equal(chrono[j].date) {
			return chrono[i].date.Before(chrono[j].date)
		}
		return chrono[i].legacy < chrono[j].legacy
	})
	counter := map[int64]int{}
	numbers := map[int64]string{}
	for _, dr := range chrono {
		counter[dr.branch]++
		numbers[dr.legacy] = fmt.Sprintf("GR-%s/%s/%05d", s.branchCode[dr.branch], wibDate(dr.date)[:4], counter[dr.branch])
		s.find(report.Info, "GR_NUMBER", "goods_receipts", dr.legacy, "nomor penerimaan dibuat ETL: %s", numbers[dr.legacy])
	}
	sort.SliceStable(drafts, func(i, j int) bool { return drafts[i].less(drafts[j]) })
	through := int64(0)
	for _, dr := range drafts {
		g := rows[dr.legacy]
		lid := dr.legacy
		id := s.alloc("goods_receipts", lid)
		s.grOfLegacy[lid] = id
		poID, ok := s.id("purchase_orders", g.Int("id_order"))
		if !ok {
			s.find(report.Blocker, "GR_ORDER", "goods_receipts", lid, "order pembelian tidak termigrasi")
		}
		status := "posted"
		if g.Has("archived_at") {
			status = "void"
		}
		s.emit("goods_receipts", row.Row{
			"id": id, "number": numbers[lid], "branch_id": dr.branch, "purchase_order_id": poID, "received_at": fmtWIB(dr.date),
			"notes": s.optStr("goods_receipts", lid, "notes", g, "notes", 255), "created_at": tsLocalOr(g, "created_at", dr.date),
			"created_by": s.userID(g.Int("id_received_by")), "supplier_delivery_number": s.optStr("goods_receipts", lid, "supplier_delivery_number", g, "supplier_delivery_number", 60),
			"status": status, "client_ref": nil,
		})
		for _, it := range items[lid] {
			ilid := it.Int("id_goods_receipt_item")
			pid, ok1 := s.id("products", it.Int("id_product"))
			vid, ok2 := s.id("product_variants", it.Int("id_product_variant"))
			if !ok1 || !ok2 {
				s.find(report.Blocker, "GR_PRODUCT", "goods_receipt_items", ilid, "produk/varian tidak termigrasi")
				continue
			}
			var loc int64
			if l, ok := it.NInt("id_stock_location"); ok {
				loc, _ = s.id("stock_locations", l)
			} else if l, ok := g.NInt("id_stock_location"); ok {
				loc, _ = s.id("stock_locations", l)
			}
			if loc == 0 {
				loc = s.gdg[dr.branch]
			}
			factor := "1.000000000000"
			if oi := orderItems[it.Int("id_order_item")]; oi != nil {
				factor = oi.Str("uom_to_base_factor")
			}
			nid := s.nextID("goods_receipt_items")
			s.out.IDMap = append(s.out.IDMap, IDEntry{"goods_receipt_items", ilid, 1, nid})
			s.emit("goods_receipt_items", row.Row{"id": nid, "receipt_id": id, "product_id": pid, "variant_id": vid, "location_id": loc,
				"uom": it.Str("transaction_uom_snapshot"), "uom_factor": factor, "qty": it.Str("quantity"), "qty_base": it.Str("quantity_in_base_uom")})
		}
		s.out.Marks = append(s.out.Marks, Mark{DocType: docGoodsReceipt, DocID: id, BranchID: dr.branch, State: dr.st, Reason: dr.reason, LegacyJournals: dr.journals})
		if dr.band == 0 {
			through = id
		}
	}
	s.cutoverFor(docGoodsReceipt, through)
}

// ---------------------------------------------------------------- payments

func (s *state) payments() {
	allocs := map[int64][]row.Row{}
	for _, a := range sortedBy(s.raw["payment_allocations"], "id_payment_allocation") {
		if a.Has("archived_at") {
			continue
		}
		allocs[a.Int("id_payment")] = append(allocs[a.Int("id_payment")], a)
	}
	rows := map[int64]row.Row{}
	type pinfo struct {
		dir, ptype, event string
	}
	infos := map[int64]pinfo{}
	var drafts []*docDraft
	for _, p := range sortedBy(s.raw["payments"], "id_payment") {
		lid := p.Int("id_payment")
		rows[lid] = p
		branch, ok := s.id("branches", p.Int("id_branch"))
		if !ok {
			s.find(report.Blocker, "PAY_BRANCH", "payments", lid, "cabang pembayaran tidak dikenal")
			continue
		}
		var dir string
		switch p.Str("financial_side") {
		case "receivable":
			dir = "in"
		case "payable":
			dir = "out"
		default:
			if k, ok := s.orderKind[p.Int("id_order")]; ok && p.Has("id_order") {
				dir = map[string]string{"sales": "in", "purchase": "out"}[k]
			}
		}
		if dir == "" {
			s.find(report.Blocker, "PAY_SIDE", "payments", lid, "arah pembayaran tidak dapat ditentukan")
			continue
		}
		event := "customer_payment"
		ptype := "customer"
		if dir == "out" {
			event, ptype = "supplier_payment", "supplier"
		}
		infos[lid] = pinfo{dir, ptype, event}
		date, _ := p.Time("payment_date")
		dr := &docDraft{date: date, legacy: lid, branch: branch}
		switch {
		case p.Has("archived_at"):
			dr.st, dr.reason = stateIgnored, "archived"
		case p.Has("id_order") && s.orderRow[p.Int("id_order")].Has("archived_at"):
			dr.st, dr.reason = stateIgnored, "order_archived"
		default:
			dr.st, dr.reason, dr.journals = s.postState("payments", lid, date, []postKey{{"payment", lid, event}})
		}
		dr.band = band(dr.st)
		drafts = append(drafts, dr)
	}
	for _, rp := range s.retPays {
		drafts = append(drafts, &docDraft{date: rp.date, legacy: rp.idx, synthetic: true, st: rp.st, reason: rp.reason, journals: rp.journals, branch: rp.branch, band: rp.band})
	}
	sort.SliceStable(drafts, func(i, j int) bool { return drafts[i].less(drafts[j]) })
	through := int64(0)
	for _, dr := range drafts {
		if dr.synthetic {
			rp := s.retPays[dr.legacy-1]
			id := s.allocSub("payments", -dr.legacy, 1)
			s.emitRetPayment(rp, id)
			s.out.Marks = append(s.out.Marks, Mark{DocType: docPayment, DocID: id, BranchID: dr.branch, State: dr.st, Reason: dr.reason, LegacyJournals: dr.journals})
			if dr.band == 0 {
				through = id
			}
			continue
		}
		lid := dr.legacy
		p := rows[lid]
		in := infos[lid]
		id := s.alloc("payments", lid)
		var party int64
		if b, ok := p.NInt("id_business_party"); ok {
			party, _ = s.id("parties", b)
		} else if p.Has("id_order") {
			if o, ok := s.orderRow[p.Int("id_order")]; ok {
				if rp, ok := o.NInt("id_related_party"); ok {
					party, _ = s.id("parties", rp)
				}
			}
		}
		method := p.Str("payment_method")
		switch method {
		case "cash", "check":
		case "bank_transfer":
			method = "transfer"
		default:
			s.find(report.Warn, "PAY_METHOD", "payments", lid, "metode %q dipetakan ke transfer", method)
			method = "transfer"
		}
		status := "active"
		if p.Has("archived_at") {
			status = "cancelled"
		}
		amount := s.money.R("payments", lid, "amount", p, "amount")
		var tendered any
		if p.Has("amount_tendered") {
			tendered = s.money.R("payments", lid, "amount_tendered", p, "amount_tendered")
		}
		s.emit("payments", row.Row{
			"id": id, "number": s.trunc("payments", lid, "number", p.Str("payment_number"), 60), "branch_id": dr.branch, "party_id": party,
			"party_type": in.ptype, "direction": in.dir, "amount": amount, "method": method, "paid_at": fmtWIB(dr.date),
			"notes": s.optStr("payments", lid, "notes", p, "notes", 255), "status": status, "created_at": tsLocalOr(p, "created_at", dr.date),
			"created_by": s.userID(p.Int("id_created_by")), "client_ref": nil, "amount_tendered": tendered,
			"reference_number": s.optStr("payments", lid, "reference_number", p, "reference_number", 100),
		})
		// allocations: copy active ones; a per-order payment has none (design V6): synthesize one.
		otype := "sales"
		if in.dir == "out" {
			otype = "purchase"
		}
		type al struct{ order, amount int64 }
		var list []al
		if as := allocs[lid]; len(as) > 0 {
			for _, a := range as {
				kind := s.orderKind[a.Int("id_order")]
				oid, ok := s.id(kind+"_orders", a.Int("id_order"))
				if !ok || kind != otype {
					s.find(report.Blocker, "PAY_ALLOC_ORDER", "payment_allocations", a.Int("id_payment_allocation"), "alokasi menunjuk order yang tidak cocok/tidak termigrasi")
					continue
				}
				list = append(list, al{oid, s.money.R("payment_allocations", a.Int("id_payment_allocation"), "amount", a, "allocated_amount")})
			}
		} else if p.Has("id_order") {
			kind := s.orderKind[p.Int("id_order")]
			oid, ok := s.id(kind+"_orders", p.Int("id_order"))
			if !ok || kind != otype {
				s.find(report.Blocker, "PAY_ALLOC_ORDER", "payments", lid, "order pembayaran tidak cocok/tidak termigrasi")
			} else {
				list = append(list, al{oid, amount})
				s.out.Explain["synthetic_allocations"]++
			}
		} else {
			s.find(report.Blocker, "PAY_ORPHAN", "payments", lid, "pembayaran tanpa order dan tanpa alokasi")
		}
		var total int64
		for _, a := range list {
			total += a.amount
		}
		if len(list) > 0 && total != amount {
			d := amount - total
			if abs64(d) > 1 {
				s.find(report.Blocker, "PAY_ALLOC_SUM", "payments", lid, "jumlah alokasi %d != pembayaran %d", total, amount)
			} else {
				best := 0
				for i, a := range list {
					if a.amount > list[best].amount {
						best = i
					}
				}
				list[best].amount += d
				s.log.Adjust("payment_allocations", lid, "amount", fmt.Sprint(total), fmt.Sprint(amount), "absorb_into_largest_allocation")
			}
		}
		for _, a := range list {
			s.emit("payment_allocations", row.Row{"id": s.nextID("payment_allocations"), "payment_id": id, "order_type": otype, "order_id": a.order, "amount": a.amount})
		}
		if k := p.Str("proof_file_path"); k != "" {
			s.proofs = append(s.proofs, proofRef{newPayment: id, legacy: lid, key: k, date: dr.date})
		}
		s.out.Marks = append(s.out.Marks, Mark{DocType: docPayment, DocID: id, BranchID: dr.branch, State: dr.st, Reason: dr.reason, LegacyJournals: dr.journals})
		if dr.band == 0 {
			through = id
		}
	}
	s.linkPairs()
	s.cutoverFor(docPayment, through)
}

// ---------------------------------------------------------------- transfers

func (s *state) transfers() {
	items := map[int64][]row.Row{}
	for _, it := range sortedBy(s.raw["stock_transfer_items"], "id_stock_transfer_item") {
		items[it.Int("id_stock_transfer")] = append(items[it.Int("id_stock_transfer")], it)
	}
	ts_ := append([]row.Row(nil), s.raw["stock_transfers"]...)
	sort.SliceStable(ts_, func(i, j int) bool {
		a, _ := ts_[i].Time("transfer_date")
		b, _ := ts_[j].Time("transfer_date")
		if !a.Equal(b) {
			return a.Before(b)
		}
		return ts_[i].Int("id_stock_transfer") < ts_[j].Int("id_stock_transfer")
	})
	for _, t := range ts_ {
		lid := t.Int("id_stock_transfer")
		fb, ok1 := s.id("branches", t.Int("id_from_branch"))
		tb, ok2 := s.id("branches", t.Int("id_to_branch"))
		if !ok1 || !ok2 {
			s.find(report.Blocker, "TRF_BRANCH", "stock_transfers", lid, "cabang transfer tidak dikenal")
			continue
		}
		status := t.Str("status")
		switch status {
		case "draft", "in_transit", "received", "cancelled":
		default:
			s.find(report.Blocker, "TRF_STATUS", "stock_transfers", lid, "status transfer %q tidak dikenal", status)
			continue
		}
		if t.Has("archived_at") {
			status = "cancelled"
		}
		type gk struct{ from, to int64 }
		var order []gk
		groups := map[gk][]row.Row{}
		for _, it := range items[lid] {
			f, ok1 := s.id("stock_locations", it.Int("id_from_location"))
			to, ok2 := s.id("stock_locations", it.Int("id_to_location"))
			if !ok1 || !ok2 {
				s.find(report.Blocker, "TRF_LOCATION", "stock_transfer_items", it.Int("id_stock_transfer_item"), "lokasi transfer tidak termigrasi")
				continue
			}
			k := gk{f, to}
			if _, seen := groups[k]; !seen {
				order = append(order, k)
			}
			groups[k] = append(groups[k], it)
		}
		date, _ := t.Time("transfer_date")
		for n, k := range order {
			sub := int64(n + 1)
			id := s.allocSub("stock_transfers", lid, sub)
			if n == 0 {
				s.maps["stock_transfers"] = ensure(s.maps["stock_transfers"])
				s.maps["stock_transfers"][lid] = id
				s.transferFirst[lid] = id
			} else {
				s.out.Explain["transfer_groups_extra"]++
				s.find(report.Info, "TRF_SPLIT", "stock_transfers", lid, "transfer dipecah per pasangan lokasi (kelompok %d)", n+1)
			}
			number := t.Str("transfer_number")
			if n > 0 {
				number += "-" + string(rune('A'+n))
			}
			s.emit("stock_transfers", row.Row{
				"id": id, "number": s.trunc("stock_transfers", lid, "number", number, 60), "from_branch_id": fb, "to_branch_id": tb,
				"from_location_id": k.from, "to_location_id": k.to, "status": status, "notes": emptyNil(t.Str("notes")),
				// ST-02: driver, vehicle and the transfer date of the legacy document.
				"driver_name":   emptyNil(s.trunc("stock_transfers", lid, "driver_name", t.Str("driver_name"), 150)),
				"vehicle_plate": emptyNil(s.trunc("stock_transfers", lid, "vehicle_plate", t.Str("vehicle_number"), 30)),
				"transfer_date": fmtWIB(timeOr(t, "transfer_date", date)),
				"dispatched_at": ts(t, "dispatched_at"), "received_at": ts(t, "received_at"), "created_at": tsLocalOr(t, "created_at", date),
				"created_by": s.userID(t.Int("id_created_by")),
			})
			for _, it := range groups[k] {
				p, ok1 := s.id("products", it.Int("id_product"))
				v, ok2 := s.id("product_variants", it.Int("id_product_variant"))
				if !ok1 || !ok2 {
					s.find(report.Blocker, "TRF_PRODUCT", "stock_transfer_items", it.Int("id_stock_transfer_item"), "produk/varian tidak termigrasi")
					continue
				}
				s.transferOf[transferKey{lid, p, v, k.from, "out"}] = id
				s.transferOf[transferKey{lid, p, v, k.to, "in"}] = id
				nid := s.nextID("stock_transfer_items")
				s.out.IDMap = append(s.out.IDMap, IDEntry{"stock_transfer_items", it.Int("id_stock_transfer_item"), 1, nid})
				s.emit("stock_transfer_items", row.Row{"id": nid, "transfer_id": id, "product_id": p, "variant_id": v,
					"qty_base": it.Str("quantity"), "notes": emptyNil(it.Str("notes")), "to_location_id": k.to})
			}
		}
	}
}

// syntheticTransfers rebuilds a transfer document for legacy one-step location
// moves. Legacy wrote those as an out/in pair of inventory_movements typed
// "transfer" with a NULL id_reference and no stock_transfers row. Each pair
// (same branch, product, variant and quantity, different location, moments
// apart) becomes an item of a received transfer, grouped per (branch, from, to,
// second) - the same shape the revamp's MoveLocation produces. Movements that
// cannot be paired stay unmapped and surface as MOV_REF blockers.
func (s *state) syntheticTransfers() {
	type mv struct {
		row  row.Row
		id   int64
		when time.Time
	}
	var outs, ins []mv
	for _, m := range sortedBy(s.raw["inventory_movements"], "id_inventory_movement") {
		if m.Has("id_reference") {
			continue
		}
		if t := m.Str("reference_type"); t != "transfer" && t != "stock_transfer" {
			continue
		}
		when, _ := m.Time("moved_at")
		x := mv{m, m.Int("id_inventory_movement"), when}
		switch m.Str("movement_type") {
		case "out":
			outs = append(outs, x)
		case "in":
			ins = append(ins, x)
		}
	}
	used := map[int64]bool{}
	type pair struct{ out, in mv }
	type gk struct {
		branch, from, to int64
		sec              int64
	}
	var order []gk
	groups := map[gk][]pair{}
	for _, o := range outs {
		var best *mv
		for i := range ins {
			c := &ins[i]
			if used[c.id] || c.row.Int("id_branch") != o.row.Int("id_branch") ||
				c.row.Int("id_product") != o.row.Int("id_product") || c.row.Int("id_product_variant") != o.row.Int("id_product_variant") ||
				c.row.Int("id_stock_location") == o.row.Int("id_stock_location") || c.row.Rat("quantity").Cmp(o.row.Rat("quantity")) != 0 {
				continue
			}
			d := c.when.Sub(o.when)
			if d < 0 {
				d = -d
			}
			if d > 5*time.Second {
				continue
			}
			if best == nil || c.id < best.id {
				best = c
			}
		}
		if best == nil {
			continue
		}
		used[best.id] = true
		k := gk{o.row.Int("id_branch"), o.row.Int("id_stock_location"), best.row.Int("id_stock_location"), o.when.Unix()}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], pair{o, *best})
	}
	for _, k := range order {
		ps := groups[k]
		first := ps[0].out
		br, ok1 := s.id("branches", k.branch)
		f, ok2 := s.id("stock_locations", k.from)
		to, ok3 := s.id("stock_locations", k.to)
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		id := s.nextID("stock_transfers")
		s.out.IDMap = append(s.out.IDMap, IDEntry{"stock_transfers_synthetic", first.id, 0, id})
		s.out.Explain["transfer_groups_extra"]++
		s.find(report.Info, "TRF_SYNTH", "inventory_movements", first.id, "pindah-lokasi tanpa dokumen dibuatkan transfer sintetis (%d item)", len(ps))
		s.emit("stock_transfers", row.Row{
			"id": id, "number": fmt.Sprintf("TRF-%s/MIG/%d", s.branchCode[br], first.id), "from_branch_id": br, "to_branch_id": br,
			"from_location_id": f, "to_location_id": to, "status": "received", "notes": emptyNil(first.row.Str("reason_text")),
			"dispatched_at": fmtUTC(first.when), "received_at": fmtUTC(ps[len(ps)-1].in.when), "created_at": fmtUTC(first.when),
			"created_by": s.userID(first.row.Int("id_moved_by")),
		})
		for _, pr := range ps {
			p, ok1 := s.id("products", pr.out.row.Int("id_product"))
			v, ok2 := s.id("product_variants", pr.out.row.Int("id_product_variant"))
			if !ok1 || !ok2 {
				continue
			}
			s.orphanTransfer[pr.out.id] = id
			s.orphanTransfer[pr.in.id] = id
			s.out.Explain["synthetic_transfer_items"]++
			s.emit("stock_transfer_items", row.Row{"id": s.nextID("stock_transfer_items"), "transfer_id": id, "product_id": p, "variant_id": v,
				"qty_base": pr.out.row.Str("quantity"), "notes": nil, "to_location_id": to})
		}
	}
}

func ensure(m map[int64]int64) map[int64]int64 {
	if m == nil {
		return map[int64]int64{}
	}
	return m
}

type proofRef struct {
	newPayment, legacy int64
	key                string
	date               time.Time
}

// emitReplSJ writes a replacement delivery note (document_kind replacement).
func (s *state) emitReplSJ(id int64, dr *docDraft, d row.Row, items []row.Row) {
	lid := dr.legacy
	srl := d.Int("id_sales_return")
	meta := s.srMeta[srl]
	status := "confirmed"
	if d.Has("archived_at") {
		status = "cancelled"
	}
	confirmedAt, confirmedBy := s.confirmation(d, dr, status)
	s.emit("delivery_notes", row.Row{
		"id": id, "number": s.trunc("delivery_notes", lid, "number", d.Str("sj_number"), 60), "branch_id": dr.branch, "sales_order_id": meta.order,
		"delivery_date": deliveryDate(dr.date), "driver_name": s.optStr("delivery_notes", lid, "driver_name", d, "driver_name", 100),
		"vehicle_plate":                      s.optStr("delivery_notes", lid, "vehicle_plate", d, "vehicle_plate", 20),
		"warehouse_staff_name":               s.optStr("delivery_notes", lid, "warehouse_staff_name", d, "warehouse_staff_name", 100),
		"recipient_name":                     s.optStr("delivery_notes", lid, "recipient_name", d, "recipient_name", 150),
		"recipient_signature_status":         s.optStr("delivery_notes", lid, "recipient_signature_status", d, "recipient_signature_status", 30),
		"recipient_signature_missing_reason": emptyNil(d.Str("recipient_signature_missing_reason")),
		"drop_location_note":                 emptyNil(d.Str("drop_location_note")),
		"dispatched_at":                      ts(d, "dispatched_at"), "dispatched_by": s.userID(d.Int("dispatched_by")),
		"confirmed_at": confirmedAt, "confirmed_by": confirmedBy,
		"status": status, "document_kind": "replacement", "sales_return_id": s.srNew[srl],
		"notes": s.optStr("delivery_notes", lid, "notes", d, "notes", 255), "created_at": tsLocalOr(d, "created_at", dr.date),
		"created_by": s.userID(d.Int("dispatched_by")), "client_ref": nil,
	})
	byID := map[int64]row.Row{}
	for _, ri := range s.srRepl[srl] {
		byID[ri.Int("id_sales_return_item")] = ri
	}
	for _, it := range items {
		ilid := it.Int("id_delivery_note_item")
		ri, ok := byID[it.Int("id_sales_return_item")]
		if !ok {
			s.find(report.Blocker, "DN_REPL_ITEM", "delivery_note_items", ilid, "item SJ pengganti tidak menunjuk baris pengganti retur")
			continue
		}
		pid, ok1 := s.id("products", ri.Int("id_product"))
		vid, ok2 := s.id("product_variants", it.Int("id_product_variant"))
		if !ok1 || !ok2 {
			s.find(report.Blocker, "DN_PRODUCT", "delivery_note_items", ilid, "produk/varian item SJ tidak termigrasi")
			continue
		}
		loc := s.srItemLoc[ri.Int("id_sales_return_item")]
		if loc == 0 {
			loc = s.gdg[dr.branch]
		}
		nid := s.nextID("delivery_note_items")
		s.out.IDMap = append(s.out.IDMap, IDEntry{"delivery_note_items", ilid, 1, nid})
		s.emit("delivery_note_items", row.Row{"id": nid, "delivery_id": id, "product_id": pid, "variant_id": vid, "location_id": loc,
			"uom": ri.Str("transaction_uom_snapshot"), "uom_factor": ri.Str("uom_to_base_factor"), "qty": ratStr(it.Rat("quantity")),
			"qty_base": ratStr(it.Rat("quantity_in_base_uom")), "reservation_key": nil})
	}
}

// emitSynthReplSJ writes the synthetic replacement SJ of a counter exchange.
func (s *state) emitSynthReplSJ(id int64, dr *docDraft) {
	rl := dr.legacy
	m := s.srMeta[rl]
	code := s.branchCode[dr.branch]
	by := s.userID(0)
	s.emit("delivery_notes", row.Row{
		"id": id, "number": fmt.Sprintf("SJ-%s/MIG/R%05d", code, rl), "branch_id": dr.branch, "sales_order_id": m.order,
		"delivery_date": deliveryDate(dr.date), "driver_name": nil, "vehicle_plate": nil, "warehouse_staff_name": nil, "recipient_name": nil,
		"recipient_signature_status": nil, "recipient_signature_missing_reason": nil, "drop_location_note": nil,
		"dispatched_at": fmtUTC(dr.date), "dispatched_by": by, "confirmed_at": fmtUTC(dr.date), "confirmed_by": by,
		"status": "confirmed", "document_kind": "replacement", "sales_return_id": s.srNew[rl],
		"notes": "SJ pengganti sintetis migrasi: tukar langsung di kasir", "created_at": fmtUTC(dr.date), "created_by": by, "client_ref": nil,
	})
	s.find(report.Info, "DN_SYNTHETIC_REPL", "sales_returns", rl, "SJ pengganti sintetis dibuat untuk retur tukar %d", rl)
	type key struct{ p, v, l int64 }
	sums := map[key]*big.Rat{}
	var keys []key
	for _, mv := range s.movesByRef["sales_return_exchange:"+strconv.FormatInt(rl, 10)] {
		p, ok1 := s.id("products", mv.Int("id_product"))
		v, ok2 := s.id("product_variants", mv.Int("id_product_variant"))
		l, ok3 := s.id("stock_locations", mv.Int("id_stock_location"))
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		k := key{p, v, l}
		if sums[k] == nil {
			sums[k] = new(big.Rat)
			keys = append(keys, k)
		}
		sums[k].Add(sums[k], mv.Rat("quantity"))
	}
	for n, k := range keys {
		uom, factor := "pcs", big.NewRat(1, 1)
		for _, ri := range s.srRepl[rl] {
			if pid, _ := s.id("products", ri.Int("id_product")); pid == k.p {
				if f := ri.Rat("uom_to_base_factor"); f.Sign() > 0 {
					uom, factor = ri.Str("transaction_uom_snapshot"), f
				}
			}
		}
		nid := s.nextID("delivery_note_items")
		s.out.Explain["synthetic_delivery_items"]++
		s.out.IDMap = append(s.out.IDMap, IDEntry{"delivery_note_items", -rl, int64(n + 100), nid})
		s.emit("delivery_note_items", row.Row{"id": nid, "delivery_id": id, "product_id": k.p, "variant_id": k.v, "location_id": k.l,
			"uom": uom, "uom_factor": ratStr(factor), "qty": ratStr(new(big.Rat).Quo(sums[k], factor)), "qty_base": ratStr(sums[k]), "reservation_key": nil})
	}
}

// deliveryDate is the revamp's delivery_date: the WIB calendar date at 00:00,
// naive (the delivery module reads its first 10 characters as a Jakarta day,
// movedAt / posting queue / list filters). Never a UTC instant, which would
// shift every SJ one day back.
func deliveryDate(t time.Time) string { return naiveDateTime(wibDate(t)) }
