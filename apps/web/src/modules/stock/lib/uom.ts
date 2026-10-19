// Label dual-satuan untuk saldo stok (ST-06): "1 dus 5 pcs". Saldo disimpan
// dalam satuan dasar; satuan besar = satuan beli/jual dengan faktor terbesar.

export interface UomInfo {
  baseUom: string;
  /** Pasangan satuan (beli/jual) dengan faktornya ke satuan dasar. */
  units: { uom: string; factor: number }[];
}

/** Pilih satuan besar: faktor > 1 terbesar yang namanya beda dari satuan dasar. */
export function bigUnit(info: UomInfo): { uom: string; factor: number } | null {
  let best: { uom: string; factor: number } | null = null;
  for (const u of info.units) {
    if (!u.uom || u.uom === info.baseUom || !(u.factor > 1)) continue;
    if (best === null || u.factor > best.factor) best = u;
  }
  return best;
}

function trimNum(n: number): string {
  return String(Math.round(n * 1e6) / 1e6);
}

/**
 * "1 dus 5 pcs" untuk qty dasar 17 dengan 1 dus = 12 pcs. Null bila produk tak
 * punya satuan besar atau qty kurang dari satu satuan besar (tak ada gunanya
 * mengulang angka yang sama).
 */
export function dualUomLabel(qtyBase: number, info: UomInfo | undefined): string | null {
  if (!info) return null;
  const big = bigUnit(info);
  if (!big || !Number.isFinite(qtyBase)) return null;
  const abs = Math.abs(qtyBase);
  const whole = Math.floor(abs / big.factor + 1e-9);
  if (whole < 1) return null;
  const rest = abs - whole * big.factor;
  const sign = qtyBase < 0 ? "-" : "";
  const restPart = rest > 1e-9 ? ` ${trimNum(rest)} ${info.baseUom}` : "";
  return `${sign}${whole} ${big.uom}${restPart}`;
}
