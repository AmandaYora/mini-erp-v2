package transform

import (
	"math/big"
	"sort"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

// subledgerCorrections makes every party's receivable/payable in the books
// match its documents where legacy's own journals broke the link (ADR-0018
// D11h). Each rule is a class of legacy bookkeeping error found in the
// rehearsal, not a single case:
//
//   - a payment cancelled in legacy whose journal was never reversed: the
//     books still hold the money (customers: -Rp14,4 jt on one account) —
//     its journal is mirrored;
//   - a payment dated before the opening that legacy never journalled: the
//     opening balance (legacy's books) does not hold it either — it is booked;
//   - a fully received purchase order whose receipt journals booked a
//     different payable than the order's total (the order was repriced after
//     the receipt, or the receipt was never journalled and predates the
//     opening): the payable is aligned to the order, the difference against
//     the stock-difference account.
//
// Runs after the accounts are indexed and before the corrections are written.
func (s *state) subledgerCorrections() {
	journal := map[int64]row.Row{}
	reversed := map[int64]bool{}
	for _, e := range s.raw["finance_journal_entries"] {
		journal[e.Int("id_finance_journal_entry")] = e
		if r, ok := e.NInt("id_reversal_of"); ok && e.Str("status") == "posted" {
			reversed[r] = true
		}
	}
	live := func(id int64) bool {
		e, ok := journal[id]
		return ok && e.Str("status") == "posted" && !reversed[id]
	}
	lines := map[int64][]row.Row{}
	for _, l := range sortedBy(s.raw["finance_journal_lines"], "id_finance_journal_line") {
		lines[l.Int("id_finance_journal_entry")] = append(lines[l.Int("id_finance_journal_entry")], l)
	}
	posted := map[postKey]int64{} // posted source -> its journal
	for _, f := range s.raw["finance_posting_sources"] {
		if f.Str("posting_status") != legacyPostedStatus {
			continue
		}
		if j, ok := f.NInt("id_journal_entry"); ok {
			posted[postKey{f.Str("source_type"), f.Int("id_source"), f.Str("event_key")}] = j
		}
	}
	rup := func(r *big.Rat) int64 { return RoundHalfAway(r) }

	for _, p := range sortedBy(s.raw["payments"], "id_payment") {
		lid := p.Int("id_payment")
		branch, ok := s.id("branches", p.Int("id_branch"))
		if !ok {
			continue
		}
		event := "customer_payment"
		if p.Str("financial_side") == "payable" {
			event = "supplier_payment"
		}
		j, journalled := posted[postKey{"payment", lid, event}]
		switch {
		case p.Has("archived_at") && journalled && live(j):
			ls := lines[j]
			if len(ls) != 2 {
				s.find(report.Warn, "PAYMENT_CANCELLED_JOURNAL", "payments", lid,
					"pembayaran %s dibatalkan di legacy, jurnalnya %d baris masih hidup: tidak dibalik otomatis, telaah akuntan", p.Str("payment_number"), len(ls))
				continue
			}
			var debit, credit row.Row
			for _, l := range ls {
				if l.Rat("debit_amount").Sign() > 0 {
					debit = l
				} else {
					credit = l
				}
			}
			if debit == nil || credit == nil {
				continue
			}
			amt := rup(debit.Rat("debit_amount"))
			party, _ := s.id("parties", p.Int("id_business_party"))
			// The mirror: Dr what was credited, Cr what was debited.
			s.migCorr = append(s.migCorr, migCorrection{kind: corrPaymentCancelled, branch: branch, ref: p.Str("payment_number"),
				drAcct: credit.Int("id_finance_account"), crAcct: debit.Int("id_finance_account"), party: party, amount: amt,
				legacyMovement: lid, reason: "pembayaran batal " + p.Str("payment_number")})
			s.find(report.Warn, "PAYMENT_CANCELLED_JOURNAL", "payments", lid,
				"pembayaran %s (Rp%d) dibatalkan di legacy tetapi jurnalnya %s tidak pernah dibalik: dibalik dengan jurnal koreksi migrasi",
				p.Str("payment_number"), amt, journal[j].Str("journal_number"))
		case !p.Has("archived_at") && !journalled && paidBeforeOpening(p, s.open):
			party, _ := s.id("parties", p.Int("id_business_party"))
			cash := "cash_bank"
			if p.Str("payment_method") == "cash" {
				cash = "cash_cash"
			}
			amt := rup(p.Rat("amount"))
			c := migCorrection{kind: corrPaymentUnjournalled, branch: branch, ref: p.Str("payment_number"), party: party, amount: amt,
				legacyMovement: lid, reason: "pembayaran sebelum saldo awal tanpa jurnal " + p.Str("payment_number")}
			if event == "supplier_payment" {
				c.drKey, c.crKey = "accounts_payable", cash
			} else {
				c.drKey, c.crKey = cash, "accounts_receivable"
			}
			s.migCorr = append(s.migCorr, c)
			s.find(report.Warn, "PAYMENT_UNJOURNALLED", "payments", lid,
				"pembayaran %s (Rp%d) bertanggal sebelum saldo awal tidak pernah dijurnal legacy, jadi saldo awal tidak memuatnya: dibukukan dengan jurnal koreksi migrasi",
				p.Str("payment_number"), amt)
		}
	}

	// Purchase orders received in full: the receipts' payable = the order.
	grsOf := map[int64][]row.Row{}
	for _, g := range s.raw["goods_receipts"] {
		if !g.Has("archived_at") {
			grsOf[g.Int("id_order")] = append(grsOf[g.Int("id_order")], g)
		}
	}
	pending := map[int64]bool{}
	for _, m := range s.out.Marks {
		if m.DocType == "goods_receipt" && m.State == statePending {
			pending[m.DocID] = true
		}
	}
	apAccounts := map[int64]bool{}
	for lid, a := range s.acct {
		if a.sysKey == "accounts_payable" {
			apAccounts[lid] = true
		}
	}
	statusGroup := map[int64]string{}
	for _, d := range s.raw["order_status_definitions"] {
		statusGroup[d.Int("id_order_status_definition")] = d.Str("status_group")
	}
	var orders []int64
	for oid := range grsOf {
		orders = append(orders, oid)
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i] < orders[j] })
	for _, oid := range orders {
		o, ok := s.orderRow[oid]
		if !ok || s.orderKind[oid] != "purchase" || o.Has("archived_at") || statusGroup[o.Int("id_current_status")] != "completed" {
			continue
		}
		booked := new(big.Rat)
		skip := false
		for _, g := range grsOf[oid] {
			gid := g.Int("id_goods_receipt")
			if nid, ok := s.id("goods_receipts", gid); ok && pending[nid] {
				skip = true // the revamp journals it at the order's own prices
				break
			}
			j, ok := posted[postKey{"goods_receipt", gid, "purchase_received"}]
			if !ok || !live(j) {
				continue
			}
			for _, l := range lines[j] {
				if apAccounts[l.Int("id_finance_account")] {
					booked.Add(booked, new(big.Rat).Sub(l.Rat("credit_amount"), l.Rat("debit_amount")))
				}
			}
		}
		if skip {
			continue
		}
		delta := rup(o.Rat("total_amount")) - rup(booked)
		if delta == 0 {
			continue
		}
		branch, ok1 := s.id("branches", o.Int("id_branch"))
		party, ok2 := s.id("parties", o.Int("id_related_party"))
		if !ok1 || !ok2 {
			continue
		}
		s.migCorr = append(s.migCorr, migCorrection{kind: corrPayableAlign, branch: branch, ref: o.Str("order_number"), party: party,
			drKey: "stock_difference", crKey: "accounts_payable", amount: delta, legacyMovement: oid,
			reason: "hutang penerimaan ≠ total PO " + o.Str("order_number")})
		s.find(report.Warn, "PO_PAYABLE_ALIGN", "orders", oid,
			"hutang Rp%d disesuaikan: PO %s diterima penuh senilai Rp%d tetapi jurnal penerimaan legacy membukukan hutang Rp%d (harga diubah setelah penerimaan, atau penerimaan sebelum saldo awal tak pernah dijurnal); selisih ke selisih persediaan",
			delta, o.Str("order_number"), delta+rup(booked), rup(booked))
	}
}

func paidBeforeOpening(p row.Row, open string) bool {
	t, ok := p.Time("payment_date")
	return ok && wibDate(t) < open
}
