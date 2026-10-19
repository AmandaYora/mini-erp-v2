// Perencanaan tahanan stok (reservasi) keranjang POS — murni, tanpa IO.
//
// Alur (F2a-12): qty baris berubah → debounce 400 ms → suggest → hold per
// lokasi hasil saran. Hold bersifat upsert menurut key, jadi menaikkan atau
// menurunkan qty aman. Suggest membaca Available yang SUDAH dikurangi tahanan
// kita sendiri, maka suggest hanya diminta untuk selisih (desired − sudah
// ditahan), bukan seluruh qty — kalau tidak, baris yang sudah tertahan akan
// tampak kekurangan stok.

/** locationId → qty yang sedang ditahan untuk satu baris. */
export type HeldMap = Record<number, number>;

export interface SuggestionResult {
  lines: { locationId: number; qty: number }[];
  shortfall: number;
}

export interface HoldOp {
  locationId: number;
  qty: number;
}

export interface HoldPlan {
  /** Peta tahanan yang dituju setelah rencana dijalankan. */
  target: HeldMap;
  /** Qty yang tak dapat ditutup stok tersedia (0 = aman). */
  shortfall: number;
  /** hold (upsert) yang perlu dikirim: lokasi baru atau qty berubah. */
  holds: HoldOp[];
  /** Lokasi yang tahanannya harus dilepas. */
  releases: number[];
}

const EPS = 1e-9;

/** Kunci tahanan: pos:<cartRef>:<productId>:<variantId>:<locationId>.
 * Server membatasi < 100 karakter; ref ≤ 64 + 4 angka ≤ ~20 → aman. */
export function holdKey(
  cartRef: string,
  productId: number,
  variantId: number,
  locationId: number,
): string {
  return `pos:${cartRef}:${productId}:${variantId}:${locationId}`;
}

/** Kunci baris keranjang (produk+varian). */
export function lineKey(productId: number, variantId: number): string {
  return `${productId}:${variantId}`;
}

export function heldTotal(held: HeldMap): number {
  return Object.values(held).reduce((s, q) => s + q, 0);
}

/** Qty tambahan yang perlu dicarikan lokasinya lewat suggest (≤ 0 = tak perlu). */
export function neededDelta(held: HeldMap, desired: number): number {
  return desired - heldTotal(held);
}

/** Susun rencana hold untuk satu baris. `suggestion` wajib bila
 * neededDelta > 0 (hasil suggest untuk selisih itu). */
export function planHolds(
  held: HeldMap,
  desired: number,
  suggestion: SuggestionResult | null,
): HoldPlan {
  const target: HeldMap = { ...held };
  let shortfall = 0;
  const delta = neededDelta(held, desired);

  if (desired <= 0) {
    for (const loc of Object.keys(target)) delete target[Number(loc)];
  } else if (delta > EPS) {
    if (suggestion) {
      for (const l of suggestion.lines) {
        if (l.qty <= 0) continue;
        target[l.locationId] = (target[l.locationId] ?? 0) + l.qty;
      }
      shortfall = Math.max(0, suggestion.shortfall);
    } else {
      shortfall = delta;
    }
  } else if (delta < -EPS) {
    // Kurangi dari lokasi terakhir ke awal sampai total = desired.
    let excess = -delta;
    for (const loc of Object.keys(target).map(Number).reverse()) {
      if (excess <= EPS) break;
      const cut = Math.min(target[loc], excess);
      target[loc] -= cut;
      excess -= cut;
      if (target[loc] <= EPS) delete target[loc];
    }
  }

  const holds: HoldOp[] = [];
  for (const [loc, qty] of Object.entries(target)) {
    const id = Number(loc);
    if (Math.abs((held[id] ?? 0) - qty) > EPS) holds.push({ locationId: id, qty });
  }
  const releases = Object.keys(held)
    .map(Number)
    .filter((loc) => !(loc in target));
  return { target, shortfall, holds, releases };
}

/** Item surat jalan dari tahanan: satu tahanan = satu baris tanpa locationId
 * (server mengalokasikan otomatis); beberapa lokasi = satu baris per lokasi
 * dengan locationId + key masing-masing. Tanpa tahanan = baris polos
 * (alokasi otomatis server tanpa konsumsi reservasi). */
export function deliveryItemsFor(
  line: { productId: number; variantId: number; uom: string; qty: number },
  held: HeldMap,
  cartRef: string,
): {
  productId: number;
  variantId: number;
  uom: string;
  qty: number;
  locationId?: number;
  reservationKey?: string;
}[] {
  const locs = Object.keys(held).map(Number);
  const base = { productId: line.productId, variantId: line.variantId, uom: line.uom };
  if (locs.length === 0) return [{ ...base, qty: line.qty }];
  if (locs.length === 1) {
    return [
      {
        ...base,
        qty: line.qty,
        reservationKey: holdKey(cartRef, line.productId, line.variantId, locs[0]),
      },
    ];
  }
  return locs.map((loc) => ({
    ...base,
    qty: held[loc],
    locationId: loc,
    reservationKey: holdKey(cartRef, line.productId, line.variantId, loc),
  }));
}
