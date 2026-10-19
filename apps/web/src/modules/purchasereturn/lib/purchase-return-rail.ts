import type { StepperStep } from "@/shared/components/patterns";
import type { PurchaseReturn } from "@/modules/purchasereturn/types";

export interface PurchaseReturnRail {
  steps: StepperStep[];
  current: string;
  stopped: boolean;
}

/** Rel Dokumen retur beli: Draf → Barang keluar (konfirmasi) → Diselesaikan; dibatalkan = berhenti. */
export function purchaseReturnRail(
  r: Pick<PurchaseReturn, "status" | "settledTotal" | "unsettledTotal" | "total"> & { externalRemaining?: number },
): PurchaseReturnRail {
  const steps: StepperStep[] = [
    { key: "draft", label: "Draf" },
    { key: "returned", label: "Barang keluar" },
    { key: "settled", label: "Diselesaikan" },
  ];
  if (r.status === "cancelled") {
    return { steps: [...steps, { key: "cancelled", label: "Dibatalkan" }], current: "cancelled", stopped: true };
  }
  if (r.status === "draft") return { steps, current: "draft", stopped: false };
  // Selesai bila tak ada lagi yang bisa direfund / dijadikan saldo (kelebihan di
  // atas sisa hutang = 0); tanpa field itu (data lama) pakai hitungan memo.
  const settled =
    r.externalRemaining !== undefined
      ? r.externalRemaining <= 0
      : r.unsettledTotal <= 0 && (r.settledTotal > 0 || r.total <= 0);
  return { steps, current: settled ? "settled" : "returned", stopped: false };
}
