package integration

import (
	"strings"
	"testing"

	financeapp "mini-erp/internal/modules/finance/application"
	partyapp "mini-erp/internal/modules/party/application"
	"mini-erp/internal/shared/apperror"
)

// F5: a manual or opening leg on Piutang/Hutang/Uang Muka must name a party of
// the right type, and the per-party report then shows it; no other leg may
// carry a party.
func TestManualControlLegsRequireParty(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	sup, err := testMods.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Pemasok Uji"})
	if err != nil {
		t.Fatalf("supplier: %v", err)
	}
	refused := func(name, want string, lines []financeapp.ManualLine) {
		t.Helper()
		_, err := fin.Manual(ctx(), 0, fx.branchID, "2026-09-10", "koreksi", lines)
		appErr := mustAppErr(t, err)
		if appErr.Code != apperror.CodeValidation || len(appErr.Fields) == 0 || !strings.Contains(appErr.Fields[0].Message, want) {
			t.Fatalf("%s: %s %+v; want validation containing %q", name, appErr.Code, appErr.Fields, want)
		}
	}
	refused("piutang tanpa pihak", "baris 1: akun 1100 Piutang Usaha wajib memilih pelanggan",
		[]financeapp.ManualLine{{AccountCode: "1100", Debit: 5000}, {AccountCode: "3000", Credit: 5000}})
	refused("hutang tanpa pihak", "baris 2: akun 2000 Hutang Usaha wajib memilih pemasok",
		[]financeapp.ManualLine{{AccountCode: "3000", Debit: 5000}, {AccountCode: "2000", Credit: 5000}})
	refused("uang muka pelanggan tanpa pihak", "wajib memilih pelanggan",
		[]financeapp.ManualLine{{AccountCode: "1000", Debit: 5000}, {AccountCode: "2200", Credit: 5000}})
	refused("piutang dengan pemasok", "hanya untuk pelanggan",
		[]financeapp.ManualLine{{AccountCode: "1100", Debit: 5000, PartyID: sup.ID}, {AccountCode: "3000", Credit: 5000}})
	refused("pihak tak dikenal", "pelanggan tidak ditemukan",
		[]financeapp.ManualLine{{AccountCode: "1100", Debit: 5000, PartyID: 999999}, {AccountCode: "3000", Credit: 5000}})
	refused("pihak di akun kas", "baris 2: akun 1000 Kas Tunai tidak memakai pelanggan/pemasok",
		[]financeapp.ManualLine{{AccountCode: "1100", Debit: 5000, PartyID: fx.customerID}, {AccountCode: "1000", Credit: 5000, PartyID: fx.customerID}})

	e, err := fin.Manual(ctx(), 0, fx.branchID, "2026-09-10", "koreksi", []financeapp.ManualLine{
		{AccountCode: "1100", Debit: 5000, PartyID: fx.customerID}, {AccountCode: "3000", Credit: 5000},
	})
	if err != nil {
		t.Fatalf("manual with party: %v", err)
	}
	if e.Lines[0].PartyID != fx.customerID || e.Lines[1].PartyID != 0 {
		t.Fatalf("stored parties = %d, %d; want %d on the receivable leg only", e.Lines[0].PartyID, e.Lines[1].PartyID, fx.customerID)
	}
	rows, total, err := fin.Receivables(ctx(), fx.branchID, "2026-09-30")
	if err != nil {
		t.Fatalf("receivables: %v", err)
	}
	if len(rows) != 1 || rows[0].PartyID != fx.customerID || rows[0].Balance != 5000 || total != 5000 {
		t.Fatalf("receivables = %+v total %d; want the customer at 5.000", rows, total)
	}
	if _, err := fin.Manual(ctx(), 0, fx.branchID, "2026-09-10", "hutang", []financeapp.ManualLine{
		{AccountCode: "3000", Debit: 2500}, {AccountCode: "2000", Credit: 2500, PartyID: sup.ID},
	}); err != nil {
		t.Fatalf("payable with supplier: %v", err)
	}
	if rows, total, err := fin.Payables(ctx(), fx.branchID, "2026-09-30"); err != nil || len(rows) != 1 || rows[0].PartyID != sup.ID || total != 2500 {
		t.Fatalf("payables = %+v total %d err %v; want the supplier at 2.500", rows, total, err)
	}

	// The opening wizard applies the same rule, and posts the party.
	if _, err := fin.PreviewOpening(ctx(), fx.branchID, "2026-09-01", "", []financeapp.ManualLine{
		{AccountCode: "1100", Debit: 4000}, {AccountCode: "3000", Credit: 4000},
	}); err == nil || !strings.Contains(mustAppErr(t, err).Fields[0].Message, "wajib memilih pelanggan") {
		t.Fatalf("opening without party: %v; want refused", err)
	}
	op, err := fin.PostOpening(ctx(), 0, fx.branchID, "2026-09-01", "", []financeapp.ManualLine{
		{AccountCode: "1100", Debit: 4000, PartyID: fx.customerID}, {AccountCode: "3000", Credit: 4000},
	})
	if err != nil {
		t.Fatalf("opening with party: %v", err)
	}
	if op.Lines[0].PartyID != fx.customerID {
		t.Fatalf("opening receivable party = %d, want %d", op.Lines[0].PartyID, fx.customerID)
	}
	// Fiscal adjustments are never read per party: a party is optional there.
	if _, err := fin.CreateTaxAdjustment(ctx(), 0, fx.branchID, "2026-09-15", "koreksi fiskal", []financeapp.ManualLine{
		{AccountCode: "6900", Debit: 1000}, {AccountCode: "1100", Credit: 1000},
	}); err != nil {
		t.Fatalf("tax adjustment without party: %v", err)
	}
}
