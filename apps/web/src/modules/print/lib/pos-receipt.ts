// Baris pembayaran struk POS dari pembayaran TERSIMPAN (bukan state klien):
// cetak ulang memberi angka yang sama. Backend mengembalikan amountTendered
// dan change (turunan) pada paymentView; tunai diterima hanya untuk metode cash.

import { methodLabel } from "@/modules/payment/types";

export interface ReceiptPayment {
  method: string;
  amount: number;
  amountTendered: number;
  change: number;
}

export interface ReceiptPaymentLines {
  paid: number | null;
  /** Hanya tunai dengan uang diterima tercatat. */
  tendered: number | null;
  change: number | null;
  outstanding: number | null;
  method: string;
}

export function receiptPaymentLines(
  payment: ReceiptPayment | null,
  orderTotal: number,
  paymentTerms: string,
): ReceiptPaymentLines {
  if (payment === null) {
    return {
      paid: null,
      tendered: null,
      change: null,
      outstanding: null,
      method: paymentTerms === "net" ? "Tempo" : "-",
    };
  }
  const isCash = payment.method === "cash";
  const hasTendered = isCash && payment.amountTendered > 0;
  const outstanding = Math.max(0, orderTotal - payment.amount);
  return {
    paid: payment.amount,
    tendered: hasTendered ? payment.amountTendered : null,
    // Kembalian dari server; bila kosong turunkan dari selisih (tidak pernah negatif).
    change: hasTendered ? (payment.change > 0 ? payment.change : Math.max(0, payment.amountTendered - payment.amount)) : null,
    outstanding: outstanding > 0 ? outstanding : null,
    method: methodLabel(payment.method),
  };
}

interface ReceiptLine {
  qty: number;
  unitPrice: number;
  lineTotal: number;
}

interface ReceiptOrder<L extends ReceiptLine> {
  subtotal: number;
  discountTotal: number;
  grandTotal: number;
  items: L[];
}

export interface ReceiptView<L extends ReceiptLine, P extends ReceiptPayment | null> {
  subtotal: number;
  /** null = baris diskon tidak dicetak. */
  discount: number | null;
  total: number;
  /** Jumlah baris yang dicetak: bruto bila diskon disembunyikan. */
  lineAmount: (line: L) => number;
  payment: P;
}

/**
 * Angka struk menurut pilihan "Sembunyikan diskon" (SL-05, paritas legacy):
 * harga penuh per baris, tanpa baris diskon; total dan pembayaran yang
 * melunasi naik sebesar diskon sehingga sisa tagihan tetap riil.
 */
export function receiptView<L extends ReceiptLine, P extends ReceiptPayment | null>(
  order: ReceiptOrder<L>,
  payment: P,
  hideDiscount: boolean,
): ReceiptView<L, P> {
  const gross = (l: L) => Math.round(l.qty * l.unitPrice);
  if (!hideDiscount) {
    return {
      subtotal: order.subtotal,
      discount: order.discountTotal > 0 ? order.discountTotal : null,
      total: order.grandTotal,
      lineAmount: (l) => l.lineTotal,
      payment,
    };
  }
  const raise = order.discountTotal > 0 ? order.discountTotal : 0;
  const settled = payment !== null && payment.amount >= order.grandTotal;
  return {
    subtotal: order.items.reduce((s, l) => s + gross(l), 0),
    discount: null,
    total: order.grandTotal + raise,
    lineAmount: gross,
    payment: (settled && payment !== null ? { ...payment, amount: payment.amount + raise } : payment) as P,
  };
}
