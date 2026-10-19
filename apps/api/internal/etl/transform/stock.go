package transform

import (
	"math/big"
	"sort"
	"time"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
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

func (s *state) stock() error {
	legacy := sortedBy(s.raw["inventory_movements"], "id_inventory_movement")
	sort.SliceStable(legacy, func(i, j int) bool {
		a, _ := legacy[i].Time("moved_at")
		b, _ := legacy[j].Time("moved_at")
		return a.Before(b)
	})
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
			mtype = "adjustment"
			dir = "in"
			if m.Rat("balance_after").Cmp(m.Rat("balance_before")) < 0 {
				dir = "out"
			}
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
	}

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
		if t, ok := b.Time("updated_at"); ok && t.After(balUpdated[k]) {
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

// costLedger migrates the HPP ledger (design V10). The revamp cost cursor is
// MAX(stock_movement_id) per branch, so every migrated movement below that
// cursor must carry a cost row; missing ones get an estimated row.
func (s *state) costLedger(movs []*mov) {
	byNew := map[int64]*mov{}
	for _, x := range movs {
		byNew[x.newID] = x
	}
	type crow struct {
		mv     *mov
		qty    *big.Rat
		total  int64
		real   bool
		date   time.Time
		branch int64
	}
	rows := map[int64]*crow{}
	for _, c := range sortedBy(s.raw["finance_inventory_cost_movements"], "id_finance_inventory_cost_movement") {
		lid := c.Int("id_finance_inventory_cost_movement")
		mid, ok := c.NInt("id_inventory_movement")
		if !ok {
			s.find(report.Warn, "COST_NO_MOVEMENT", "finance_inventory_cost_movements", lid, "baris biaya tanpa mutasi tidak dimigrasi (TODO: mutasi pembukaan sintetis, Q-HPP-2)")
			s.out.Explain["cost_rows_unmigrated"]++
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
	maxCost := map[int64]int64{}
	for nid, r := range rows {
		if nid > maxCost[r.branch] {
			maxCost[r.branch] = nid
		}
	}
	// running position state, in movement order, to estimate missing rows
	type st struct {
		qty   *big.Rat
		value int64
	}
	running := map[[3]int64]*st{}
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
		if r == nil {
			if x.newID >= maxCost[x.pos.branch] {
				// Above the cursor (this includes the correction movements, which
				// always carry the highest ids): the revamp Sync prices them later.
				// Costing a correction here would push the cursor to the top and
				// force estimated rows under every movement of the branch.
				continue
			}
			unit := new(big.Rat)
			if x.dir == "out" && cur.qty.Sign() > 0 && cur.value > 0 {
				unit = new(big.Rat).Quo(new(big.Rat).SetInt64(cur.value), cur.qty)
			} else {
				unit = row.ParseRat(s.productRow[x.pos.product].Str("purchase_price"))
			}
			total := RoundHalfAway(new(big.Rat).Mul(x.qty, unit))
			r = &crow{mv: x, qty: new(big.Rat).Set(x.qty), total: total, real: false, date: x.when, branch: x.pos.branch}
			rows[x.newID] = r
			s.out.Explain["estimated_cost_rows"]++
			s.find(report.Info, "COST_ESTIMATED", "stock_movements", x.legacy, "baris biaya estimasi dibuat untuk mutasi %d (di bawah kursor HPP)", x.newID)
		}
		if x.dir == "in" {
			cur.qty = new(big.Rat).Add(cur.qty, r.qty)
			cur.value += r.total
		} else {
			cur.qty = new(big.Rat).Sub(cur.qty, r.qty)
			cur.value -= r.total
		}
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
		s.emit("finance_inventory_cost_movements", row.Row{
			"id": s.nextID("finance_inventory_cost_movements"), "branch_id": r.branch, "product_id": r.mv.pos.product, "variant_id": r.mv.pos.variant,
			"stock_movement_id": id, "ref_type": r.mv.refType, "ref_id": ref, "direction": r.mv.dir, "qty_base": ratStr(r.qty),
			"unit_cost": unit, "total_cost": itoa(r.total) + ".00", "is_estimated": b2i(!r.real), "created_at": fmtUTC(date), "moved_at": fmtUTC(date),
		})
	}
}
