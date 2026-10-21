package transform

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

// statusFromGroup is the design's status map (F5-4): an unknown group fails the
// transform instead of silently becoming a draft.
func statusFromGroup(group string) (string, error) {
	switch group {
	case "pending":
		return "draft", nil
	case "active":
		return "confirmed", nil
	case "completed":
		return "completed", nil
	case "cancelled":
		return "cancelled", nil
	}
	return "", fmt.Errorf("status_group %q tidak dikenal", group)
}

func (s *state) nextID(table string) int64 {
	s.next[table]++
	return s.next[table]
}

func taxType(r row.Row) string {
	switch {
	case !r.Bool("taxable"):
		return "none"
	case r.Bool("price_includes_tax"):
		return "include"
	}
	return "exclude"
}

// rate2 fits a DECIMAL(9,4) rate into the target DECIMAL(5,2), logging a loss.
func (s *state) rate2(entity string, id int64, v *big.Rat) string {
	out := v.FloatString(2)
	if row.ParseRat(out).Cmp(v) != 0 {
		s.log.Adjust(entity, id, "tax_rate", v.FloatString(4), out, "rate_scale")
	}
	return out
}

func (s *state) orders() error {
	defs := map[int64]string{}
	for _, r := range s.raw["order_status_definitions"] {
		defs[r.Int("id_order_status_definition")] = r.Str("status_group")
	}
	items := map[int64][]row.Row{}
	for _, it := range sortedBy(s.raw["order_items"], "id_order_item") {
		items[it.Int("id_order")] = append(items[it.Int("id_order")], it)
	}
	for _, list := range items {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Int("line_no") < list[j].Int("line_no") })
	}
	ords := append([]row.Row(nil), s.raw["orders"]...)
	sort.SliceStable(ords, func(i, j int) bool {
		a, _ := ords[i].Time("order_date")
		b, _ := ords[j].Time("order_date")
		if !a.Equal(b) {
			return a.Before(b)
		}
		return ords[i].Int("id_order") < ords[j].Int("id_order")
	})
	for _, o := range ords {
		lid := o.Int("id_order")
		kind := o.Str("order_kind")
		if kind != "sales" && kind != "purchase" {
			s.find(report.Blocker, "ORDER_KIND", "orders", lid, "order_kind %q tidak dikenal", kind)
			continue
		}
		branch, ok := s.id("branches", o.Int("id_branch"))
		if !ok {
			s.find(report.Blocker, "ORDER_BRANCH", "orders", lid, "cabang order tidak dikenal")
			continue
		}
		group, ok := defs[o.Int("id_current_status")]
		if !ok {
			return fmt.Errorf("order %d: status %d tidak ada di order_status_definitions", lid, o.Int("id_current_status"))
		}
		status, err := statusFromGroup(group)
		if err != nil {
			return fmt.Errorf("order %d: %w", lid, err)
		}
		if o.Has("archived_at") {
			status = "cancelled"
		}
		table := kind + "_orders"
		itemTable := kind + "_order_items"
		s.orderKind[lid] = kind
		s.orderRow[lid] = o
		id := s.alloc(table, lid)

		var party int64
		if p, ok := o.NInt("id_related_party"); ok {
			pid, found := s.id("parties", p)
			if !found {
				s.find(report.Blocker, "ORDER_PARTY", "orders", lid, "pihak %d tidak termigrasi", p)
			}
			party = pid
		} else if kind == "purchase" {
			s.find(report.Blocker, "ORDER_PARTY", "orders", lid, "pembelian tanpa pemasok")
		}

		terms := o.Str("payment_terms")
		switch terms {
		case "net", "cod":
		case "prepaid":
			terms = "cod"
			s.find(report.Info, "ORDER_PREPAID", "orders", lid, "termin prepaid dipetakan ke cod")
		default:
			terms = "cod"
			s.find(report.Warn, "ORDER_TERMS", "orders", lid, "termin %q tidak dikenal, dianggap cod", o.Str("payment_terms"))
		}
		odate, _ := o.Time("order_date")
		var due any
		if t, ok := o.Time("due_date"); ok {
			due = fmtWIB(t)
		} else if terms == "net" {
			days := int(o.Int("payment_term_days"))
			due = fmtWIB(odate.AddDate(0, 0, days))
			s.find(report.Warn, "ORDER_NET_NO_DUE", "orders", lid, "termin net tanpa jatuh tempo: order_date + %d hari", days)
		}
		tt := taxType(o)
		grand := s.money.R(table, lid, "grand_total", o, "total_amount")
		tax := s.money.R(table, lid, "tax_total", o, "tax_amount")
		disc := s.money.R(table, lid, "discount_total", o, "discount_amount")
		subtotal := grand
		if tt == "exclude" {
			subtotal = grand - tax
		}
		if tt == "none" {
			tax = 0
		}

		// items
		var itemRows []row.Row
		var sum int64
		for i, it := range items[lid] {
			ilid := it.Int("id_order_item")
			pid, ok1 := s.id("products", it.Int("id_product"))
			vid, ok2 := s.id("product_variants", it.Int("id_product_variant"))
			if !ok1 || !ok2 {
				s.find(report.Blocker, "ITEM_PRODUCT", "order_items", ilid, "produk/varian item tidak termigrasi")
				continue
			}
			qty := it.Rat("quantity")
			gross := new(big.Rat).Mul(qty, it.Rat("unit_price_before_discount"))
			nominal := RoundHalfAway(gross) - s.money.Round(itemTable, ilid, "line_total_before_tax", it.Rat("line_total_before_tax"))
			lineTotal := s.money.R(itemTable, ilid, "line_total", it, "line_total_after_tax")
			sum += lineTotal
			itype := "barang"
			if s.prodType[pid] == "jasa" {
				itype = "jasa"
			}
			ir := row.Row{
				"id": s.nextID(itemTable), "order_id": id, "product_id": pid, "variant_id": vid,
				"product_code": s.trunc(itemTable, ilid, "product_code", it.Str("product_code_snapshot"), 50),
				"product_name": s.trunc(itemTable, ilid, "product_name", it.Str("product_name_snapshot"), 255),
				"uom":          it.Str("transaction_uom_snapshot"), "uom_factor": it.Str("uom_to_base_factor"), "qty": it.Str("quantity"),
				"qty_base":     it.Str("quantity_in_base_uom"),
				"unit_price":   s.money.R(itemTable, ilid, "unit_price", it, "unit_price_before_discount"),
				"discount_pct": "0.00", "discount_nominal": nominal, "line_total": lineTotal,
				"stock_tracked": b2i(it.Bool("stock_tracked_snapshot")), "item_type": itype,
				"variant_name":    s.optStr(itemTable, ilid, "variant_name", it, "variant_name_snapshot", 255),
				"variant_code":    s.optStr(itemTable, ilid, "variant_code", it, "variant_code_snapshot", 50),
				"base_uom":        emptyNil(it.Str("base_uom_snapshot")),
				"discount_reason": s.optStr(itemTable, ilid, "discount_reason", it, "discount_reason", 255),
				"line_note":       s.optStr(itemTable, ilid, "line_note", it, "notes", 255),
				"taxable":         b2i(it.Bool("taxable")), "tax_base": s.money.R(itemTable, ilid, "tax_base", it, "tax_base_amount"),
				"tax_amount": s.money.R(itemTable, ilid, "tax_amount", it, "tax_amount"),
				"created_at": tsLocalOr(it, "created_at", odate),
			}
			if kind == "sales" {
				ir["pricing_source"] = "catalog"
				ir["base_price"] = nil
			} else {
				// PO-03: the rate the line was taxed at (0 when exempt).
				ir["tax_rate"] = "0.0000"
				if it.Bool("taxable") {
					if v := it.Str("tax_rate_snapshot"); v != "" {
						ir["tax_rate"] = v
					}
				}
			}
			_ = i
			itemRows = append(itemRows, ir)
		}
		if len(itemRows) > 0 && sum != grand {
			best := 0
			for i, ir := range itemRows {
				if abs64(ir.Int("line_total")) > abs64(itemRows[best].Int("line_total")) {
					best = i
				}
			}
			delta := grand - sum
			itemRows[best]["line_total"] = itemRows[best].Int("line_total") + delta
			s.log.Adjust(itemTable, lid, "line_total", fmt.Sprint(sum), fmt.Sprint(grand), "absorb_into_largest_line")
		}
		for _, ir := range itemRows {
			s.emit(itemTable, ir)
		}

		hdr := row.Row{
			"id": id, "number": s.trunc(table, lid, "number", o.Str("order_number"), 60), "branch_id": branch, "party_id": party,
			"order_date": fmtWIB(odate), "due_date": due, "payment_terms": terms, "tax_type": tt,
			"tax_rate": s.rate2(table, lid, o.Rat("tax_rate_snapshot")), "subtotal": subtotal, "discount_total": disc, "tax_total": tax,
			"grand_total": grand, "status": status, "notes": emptyNil(o.Str("notes")), "created_at": tsLocalOr(o, "created_at", odate),
			"created_by": s.userID(o.Int("id_created_by")),
		}
		if kind == "sales" {
			hdr["tax_invoice_number"] = s.optStr(table, lid, "tax_invoice_number", o, "tax_invoice_number", 50)
			hdr["tax_invoice_date"] = tsWIB(o, "tax_invoice_date")
			hdr["channel"] = "regular"
			if o.Str("source") == "pos" {
				hdr["channel"] = "pos"
			}
			hdr["ship_to_address_id"] = nil
			if a, ok := o.NInt("id_ship_to_address"); ok {
				if aid, ok := s.id("party_delivery_addresses", a); ok {
					hdr["ship_to_address_id"] = aid
				}
			}
			hdr["ship_to_label"] = s.optStr(table, lid, "ship_to_label", o, "ship_to_label_snapshot", 100)
			hdr["ship_to_recipient"] = s.optStr(table, lid, "ship_to_recipient", o, "ship_to_recipient_snapshot", 150)
			hdr["ship_to_phone"] = s.optStr(table, lid, "ship_to_phone", o, "ship_to_phone_snapshot", 30)
			hdr["ship_to_address"] = s.optStr(table, lid, "ship_to_address", o, "ship_to_address_snapshot", 255)
			hdr["member_code"] = nil
			hdr["client_ref"] = nil
			// Who is in charge and who approved credit (SL-09): carried over, no longer
			// archive-only. Unmapped users stay NULL like created_by.
			hdr["assigned_user_id"] = s.userID(o.Int("id_assigned_user"))
			if o.Bool("credit_approved") {
				hdr["credit_approved_by"] = s.userID(o.Int("credit_approved_by"))
				hdr["credit_approved_at"] = ts(o, "credit_approved_at")
			} else {
				hdr["credit_approved_by"], hdr["credit_approved_at"] = nil, nil
			}
			if status == "completed" {
				t, ok := o.Time("goods_delivered_at")
				if !ok {
					t, _ = o.LocalTime("updated_at")
				}
				hdr["completed_at"] = fmtUTC(t)
			} else {
				hdr["completed_at"] = nil
			}
		} else {
			hdr["supplier_invoice_number"] = s.optStr(table, lid, "supplier_invoice_number", o, "supplier_invoice_number", 50)
			hdr["supplier_invoice_date"] = tsWIB(o, "supplier_invoice_date")
			hdr["tax_invoice_number"] = s.optStr(table, lid, "tax_invoice_number", o, "tax_invoice_number", 100)
			hdr["tax_invoice_date"] = tsWIB(o, "tax_invoice_date")
			hdr["assigned_user_id"] = s.userID(o.Int("id_assigned_user"))
			if n, ok := o.NInt("payment_term_days"); ok {
				hdr["payment_term_days"] = n
			} else {
				hdr["payment_term_days"] = nil
			}
		}
		s.emit(table, hdr)
	}
	return nil
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// userID maps a legacy user to the revamp id (nil when not migrated).
func (s *state) userID(legacy int64) any {
	if id, ok := s.id("users", legacy); ok {
		return id
	}
	return nil
}

var _ = time.Second
