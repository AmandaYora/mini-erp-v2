import type {
  AgingBuckets,
  AgingDoc,
  CashFlowRow,
  IntegrityReport,
} from "@/modules/finance/types";

/** Label + nilai pita umur, berurutan dari yang terbaru. */
export function agingBucketRows(b: AgingBuckets): { key: string; label: string; amount: number }[] {
  return [
    { key: "d0_30", label: "0–30 hari", amount: b.d0_30 },
    { key: "d31_60", label: "31–60 hari", amount: b.d31_60 },
    { key: "d61_90", label: "61–90 hari", amount: b.d61_90 },
    { key: "over90", label: "Lebih dari 90 hari", amount: b.over90 },
  ];
}

/** Pita umur dokumen berdasarkan ageDays (sama dengan sisi server). */
export function agingBucketKey(ageDays: number): keyof AgingBuckets {
  if (ageDays <= 30) return "d0_30";
  if (ageDays <= 60) return "d31_60";
  if (ageDays <= 90) return "d61_90";
  return "over90";
}

/** Dokumen sudah lewat tempo bila overdueDays > 0. */
export function isOverdue(d: Pick<AgingDoc, "overdueDays">): boolean {
  return d.overdueDays > 0;
}

/** Saldo awal + masuk - keluar harus sama dengan saldo akhir. */
export function cashRowReconciles(r: CashFlowRow): boolean {
  return r.opening + r.in - r.out === r.closing;
}

/** Total masuk / keluar seluruh rekening (server hanya mengirim total awal/akhir). */
export function cashFlowTotals(rows: CashFlowRow[]): { in: number; out: number } {
  return rows.reduce((a, r) => ({ in: a.in + r.in, out: a.out + r.out }), { in: 0, out: 0 });
}

/** Ringkasan satu kalimat hasil pemeriksaan integritas pembukuan. */
export function integritySummary(r: IntegrityReport): string {
  if (r.consistent) return "Pembukuan konsisten: rollup harian cocok dengan baris jurnal.";
  const parts: string[] = [];
  if (r.rollupDiffs.length > 0) parts.push(`${r.rollupDiffs.length} selisih rollup`);
  if (r.unbalanced > 0) parts.push(`${r.unbalanced} jurnal tidak seimbang`);
  if (r.inventory && !r.inventory.consistent) parts.push("persediaan buku berbeda dari ledger biaya");
  return `Ditemukan masalah: ${parts.join(", ")}.`;
}
