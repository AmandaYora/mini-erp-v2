import type { RelatedDocument, StepperStep } from "@/shared/components/patterns";
import { statusLabel, statusTone } from "@/shared/lib/format";

// Rel Dokumen penerimaan barang: Menunggu posting → Diterima, atau berhenti di Dibatalkan
// (status server: pending | posted | void). Terkait = PO asal.

const STEPS: StepperStep[] = [
  { key: "pending", label: "Menunggu posting" },
  { key: "posted", label: "Diterima" },
];

const STEPS_VOID: StepperStep[] = [
  { key: "pending", label: "Menunggu posting" },
  { key: "void", label: "Dibatalkan" },
];

export interface GoodsReceiptRail {
  steps: StepperStep[];
  current: string;
  stopped: boolean;
  related: RelatedDocument[];
}

export function goodsReceiptRail(
  status: string,
  order: { id: number; number: string; status?: string | null } | null,
  orderPath: string,
): GoodsReceiptRail {
  const isVoid = status === "void";
  return {
    steps: isVoid ? STEPS_VOID : STEPS,
    current: isVoid ? "void" : status,
    stopped: isVoid,
    related:
      order && order.number !== ""
        ? [
            {
              label: order.number,
              kind: "Order beli",
              to: `${orderPath}/${order.id}`,
              status: order.status ? { label: statusLabel(order.status), tone: statusTone(order.status) } : undefined,
            },
          ]
        : [],
  };
}
