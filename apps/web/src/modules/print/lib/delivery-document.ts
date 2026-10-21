// Logika murni surat jalan cetak (paritas delivery-note-print-page.tsx sistem lama).

/**
 * Label STATUS surat jalan = status pembayaran order (istilah sistem lama),
 * bukan status pengiriman. Surat jalan yang dibatalkan tidak pernah tercetak
 * seolah sah. SJ Langsung yang belum diorder (ADR-0022) belum punya tagihan:
 * "Belum Diorder".
 */
export function deliveryStatusLabel(
  noteStatus: string,
  paymentStatus?: string,
  paymentTerms?: string,
  unorderedDirect = false,
): string {
  if (noteStatus === "cancelled") return "DIBATALKAN";
  if (unorderedDirect) return "Belum Diorder";
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

interface ProductLikeVariant {
  id: number;
  code: string;
  name: string;
  isHidden?: boolean;
}

interface ProductLike {
  id: number;
  code: string;
  name: string;
  variants: ProductLikeVariant[];
}

export interface PrintableLine {
  productId: number;
  variantId: number;
  uom: string;
  productName: string;
  productCode: string;
  variantName: string;
  variantCode: string;
}

/**
 * Baris cetak dari master produk untuk SJ tanpa SO (SJ Langsung belum
 * diorder): nama/kode produk + varian bila bukan varian default tersembunyi.
 * Bentuknya sama dengan baris SO agar `printItemName` dipakai apa adanya.
 */
export function productLinesForNote<P extends ProductLike>(
  items: NoteItemKey[],
  products: P[],
): PrintableLine[] {
  const byId = new Map(products.map((p) => [p.id, p] as const));
  const out: PrintableLine[] = [];
  for (const it of items) {
    const p = byId.get(it.productId);
    if (!p) continue;
    const v = p.variants.find((x) => x.id === it.variantId);
    const hidden = v?.isHidden === true;
    out.push({
      productId: it.productId,
      variantId: it.variantId,
      uom: it.uom,
      productName: p.name,
      productCode: p.code,
      variantName: v && !hidden ? v.name : "",
      variantCode: v && !hidden ? v.code : "",
    });
  }
  return out;
}

/**
 * Identitas pihak di kepala SJ: dari SO bila ada, else snapshot pelanggan dan
 * alamat kirim pada SJ (SJ Langsung belum diorder).
 */
export function deliveryPartyBlock(input: {
  orderPartyName?: string | null;
  orderShipToRecipient?: string | null;
  notePartyName?: string | null;
  noteShipToRecipient?: string | null;
}): { partyName: string; shipToRecipient: string } {
  const orderName = (input.orderPartyName ?? "").trim();
  if (orderName !== "") {
    return { partyName: orderName, shipToRecipient: (input.orderShipToRecipient ?? "").trim() };
  }
  const noteName = (input.notePartyName ?? "").trim();
  return {
    partyName: noteName !== "" ? noteName : "-",
    shipToRecipient: (input.noteShipToRecipient ?? "").trim(),
  };
}
