// Tautan cetak di layar sukses POS (SL-05/SL-06). Murni agar bisa diuji.

export interface ReceiptLinkInput {
  orderId: number;
  paymentId?: number | null;
  /** Pilihan "Sembunyikan diskon" saat checkout: ikut ke struk yang dicetak. */
  hideDiscount: boolean;
}

/** Struk termal: ?paymentId= dan ?hideDiscount=1 (dibaca PosThermalReceiptPage). */
export function posReceiptHref({ orderId, paymentId, hideDiscount }: ReceiptLinkInput): string {
  const q = new URLSearchParams();
  if (paymentId != null) q.set("paymentId", String(paymentId));
  if (hideDiscount) q.set("hideDiscount", "1");
  const qs = q.toString();
  return qs === "" ? `/print/pos/${orderId}` : `/print/pos/${orderId}?${qs}`;
}

/** Nota/tagihan penuh dari order (judul menurut status bayar); ikut menyembunyikan diskon. */
export function invoiceHref({ orderId, hideDiscount }: ReceiptLinkInput): string {
  return hideDiscount ? `/print/sales/${orderId}?hideDiscount=1` : `/print/sales/${orderId}`;
}

/** Bukti bayar (kwitansi); null bila transaksi tidak punya pembayaran tercatat. */
export function paymentProofHref({ paymentId }: ReceiptLinkInput): string | null {
  return paymentId != null ? `/print/payments/${paymentId}` : null;
}
