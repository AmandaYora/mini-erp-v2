import type { RelatedDocument, StepperStep } from "@/shared/components/patterns";
import { statusLabel, statusTone } from "@/shared/lib/format";

// Rel Dokumen surat jalan: Draf → Dikirim → Diterima. SJ Langsung (ADR-0022)
// menambah langkah "Diorder" setelah Diterima: order dibuat belakangan dari
// SJ, dan SJ berlabel "Belum diorder" sampai tertaut (salesOrderId > 0).
// Fungsi murni dari data yang sudah dimuat halaman detail (surat jalan +
// order jual); tanpa endpoint baru.

export const DELIVERY_STEPS: StepperStep[] = [
  { key: "draft", label: "Draf" },
  { key: "dispatched", label: "Dikirim" },
  { key: "received", label: "Diterima" },
];

export const DIRECT_DELIVERY_STEPS: StepperStep[] = [...DELIVERY_STEPS, { key: "ordered", label: "Diorder" }];

export interface DeliveryRailInput {
  status: string;
  dispatchedAt: string | null;
  salesOrderId: number;
  orderNumber: string;
  /** Status order jual bila berhasil dimuat. */
  orderStatus?: string | null;
  salesReturnId?: number | null;
  /** "direct" = SJ Langsung; selain itu SJ order biasa. */
  origin?: string | null;
}

export interface DeliveryRail {
  steps: StepperStep[];
  current: string;
  stopped: boolean;
  related: RelatedDocument[];
}

export function deliveryRailCurrent(
  i: Pick<DeliveryRailInput, "status" | "dispatchedAt"> & Partial<Pick<DeliveryRailInput, "origin" | "salesOrderId">>,
): string {
  if (i.status === "confirmed") {
    if (i.origin === "direct" && (i.salesOrderId ?? 0) > 0) return "ordered";
    return "received";
  }
  if (i.dispatchedAt) return "dispatched";
  return "draft";
}

export function deliveryRail(i: DeliveryRailInput): DeliveryRail {
  const related: RelatedDocument[] = [];
  if (i.salesOrderId > 0) {
    related.push({
      label: i.orderNumber || `#${i.salesOrderId}`,
      kind: "Order jual",
      to: `/sales-orders/${i.salesOrderId}`,
      status: i.orderStatus ? { label: statusLabel(i.orderStatus), tone: statusTone(i.orderStatus) } : undefined,
    });
  }
  if (typeof i.salesReturnId === "number" && i.salesReturnId > 0) {
    related.push({ label: `Retur #${i.salesReturnId}`, kind: "Retur jual", to: `/sales-returns/${i.salesReturnId}` });
  }
  return {
    steps: i.origin === "direct" ? DIRECT_DELIVERY_STEPS : DELIVERY_STEPS,
    current: deliveryRailCurrent(i),
    stopped: i.status === "cancelled",
    related,
  };
}
