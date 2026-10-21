// Logika murni Bukti Pembayaran (paritas payment-receipt-print-page.tsx sistem
// lama): ringkasan order yang dibayar — Total Tagihan, Jumlah Pembayaran Ini,
// Total Terbayar Setelah Ini, Sisa Setelah Pembayaran — dihitung dari riwayat
// pembayaran order itu berurutan waktu, sampai dan termasuk pembayaran ini.

export interface ReceiptAllocation {
  orderType: string;
  orderId: number;
  amount: number;
}

export interface ReceiptPaymentLike {
  id: number;
  paidAt: string;
  status: string;
  allocations: ReceiptAllocation[];
}

/** Order yang dibayar satu pembayaran — hanya bila dialokasi ke tepat satu order. */
export function receiptOrderRef(p: { allocations: ReceiptAllocation[] }): ReceiptAllocation | null {
  return p.allocations.length === 1 ? p.allocations[0] : null;
}

/** "Customer" untuk tagihan jual/retur jual, "Supplier" untuk beli/retur beli. */
export function receiptPartySide(orderType: string | undefined, partyType: string, direction: string): "customer" | "supplier" {
  if (orderType === "sales" || orderType === "sales_return") return "customer";
  if (orderType === "purchase" || orderType === "purchase_return") return "supplier";
  if (partyType === "customer" || partyType === "supplier") return partyType;
  return direction === "in" ? "customer" : "supplier";
}

export interface ReceiptSummary {
  totalAmount: number;
  thisAmount: number;
  paidUntilThis: number;
  outstandingAfter: number;
}

function paidTime(p: ReceiptPaymentLike): number {
  // Server mengirim "YYYY-MM-DD hh:mm:ss"; ganti spasi agar terurai di semua mesin JS.
  const v = p.paidAt.includes(" ") && !p.paidAt.includes("T") ? p.paidAt.replace(" ", "T") : p.paidAt;
  const t = new Date(v).getTime();
  return Number.isNaN(t) ? 0 : t;
}

/**
 * Ringkasan setelah pembayaran ini atas satu order. Pembayaran batal tidak
 * dihitung; urutan = tanggal bayar lalu id (catatan sama-waktu tetap stabil).
 * Pembayaran ini selalu ikut dihitung (dari data detailnya), walau riwayat
 * tidak memuatnya.
 */
export function receiptSummary(
  payment: ReceiptPaymentLike,
  ref: ReceiptAllocation,
  orderTotal: number,
  history: ReceiptPaymentLike[],
): ReceiptSummary {
  const amountFor = (p: ReceiptPaymentLike) =>
    p.allocations
      .filter((a) => a.orderType === ref.orderType && a.orderId === ref.orderId)
      .reduce((sum, a) => sum + a.amount, 0);
  // Pembayaran ini selalu dari data detailnya (alokasi lengkap), bukan salinan di riwayat.
  const all = [...history.filter((p) => p.id !== payment.id), payment];
  const before = (p: ReceiptPaymentLike) =>
    paidTime(p) < paidTime(payment) || (paidTime(p) === paidTime(payment) && p.id <= payment.id);
  const paidUntilThis = all
    .filter((p) => p.status !== "cancelled" && before(p))
    .reduce((sum, p) => sum + amountFor(p), 0);
  return {
    totalAmount: orderTotal,
    thisAmount: ref.amount,
    paidUntilThis,
    outstandingAfter: Math.max(0, orderTotal - paidUntilThis),
  };
}
