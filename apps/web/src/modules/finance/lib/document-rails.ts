import type { RelatedDocument, StepperStep } from "@/shared/components/patterns";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import type { Expense, JournalEntry } from "@/modules/finance/types";

// Rel Dokumen keuangan: fungsi murni dari data yang sudah dimuat halaman (tanpa endpoint baru).

export const JOURNAL_RAIL_STEPS: StepperStep[] = [
  { key: "posted", label: "Diposting" },
  { key: "reversed", label: "Dibalik" },
];

export function journalRailCurrent(e: Pick<JournalEntry, "status">): string {
  return e.status === "reversed" ? "reversed" : "posted";
}

export function journalRelated(e: Pick<JournalEntry, "reversedBy">): RelatedDocument[] {
  return e.reversedBy > 0
    ? [{ label: "Jurnal pembalik", kind: "Jurnal pembalik", to: `${ROUTE_PATHS.financeJournals}/${e.reversedBy}` }]
    : [];
}

export const EXPENSE_RAIL_STEPS: StepperStep[] = [
  { key: "recorded", label: "Dicatat" },
  { key: "posted", label: "Dijurnalkan" },
];

/** Biaya yang dibatalkan berhenti di langkah terakhir (garis putus-putus); jurnalnya dibalik. */
export function expenseRailCurrent(e: Pick<Expense, "status" | "journalEntryId">): string {
  return e.journalEntryId > 0 || e.status === "posted" || e.status === "cancelled" ? "posted" : "recorded";
}

export function expenseRelated(e: Pick<Expense, "journalEntryId">): RelatedDocument[] {
  return e.journalEntryId > 0
    ? [{ label: "Jurnal biaya", kind: "Jurnal otomatis", to: `${ROUTE_PATHS.financeJournals}/${e.journalEntryId}` }]
    : [];
}
