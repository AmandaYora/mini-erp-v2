import { manualJournalSchema } from "@/modules/finance/schemas/finance.schema";
import type { ManualJournalValues } from "@/modules/finance/schemas/finance.schema";
import { entryTotals } from "@/modules/finance/types";
import type { JournalLine } from "@/modules/finance/types";

/** Satu baris isian editor jurnal; nominal disimpan sebagai teks agar kolom boleh kosong. */
export interface DraftLine {
  key: number;
  accountCode: string;
  debit: string;
  credit: string;
}

export function emptyDraftLine(key: number): DraftLine {
  return { key, accountCode: "", debit: "", credit: "" };
}

function amount(raw: string): number {
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? n : 0;
}

export function draftTotals(lines: DraftLine[]): { debit: number; credit: number } {
  return lines.reduce(
    (s, l) => ({ debit: s.debit + amount(l.debit), credit: s.credit + amount(l.credit) }),
    { debit: 0, credit: 0 },
  );
}

/** Selisih debit − kredit; 0 berarti seimbang. */
export function draftDifference(lines: DraftLine[]): number {
  const t = draftTotals(lines);
  return t.debit - t.credit;
}

export function isDraftBalanced(lines: DraftLine[]): boolean {
  const t = draftTotals(lines);
  return t.debit > 0 && t.debit === t.credit;
}

/**
 * Validasi editor dengan skema Zod yang sama dengan sebelumnya. Mengembalikan nilai terurai
 * atau peta galat (`memo`, `lines`, `lines.<i>.debit`, `lines.<i>.accountCode`).
 */
export function parseJournalDraft(input: {
  date: string;
  memo: string;
  lines: DraftLine[];
}): { ok: true; values: ManualJournalValues } | { ok: false; errors: Record<string, string> } {
  const parsed = manualJournalSchema.safeParse({
    date: input.date.trim() ? input.date.trim() : undefined,
    memo: input.memo.trim(),
    lines: input.lines.map((r) => ({
      accountCode: r.accountCode,
      debit: r.debit.trim() === "" ? 0 : r.debit,
      credit: r.credit.trim() === "" ? 0 : r.credit,
    })),
  });
  if (parsed.success) return { ok: true, values: parsed.data };
  const errors: Record<string, string> = {};
  for (const issue of parsed.error.issues) {
    const k = issue.path.join(".");
    if (!errors[k]) errors[k] = issue.message;
    if (issue.path[0] === "lines" && !errors.lines) errors.lines = issue.message;
  }
  return { ok: false, errors };
}

/** Jurnal tersimpan seimbang: debit = kredit dan lebih dari nol. */
export function isEntryBalanced(lines: JournalLine[] | undefined | null): boolean {
  const t = entryTotals(lines ?? undefined);
  return t.debit > 0 && t.debit === t.credit;
}
