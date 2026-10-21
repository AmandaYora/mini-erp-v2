package transform

import (
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/shared/money"
)

type posKey struct{ branch, product, variant, loc int64 }

type mov struct {
	legacy  int64
	pos     posKey
	dir     string
	mtype   string
	qty     *big.Rat
	refType string
	refID   any
	when    time.Time
	notes   string
	by      any
	merged  int64 // extra rows folded into this one
	newID   int64
	before  *big.Rat
	after   *big.Rat
	unit    any
}

type uniqKey struct {
	refType string
	refID   int64
	pos     posKey
	dir     string
}

// refFor maps a legacy movement reference to the revamp (ref_type, ref_id).
func (s *state) refFor(m row.Row, pos posKey, dir string, branch int64) (string, any, bool) {
	rid := m.Int("id_reference")
	has := m.Has("id_reference")
	switch t := m.Str("reference_type"); t {
	case "delivery_note":
		id, ok := s.dnOfLegacy[rid]
		return "delivery", id, ok
	case "goods_receipt":
		id, ok := s.grOfLegacy[rid]
		return "goods_receipt", id, ok
	case "order":
		id, ok := s.directSJ[rid]
		return "delivery", id, ok
	case "stock_transfer", "transfer":
		if !has {
			id, ok := s.orphanTransfer[m.Int("id_inventory_movement")]
			return "transfer", id, ok
		}
		if id, ok := s.transferOf[transferKey{rid, pos.product, pos.variant, pos.loc, dir}]; ok {
			return "transfer", id, true
		}
		id, ok := s.transferFirst[rid]
		return "transfer", id, ok
	case "opening_stock":
		return "opening", branch, true
	case "manual_adjustment":
		return "adjustment", nil, true
	case "damaged_stock", "damaged_restore":
		return "damaged", nil, true
	case "damaged_write_off":
		return "write_off", nil, true
	case "sales_return":
		id, ok := s.srNew[rid]
		return "sales_return", id, ok
	case "sales_return_exchange":
		id, ok := s.replSJ[rid]
		return "delivery", id, ok
	case "purchase_return":
		id, ok := s.prNew[rid]
		return "purchase_return", id, ok
	case "":
		_ = has
		return "", nil, false
	default:
		return t, nil, false
	}
}

// moveRank orders legacy movements of the same instant: stock that arrives
// (opening stock, then adjustments/stock counts, then other ins) before stock
// that leaves, so an opening imported in the same second as the first sale
// does not leave an interim negative balance on the stock card.
func moveRank(m row.Row) int {
	switch {
	case m.Str("reference_type") == "opening_stock":
		return 0
	case m.Str("movement_type") == "adjustment" || m.Str("reference_type") == "manual_adjustment":
		return 1
	case m.Str("movement_type") == "in":
		return 2
	}
	return 3
}

func (s *state) stock() error {
	// Deterministic order: moved_at, then the same-instant rank, then the
	// legacy id. The running balances (stock card) are recomputed in this
	// order; legacy's own balance_before/after are kept in the archive.
	legacy := sortedBy(s.raw["inventory_movements"], "id_inventory_movement")
	sort.SliceStable(legacy, func(i, j int) bool {
		a, _ := legacy[i].Time("moved_at")
		b, _ := legacy[j].Time("moved_at")
		if !a.Equal(b) {
			return a.Before(b)
		}
		return moveRank(legacy[i]) < moveRank(legacy[j])
	})
	legacyRow := map[int64]row.Row{}
	for _, m := range legacy {
		legacyRow[m.Int("id_inventory_movement")] = m
	}
	var movs []*mov
	byUniq := map[uniqKey]*mov{}
	mergedInto := map[int64]*mov{}
	for _, m := range legacy {
		lid := m.Int("id_inventory_movement")
		br, ok1 := s.id("branches", m.Int("id_branch"))
		p, ok2 := s.id("products", m.Int("id_product"))
		v, ok3 := s.id("product_variants", m.Int("id_product_variant"))
		l, ok4 := s.id("stock_locations", m.Int("id_stock_location"))
		if !ok1 || !ok2 || !ok3 || !ok4 {
			s.find(report.Blocker, "MOV_REFS", "inventory_movements", lid, "cabang/produk/varian/lokasi mutasi tidak termigrasi")
			continue
		}
		pos := posKey{br, p, v, l}
		qty := m.Rat("quantity")
		var dir, mtype string
		switch m.Str("movement_type") {
		case "in":
			dir, mtype = "in", "in"
		case "out":
			dir, mtype = "out", "out"
		case "adjustment":
			// A legacy adjustment OVERWRITES the balance (stock opname, BR-07):
			// its quantity is the new count, not the change. The movement is
			// the difference, which is also what legacy costed.
			mtype = "adjustment"
			dir = "in"
			if m.Rat("balance_after").Cmp(m.Rat("balance_before")) < 0 {
				dir = "out"
			}
			qty = new(big.Rat).Sub(m.Rat("balance_after"), m.Rat("balance_before"))
		default:
			s.find(report.Blocker, "MOV_TYPE", "inventory_movements", lid, "movement_type %q tidak dikenal", m.Str("movement_type"))
			continue
		}
		rt, rid, ok := s.refFor(m, pos, dir, br)
		if !ok {
			code := "MOV_REF"
			if rt == "" {
				code = "MOV_REF_NULL"
			}
			s.find(report.Blocker, code, "inventory_movements", lid, "referensi mutasi %q/%s tidak dapat dipetakan", m.Str("reference_type"), m.Str("id_reference"))
			if rt == "" {
				rt = "adjustment"
			}
		}
		when, _ := m.Time("moved_at")
		x := &mov{legacy: lid, pos: pos, dir: dir, mtype: mtype, qty: new(big.Rat).Abs(qty), refType: rt, refID: rid, when: when,
			notes: s.trunc("stock_movements", lid, "notes", m.Str("reason_text"), 255), by: s.userID(m.Int("id_moved_by"))}
		if rid != nil {
			k := uniqKey{rt, rid.(int64), pos, dir}
			if prev, dup := byUniq[k]; dup {
				prev.qty.Add(prev.qty, x.qty)
				prev.merged++
				mergedInto[lid] = prev
				s.out.Explain["movements_merged_extra"]++
				s.find(report.Info, "MOV_MERGE", "inventory_movements", lid, "digabung ke mutasi %d (kunci unik idempotensi)", prev.legacy)
				continue
			}
			byUniq[k] = x
		}
		movs = append(movs, x)
	}
	for _, x := range movs {
		x.newID = s.alloc("stock_movements", x.legacy)
	}
	// Sorted: map order would make id_map.jsonl differ between runs.
	merged := make([]int64, 0, len(mergedInto))
	for lid := range mergedInto {
		merged = append(merged, lid)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i] < merged[j] })
	for _, lid := range merged {
		s.alias("stock_movements", lid, mergedInto[lid].newID)
	}

	// running balances per position
	run := map[posKey]*big.Rat{}
	lastWhen := map[posKey]time.Time{}
	var negatives, differ int64
	for _, x := range movs {
		cur := run[x.pos]
		if cur == nil {
			cur = new(big.Rat)
		}
		x.before = new(big.Rat).Set(cur)
		if x.dir == "in" {
			cur = new(big.Rat).Add(cur, x.qty)
		} else {
			cur = new(big.Rat).Sub(cur, x.qty)
		}
		x.after = cur
		run[x.pos] = cur
		lastWhen[x.pos] = x.when
		if cur.Sign() < 0 {
			negatives++
		}
		if l := legacyRow[x.legacy]; x.merged == 0 && l != nil && (l.Rat("balance_before").Cmp(x.before) != 0 || l.Rat("balance_after").Cmp(x.after) != 0) {
			differ++
		}
	}
	s.out.Explain["stock_interim_negative"] = negatives
	s.out.Explain["stock_balance_differs_legacy"] = differ
	s.find(report.Info, "STOCK_CARD_RECOMPUTED", "inventory_movements", 0,
		"saldo kartu stok dihitung ulang urut waktu mutasi (seketika: stok awal, penyesuaian, masuk, lalu keluar; lalu id legacy): %d mutasi bersaldo sesudah negatif sementara, %d mutasi bersaldo berbeda dari balance_before/after legacy (saldo legacy diarsip di archive/raw/inventory_movements)",
		negatives, differ)

	// legacy balances, merged per target position
	legacyBal := map[posKey]*big.Rat{}
	balUpdated := map[posKey]time.Time{}
	balLegacy := map[posKey]int64{}
	for _, b := range sortedBy(s.raw["inventory_balances"], "id_inventory_balance") {
		br, ok1 := s.id("branches", b.Int("id_branch"))
		p, ok2 := s.id("products", b.Int("id_product"))
		v, ok3 := s.id("product_variants", b.Int("id_product_variant"))
		l, ok4 := s.id("stock_locations", b.Int("id_stock_location"))
		if !ok1 || !ok2 || !ok3 || !ok4 {
			s.find(report.Blocker, "BAL_REFS", "inventory_balances", b.Int("id_inventory_balance"), "saldo menunjuk data yang tidak termigrasi")
			continue
		}
		k := posKey{br, p, v, l}
		if legacyBal[k] == nil {
			legacyBal[k] = new(big.Rat)
			balLegacy[k] = b.Int("id_inventory_balance")
		}
		legacyBal[k].Add(legacyBal[k], b.Rat("on_hand_qty"))
		if t, ok := b.LocalTime("updated_at"); ok && t.After(balUpdated[k]) {
			balUpdated[k] = t
		}
	}
	keys := map[posKey]bool{}
	for k := range run {
		keys[k] = true
	}
	for k := range legacyBal {
		keys[k] = true
	}
	sorted := make([]posKey, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.branch != b.branch {
			return a.branch < b.branch
		}
		if a.product != b.product {
			return a.product < b.product
		}
		if a.variant != b.variant {
			return a.variant < b.variant
		}
		return a.loc < b.loc
	})
	// correction movements: invariant "balance changes always pair with a movement"
	var corrections []*mov
	for _, k := range sorted {
		end := run[k]
		if end == nil {
			end = new(big.Rat)
		}
		want := legacyBal[k]
		if want == nil {
			want = new(big.Rat)
		}
		diff := new(big.Rat).Sub(want, end)
		if diff.Sign() == 0 {
			continue
		}
		dir := "in"
		if diff.Sign() < 0 {
			dir = "out"
		}
		when := lastWhen[k]
		if when.IsZero() {
			when = s.epoch()
		}
		c := &mov{legacy: balLegacy[k], pos: k, dir: dir, mtype: "adjustment", qty: new(big.Rat).Abs(diff), refType: "adjustment", refID: nil,
			when: when, notes: "Koreksi migrasi", before: new(big.Rat).Set(end), after: new(big.Rat).Set(want)}
		c.newID = s.allocSub("stock_movements", balLegacy[k], 9)
		corrections = append(corrections, c)
		s.out.Explain["correction_movements"]++
		s.find(report.Warn, "STOCK_DRIFT", "inventory_balances", balLegacy[k], "saldo %s != Σ mutasi %s: mutasi koreksi %s ditambahkan", want.FloatString(6), end.FloatString(6), diff.FloatString(6))
		run[k] = want
	}
	allMovs := append(append([]*mov(nil), movs...), corrections...)

	s.costLedger(allMovs)

	movRows := map[int64]row.Row{}
	for _, x := range allMovs {
		notes := any(nil)
		if x.notes != "" {
			notes = x.notes
		}
		if x.merged > 0 {
			notes = "gabungan " + itoa(x.merged+1) + " mutasi"
		}
		var ref any
		if x.refID != nil {
			ref = x.refID
		}
		r := row.Row{
			"id": x.newID, "branch_id": x.pos.branch, "product_id": x.pos.product, "variant_id": x.pos.variant, "location_id": x.pos.loc,
			"direction": x.dir, "movement_type": x.mtype, "qty_base": ratStr(x.qty), "balance_before": ratStr(x.before), "balance_after": ratStr(x.after),
			"unit_cost": x.unit, "ref_type": x.refType, "ref_id": ref, "notes": notes, "created_by": x.by,
			"created_at": fmtUTC(x.when), "moved_at": fmtUTC(x.when),
		}
		movRows[x.newID] = r
	}
	ids := make([]int64, 0, len(movRows))
	for id := range movRows {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var maxID int64
	for _, id := range ids {
		s.emit("stock_movements", movRows[id])
		maxID = id
	}
	for _, k := range sorted {
		on := legacyBal[k]
		if on == nil {
			on = new(big.Rat)
		}
		upd := balUpdated[k]
		if upd.IsZero() {
			upd = lastWhen[k]
		}
		if upd.IsZero() {
			upd = s.epoch()
		}
		s.emit("stock_balances", row.Row{"id": s.nextID("stock_balances"), "branch_id": k.branch, "product_id": k.product, "variant_id": k.variant,
			"location_id": k.loc, "on_hand": ratStr(on), "reserved": ratStr(new(big.Rat)), "updated_at": fmtUTC(upd)})
	}
	if maxID > 0 {
		s.cutoverFor(docStockMovement, maxID)
	}
	return nil
}

// crow is one revamp cost-ledger row being built (total unsigned, in rupiah).
type crow struct {
	mv     *mov
	qty    *big.Rat
	total  int64
	real   bool
	date   time.Time
	branch int64
}

// openingPool is the legacy finance opening of one (branch, product).
type openingPool struct {
	qty    *big.Rat
	total  int64
	legacy []int64
}

// costLedger migrates the HPP ledger (design V10). The revamp cost cursor is
// MAX(stock_movement_id) per branch, so every migrated movement below that
// cursor must carry a cost row; missing ones get an estimated row.
func (s *state) costLedger(movs []*mov) {
	byNew := map[int64]*mov{}
	for _, x := range movs {
		byNew[x.newID] = x
	}
	rows := map[int64]*crow{}
	openings := map[[2]int64]*openingPool{}
	for _, c := range sortedBy(s.raw["finance_inventory_cost_movements"], "id_finance_inventory_cost_movement") {
		lid := c.Int("id_finance_inventory_cost_movement")
		mid, ok := c.NInt("id_inventory_movement")
		if !ok {
			total := s.money.R("finance_inventory_cost_movements", lid, "total_cost", c, "total_cost_amount")
			qty := new(big.Rat).Abs(c.Rat("quantity_in_base_uom"))
			switch {
			case c.Str("movement_type") == "opening":
				br, ok1 := s.id("branches", c.Int("id_branch"))
				p, ok2 := s.id("products", c.Int("id_product"))
				if !ok1 || !ok2 {
					s.find(report.Blocker, "COST_OPENING_UNMATCHED", "finance_inventory_cost_movements", lid, "cabang/produk saldo awal biaya tidak termigrasi")
					s.out.Explain["cost_rows_unmigrated"]++
					continue
				}
				k := [2]int64{br, p}
				if openings[k] == nil {
					openings[k] = &openingPool{qty: new(big.Rat)}
				}
				o := openings[k]
				o.qty.Add(o.qty, qty)
				o.total += abs64(total)
				o.legacy = append(o.legacy, lid)
			case qty.Sign() == 0 && total == 0:
				s.find(report.Info, "COST_NOOP", "finance_inventory_cost_movements", lid, "baris biaya tanpa mutasi bernilai nol (%s) tidak dimigrasi", c.Str("source_type"))
				s.out.Explain["cost_rows_unmigrated"]++
			default:
				s.find(report.Warn, "COST_NO_MOVEMENT", "finance_inventory_cost_movements", lid, "baris biaya %s tanpa mutasi tidak dimigrasi", c.Str("movement_type"))
				s.out.Explain["cost_rows_unmigrated"]++
			}
			continue
		}
		nid, ok := s.id("stock_movements", mid)
		x := byNew[nid]
		if !ok || x == nil {
			s.find(report.Warn, "COST_MOVEMENT_UNMAPPED", "finance_inventory_cost_movements", lid, "mutasi %d tidak termigrasi", mid)
			s.out.Explain["cost_rows_unmigrated"]++
			continue
		}
		total := s.money.R("finance_inventory_cost_movements", lid, "total_cost", c, "total_cost_amount")
		qty := new(big.Rat).Abs(c.Rat("quantity_in_base_uom"))
		date, _ := c.Time("movement_date")
		if r := rows[nid]; r != nil {
			r.qty.Add(r.qty, qty)
			r.total += abs64(total)
			s.out.Explain["cost_rows_merged_extra"]++
			continue
		}
		rows[nid] = &crow{mv: x, qty: qty, total: abs64(total), real: true, date: date, branch: x.pos.branch}
	}
	// Opening value (Q-HPP-2). Legacy posts the finance opening as cost rows
	// WITHOUT a movement, one per (branch, product) — its cost granularity —
	// while the stock opening import already wrote the quantity as
	// opening_stock movements (per variant/location). Both describe the same
	// goods, so no synthetic stock movement is made (that would count the
	// opening twice): the value is spread over the still-uncosted opening
	// movements of the product, pro rata qty, as real cost rows. This only
	// holds when the quantities agree exactly; otherwise legacy books and
	// stock disagree and the owner decides (blocker).
	byProduct := map[[2]int64][]*mov{}
	for _, x := range movs {
		if x.refType == "opening" && rows[x.newID] == nil {
			k := [2]int64{x.pos.branch, x.pos.product}
			byProduct[k] = append(byProduct[k], x)
		}
	}
	pools := make([][2]int64, 0, len(openings))
	for k := range openings {
		pools = append(pools, k)
	}
	sort.Slice(pools, func(i, j int) bool {
		if pools[i][0] != pools[j][0] {
			return pools[i][0] < pools[j][0]
		}
		return pools[i][1] < pools[j][1]
	})
	for _, k := range pools {
		o := openings[k]
		cands := byProduct[k]
		sort.Slice(cands, func(i, j int) bool { return cands[i].newID < cands[j].newID })
		held := new(big.Rat)
		qtys := make([]*big.Rat, len(cands))
		for i, x := range cands {
			held.Add(held, x.qty)
			qtys[i] = x.qty
		}
		if len(cands) == 0 || held.Cmp(o.qty) != 0 {
			for _, lid := range o.legacy {
				s.find(report.Blocker, "COST_OPENING_UNMATCHED", "finance_inventory_cost_movements", lid,
					"qty saldo awal biaya %s != Σ mutasi opening_stock produk %s (%d mutasi): perlu keputusan pemilik (Q-HPP-2)",
					o.qty.FloatString(6), held.FloatString(6), len(cands))
				s.out.Explain["cost_rows_unmigrated"]++
			}
			continue
		}
		for i, share := range SplitByQty(o.total, qtys) {
			x := cands[i]
			rows[x.newID] = &crow{mv: x, qty: new(big.Rat).Set(x.qty), total: share, real: true, date: x.when, branch: x.pos.branch}
		}
		s.out.Explain["opening_cost_split_extra"] += int64(len(cands) - len(o.legacy))
		for _, lid := range o.legacy {
			s.find(report.Info, "COST_OPENING_SPLIT", "finance_inventory_cost_movements", lid,
				"nilai saldo awal Rp%d produk dibagi ke %d mutasi opening_stock (qty %s)", o.total, len(cands), o.qty.FloatString(6))
		}
	}
	// running position state, in movement order, to estimate missing rows
	// with the revamp's own chain (costOf, finance/application/costing.go):
	// a receipt at its PO tax base, any other in or out at the running
	// average, then the last inflow cost, then the catalog price per base unit.
	type st struct {
		qty    *big.Rat
		value  int64
		lastIn *big.Rat
	}
	running := map[[3]int64]*st{}
	receipt := s.receiptCosts()
	// Cost already relieved per (document, product, variant): a legacy SJ
	// cancellation brings goods back with no cost row, and it reverses exactly
	// what the SJ took out.
	type docKey struct {
		ref                  string
		id, product, variant int64
	}
	type relieved struct {
		qty   *big.Rat
		total int64
		real  bool // costed (and so journalled) by legacy
	}
	outOf := map[docKey]*relieved{}
	dnCorrected := map[int64]bool{}
	relieve := func(x *mov, r *crow) {
		if x.dir != "out" || x.refID == nil {
			return
		}
		dk := docKey{x.refType, x.refID.(int64), x.pos.product, x.pos.variant}
		if outOf[dk] == nil {
			outOf[dk] = &relieved{qty: new(big.Rat)}
		}
		outOf[dk].qty.Add(outOf[dk].qty, r.qty)
		outOf[dk].total += r.total
		outOf[dk].real = outOf[dk].real || r.real
	}
	// Legacy dates an SJ's outs by the delivery DATE (00:00) but its
	// cancellation by the clock, so the cancellation can sort first: take the
	// real (legacy) out costs up front.
	for _, x := range movs {
		if r := rows[x.newID]; r != nil && r.real {
			relieve(x, r)
		}
	}
	// Legacy's per-product state is the value each position holds after its
	// last legacy-costed movement; it lands there, so estimates for documents
	// legacy never costed (they come later) see the same average the
	// revamp's Sync would use, never the pre-restatement one.
	baseline := s.costBaseline(rows)
	delta := map[[3]int64]int64{}
	for _, b := range baseline {
		delta[[3]int64{b.branch, b.product, b.variant}] += b.total
	}
	lastReal := map[[3]int64]int64{}
	for id, r := range rows {
		if k := [3]int64{r.branch, r.mv.pos.product, r.mv.pos.variant}; r.real && id > lastReal[k] {
			lastReal[k] = id
		}
	}
	applied := map[[3]int64]bool{}
	ordered := append([]*mov(nil), movs...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].newID < ordered[j].newID })
	for _, x := range ordered {
		if s.productRow[x.pos.product].Int("stock_tracked") == 0 {
			continue
		}
		k := [3]int64{x.pos.branch, x.pos.product, x.pos.variant}
		cur := running[k]
		if cur == nil {
			cur = &st{qty: new(big.Rat)}
			running[k] = cur
		}
		r := rows[x.newID]
		estimated := r == nil
		if r == nil {
			// Every movement is costed here, the corrections included: the
			// migration must know each value to keep books = ledger (V-27); the
			// revamp's Sync starts after the last migrated movement.
			var unit *big.Rat
			switch {
			case x.dir == "in" && x.refType == "goods_receipt":
				unit = receipt(x)
			case x.dir == "in" && x.refType == "delivery" && x.refID != nil && outOf[docKey{x.refType, x.refID.(int64), x.pos.product, x.pos.variant}] != nil:
				o := outOf[docKey{x.refType, x.refID.(int64), x.pos.product, x.pos.variant}]
				unit = new(big.Rat).Quo(new(big.Rat).SetInt64(o.total), o.qty)
				v := RoundHalfAway(new(big.Rat).Mul(x.qty, unit))
				if o.real {
					// Legacy journalled the SJ's COGS and never reversed it: the
					// goods are back but its books still count them sold. A
					// migration correction journal puts them back (ADR-0018 D11).
					s.migCorr = append(s.migCorr, migCorrection{kind: corrDNCancel, branch: x.pos.branch, doc: x.refID.(int64),
						product: x.pos.product, variant: x.pos.variant, movement: x.newID, legacyMovement: x.legacy, amount: v})
					dnCorrected[x.newID] = true
					s.out.Explain["dn_cancel_value"] += v
					s.find(report.Warn, "DN_CANCEL_COGS", "inventory_movements", x.legacy,
						"pembatalan SJ: barang kembali senilai Rp%d (biaya keluar SJ itu); HPP-nya tidak pernah dibalik di legacy: jurnal koreksi migrasi Dr Persediaan / Cr HPP", v)
				} else {
					s.find(report.Info, "DN_CANCEL_UNCOSTED", "inventory_movements", x.legacy,
						"pembatalan SJ yang keluarnya tidak pernah berbiaya/dijurnal di legacy: masuk Rp%d menetralkan estimasi keluarnya, tanpa koreksi", v)
				}
			case x.refType != "opening" && cur.qty.Sign() > 0 && cur.value > 0:
				// Positive average only: legacy costs per PRODUCT, so one variant's
				// share of it can run negative (COST_VARIANT_NEGATIVE); a negative
				// cost would turn an in into an out of value.
				unit = new(big.Rat).Quo(new(big.Rat).SetInt64(cur.value), cur.qty)
			case x.refType != "opening" && cur.lastIn != nil:
				unit = cur.lastIn
			default:
				unit = s.catalogBaseCost(x.pos.product)
			}
			total := RoundHalfAway(new(big.Rat).Mul(x.qty, unit))
			r = &crow{mv: x, qty: new(big.Rat).Set(x.qty), total: total, real: false, date: x.when, branch: x.pos.branch}
			rows[x.newID] = r
			s.out.Explain["estimated_cost_rows"]++
			s.find(report.Info, "COST_ESTIMATED", "stock_movements", x.legacy, "baris biaya estimasi dibuat untuk mutasi %d (legacy tidak membiayainya)", x.newID)
		}
		if x.dir == "in" {
			cur.qty = new(big.Rat).Add(cur.qty, r.qty)
			cur.value += r.total
			if r.qty.Sign() > 0 && r.total > 0 {
				cur.lastIn = new(big.Rat).Quo(new(big.Rat).SetInt64(r.total), r.qty)
			}
		} else {
			cur.qty = new(big.Rat).Sub(cur.qty, r.qty)
			cur.value -= r.total
			if estimated {
				relieve(x, r)
			}
		}
		if lastReal[k] == x.newID && !applied[k] {
			cur.value += delta[k]
			applied[k] = true
		}
	}
	for k, d := range delta {
		if !applied[k] {
			if running[k] == nil {
				running[k] = &st{qty: new(big.Rat)}
			}
			running[k].value += d
		}
	}
	s.unjournalled(rows, dnCorrected)
	// A position can still end with a negative value: legacy's state (its
	// books) sits below what its own rows say for a few products, and an
	// estimated out of a document legacy never costed then takes it under
	// zero. The revamp's readiness blocks on it (negative_inventory); the fix
	// is an operator restatement (costrebuild), not a silent rewrite.
	keys := make([][3]int64, 0, len(running))
	for k, cur := range running {
		if cur.value < 0 && cur.qty.Sign() >= 0 {
			keys = append(keys, k)
		}
	}
	sortKeys3(keys)
	for _, k := range keys {
		cur := running[k]
		s.out.Explain["cost_variant_negative"]++
		s.find(report.Warn, "COST_VARIANT_NEGATIVE", "product_variants", 0, "produk %s varian %d: qty %s bernilai Rp%d di ledger HPP (keadaan biaya legacy + estimasi dokumen yang belum diposting): luruskan dengan costrebuild",
			s.productRow[k[1]].Str("code"), k[2], cur.qty.FloatString(6), cur.value)
	}
	ids := make([]int64, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		r := rows[id]
		var unit any
		if r.qty.Sign() > 0 {
			u := new(big.Rat).Quo(new(big.Rat).SetInt64(r.total), r.qty)
			unit = u.FloatString(4)
			r.mv.unit = unit
		}
		date := r.date
		if date.IsZero() {
			date = r.mv.when
		}
		var ref any
		if r.mv.refID != nil {
			ref = r.mv.refID
		}
		// The revamp ledger stores total_cost SIGNED (an out is negative,
		// InsertCostRow) and every position is SUM(total_cost); the unit stays
		// positive.
		total := r.total
		if r.mv.dir == "out" {
			total = -total
		}
		s.emit("finance_inventory_cost_movements", row.Row{
			"id": s.nextID("finance_inventory_cost_movements"), "branch_id": r.branch, "product_id": r.mv.pos.product, "variant_id": r.mv.pos.variant,
			"stock_movement_id": id, "ref_type": r.mv.refType, "ref_id": ref, "direction": r.mv.dir, "qty_base": ratStr(r.qty),
			"unit_cost": unit, "total_cost": itoa(total) + ".00", "is_estimated": b2i(!r.real), "created_at": fmtUTC(date), "moved_at": fmtUTC(date),
		})
	}
	for i, b := range baseline {
		dir := "in"
		if b.total < 0 {
			dir = "out"
		}
		s.emit("finance_inventory_cost_movements", row.Row{
			"id": s.nextID("finance_inventory_cost_movements"), "branch_id": b.branch, "product_id": b.product, "variant_id": b.variant,
			"stock_movement_id": syntheticCostBase + int64(i), "ref_type": refOpeningCost, "ref_id": nil, "direction": dir, "qty_base": ratStr(new(big.Rat)),
			"unit_cost": nil, "total_cost": itoa(b.total) + ".00", "is_estimated": 0, "created_at": fmtUTC(b.date), "moved_at": fmtUTC(b.date),
		})
	}
}

// The revamp's cost-only ledger rows (finance/infrastructure/opening_cost.go):
// qty 0, value only, ids from SyntheticCostBase so the costing cursor never
// sees them.
const (
	syntheticCostBase int64 = 1 << 62
	refOpeningCost          = "opening_cost"
)

type baselineRow struct {
	branch, product, variant, total int64
	date                            time.Time
}

// costBaseline translates legacy's per-PRODUCT cost into the revamp's
// per-variant ledger. Legacy's authority is finance_inventory_cost_states
// (it equals its Persediaan account); corrections such as the 2026-08-13
// pre-ledger baseline moved that state and the books without cost rows, and
// one average serves every variant, so a variant's own rows can drift (even
// negative) while the product is right. Per (branch, product) the state value
// is spread over the held variants by qty and cost-only rows (qty 0) carry
// each variant from its migrated rows to that share: Σ per product = state.
// Shares follow the qty legacy's own rows hold per variant (estimated rows
// of documents legacy never costed then move each variant from there).
func (s *state) costBaseline(rows map[int64]*crow) []baselineRow {
	have := map[[3]int64]int64{}
	held := map[[3]int64]*big.Rat{}
	for _, r := range rows {
		if !r.real {
			continue
		}
		k := [3]int64{r.branch, r.mv.pos.product, r.mv.pos.variant}
		if held[k] == nil {
			held[k] = new(big.Rat)
		}
		v := r.total
		if r.mv.dir == "out" {
			v = -v
			held[k].Sub(held[k], r.qty)
		} else {
			held[k].Add(held[k], r.qty)
		}
		have[k] += v
	}
	variants := map[[2]int64][][3]int64{}
	add := func(k [3]int64) {
		pk := [2]int64{k[0], k[1]}
		for _, e := range variants[pk] {
			if e == k {
				return
			}
		}
		variants[pk] = append(variants[pk], k)
	}
	for k := range have {
		add(k)
	}
	for k, q := range held {
		if q.Sign() > 0 {
			add(k)
		}
	}
	var out []baselineRow
	for _, c := range sortedBy(s.raw["finance_inventory_cost_states"], "id_finance_inventory_cost_state") {
		lid := c.Int("id_finance_inventory_cost_state")
		br, ok1 := s.id("branches", c.Int("id_branch"))
		p, ok2 := s.id("products", c.Int("id_product"))
		if !ok1 || !ok2 {
			s.find(report.Blocker, "COST_STATE_REFS", "finance_inventory_cost_states", lid, "cabang/produk keadaan biaya tidak termigrasi")
			continue
		}
		want := s.money.R("finance_inventory_cost_states", lid, "inventory_value", c, "inventory_value")
		keys := variants[[2]int64{br, p}]
		sortKeys3(keys)
		var rowsValue int64
		var heldKeys [][3]int64
		var qtys []*big.Rat
		for _, k := range keys {
			rowsValue += have[k]
			if q := held[k]; q != nil && q.Sign() > 0 {
				heldKeys = append(heldKeys, k)
				qtys = append(qtys, q)
			}
		}
		target := map[[3]int64]int64{}
		if len(heldKeys) == 0 && want != 0 {
			// Inventory without units cannot carry value: the ledger goes to
			// zero and a migration correction writes the books down by the same
			// amount against the stock-difference account (ADR-0018 D11).
			v := s.defVariant[p]
			if len(keys) > 0 {
				v = keys[0][2]
			}
			s.migCorr = append(s.migCorr, migCorrection{kind: corrStateNoQty, branch: br, product: p, variant: v, legacyState: lid, amount: want})
			s.find(report.Warn, "COST_STATE_NO_QTY", "finance_inventory_cost_states", lid,
				"keadaan biaya legacy tanpa qty bernilai Rp%d: ledger dinolkan, jurnal koreksi migrasi Dr selisih persediaan / Cr Persediaan", want)
		} else {
			value := want
			if want <= 0 && rowsValue > 0 {
				// Units held at no value while legacy's own rows price them:
				// the state lost the value, the books with it. The ledger takes
				// the rows' value and a correction writes the books up by the
				// same amount (ADR-0018 D11).
				value = rowsValue
				s.migCorr = append(s.migCorr, migCorrection{kind: corrStateNoValue, branch: br, product: p, variant: heldKeys[0][2], legacyState: lid, amount: rowsValue - want})
				s.find(report.Warn, "COST_STATE_NO_VALUE", "finance_inventory_cost_states", lid,
					"keadaan biaya legacy ber-qty bernilai Rp%d padahal baris biayanya Rp%d: ditulis naik Rp%d, jurnal koreksi migrasi Dr Persediaan / Cr selisih persediaan", rowsValue-want, rowsValue, want)
			}
			for i, part := range SplitByQty(value, qtys) {
				target[heldKeys[i]] = part
			}
		}
		date, ok := c.LocalTime("updated_at")
		if !ok {
			date = s.epoch()
		}
		moved := 0
		for _, k := range keys {
			if d := target[k] - have[k]; d != 0 {
				out = append(out, baselineRow{branch: br, product: p, variant: k[2], total: d, date: date})
				s.out.Explain["baseline_cost_rows"]++
				moved++
			}
		}
		if want != rowsValue {
			// The cost-only row moves the unit cost the revamp will use
			// (state value / units the ledger holds) away from legacy's own
			// average when legacy's value and its rows disagree: the
			// accountant reviews every product that moves more than 5%.
			held := new(big.Rat)
			for _, q := range qtys {
				held.Add(held, q)
			}
			avg := c.Rat("average_cost")
			if !c.Has("average_cost") {
				avg = c.Rat("unit_cost")
			}
			sev := report.Info
			if held.Sign() > 0 && avg.Sign() > 0 {
				var value int64
				for _, k := range heldKeys {
					value += target[k]
				}
				unit := new(big.Rat).Quo(new(big.Rat).SetInt64(value), held)
				dev := new(big.Rat).Abs(new(big.Rat).Sub(unit, avg))
				if dev.Cmp(new(big.Rat).Mul(avg, big.NewRat(5, 100))) > 0 {
					sev = report.Warn
					delta := RoundHalfAway(new(big.Rat).Mul(new(big.Rat).Sub(unit, avg), held))
					s.out.Explain["cost_review_products"]++
					s.out.Explain["cost_review_abs_delta"] += abs64(delta)
					s.emit("cost_review", row.Row{"branch": s.branchCode[br], "product_code": s.productRow[p].Str("code"), "product_name": s.productRow[p].Str("name"),
						"legacy_avg_cost": avg.FloatString(2), "v2_unit_cost": unit.FloatString(2), "qty": held.FloatString(4),
						"legacy_value_at_avg": RoundHalfAway(new(big.Rat).Mul(avg, held)), "v2_value": value, "delta": delta, "legacy_state": lid})
				}
			}
			s.find(sev, "COST_BASELINE", "finance_inventory_cost_states", lid,
				"nilai keadaan biaya legacy Rp%d - Σ baris biaya Rp%d = Rp%d: ditambahkan sebagai baris biaya-saja (qty 0)", want, rowsValue, want-rowsValue)
		} else if moved > 0 {
			s.find(report.Info, "COST_VARIANT_REALIGN", "finance_inventory_cost_states", lid,
				"nilai produk Rp%d dibagi ulang ke %d varian berstok menurut qty (legacy: satu harga modal per produk)", want, len(heldKeys))
		}
	}
	if n := s.out.Explain["cost_review_products"]; n > 0 {
		s.find(report.Warn, "COST_REVIEW_LIST", "finance_inventory_cost_states", 0,
			"%d produk berharga modal revamp menyimpang >5%% dari average_cost legacy (Σ|selisih nilai| Rp%d): telaah akuntan, daftar di report/cost_baseline_review.csv",
			n, s.out.Explain["cost_review_abs_delta"])
	}
	return out
}

func sortKeys3(keys [][3]int64) {
	sort.Slice(keys, func(i, j int) bool {
		for n := 0; n < 3; n++ {
			if keys[i][n] != keys[j][n] {
				return keys[i][n] < keys[j][n]
			}
		}
		return false
	})
}

// catalogBaseCost is the catalog purchase price per base unit (the revamp's
// PurchaseBaseCostsByIDs: price / purchase_to_base_factor).
func (s *state) catalogBaseCost(product int64) *big.Rat {
	p := s.productRow[product]
	price := row.ParseRat(p.Str("purchase_price"))
	f := row.ParseRat(p.Str("purchase_to_base_factor"))
	if f.Sign() <= 0 {
		return price
	}
	return new(big.Rat).Quo(price, f)
}

// receiptCosts prices a goods-receipt movement like the revamp's
// receiptUnitCost: the first PO line of the product/variant, its tax BASE
// over its base qty (money.LineAmounts on the migrated line). Zero when the
// receipt or line cannot be found, as in the revamp.
func (s *state) receiptCosts() func(x *mov) *big.Rat {
	grPO := map[int64]int64{}
	for _, g := range s.out.Tables["goods_receipts"] {
		grPO[g.Int("id")] = g.Int("purchase_order_id")
	}
	po := map[int64]row.Row{}
	for _, o := range s.out.Tables["purchase_orders"] {
		po[o.Int("id")] = o
	}
	lines := map[int64][]row.Row{}
	for _, l := range s.out.Tables["purchase_order_items"] {
		lines[l.Int("order_id")] = append(lines[l.Int("order_id")], l)
	}
	ratF := func(r row.Row, col string) float64 { f, _ := r.Rat(col).Float64(); return f }
	return func(x *mov) *big.Rat {
		id, _ := x.refID.(int64)
		o, ok := po[grPO[id]]
		if !ok {
			return new(big.Rat)
		}
		for _, l := range lines[o.Int("id")] {
			if l.Int("product_id") != x.pos.product || l.Int("variant_id") != x.pos.variant || l.Rat("qty_base").Sign() <= 0 {
				continue
			}
			_, base, _, _ := money.LineAmounts(ratF(l, "qty"), l.Int("unit_price"), ratF(l, "discount_pct"), l.Int("discount_nominal"),
				o.Str("tax_type"), ratF(o, "tax_rate")).Split()
			return new(big.Rat).Quo(new(big.Rat).SetInt64(base), l.Rat("qty_base"))
		}
		return new(big.Rat)
	}
}

// unjournalled finds the estimated cost rows no revamp journal will ever
// cover and turns each into a migration correction (ADR-0018 D11), so the
// books follow the ledger. Pending documents are left alone: the revamp's
// posting queue journals them at these very values. Transfers move value
// between positions with no journal by design and are left alone too.
//
//   - an SJ's goods out (or back) whose document legacy never journalled
//     (posted only in part, archived, before the opening date): HPP;
//   - any other movement without a journal (migration drift corrections,
//     adjustments legacy left unposted or ignored, receipts/returns before
//     the opening date): the stock-difference account.
func (s *state) unjournalled(rows map[int64]*crow, done map[int64]bool) {
	mark := map[string]string{}
	for _, m := range s.out.Marks {
		mark[m.DocType+":"+strconv.FormatInt(m.DocID, 10)] = m.State + "/" + m.Reason
	}
	ids := make([]int64, 0, len(rows))
	for id, r := range rows {
		if !r.real && !done[id] && r.total != 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		r := rows[id]
		x := r.mv
		why := ""
		switch x.refType {
		case "transfer":
			continue
		case "delivery", "goods_receipt", "sales_return", "purchase_return":
			if x.refID == nil {
				why = "tanpa dokumen"
				break
			}
			st := mark[x.refType+":"+strconv.FormatInt(x.refID.(int64), 10)]
			if strings.HasPrefix(st, statePending+"/") {
				continue
			}
			why = "dokumen " + strings.TrimSuffix(st, "/")
		case "adjustment":
			why = "penyesuaian stok tanpa jurnal legacy"
			if x.notes == "Koreksi migrasi" {
				why = "koreksi saldo migrasi (STOCK_DRIFT)"
			}
		default:
			why = "mutasi " + x.refType + " tanpa jurnal legacy"
		}
		v := r.total
		if x.dir == "out" {
			v = -v
		}
		kind, code := corrStockUnposted, "STOCK_UNPOSTED"
		if x.refType == "delivery" {
			kind, code = corrCOGSUnposted, "COGS_UNPOSTED"
		}
		var doc int64
		if x.refID != nil {
			doc = x.refID.(int64)
		}
		s.migCorr = append(s.migCorr, migCorrection{kind: kind, branch: x.pos.branch, doc: doc, refType: x.refType, product: x.pos.product,
			variant: x.pos.variant, movement: id, legacyMovement: x.legacy, amount: v, reason: why})
		s.find(report.Warn, code, "inventory_movements", x.legacy,
			"nilai persediaan Rp%d (%s %s) tanpa jurnal yang akan datang: jurnal koreksi migrasi", v, x.refType, why)
	}
}
