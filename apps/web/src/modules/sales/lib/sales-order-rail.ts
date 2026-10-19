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

export interface SalesOrderRailInput {
  status: string;
  channel: string;
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
  if (i.status === "draft") return "draft";
  if (i.status === "completed") return "done";
  if (i.status === "cancelled") return i.events.length > 0 ? "confirmed" : "draft";
  // Order POS diserahkan di kasir: tidak ada surat jalan, dianggap sudah dikirim.
  const shipped = i.channel === "pos" || i.events.some((e) => e.kind === "delivery" && e.status === "confirmed");
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
    steps: SALES_ORDER_STEPS,
    current: salesOrderRailCurrent(i),
    stopped: i.status === "cancelled",
    related: salesOrderRelated(i.events),
  };
}
