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

export interface HideDiscountOption {
  /** Centang tampil: ada diskon baris atau harga member yang bisa disembunyikan (seperti sistem lama). */
  visible: boolean;
  /** Boleh dicentang: transaksi lunas saat itu (ADR-0018 D12). */
  allowed: boolean;
}

/**
 * Opsi "Sembunyikan diskon" di layar sukses POS. Hanya tampil bila ada diskon,
 * dan hanya aktif bila transaksi lunas: bayar nanti, pembayaran yang gagal
 * tercatat, atau tunai yang kurang dari total server membuat tagihan masih
 * bersisa — tagihan harus menunjukkan jumlah yang benar-benar terutang.
 */
export function hideDiscountOption(input: {
  hasDiscount: boolean;
  payLater: boolean;
  paymentPending: boolean;
  change: number;
}): HideDiscountOption {
  const settled = !input.payLater && !input.paymentPending && input.change >= 0;
  return { visible: input.hasDiscount, allowed: input.hasDiscount && settled };
}
