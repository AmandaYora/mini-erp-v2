// Logika murni isi nota/faktur penjualan (paritas legacy sales-document-print):
// judul menurut status bayar, baris Sudah Dibayar/Sisa Tagihan, jatuh tempo
// "-" bila lunas atau tanpa tempo, dan opsi sembunyikan diskon (harga penuh,
// kolom diskon "-", jumlah naik ke bruto; total tagihan tetap yang riil).

import { billingView, salesDocumentTitle } from "@/modules/payment/lib/order-billing";
import type { OrderBillingSource } from "@/modules/payment/lib/order-billing";

export const HIDE_DISCOUNT_PARAM = "hideDiscount";

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
  unitPrice: number;
  discountPct: number;
  discountNominal: number;
  lineTotal: number;
}

export interface DocOrder extends OrderBillingSource {
  subtotal: number;
  discountTotal: number;
  taxTotal: number;
  grandTotal: number;
  dueDate: string;
  items: DocLine[];
}

export function documentTitle(o: OrderBillingSource): string {
  return salesDocumentTitle(billingView(o));
}

/** Harga bruto satu baris = qty x harga satuan (bulat rupiah). */
export function grossLine(l: DocLine): number {
  return Math.round(l.qty * l.unitPrice);
}

export function hasLineDiscount(items: DocLine[]): boolean {
  return items.some((l) => l.discountNominal > 0 || l.discountPct > 0);
}

/** Teks kolom diskon; "-" bila tak ada atau disembunyikan. */
export function discountCell(l: DocLine, hide: boolean, fmtMoney: (n: number) => string, fmtNum: (n: number) => string): string {
  if (hide) return "-";
  if (l.discountNominal > 0) return fmtMoney(l.discountNominal);
  if (l.discountPct > 0) return `${fmtNum(l.discountPct)}%`;
  return "-";
}

/** Jumlah baris yang dicetak: bruto bila diskon disembunyikan, selain itu netto server. */
export function lineAmount(l: DocLine, hide: boolean): number {
  return hide ? grossLine(l) : l.lineTotal;
}

export interface DocTotals {
  subtotal: number;
  /** null = baris diskon tidak dicetak. */
  discount: number | null;
  tax: number | null;
  total: number;
  paid: number | null;
  outstanding: number | null;
}

export function documentTotals(o: DocOrder, hideDiscount: boolean): DocTotals {
  const b = billingView(o);
  const subtotal = hideDiscount ? o.items.reduce((sum, l) => sum + grossLine(l), 0) : o.subtotal;
  return {
    subtotal,
    discount: !hideDiscount && o.discountTotal > 0 ? o.discountTotal : null,
    tax: o.taxTotal > 0 ? o.taxTotal : null,
    total: o.grandTotal,
    paid: b.available && b.paid > 0 ? b.paid : null,
    outstanding: b.available && o.status !== "draft" && o.status !== "cancelled" ? b.outstanding : null,
  };
}

/** Jatuh tempo cetak: "-" bila lunas atau order tidak bertempo. */
export function dueDateCell(o: OrderBillingSource & { dueDate: string }): string | null {
  const b = billingView(o);
  if (b.available && b.status === "paid") return null;
  const due = b.dueDate !== "" ? b.dueDate : o.paymentTerms === "net" ? o.dueDate : "";
  return due.trim() === "" ? null : due;
}
