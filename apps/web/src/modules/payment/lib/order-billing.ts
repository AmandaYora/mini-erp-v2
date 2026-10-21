import type { Tone } from "@/shared/lib/format";

// Logika murni ringkasan tagihan order (SO dan PO). Backend mengembalikan
// paidAmount/outstandingAmount/paymentStatus/effectiveDueDate/isOverdue pada
// GET /sales-orders/{id} dan /purchase-orders/{id} (payment_view.go). Sisa
// tagihan = total bersih setelah retur - uang diterima (SL-03; server menghitung).

export type PaymentStatus = "unpaid" | "partial" | "paid";

/** Subset field order yang dibutuhkan; field pembayaran opsional (list/get saja). */
export interface OrderBillingSource {
  status: string;
  grandTotal: number;
  paymentTerms?: string;
  channel?: string;
  paidAmount?: number;
  outstandingAmount?: number;
  paymentStatus?: string;
  effectiveDueDate?: string;
  isOverdue?: boolean;
}

export interface BillingView {
  /** false = server tidak mengirim ringkasan pembayaran (mis. respons mutasi). */
  available: boolean;
  total: number;
  paid: number;
  outstanding: number;
  status: PaymentStatus;
  overdue: boolean;
  /** YYYY-MM-DD, kosong bila order tidak bertempo. */
  dueDate: string;
}

function normalizeStatus(v: string | undefined, paid: number, outstanding: number): PaymentStatus {
  if (v === "paid" || v === "partial" || v === "unpaid") return v;
  if (outstanding <= 0 && paid > 0) return "paid";
  return paid > 0 ? "partial" : "unpaid";
}

export function billingView(o: OrderBillingSource): BillingView {
  const available =
    o.paidAmount !== undefined && o.outstandingAmount !== undefined && o.paymentStatus !== undefined;
  const paid = o.paidAmount ?? 0;
  const outstanding = o.outstandingAmount ?? 0;
  return {
    available,
    total: o.grandTotal,
    paid,
    outstanding,
    status: normalizeStatus(o.paymentStatus, paid, outstanding),
    overdue: o.isOverdue === true,
    dueDate: (o.effectiveDueDate ?? "").slice(0, 10),
  };
}

/** Badge status bayar; jatuh tempo terlewat menimpa status belum lunas. */
export function billingBadge(b: BillingView): { label: string; tone: Tone } {
  if (b.status === "paid") return { label: "Lunas", tone: "success" };
  if (b.overdue) return { label: "Jatuh tempo", tone: "danger" };
  if (b.status === "partial") return { label: "Bayar sebagian", tone: "warning" };
  return { label: "Belum bayar", tone: "neutral" };
}

/** Pembayaran dicatat hanya pada order terkonfirmasi/selesai yang masih punya sisa. */
export function canRecordPayment(o: OrderBillingSource): boolean {
  const b = billingView(o);
  return b.available && (o.status === "confirmed" || o.status === "completed") && b.outstanding > 0;
}

/** Order prabayar yang belum lunas: server menolak SJ/penerimaan. */
export function isPrepaidBlocked(o: OrderBillingSource): boolean {
  const b = billingView(o);
  return o.paymentTerms === "prepaid" && b.available && b.outstanding > 0;
}

/** Surat jalan baru: SO terkonfirmasi, bukan POS (diserahkan di kasir), bukan prabayar belum lunas. */
export function canCreateDelivery(o: OrderBillingSource): boolean {
  return o.status === "confirmed" && o.channel !== "pos" && !isPrepaidBlocked(o);
}

/** Penerimaan barang: PO terkonfirmasi dan bukan prabayar belum lunas. */
export function canCreateReceipt(o: OrderBillingSource): boolean {
  return o.status === "confirmed" && !isPrepaidBlocked(o);
}

/**
 * Retur hanya bermakna setelah ada barang keluar/masuk: order selesai, atau
 * terkonfirmasi dengan minimal satu surat jalan/penerimaan terposting.
 */
export function canCreateReturn(o: OrderBillingSource, hasFulfilledDocument: boolean): boolean {
  return o.status === "completed" || (o.status === "confirmed" && hasFulfilledDocument);
}

/**
 * Setujui kredit (alihkan ke tempo): hanya SO terkonfirmasi bertermin COD
 * yang belum lunas (sales/application ApproveCredit).
 */
export function canSwitchToNet(o: OrderBillingSource): boolean {
  const b = billingView(o);
  // cod ATAU prabayar: sama dengan SwitchToNet/ApproveCredit di server (SL-10).
  return (
    o.status === "confirmed" &&
    (o.paymentTerms === "cod" || o.paymentTerms === "prepaid") &&
    (!b.available || b.outstanding > 0)
  );
}

/** Judul tombol cetak di detail SO. */
export function printButtonLabel(b: BillingView): "Cetak Nota" | "Cetak Tagihan" {
  return b.available && b.outstanding > 0 ? "Cetak Tagihan" : "Cetak Nota";
}

/** Nominal awal form pembayaran = seluruh sisa tagihan. */
export function defaultPaymentAmount(b: BillingView): number {
  return b.outstanding > 0 ? b.outstanding : 0;
}
