// Logika murni surat jalan cetak (paritas delivery-note-print-page.tsx sistem lama).

/**
 * Label STATUS surat jalan = status pembayaran order (istilah sistem lama),
 * bukan status pengiriman. Surat jalan yang dibatalkan tidak pernah tercetak
 * seolah sah.
 */
export function deliveryStatusLabel(
  noteStatus: string,
  paymentStatus?: string,
  paymentTerms?: string,
): string {
  if (noteStatus === "cancelled") return "DIBATALKAN";
  if (paymentStatus === "paid") return "Lunas";
  if (paymentStatus === "partial") return "Bayar Sebagian";
  if (paymentTerms === "cod") return "Bayar Saat Serah (COD)";
  if (paymentTerms === "net") return "Tempo";
  return "Belum Lunas";
}

interface NoteItemKey {
  productId: number;
  variantId: number;
  uom: string;
}

interface OrderLineKey {
  productId: number;
  variantId: number;
  uom: string;
}

/**
 * Baris SO milik satu baris surat jalan: produk + varian + satuan sama, lalu
 * produk + varian saja (snapshot nama & varian ada di baris SO).
 */
export function deliveryItemLine<L extends OrderLineKey>(item: NoteItemKey, lines: L[]): L | null {
  const same = (l: L) => l.productId === item.productId && l.variantId === item.variantId;
  return lines.find((l) => same(l) && l.uom === item.uom) ?? lines.find(same) ?? null;
}
