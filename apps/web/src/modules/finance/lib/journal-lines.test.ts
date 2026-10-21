import { describe, expect, it } from "vitest";
import {
  draftDifference,
  draftTotals,
  emptyDraftLine,
  isDraftBalanced,
  isEntryBalanced,
  parseJournalDraft,
  partyAccountsFromMappings,
} from "@/modules/finance/lib/journal-lines";
import type { DraftLine } from "@/modules/finance/lib/journal-lines";
import type { Account, AccountMappings } from "@/modules/finance/types";

const line = (key: number, accountCode: string, debit: string, credit: string, partyId: number | null = null): DraftLine => ({
  key,
  accountCode,
  debit,
  credit,
  partyId,
});

const acc = (id: number, code: string): Account =>
  ({ id, code, name: `Akun ${code}`, type: "aset", isCash: false, status: "active" }) as Account;

// Kode piutang sengaja bukan 1100: kode akun kendali datang dari pemetaan, bukan tertanam.
const maps: AccountMappings = {
  cash: acc(1, "1000"),
  receivable: acc(2, "1150"),
  payable: acc(3, "2000"),
  customer_advance: acc(4, "2200"),
  supplier_advance: acc(5, "2300"),
};

describe("akun kendali per pihak (F5)", () => {
  it("dibaca dari pemetaan akun", () => {
    const pa = partyAccountsFromMappings(maps);
    expect([...pa.entries()].sort()).toEqual([
      ["1150", "customer"],
      ["2000", "supplier"],
      ["2200", "customer"],
      ["2300", "supplier"],
    ]);
    expect(partyAccountsFromMappings(null).size).toBe(0);
  });

  it("baris akun kendali tanpa pihak ditolak per baris", () => {
    const res = parseJournalDraft({
      date: "",
      memo: "Koreksi",
      lines: [line(1, "3000", "500", ""), line(2, "2000", "", "500")],
      partyAccounts: partyAccountsFromMappings(maps),
    });
    expect(res.ok).toBe(false);
    if (!res.ok) {
      expect(res.errors["lines.1.partyId"]).toBe("Pilih pemasok");
      expect(res.errors.lines).toBe("Baris 2: akun 2000 wajib memilih pemasok");
    }
  });

  it("pihak hanya dikirim di baris akun kendali", () => {
    const res = parseJournalDraft({
      date: "",
      memo: "Koreksi",
      // Baris kas membawa partyId sisa pilihan lama: tidak ikut terkirim.
      lines: [line(1, "1150", "500", "", 7), line(2, "1000", "", "500", 7)],
      partyAccounts: partyAccountsFromMappings(maps),
    });
    expect(res.ok).toBe(true);
    if (res.ok) {
      expect(res.values.lines[0]).toEqual({ accountCode: "1150", debit: 500, credit: 0, partyId: 7 });
      expect(res.values.lines[1]).not.toHaveProperty("partyId");
    }
  });

  it("tanpa peta akun kendali (koreksi fiskal) tidak ada pihak yang diminta atau dikirim", () => {
    const res = parseJournalDraft({ date: "", memo: "Fiskal", lines: [line(1, "6900", "500", ""), line(2, "1150", "", "500", 7)] });
    expect(res.ok).toBe(true);
    if (res.ok) expect(res.values.lines[1]).not.toHaveProperty("partyId");
  });
});

describe("editor jurnal", () => {
  it("menjumlah debit/kredit dan mengabaikan isian kosong atau tak valid", () => {
    const lines = [line(1, "1000", "1500", ""), line(2, "4000", "", "1000"), line(3, "", "abc", "-5")];
    expect(draftTotals(lines)).toEqual({ debit: 1500, credit: 1000 });
    expect(draftDifference(lines)).toBe(500);
    expect(isDraftBalanced(lines)).toBe(false);
  });

  it("seimbang hanya bila debit = kredit dan lebih dari nol", () => {
    expect(isDraftBalanced([emptyDraftLine(1), emptyDraftLine(2)])).toBe(false);
    expect(isDraftBalanced([line(1, "1000", "700", ""), line(2, "4000", "", "700")])).toBe(true);
  });

  it("parse menghasilkan nilai bila valid", () => {
    const res = parseJournalDraft({
      date: "2026-10-16",
      memo: " Koreksi kas ",
      lines: [line(1, "1000", "700", ""), line(2, "4000", "", "700")],
    });
    expect(res.ok).toBe(true);
    if (res.ok) {
      expect(res.values.memo).toBe("Koreksi kas");
      expect(res.values.lines).toEqual([
        { accountCode: "1000", debit: 700, credit: 0 },
        { accountCode: "4000", debit: 0, credit: 700 },
      ]);
    }
  });

  it("parse memetakan galat per kolom", () => {
    const res = parseJournalDraft({
      date: "",
      memo: "",
      lines: [line(1, "", "700", ""), line(2, "4000", "", "500")],
    });
    expect(res.ok).toBe(false);
    if (!res.ok) {
      expect(res.errors.memo).toBeTruthy();
      expect(res.errors["lines.0.accountCode"]).toBe("Pilih akun");
      expect(res.errors.lines).toBeTruthy();
    }
  });
});

describe("jurnal tersimpan", () => {
  const l = (debit: number, credit: number) => ({ accountId: 1, accountCode: "1000", accountName: "Kas", debit, credit });
  it("seimbang bila total debit = kredit > 0", () => {
    expect(isEntryBalanced([l(500, 0), l(0, 500)])).toBe(true);
    expect(isEntryBalanced([l(500, 0), l(0, 400)])).toBe(false);
    expect(isEntryBalanced(null)).toBe(false);
  });
});
