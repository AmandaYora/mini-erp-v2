import type { RestateGate, RestateReport } from "@/modules/finance/types";

const GATE_LABELS: Record<string, string> = {
  trial_balance: "Neraca saldo seimbang",
  inventory_recon: "Persediaan buku = ledger biaya",
  pending_documents: "Tidak ada dokumen belum dijurnal",
  period_open: "Periode tanggal jurnal terbuka",
};

/** Label gerbang hitung ulang HPP (kunci tak dikenal tampil apa adanya). */
export function gateLabel(key: string): string {
  return GATE_LABELS[key] ?? key;
}

const OFFSET_LABELS: Record<string, string> = {
  cogs: "HPP",
  adjustment: "Selisih stok",
  opening_correction: "Koreksi saldo awal",
  writeoff: "Hapus stok",
  purchase_return_difference: "Selisih retur beli",
};

/** "HPP −6.000, Selisih stok 1.000" — ke akun mana koreksi satu produk dibukukan. */
export function offsetSummary(offsets: Record<string, number> | null | undefined, fmt: (n: number) => string): string {
  if (!offsets) return "-";
  const parts = Object.keys(offsets)
    .sort()
    .filter((k) => offsets[k] !== 0)
    .map((k) => `${OFFSET_LABELS[k] ?? k} ${fmt(offsets[k])}`);
  return parts.length > 0 ? parts.join(", ") : "-";
}

export function failedGates(gates: RestateGate[] | null | undefined): RestateGate[] {
  return (gates ?? []).filter((g) => !g.ok);
}

export type RestateState = "nothing" | "blocked" | "ready";

/** Apa yang bisa dilakukan dengan hasil dry run: tak ada selisih, terhalang gerbang, atau siap dikomit. */
export function restateState(report: RestateReport): RestateState {
  if (report.rowsChanged === 0) return "nothing";
  return failedGates(report.gates).length > 0 ? "blocked" : "ready";
}

/** Nilai sesudah revaluasi untuk pratinjau klien (server tetap sumber kebenaran): round(qty × harga). */
export function revaluedValue(qty: number, unitCost: number): number {
  return Math.round(qty * unitCost);
}

/** Kunci idempoten satu kali komit (replay → server mengembalikan komit pertama). */
export function newCorrectionRef(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `rv-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}
