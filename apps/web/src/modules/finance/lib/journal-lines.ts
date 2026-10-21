import { manualJournalSchema } from "@/modules/finance/schemas/finance.schema";
import type { ManualJournalValues } from "@/modules/finance/schemas/finance.schema";
import { entryTotals } from "@/modules/finance/types";
import type { AccountMappings, JournalLine } from "@/modules/finance/types";

/** Satu baris isian editor jurnal; nominal disimpan sebagai teks agar kolom boleh kosong. */
export interface DraftLine {
  key: number;
  accountCode: string;
  debit: string;
  credit: string;
  /** Pelanggan/pemasok baris akun kendali per pihak (Piutang, Hutang, Uang Muka); null = belum dipilih. */
  partyId: number | null;
}

export function emptyDraftLine(key: number): DraftLine {
  return { key, accountCode: "", debit: "", credit: "", partyId: null };
}

/** Jenis pihak yang dipakai sebuah akun kendali. */
export type PartyAccountKind = "customer" | "supplier";

/** Kode akun kendali per pihak → jenis pihaknya. */
export type PartyAccounts = ReadonlyMap<string, PartyAccountKind>;

// Kunci pemetaan akun yang dibukukan PER PIHAK (backend: application/partylegs.go partyControlKeys).
// Baris jurnal manual/saldo awal ke akun ini wajib memilih pihaknya (F5).
const PARTY_CONTROL_KEYS: Record<string, PartyAccountKind> = {
  receivable: "customer",
  customer_advance: "customer",
  payable: "supplier",
  supplier_advance: "supplier",
};

/** Akun kendali per pihak menurut pemetaan akun yang berlaku (bukan kode tertanam). */
export function partyAccountsFromMappings(maps: AccountMappings | null | undefined): PartyAccounts {
  const out = new Map<string, PartyAccountKind>();
  for (const [key, kind] of Object.entries(PARTY_CONTROL_KEYS)) {
    const code = maps?.[key]?.code;
    if (code) out.set(code, kind);
  }
  return out;
}

export function partyKindLabel(kind: PartyAccountKind): string {
  return kind === "supplier" ? "pemasok" : "pelanggan";
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
 * atau peta galat (`memo`, `lines`, `lines.<i>.debit`, `lines.<i>.accountCode`, `lines.<i>.partyId`).
 *
 * `partyAccounts`: akun kendali per pihak. Baris ke akun itu wajib memilih pihak dan hanya baris
 * itu yang mengirim `partyId` — baris lain tidak pernah membawa pihak (backend menolaknya).
 * Tanpa `partyAccounts` (koreksi fiskal) tidak ada baris yang mengirim pihak.
 */
export function parseJournalDraft(input: {
  date: string;
  memo: string;
  lines: DraftLine[];
  partyAccounts?: PartyAccounts;
}): { ok: true; values: ManualJournalValues } | { ok: false; errors: Record<string, string> } {
  const kindOf = (r: DraftLine) => input.partyAccounts?.get(r.accountCode);
  const parsed = manualJournalSchema.safeParse({
    date: input.date.trim() ? input.date.trim() : undefined,
    memo: input.memo.trim(),
    lines: input.lines.map((r) => ({
      accountCode: r.accountCode,
      debit: r.debit.trim() === "" ? 0 : r.debit,
      credit: r.credit.trim() === "" ? 0 : r.credit,
      ...(kindOf(r) && r.partyId ? { partyId: r.partyId } : {}),
    })),
  });
  const errors: Record<string, string> = {};
  input.lines.forEach((r, i) => {
    const kind = kindOf(r);
    if (kind && !r.partyId) {
      errors[`lines.${i}.partyId`] = `Pilih ${partyKindLabel(kind)}`;
      if (!errors.lines) errors.lines = `Baris ${i + 1}: akun ${r.accountCode} wajib memilih ${partyKindLabel(kind)}`;
    }
  });
  if (parsed.success && Object.keys(errors).length === 0) return { ok: true, values: parsed.data };
  for (const issue of parsed.success ? [] : parsed.error.issues) {
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
