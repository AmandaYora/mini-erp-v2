// Tombol nominal cepat "Uang diterima" (logika murni, diuji di quick-cash.test.ts).
// Hanya mempercepat pengisian — angka yang dikirim tetap lewat validasi checkout.

export interface QuickCashOption {
  kind: "exact" | "amount";
  amount: number;
}

const DENOMINATIONS = [50_000, 100_000];
const ROUND_STEPS = [10_000, 50_000, 100_000];

/** Bulatkan `total` ke atas ke kelipatan `step`. */
export function roundUpCash(total: number, step: number): number {
  if (step <= 0 || total <= 0) return Math.max(0, total);
  return Math.ceil(total / step) * step;
}

/**
 * Uang pas lebih dulu, lalu nominal tunai yang menutup total (pecahan 50.000 /
 * 100.000 dan pembulatan ke atas), urut naik, tanpa duplikat, maksimal 3 pilihan
 * selain uang pas.
 */
export function quickCashOptions(total: number): QuickCashOption[] {
  if (!Number.isFinite(total) || total <= 0) return [];
  const extra = new Set<number>();
  for (const d of DENOMINATIONS) if (d >= total) extra.add(d);
  for (const s of ROUND_STEPS) extra.add(roundUpCash(total, s));
  const sorted = [...extra].filter((a) => a > total).sort((a, b) => a - b);
  return [
    { kind: "exact", amount: total },
    ...sorted.slice(0, 3).map((amount) => ({ kind: "amount" as const, amount })),
  ];
}
