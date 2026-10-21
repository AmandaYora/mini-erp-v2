// Logika murni angka nota/faktur & struk penjualan — sama persis dengan
// sales-print-shared.tsx sistem lama (identitas dokumen ke pelanggan):
// - harga satuan tercetak = harga penuh sebelum diskon; harga member tercetak
//   dengan harga katalognya dan selisihnya sebagai diskon;
// - kolom Disc = nominal diskon baris; "Jumlah (Rp)" = bruto (harga penuh x qty);
// - "sembunyikan diskon": baris diskon hilang, total & pembayaran naik ke harga
//   penuh. HANYA untuk dokumen yang sudah lunas (ADR-0018 D12): tagihan yang
//   masih bersisa harus menunjukkan jumlah yang benar-benar terutang, jadi
//   permintaan menyembunyikan diskon di sana diabaikan (hideDiscountAllowed);
// - judul menurut sisa tagihan: Nota Penjualan Lunas / Tagihan - Bayar
//   Sebagian / Tagihan Penjualan.

export const HIDE_DISCOUNT_PARAM = "hideDiscount";

/** Penjelasan saat opsi sembunyikan diskon tidak tersedia (dokumen belum lunas). */
export const HIDE_DISCOUNT_UNPAID_NOTE = "Tagihan harus menunjukkan jumlah yang benar-benar terutang";

/**
 * Diskon boleh disembunyikan hanya bila server menyatakan order lunas (sisa 0).
 * Tanpa ringkasan bayar dari server, lunas tidak bisa dipastikan -> tidak boleh.
 */
export function hideDiscountAllowed(o: { paidAmount?: number; outstandingAmount?: number }): boolean {
  return o.paidAmount !== undefined && o.outstandingAmount !== undefined && o.outstandingAmount <= 0;
}

/** Permintaan sembunyikan diskon yang benar-benar berlaku untuk order ini. */
export function effectiveHideDiscount(requested: boolean, o: { paidAmount?: number; outstandingAmount?: number }): boolean {
  return requested && hideDiscountAllowed(o);
}

export function readHideDiscount(params: URLSearchParams): boolean {
  return params.get(HIDE_DISCOUNT_PARAM) === "1";
}

export function withHideDiscountParam(params: URLSearchParams, hide: boolean): URLSearchParams {
  const next = new URLSearchParams(params);
  if (hide) next.set(HIDE_DISCOUNT_PARAM, "1");
  else next.delete(HIDE_DISCOUNT_PARAM);
  return next;
}

export interface DocLine {
  qty: number;
  /** Harga satuan sebelum diskon baris (harga member untuk baris member). */
  unitPrice: number;
  discountPct: number;
  /** Diskon nominal untuk seluruh baris (bukan per unit). */
  discountNominal: number;
  /** Total baris dari server: setelah diskon, termasuk pajak bila pajak di luar harga. */
  lineTotal: number;
  pricingSource?: string;
  /** Harga katalog saat order dibuat. */
  basePrice?: number;
}

export interface DocOrder {
  grandTotal: number;
  discountTotal: number;
  taxTotal: number;
  dueDate: string;
  paymentTerms?: string;
  paidAmount?: number;
  outstandingAmount?: number;
  paymentStatus?: string;
  effectiveDueDate?: string;
  items: DocLine[];
}

/** Harga satuan tercetak: harga katalog bila harga member lebih murah darinya. */
export function linePrintUnitPrice(l: DocLine): number {
  const base = l.basePrice ?? 0;
  if (l.pricingSource === "member_rule" && base > l.unitPrice) return base;
  return l.unitPrice;
}

/** Bruto tercetak satu baris = harga satuan tercetak x qty (rupiah bulat). */
export function linePrintSubtotal(l: DocLine): number {
  return Math.round(l.qty * linePrintUnitPrice(l));
}

/** Diskon satu baris: diskon baris (persen + nominal) + selisih harga member. */
export function lineDiscountTotal(l: DocLine): number {
  const gross = Math.round(l.qty * l.unitPrice);
  const lineDiscount = Math.round((gross * l.discountPct) / 100) + l.discountNominal;
  const memberDiscount = linePrintSubtotal(l) - gross;
  return Math.max(0, lineDiscount + memberDiscount);
}

/** "Jumlah (Rp)" / Subtotal struk: bruto seluruh baris. */
export function orderPrintSubtotal(o: DocOrder): number {
  return o.items.reduce((sum, l) => sum + linePrintSubtotal(l), 0);
}

/** Diskon tercetak: jumlah diskon baris, atau diskon order bila tak ada di baris. */
export function orderPrintDiscount(o: DocOrder): number {
  const lines = o.items.reduce((sum, l) => sum + lineDiscountTotal(l), 0);
  return lines > 0 ? lines : o.discountTotal;
}

export function hasLineDiscount(items: DocLine[]): boolean {
  return items.some((l) => lineDiscountTotal(l) > 0);
}

/** Kolom "Harga Jual": total baris, atau harga penuhnya bila diskon disembunyikan (nilai berlaku). */
export function lineSellingAmount(l: DocLine, hideDiscount: boolean): number {
  return hideDiscount ? l.lineTotal + lineDiscountTotal(l) : l.lineTotal;
}

export interface DocTotals {
  /** Baris "Jumlah (Rp)": selalu bruto. */
  grossSubtotal: number;
  /** null = baris Diskon tidak dicetak. */
  discount: number | null;
  /** null = baris Pajak tidak dicetak. */
  tax: number | null;
  /** Total tercetak (naik ke harga penuh bila diskon disembunyikan). */
  total: number;
  /** null = baris Sudah Dibayar tidak dicetak (belum ada pembayaran). */
  paid: number | null;
  outstanding: number;
  /** Nilai riil (untuk judul & jatuh tempo), tak terpengaruh opsi cetak. */
  realPaid: number;
  realOutstanding: number;
}

export function documentTotals(o: DocOrder, hideRequested: boolean): DocTotals {
  const hideDiscount = effectiveHideDiscount(hideRequested, o);
  const realPaid = Math.max(0, o.paidAmount ?? 0);
  const realOutstanding = Math.max(0, o.outstandingAmount ?? o.grandTotal - realPaid);
  const printDiscount = orderPrintDiscount(o);
  const tax = o.taxTotal > 0 ? o.taxTotal : null;
  if (!hideDiscount) {
    return {
      grossSubtotal: orderPrintSubtotal(o),
      discount: printDiscount > 0 ? printDiscount : null,
      tax,
      total: o.grandTotal,
      paid: realPaid > 0 ? realPaid : null,
      outstanding: realOutstanding,
      realPaid,
      realOutstanding,
    };
  }
  // Lunas (dijamin hideDiscountAllowed): total & yang dibayar naik ke harga penuh, sisa 0.
  return {
    grossSubtotal: orderPrintSubtotal(o),
    discount: null,
    tax,
    total: o.grandTotal + printDiscount,
    paid: realPaid > 0 ? realPaid + printDiscount : null,
    outstanding: 0,
    realPaid,
    realOutstanding,
  };
}

/** Judul nota menurut sisa tagihan riil (sistem lama getSalesDocumentTitle). */
export function salesDocumentTitle(t: Pick<DocTotals, "realPaid" | "realOutstanding">): string {
  if (t.realOutstanding <= 0) return "Nota Penjualan Lunas";
  if (t.realPaid > 0) return "Tagihan - Bayar Sebagian";
  return "Tagihan Penjualan";
}

/** Tgl. Jtp tercetak: hanya selagi masih ada sisa tagihan dan order punya jatuh tempo. */
export function dueDateCell(o: DocOrder): string | null {
  const { realOutstanding } = documentTotals(o, false);
  if (realOutstanding <= 0) return null;
  const due = o.dueDate.trim() !== "" ? o.dueDate : (o.effectiveDueDate ?? "").slice(0, 10);
  return due.trim() === "" ? null : due;
}
