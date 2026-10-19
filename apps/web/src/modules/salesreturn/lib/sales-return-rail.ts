import type { StepperStep } from "@/shared/components/patterns";
import type { SalesReturn } from "@/modules/salesreturn/types";

export interface ReturnRail {
  steps: StepperStep[];
  current: string;
  stopped: boolean;
}

/**
 * Rel Dokumen retur jual: Draf → Barang kembali (konfirmasi memasukkan stok) → [Pengganti, bila tukar]
 * → Diselesaikan; dibatalkan = berhenti. Fungsi murni dari data yang sudah dimuat halaman.
 */
export function salesReturnRail(
  r: Pick<SalesReturn, "status" | "returnMode" | "replacementDeliveryStatus" | "settledTotal" | "unsettledTotal" | "total"> & {
    externalRemaining?: number;
    chargeRemaining?: number;
  },
): ReturnRail {
  const exchange = r.returnMode === "exchange";
  const steps: StepperStep[] = [
    { key: "draft", label: "Draf" },
    { key: "returned", label: "Barang kembali" },
    ...(exchange ? [{ key: "replacement", label: "Pengganti" }] : []),
    { key: "settled", label: "Diselesaikan" },
  ];
  if (r.status === "cancelled") {
    return { steps: [...steps, { key: "cancelled", label: "Dibatalkan" }], current: "cancelled", stopped: true };
  }
  if (r.status === "draft") return { steps, current: "draft", stopped: false };

  const replacementDone = !exchange || r.replacementDeliveryStatus === "confirmed";
  // Selesai bila tak ada lagi yang bisa direfund / dijadikan saldo / ditagih
  // (server menghitungnya dari arah selisih); data lama tanpa field itu memakai memo.
  const settled =
    r.externalRemaining !== undefined && r.chargeRemaining !== undefined
      ? r.externalRemaining <= 0 && r.chargeRemaining <= 0
      : r.unsettledTotal <= 0 && (r.settledTotal > 0 || r.total <= 0);
  if (settled && replacementDone) return { steps, current: "settled", stopped: false };
  if (!replacementDone) return { steps, current: "replacement", stopped: false };
  return { steps, current: "returned", stopped: false };
}
