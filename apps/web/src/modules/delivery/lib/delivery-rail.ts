import type { RelatedDocument, StepperStep } from "@/shared/components/patterns";
import { statusLabel, statusTone } from "@/shared/lib/format";

// Rel Dokumen surat jalan: Draf → Dikirim → Diterima. Fungsi murni dari data yang
// sudah dimuat halaman detail (surat jalan + order jual); tanpa endpoint baru.

export const DELIVERY_STEPS: StepperStep[] = [
  { key: "draft", label: "Draf" },
  { key: "dispatched", label: "Dikirim" },
  { key: "received", label: "Diterima" },
];

export interface DeliveryRailInput {
  status: string;
  dispatchedAt: string | null;
  salesOrderId: number;
  orderNumber: string;
  /** Status order jual bila berhasil dimuat. */
  orderStatus?: string | null;
  salesReturnId?: number | null;
}

export interface DeliveryRail {
  steps: StepperStep[];
  current: string;
  stopped: boolean;
  related: RelatedDocument[];
}

export function deliveryRailCurrent(i: Pick<DeliveryRailInput, "status" | "dispatchedAt">): string {
  if (i.status === "confirmed") return "received";
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
    steps: DELIVERY_STEPS,
    current: deliveryRailCurrent(i),
    stopped: i.status === "cancelled",
    related,
  };
}
