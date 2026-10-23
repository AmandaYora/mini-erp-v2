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

// --- Pemecahan halaman (temuan 3, brief desain-cetak-sj-paging) ---

/** Baris barang per lembar surat jalan (permintaan pengguna: "dipecah per 8"), kedua jenis kertas. */
export const SJ_ROWS_PER_PAGE = 8;

/**
 * Kalibrasi tinggi per jenis kertas (diukur lewat tes PDF e2e E25):
 * - charsPerLine: karakter nama produk per baris teks di kolom Produk;
 * - extraLinesBudget: baris teks tambahan (nama panjang yang terbungkus, catatan SJ lebih dari satu
 *   baris) yang masih muat di satu lembar bersama 8 baris barang.
 */
export const SJ_PAGING = {
  continuous: { charsPerLine: 60, extraLinesBudget: 3, notesCharsPerLine: 120 },
  a4: { charsPerLine: 95, extraLinesBudget: 6, notesCharsPerLine: 180 },
} as const;

/** Satu lembar hasil pemecahan. No berlanjut antarlembar (firstNo..lastNo; 0 bila tanpa barang). */
export interface SheetChunk<T> {
  index: number;
  total: number;
  items: T[];
  firstNo: number;
  lastNo: number;
  /** Baris kosong pengisi agar setiap lembar punya rowsPerPage slot. */
  fillers: number;
}

/**
 * Bagi baris menjadi lembar berisi rowsPerPage slot (generik — dokumen lain bisa memakainya).
 * Baris yang memakan teks lebih (extraLines) mengurangi anggaran lembar; bila anggaran habis, baris
 * berikutnya pindah lembar (pengecualian dari "persis N per lembar", hanya untuk nama sangat panjang).
 * Tanpa baris = satu lembar kosong.
 */
export function paginateRows<T>(
  rows: readonly T[],
  opts: { rowsPerPage: number; extraLinesBudget: number; extraLines: (row: T) => number },
): SheetChunk<T>[] {
  const groups: T[][] = [];
  let cur: T[] = [];
  let used = 0;
  for (const row of rows) {
    const extra = Math.max(0, opts.extraLines(row));
    if (cur.length > 0 && (cur.length >= opts.rowsPerPage || used + extra > opts.extraLinesBudget)) {
      groups.push(cur);
      cur = [];
      used = 0;
    }
    cur.push(row);
    used += extra;
  }
  if (cur.length > 0 || groups.length === 0) groups.push(cur);
  let no = 0;
  return groups.map((items, index) => {
    const firstNo = items.length > 0 ? no + 1 : 0;
    no += items.length;
    return {
      index,
      total: groups.length,
      items,
      firstNo,
      lastNo: items.length > 0 ? no : 0,
      fillers: Math.max(0, opts.rowsPerPage - items.length),
    };
  });
}

/** Baris teks tambahan untuk teks sepanjang `length` di kolom selebar `charsPerLine` karakter. */
export function wrapExtraLines(length: number, charsPerLine: number): number {
  return Math.max(0, Math.ceil(length / charsPerLine) - 1);
}

/**
 * Lembar surat jalan: 8 slot per lembar; nama produk panjang dan catatan SJ multi-baris memakan
 * anggaran tinggi yang sama di setiap lembar (catatan tercetak di setiap lembar).
 */
export function paginateDeliveryLines<T>(
  lines: readonly T[],
  opts: { nameOf: (line: T) => string; paper: keyof typeof SJ_PAGING; notes?: string },
): SheetChunk<T>[] {
  const cal = SJ_PAGING[opts.paper];
  const notes = (opts.notes ?? "").trim();
  const notesExtra = notes === "" ? 0 : wrapExtraLines(notes.length, cal.notesCharsPerLine);
  return paginateRows(lines, {
    rowsPerPage: SJ_ROWS_PER_PAGE,
    extraLinesBudget: Math.max(0, cal.extraLinesBudget - notesExtra),
    extraLines: (line) => wrapExtraLines(opts.nameOf(line).length, cal.charsPerLine),
  });
}
