// Matematika keranjang POS sisi klien (estimasi; total resmi dari server).
// Diekstrak dari PosPage agar bisa diuji. Rumus baris mengikuti
// shared/money LineAmounts di backend: gross = qty × harga satuan,
// net = gross − round(gross × pct / 100) − nominal (nominal = potongan per
// BARIS dalam rupiah, bukan per satuan).
export interface CartLineEstimate {
  qty: number;
  unitPrice: number;
  /** Diskon persen baris, 0–100 (opsional). */
  discountPct?: number;
  /** Diskon nominal baris dalam rupiah bulat (opsional). */
  discountNominal?: number;
}

/** Bruto baris, dibulatkan ke rupiah. */
export function lineGross(line: Pick<CartLineEstimate, "qty" | "unitPrice">): number {
  return Math.round(line.qty * line.unitPrice);
}

/** Total potongan baris (persen + nominal). */
export function lineDiscount(line: CartLineEstimate): number {
  const pct = line.discountPct ?? 0;
  const nominal = line.discountNominal ?? 0;
  return Math.round((lineGross(line) * pct) / 100) + nominal;
}

/** Estimasi satu baris (netto setelah diskon). */
export function lineTotal(line: CartLineEstimate): number {
  return lineGross(line) - lineDiscount(line);
}

/** Estimasi total = Σ netto baris. */
export function cartEstimate(lines: CartLineEstimate[]): number {
  return lines.reduce((sum, l) => sum + lineTotal(l), 0);
}

/** Kembalian = tunai − total (negatif = kurang bayar). */
export function cashChange(tendered: number, total: number): number {
  return tendered - total;
}

/** Potongan maksimum satu baris agar harga satuan efektif tetap ≥
 * minSellingPrice (server menolak netto < round(min × qty)). min ≤ 0 =
 * tanpa batas bawah selain nol. */
export function maxLineDiscount(
  line: Pick<CartLineEstimate, "qty" | "unitPrice">,
  minSellingPrice: number,
): number {
  const gross = lineGross(line);
  const floor = minSellingPrice > 0 ? Math.round(minSellingPrice * line.qty) : 0;
  return Math.max(0, gross - floor);
}

export interface LineDiscount {
  discountPct: number;
  discountNominal: number;
}

export interface ClampedDiscount extends LineDiscount {
  /** true bila nilai diminta diturunkan oleh klem. */
  clamped: boolean;
}

/** Rapikan diskon (pct 0–100, nominal bulat ≥ 0) lalu klem agar total
 * potongan ≤ maxLineDiscount. Persen dikurangi lebih dulu bila sendirian
 * melampaui batas; selain itu nominal yang dipotong. */
export function clampLineDiscount(
  line: Pick<CartLineEstimate, "qty" | "unitPrice">,
  requested: Partial<LineDiscount>,
  minSellingPrice: number,
): ClampedDiscount {
  let pct = Number.isFinite(requested.discountPct) ? (requested.discountPct as number) : 0;
  let nominal = Number.isFinite(requested.discountNominal)
    ? Math.floor(requested.discountNominal as number)
    : 0;
  pct = Math.min(100, Math.max(0, pct));
  nominal = Math.max(0, nominal);
  const gross = lineGross(line);
  const ceiling = maxLineDiscount(line, minSellingPrice);
  let clamped = false;

  let pctAmount = Math.round((gross * pct) / 100);
  if (pctAmount > ceiling) {
    // Turunkan persen ke bawah (2 desimal) supaya pembulatan tak melampaui.
    pct = gross > 0 ? Math.floor(((ceiling * 100) / gross) * 100) / 100 : 0;
    pctAmount = Math.round((gross * pct) / 100);
    clamped = true;
  }
  if (pctAmount + nominal > ceiling) {
    nominal = Math.max(0, ceiling - pctAmount);
    clamped = true;
  }
  return { discountPct: pct, discountNominal: nominal, clamped };
}
