/** Baca isian jumlah (teks) menjadi angka >= 0; kosong/tak valid = null. */
export function parseQty(text: string): number | null {
  const t = text.trim().replace(",", ".");
  if (t === "") return null;
  const n = Number(t);
  return Number.isFinite(n) && n >= 0 ? n : null;
}

/** Selisih fisik − sistem; null bila salah satunya belum diketahui. */
export function physicalDelta(system: number | null, physical: number | null): number | null {
  if (system === null || physical === null) return null;
  return Math.round((physical - system) * 1e6) / 1e6;
}

/** Saldo hasil koreksi mode tambah/kurang; null bila qty sistem atau jumlah belum diketahui. */
export function resultingQty(system: number | null, qty: number | null, mode: "in" | "out"): number | null {
  if (system === null || qty === null) return null;
  return Math.round((mode === "in" ? system + qty : system - qty) * 1e6) / 1e6;
}
