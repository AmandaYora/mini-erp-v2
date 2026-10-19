import type { StepperStep } from "@/shared/components/patterns";
import type { Payment } from "@/modules/payment/types";

export interface PaymentRail {
  steps: StepperStep[];
  current: string;
  /** Alur berhenti (pembayaran dibatalkan): garis abu bergaris. */
  stopped: boolean;
}

/**
 * Langkah Rel Dokumen pembayaran: Tercatat → Dialokasikan → (Dibatalkan).
 * Fungsi murni dari data yang sudah dimuat halaman detail.
 */
export function paymentRail(p: Pick<Payment, "status" | "allocations">): PaymentRail {
  const base: StepperStep[] = [
    { key: "recorded", label: "Tercatat" },
    { key: "allocated", label: "Dialokasikan" },
  ];
  if (p.status === "cancelled") {
    return { steps: [...base, { key: "cancelled", label: "Dibatalkan" }], current: "cancelled", stopped: true };
  }
  return { steps: base, current: p.allocations.length > 0 ? "allocated" : "recorded", stopped: false };
}

/** Tautan ke dokumen yang dibayar, hanya untuk jenis dokumen yang punya halaman detail. */
export function allocationHref(orderType: string, orderId: number): string | null {
  switch (orderType) {
    case "sales":
      return `/sales-orders/${orderId}`;
    case "purchase":
      return `/purchase-orders/${orderId}`;
    case "sales_return":
      return `/sales-returns/${orderId}`;
    case "purchase_return":
      return `/purchase-returns/${orderId}`;
    default:
      return null;
  }
}
