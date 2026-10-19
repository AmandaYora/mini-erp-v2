import { describe, expect, it } from "vitest";
import {
  draftDifference,
  draftTotals,
  emptyDraftLine,
  isDraftBalanced,
  isEntryBalanced,
  parseJournalDraft,
} from "@/modules/finance/lib/journal-lines";
import type { DraftLine } from "@/modules/finance/lib/journal-lines";

const line = (key: number, accountCode: string, debit: string, credit: string): DraftLine => ({
  key,
  accountCode,
  debit,
  credit,
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
