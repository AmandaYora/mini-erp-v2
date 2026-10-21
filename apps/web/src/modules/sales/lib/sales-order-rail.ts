import type { RelatedDocument, StepperStep } from "@/shared/components/patterns";
import { statusLabel, statusTone } from "@/shared/lib/format";
import type { BillingView } from "@/modules/payment/lib/order-billing";
import type { HistoryEvent } from "@/modules/payment/lib/order-history";
import { HISTORY_KIND_LABEL } from "@/modules/payment/lib/order-history";

// Definisi Rel Dokumen order jual: Draf → Terkonfirmasi → Dikirim → Lunas → Selesai.
// Fungsi murni: hanya memakai data yang sudah dimuat halaman detail (status order,
// ringkasan tagihan, riwayat dokumen terkait) — tanpa endpoint baru.

export const SALES_ORDER_STEPS: StepperStep[] = [
  { key: "draft", label: "Draf" },
  { key: "confirmed", label: "Terkonfirmasi" },
  { key: "shipped", label: "Dikirim" },
  { key: "paid", label: "Lunas" },
  { key: "done", label: "Selesai" },
];

/** Order dari SJ Langsung (ADR-0022): barang sudah keluar sebelum order ada, jadi rel
 * dimulai dari Dikirim dan tidak pernah menunggu surat jalan. */
export const SALES_ORDER_FROM_DELIVERY_STEPS: StepperStep[] = [
  { key: "shipped", label: "Dikirim" },
  { key: "confirmed", label: "Terkonfirmasi" },
  { key: "paid", label: "Lunas" },
  { key: "done", label: "Selesai" },
];

export interface SalesOrderRailInput {
  status: string;
  channel: string;
  /** standard | delivery; kosong = standard. */
  origin?: string;
  billing: BillingView;
  events: readonly HistoryEvent[];
}

export interface SalesOrderRail {
  steps: StepperStep[];
  current: string;
  stopped: boolean;
  related: RelatedDocument[];
}

/** Langkah aktif = kemajuan terjauh yang bisa dibuktikan dari data yang ada. */
export function salesOrderRailCurrent(i: SalesOrderRailInput): string {
  const fromDelivery = i.origin === "delivery";
  if (i.status === "draft") return fromDelivery ? "shipped" : "draft";
  if (i.status === "completed") return "done";
  if (i.status === "cancelled") return fromDelivery ? "shipped" : i.events.length > 0 ? "confirmed" : "draft";
  // Order POS diserahkan di kasir dan order dari SJ Langsung lahir setelah barang keluar:
  // keduanya tanpa menunggu surat jalan, dianggap sudah dikirim.
  const shipped =
    i.channel === "pos" || fromDelivery || i.events.some((e) => e.kind === "delivery" && e.status === "confirmed");
  const paid = i.billing.available && i.billing.status === "paid";
  if (shipped && paid) return "paid";
  if (shipped) return "shipped";
  return "confirmed";
}

export function salesOrderRelated(events: readonly HistoryEvent[]): RelatedDocument[] {
  return events.map((e) => ({
    label: e.number || `#${e.id}`,
    kind: HISTORY_KIND_LABEL[e.kind],
    to: e.href,
    status: { label: statusLabel(e.status), tone: statusTone(e.status) },
  }));
}

export function salesOrderRail(i: SalesOrderRailInput): SalesOrderRail {
  return {
    steps: i.origin === "delivery" ? SALES_ORDER_FROM_DELIVERY_STEPS : SALES_ORDER_STEPS,
    current: salesOrderRailCurrent(i),
    stopped: i.status === "cancelled",
    related: salesOrderRelated(i.events),
  };
}
