// Logika murni riwayat dokumen terkait satu order (SO/PO): pengiriman atau
// penerimaan, pembayaran, dan retur — digabung jadi satu garis waktu.
// Sumber data = endpoint daftar per modul yang sudah ada (tanpa join lintas
// modul di backend); fungsi di sini hanya memetakan dan mengurutkan.

export type HistoryKind = "delivery" | "receipt" | "payment" | "return" | "replacement";

export interface HistoryEvent {
  key: string;
  kind: HistoryKind;
  id: number;
  number: string;
  /** YYYY-MM-DD atau ISO; dipakai untuk urutan. */
  date: string;
  status: string;
  /** Nominal (pembayaran/retur); undefined untuk dokumen barang. */
  amount?: number;
  /** Teks pendukung singkat (mis. metode bayar, alasan retur). */
  note?: string;
  href: string;
}

export const HISTORY_KIND_LABEL: Record<HistoryKind, string> = {
  delivery: "Surat jalan",
  receipt: "Penerimaan",
  payment: "Pembayaran",
  return: "Retur",
  replacement: "Pengiriman pengganti",
};

export interface DeliveryLike {
  id: number;
  number: string;
  deliveryDate: string;
  status: string;
  documentKind?: string;
  items: { productId: number; variantId: number; uom: string; qty: number }[];
}

export interface ReceiptLike {
  id: number;
  number: string;
  receivedAt: string;
  status: string;
  items: { productId: number; variantId: number; uom: string; qty: number }[];
}

export interface PaymentLike {
  id: number;
  number: string;
  paidAt: string;
  status: string;
  method: string;
  amount: number;
  allocations: { orderType: string; orderId: number; amount: number }[];
}

export interface ReturnLike {
  id: number;
  number: string;
  returnDate: string;
  status: string;
  total: number;
  reason?: string;
}

export function deliveryEvents(notes: DeliveryLike[]): HistoryEvent[] {
  return notes.map((n) => ({
    key: `delivery:${n.id}`,
    kind: n.documentKind === "replacement" ? "replacement" : "delivery",
    id: n.id,
    number: n.number,
    date: n.deliveryDate,
    status: n.status,
    href: `/deliveries/${n.id}`,
  }));
}

export function receiptEvents(receipts: ReceiptLike[]): HistoryEvent[] {
  return receipts.map((r) => ({
    key: `receipt:${r.id}`,
    kind: "receipt" as const,
    id: r.id,
    number: r.number,
    date: r.receivedAt,
    status: r.status,
    href: `/goods-receipts/${r.id}`,
  }));
}

/** Jumlah alokasi sebuah pembayaran ke order tertentu. */
export function allocatedToOrder(p: PaymentLike, orderType: "sales" | "purchase", orderId: number): number {
  return p.allocations
    .filter((a) => a.orderType === orderType && a.orderId === orderId)
    .reduce((sum, a) => sum + a.amount, 0);
}

/** Hanya pembayaran yang benar-benar mengalokasi ke order ini. */
export function paymentEvents(
  payments: PaymentLike[],
  orderType: "sales" | "purchase",
  orderId: number,
): HistoryEvent[] {
  const out: HistoryEvent[] = [];
  for (const p of payments) {
    const amount = allocatedToOrder(p, orderType, orderId);
    if (amount <= 0) continue;
    out.push({
      key: `payment:${p.id}`,
      kind: "payment",
      id: p.id,
      number: p.number,
      date: p.paidAt,
      status: p.status,
      amount,
      note: p.method,
      href: `/payments/${p.id}`,
    });
  }
  return out;
}

/** Total alokasi pembayaran AKTIF ke order — dasar berhenti membaca buku pihak. */
export function activeAllocatedTotal(
  payments: PaymentLike[],
  orderType: "sales" | "purchase",
  orderId: number,
): number {
  return payments
    .filter((p) => p.status === "active")
    .reduce((sum, p) => sum + allocatedToOrder(p, orderType, orderId), 0);
}

export function returnEvents(kind: "sales" | "purchase", returns: ReturnLike[]): HistoryEvent[] {
  return returns.map((r) => ({
    key: `return:${kind}:${r.id}`,
    kind: "return" as const,
    id: r.id,
    number: r.number,
    date: r.returnDate,
    status: r.status,
    amount: r.total,
    note: r.reason,
    href: kind === "sales" ? `/sales-returns/${r.id}` : `/purchase-returns/${r.id}`,
  }));
}

/** Terbaru dulu; tanggal sama → id lebih besar dulu; stabil lintas jenis. */
export function sortHistory(events: HistoryEvent[]): HistoryEvent[] {
  return [...events].sort((a, b) => {
    if (a.date !== b.date) return a.date < b.date ? 1 : -1;
    if (a.id !== b.id) return b.id - a.id;
    return a.key < b.key ? -1 : 1;
  });
}

/** True bila ada dokumen barang yang sudah benar-benar bergerak. */
export function hasFulfilledDocument(events: HistoryEvent[]): boolean {
  return events.some(
    (e) =>
      (e.kind === "delivery" && e.status === "confirmed") ||
      (e.kind === "receipt" && e.status === "posted"),
  );
}

// --- progres per baris (terkirim/diterima) ---

export interface LineKeyed {
  productId: number;
  variantId: number;
}

export function lineKey(l: LineKeyed): string {
  return `${l.productId}/${l.variantId}`;
}

export interface LineProgress {
  /** Sudah keluar/masuk stok (SJ confirmed / penerimaan posted). */
  done: number;
  /** Menunggu (SJ draf / penerimaan pending): ikut mengurangi sisa yang boleh dibuat. */
  pending: number;
}

/**
 * Akumulasi qty per produk/varian dari SJ atau penerimaan. SJ pengganti
 * retur tukar dan dokumen batal/void tidak dihitung terhadap order.
 */
export function accumulateProgress(
  docs: { status: string; documentKind?: string; items: { productId: number; variantId: number; qty: number }[] }[],
  doneStatus: "confirmed" | "posted",
  pendingStatus: "draft" | "pending",
): Map<string, LineProgress> {
  const map = new Map<string, LineProgress>();
  for (const d of docs) {
    if (d.documentKind === "replacement") continue;
    const isDone = d.status === doneStatus;
    const isPending = d.status === pendingStatus;
    if (!isDone && !isPending) continue;
    for (const it of d.items) {
      const k = lineKey(it);
      const cur = map.get(k) ?? { done: 0, pending: 0 };
      if (isDone) cur.done += it.qty;
      else cur.pending += it.qty;
      map.set(k, cur);
    }
  }
  return map;
}

/** Sisa yang masih boleh dibuatkan dokumen (qty order - selesai - menunggu), tak negatif. */
export function remainingQty(orderQty: number, p: LineProgress | undefined): number {
  const used = (p?.done ?? 0) + (p?.pending ?? 0);
  const rest = orderQty - used;
  return rest > 0 ? Math.round(rest * 1e6) / 1e6 : 0;
}
