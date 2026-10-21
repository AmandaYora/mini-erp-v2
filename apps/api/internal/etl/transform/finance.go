package transform

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	finapp "mini-erp/internal/modules/finance/application"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

type acct struct {
	newID   int64
	code    string
	typ     string // legacy account_type
	sysKey  string
	bs      bool // balance-sheet account
	dimKind string
}

// Target text sizes: journal memo and line description are VARCHAR(500);
// expense notes are TEXT (65.535 bytes, at most 4 bytes per character).
const (
	maxJournalText = 500
	maxText        = 16383
)

var accountTypeMap = map[string]string{
	"asset": "aset", "liability": "kewajiban", "equity": "modal", "revenue": "pendapatan", "expense": "beban",
}

func (s *state) finance() error {
	s.accounts()
	s.journals()
	s.migrationCorrections()
	s.roundingSummary()
	s.expenses()
	s.periods()
	return nil
}

func (s *state) accounts() {
	cash := map[int64]bool{}
	for _, c := range s.raw["finance_cash_accounts"] {
		if !c.Has("archived_at") {
			cash[c.Int("id_finance_account")] = true
		}
	}
	s.acct = map[int64]*acct{}
	seenCode := map[string]int64{}
	seenKey := map[string]int64{}
	for _, a := range sortedBy(s.raw["finance_accounts"], "id_finance_account") {
		lid := a.Int("id_finance_account")
		typ, ok := accountTypeMap[a.Str("account_type")]
		if !ok {
			s.find(report.Blocker, "ACCT_TYPE", "finance_accounts", lid, "tipe akun %q tidak dikenal", a.Str("account_type"))
			continue
		}
		code := a.Str("account_code")
		if len(code) > 20 {
			s.find(report.Blocker, "ACCT_CODE", "finance_accounts", lid, "kode akun %q melebihi 20 karakter", code)
		}
		if prev, dup := seenCode[code]; dup {
			s.find(report.Blocker, "ACCT_DUP", "finance_accounts", lid, "kode akun %q ganda dengan %d", code, prev)
			continue
		}
		seenCode[code] = lid
		id := s.alloc("finance_accounts", lid)
		st := "active"
		if a.Has("archived_at") || a.Str("status") != "active" {
			st = "archived"
		}
		ac := &acct{newID: id, code: code, typ: a.Str("account_type"), sysKey: a.Str("system_key"),
			bs: a.Str("account_type") == "asset" || a.Str("account_type") == "liability" || a.Str("account_type") == "equity"}
		switch ac.sysKey {
		case "accounts_receivable", "accounts_payable":
			ac.dimKind = "party"
		case "inventory":
			ac.dimKind = "product"
		}
		s.acct[lid] = ac
		s.emit("finance_accounts", row.Row{"id": id, "code": code, "name": s.trunc("finance_accounts", lid, "name", a.Str("account_name"), 150),
			"type": typ, "is_cash": b2i(cash[lid]), "status": st, "created_at": tsLocalOr(a, "created_at", s.epoch()), "updated_at": tsLocalOr(a, "updated_at", s.epoch())})
		if ac.sysKey != "" {
			mk, known := finapp.LegacySystemKeyToMappingKey[ac.sysKey]
			if !known {
				s.find(report.Info, "ACCT_SYSKEY", "finance_accounts", lid, "system_key %q tidak punya kunci pemetaan revamp: dimigrasi sebagai akun biasa", ac.sysKey)
				continue
			}
			if prev, dup := seenKey[mk]; dup {
				s.find(report.Blocker, "ACCT_MAPPING_DUP", "finance_accounts", lid, "kunci pemetaan %q ganda dengan akun %d", mk, prev)
				continue
			}
			seenKey[mk] = lid
			s.emit("finance_account_mappings", row.Row{"mapping_key": mk, "account_id": id})
		}
	}
}

type dimKey struct {
	acct                    int64
	party, product, variant int64
}

type jline struct {
	acct                    *acct
	legacyAcct              int64
	debit, credit           int64
	party, product, variant any
	desc                    any
}

func (s *state) journals() {
	lines := map[int64][]row.Row{}
	for _, l := range sortedBy(s.raw["finance_journal_lines"], "id_finance_journal_line") {
		lines[l.Int("id_finance_journal_entry")] = append(lines[l.Int("id_finance_journal_entry")], l)
	}
	for _, ls := range lines {
		sort.SliceStable(ls, func(i, j int) bool { return ls[i].Int("line_no") < ls[j].Int("line_no") })
	}
	type centry struct {
		src   row.Row
		lines []jline
		date  time.Time
		wib   string
		br    int64
	}
	var current []*centry
	prior := map[int64]map[dimKey]int64{} // branch -> dims -> net (debit-credit)
	nullBranch := 0
	entries := append([]row.Row(nil), s.raw["finance_journal_entries"]...)
	sort.SliceStable(entries, func(i, j int) bool {
		a, _ := entries[i].Time("journal_date")
		b, _ := entries[j].Time("journal_date")
		if !a.Equal(b) {
			return a.Before(b)
		}
		return entries[i].Int("id_finance_journal_entry") < entries[j].Int("id_finance_journal_entry")
	})
	for _, e := range entries {
		lid := e.Int("id_finance_journal_entry")
		if st := e.Str("status"); st != "posted" && st != "reversed" {
			s.out.Explain["journals_not_migrated"]++
			continue
		}
		date, _ := e.Time("journal_date")
		wib := wibDate(date)
		var br int64
		if b, ok := e.NInt("id_branch"); ok {
			br, _ = s.id("branches", b)
		} else {
			br = s.headBranch
			nullBranch++
		}
		if br == 0 {
			s.find(report.Blocker, "JRN_BRANCH", "finance_journal_entries", lid, "cabang jurnal tidak dikenal")
			continue
		}
		if len(e.Str("journal_number")) > 60 {
			s.find(report.Blocker, "JRN_NUMBER", "finance_journal_entries", lid, "nomor jurnal melebihi 60 karakter")
		}
		var jl []jline
		var bl []Line
		okEntry := true
		for _, l := range lines[lid] {
			a := s.acct[l.Int("id_finance_account")]
			if a == nil {
				s.find(report.Blocker, "JRN_ACCOUNT", "finance_journal_lines", l.Int("id_finance_journal_line"), "akun jurnal tidak terpeta")
				okEntry = false
				continue
			}
			ll := l.Int("id_finance_journal_line")
			d := s.money.R("finance_journal_lines", ll, "debit", l, "debit_amount")
			c := s.money.R("finance_journal_lines", ll, "credit", l, "credit_amount")
			if d > 0 && c > 0 {
				s.find(report.Blocker, "JRN_TWO_SIDES", "finance_journal_lines", ll, "baris bersisi ganda")
				okEntry = false
			}
			if d == 0 && c == 0 {
				continue
			}
			j := jline{acct: a, legacyAcct: l.Int("id_finance_account"), debit: d, credit: c, desc: s.optStr("finance_journal_lines", ll, "description", l, "description", maxJournalText)}
			if p, ok := l.NInt("id_business_party"); ok {
				if id, ok := s.id("parties", p); ok {
					j.party = id
				}
			}
			if p, ok := l.NInt("id_product"); ok {
				if id, ok := s.id("products", p); ok {
					j.product = id
				}
			}
			if v, ok := l.NInt("id_product_variant"); ok {
				if id, ok := s.id("product_variants", v); ok {
					j.variant = id
				}
			}
			jl = append(jl, j)
			bl = append(bl, Line{d, c})
		}
		if !okEntry {
			continue
		}
		if len(jl) == 0 && len(lines[lid]) > 0 {
			// Every line is Rp0 (e.g. a value-correction journal that changed nothing):
			// the revamp rejects zero lines and an entry without lines, so it has no
			// books effect to carry. Counted out in verify (V-01x, V-17e).
			s.find(report.Info, "JRN_ZERO_VALUE", "finance_journal_entries", lid, "jurnal %s hanya berisi baris Rp0: tidak dimigrasi (tanpa efek pembukuan)", e.Str("journal_number"))
			continue
		}
		idx, delta, side, err := BalanceEntry(bl, s.in.Cfg.MaxBalanceDelta)
		if err != nil {
			s.find(report.Blocker, "JRN_UNBALANCED", "finance_journal_entries", lid, "%v", err)
			continue
		}
		if idx >= 0 {
			before := jl[idx].debit
			if side == "credit" {
				before = jl[idx].credit
				jl[idx].credit += delta
			} else {
				jl[idx].debit += delta
			}
			s.log.Adjust("journal_balance", lid, fmt.Sprintf("acct=%d;branch=%d;side=%s;prior=%d", jl[idx].legacyAcct, br, side, b2i(wib < s.open)),
				fmt.Sprint(before), fmt.Sprint(before+delta), "balance_entry")
		}
		if wib < s.open {
			if prior[br] == nil {
				prior[br] = map[dimKey]int64{}
			}
			for _, j := range jl {
				if !j.acct.bs {
					k := dimKey{acct: -1}
					prior[br][k] += j.debit - j.credit // P&L closing bucket
					continue
				}
				k := dimKey{acct: j.acct.newID}
				switch j.acct.dimKind {
				case "party":
					k.party = anyInt(j.party)
				case "product":
					k.product, k.variant = anyInt(j.product), anyInt(j.variant)
				}
				prior[br][k] += j.debit - j.credit
			}
			continue
		}
		current = append(current, &centry{src: e, lines: jl, date: date, wib: wib, br: br})
	}
	if nullBranch > 0 {
		s.find(report.Info, "JRN_NULL_BRANCH", "finance_journal_entries", 0, "%d jurnal tanpa cabang dipindah ke kantor pusat", nullBranch)
	}

	// opening balances: one journal per branch
	var equity *acct
	for _, a := range s.acct {
		if a.sysKey == "opening_equity" {
			equity = a
		}
	}
	var brs []int64
	for b := range prior {
		brs = append(brs, b)
	}
	sort.Slice(brs, func(i, j int) bool { return brs[i] < brs[j] })
	openTime := s.in.Cfg.OpeningTime()
	byNew := map[int64]*acct{}
	for _, a := range s.acct {
		byNew[a.newID] = a
	}
	nextLine := func() int64 { return s.nextID("finance_journal_lines") }
	for _, b := range brs {
		nets := prior[b]
		keys := make([]dimKey, 0, len(nets))
		for k := range nets {
			if k.acct == -1 {
				continue
			}
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			ai, aj := byNew[keys[i].acct].code, byNew[keys[j].acct].code
			if ai != aj {
				return ai < aj
			}
			if keys[i].party != keys[j].party {
				return keys[i].party < keys[j].party
			}
			if keys[i].product != keys[j].product {
				return keys[i].product < keys[j].product
			}
			return keys[i].variant < keys[j].variant
		})
		var olines []row.Row
		var sum int64
		for _, k := range keys {
			a := byNew[k.acct]
			if a.sysKey == "opening_equity" {
				continue
			}
			net := nets[k]
			if net == 0 {
				continue
			}
			sum += net
			olines = append(olines, s.openingLine(a, k, net))
		}
		if len(olines) == 0 && sum == 0 {
			continue
		}
		if equity == nil {
			s.find(report.Blocker, "OPEN_NO_EQUITY", "finance_accounts", 0, "tidak ada akun opening_equity untuk menyeimbangkan saldo awal")
			continue
		}
		if sum != 0 {
			olines = append(olines, s.openingLine(equity, dimKey{acct: equity.newID}, -sum))
		}
		eid := s.allocSub("finance_journal_entries", b, 7)
		s.out.Explain["opening_entries"]++
		s.emit("finance_journal_entries", row.Row{
			"id": eid, "number": fmt.Sprintf("JV-%s/%s/%05d", s.branchCode[b], s.open[:4], 1), "branch_id": b, "entry_date": naiveDateTime(s.open),
			"memo": "Saldo awal migrasi (s.d. " + openTime.AddDate(0, 0, -1).Format(dateFmt) + ")", "source_doc_type": "opening", "source_doc_id": b,
			"status": "posted", "reversed_by": nil, "created_at": fmtUTC(openTime), "created_by": nil, "scope": 0, "requeue": 0,
		})
		for _, l := range olines {
			l["id"], l["entry_id"] = nextLine(), eid
			s.emit("finance_journal_lines", l)
		}
	}

	// current-year journals as migrated
	s.journalNew = map[int64]int64{}
	for _, c := range current {
		s.journalNew[c.src.Int("id_finance_journal_entry")] = s.alloc("finance_journal_entries", c.src.Int("id_finance_journal_entry"))
	}
	reversedBy := map[int64]int64{}
	for _, c := range current {
		if orig, ok := c.src.NInt("id_reversal_of"); ok {
			if _, mig := s.journalNew[orig]; mig {
				reversedBy[orig] = s.journalNew[c.src.Int("id_finance_journal_entry")]
			}
		}
	}
	for _, c := range current {
		lid := c.src.Int("id_finance_journal_entry")
		ev := c.src.Str("event_key")
		if ev == "" {
			ev = "manual"
		}
		memo := fmt.Sprintf("[L:%s:%s] %s", ev, c.src.Str("journal_number"), c.src.Str("description"))
		var rb any
		if v, ok := reversedBy[lid]; ok {
			rb = v
		} else if c.src.Str("status") == "reversed" {
			s.find(report.Warn, "JRN_REVERSAL_LINK", "finance_journal_entries", lid, "jurnal berstatus reversed tanpa jurnal pembalik tahun berjalan")
		}
		id := s.journalNew[lid]
		s.emit("finance_journal_entries", row.Row{
			"id": id, "number": c.src.Str("journal_number"), "branch_id": c.br, "entry_date": naiveDateTime(c.wib),
			"memo": s.trunc("finance_journal_entries", lid, "memo", strings.TrimSpace(memo), maxJournalText), "source_doc_type": "legacy_je", "source_doc_id": lid,
			"status": c.src.Str("status"), "reversed_by": rb, "created_at": fmtUTC(c.date), "created_by": s.userID(c.src.Int("posted_by")),
			"scope": 0, "requeue": 0,
		})
		for _, l := range c.lines {
			s.emit("finance_journal_lines", row.Row{"id": nextLine(), "entry_id": id, "account_id": l.acct.newID, "product_id": l.product, "variant_id": l.variant,
				"party_id": l.party, "debit": l.debit, "credit": l.credit, "description": l.desc})
		}
	}
}

func anyInt(v any) int64 {
	if n, ok := v.(int64); ok {
		return n
	}
	return 0
}

func (s *state) openingLine(a *acct, k dimKey, net int64) row.Row {
	r := row.Row{"account_id": a.newID, "product_id": nil, "variant_id": nil, "party_id": nil, "description": nil, "debit": int64(0), "credit": int64(0)}
	if net > 0 {
		r["debit"] = net
	} else {
		r["credit"] = -net
	}
	if k.party != 0 {
		r["party_id"] = k.party
	}
	if k.product != 0 {
		r["product_id"] = k.product
	}
	if k.variant != 0 {
		r["variant_id"] = k.variant
	}
	return r
}

func (s *state) expenses() {
	for _, e := range sortedBy(s.raw["business_expenses"], "id_business_expense") {
		lid := e.Int("id_business_expense")
		var br int64
		if b, ok := e.NInt("id_branch"); ok {
			br, _ = s.id("branches", b)
		} else {
			br = s.headBranch
		}
		ex, ok1 := s.acct[e.Int("id_finance_account_expense")]
		pay, ok2 := s.acct[e.Int("id_finance_account_payment")]
		if br == 0 || !ok1 || !ok2 {
			s.find(report.Blocker, "EXP_REFS", "business_expenses", lid, "cabang/akun biaya tidak termigrasi")
			continue
		}
		status := "posted"
		if e.Str("status") == "cancelled" || e.Has("cancelled_at") {
			status = "cancelled"
		}
		var je any
		if j, ok := e.NInt("id_finance_journal_entry"); ok {
			if id, ok := s.journalNew[j]; ok {
				je = id
			}
		}
		d, _ := e.Time("expense_date")
		// Optional counterparty (FN-10): legacy id_business_party carried over
		// when that party migrated; an unmigrated party simply leaves it empty.
		var party any
		if p, ok := e.NInt("id_business_party"); ok {
			if id, ok := s.id("parties", p); ok {
				party = id
			}
		}
		s.emit("business_expenses", row.Row{
			"id": s.alloc("business_expenses", lid), "number": s.trunc("business_expenses", lid, "number", e.Str("expense_number"), 60), "branch_id": br,
			"expense_account_id": ex.newID, "pay_account_id": pay.newID, "amount": s.money.R("business_expenses", lid, "amount", e, "amount"),
			"expense_date": naiveDateTime(d.Format(dateFmt)), "notes": s.optStr("business_expenses", lid, "notes", e, "description", maxText),
			"journal_entry_id": je, "status": status, "created_at": tsLocalOr(e, "created_at", d), "created_by": s.userID(e.Int("created_by")),
			"cancel_reason": s.optStr("business_expenses", lid, "cancel_reason", e, "cancel_reason", 255), "cancelled_at": ts(e, "cancelled_at"),
			"cancelled_by": s.userID(e.Int("cancelled_by")), "party_id": party,
		})
	}
}

// periods closes every month the legacy closed plus every month before the
// opening month (their detail lives in the opening balance), for every branch.
func (s *state) periods() {
	closed := map[string]time.Time{}
	open, _ := time.Parse(dateFmt, s.open)
	openMonth := time.Date(open.Year(), open.Month(), 1, 0, 0, 0, 0, time.UTC)
	for _, p := range s.raw["finance_periods"] {
		if p.Str("status") != "closed" {
			continue
		}
		from, ok1 := p.Time("date_from")
		to, ok2 := p.Time("date_to")
		if !ok1 || !ok2 {
			continue
		}
		at := s.epoch()
		if t, ok := p.Time("closed_at"); ok {
			at = t
		}
		for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC); !m.After(to); m = m.AddDate(0, 1, 0) {
			closed[m.Format("2006-01")] = at
		}
	}
	earliest := openMonth
	consider := func(t time.Time, ok bool) {
		if ok {
			m := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
			if m.Before(earliest) {
				earliest = m
			}
		}
	}
	for _, e := range s.raw["finance_journal_entries"] {
		t, ok := e.Time("journal_date")
		consider(t, ok)
	}
	for _, o := range s.raw["orders"] {
		t, ok := o.Time("order_date")
		consider(t, ok)
	}
	for m := earliest; m.Before(openMonth); m = m.AddDate(0, 1, 0) {
		if _, ok := closed[m.Format("2006-01")]; !ok {
			closed[m.Format("2006-01")] = openTime(s)
		}
	}
	months := make([]string, 0, len(closed))
	for m := range closed {
		months = append(months, m)
	}
	sort.Strings(months)
	var brs []int64
	for b := range s.branchCode {
		brs = append(brs, b)
	}
	sort.Slice(brs, func(i, j int) bool { return brs[i] < brs[j] })
	for _, b := range brs {
		for _, m := range months {
			var y, mo int
			fmt.Sscanf(m, "%d-%d", &y, &mo)
			s.emit("finance_periods", row.Row{"id": s.nextID("finance_periods"), "branch_id": b, "year": y, "month": mo, "status": "closed", "closed_at": fmtUTC(closed[m])})
		}
	}
}

func openTime(s *state) time.Time { return s.in.Cfg.OpeningTime() }

// roundingSummary states what rupiah rounding (ADR-0005) and journal
// balancing (K4, <= --max-balance-delta) did, so neither needs a correction
// journal and the accountant sees the size: V-11 then proves the balances.
func (s *state) roundingSummary() {
	var nRound, nBal int64
	sumRound, maxRound := new(big.Rat), new(big.Rat)
	var sumBal, maxBal int64
	for _, a := range s.log.Adjustments {
		switch a.Rule {
		case "round_half_away":
			d := new(big.Rat).Sub(row.ParseRat(a.After), row.ParseRat(a.Before))
			nRound++
			sumRound.Add(sumRound, d)
			if new(big.Rat).Abs(d).Cmp(maxRound) > 0 {
				maxRound = new(big.Rat).Abs(d)
			}
		case "balance_entry":
			d := row.ParseRat(a.After).Num().Int64() - row.ParseRat(a.Before).Num().Int64()
			if d < 0 {
				d = -d
			}
			nBal++
			sumBal += d
			if d > maxBal {
				maxBal = d
			}
		}
	}
	s.out.Explain["balance_entry_adjustments"] = nBal
	s.find(report.Info, "ROUNDING_SUMMARY", "etl", 0,
		"pembulatan rupiah: %d nilai, Σ selisih Rp%s, maks |Rp%s|; penyeimbang jurnal: %d jurnal, Σ Rp%d, maks Rp%d per jurnal (batas --max-balance-delta Rp%d); tanpa jurnal koreksi (bukti: V-11, V-13a)",
		nRound, sumRound.FloatString(4), maxRound.FloatString(4), nBal, sumBal, maxBal, s.in.Cfg.MaxBalanceDelta)
}
