import type { StepperStep } from "@/shared/components/patterns";

export const TRANSFER_RAIL_STEPS: StepperStep[] = [
  { key: "draft", label: "Draf" },
  { key: "dispatched", label: "Dikirim" },
  { key: "received", label: "Diterima" },
];

export interface TransferRail {
  steps: StepperStep[];
  current: string;
  stopped: boolean;
}

/**
 * Langkah Rel Dokumen transfer: Draf → Dikirim → Diterima. Transfer batal berhenti di "Draf":
 * transferView tidak membawa riwayat apakah sempat dikirim, jadi rel tidak menebak lebih jauh.
 */
export function transferRail(status: string | null | undefined): TransferRail {
  const s = (status ?? "").toLowerCase();
  if (s === "cancelled") return { steps: TRANSFER_RAIL_STEPS, current: "draft", stopped: true };
  // Transfer yang sudah diterima = langkah terakhir selesai (semua centang).
  if (s === "received") return { steps: TRANSFER_RAIL_STEPS, current: "received", stopped: false };
  return { steps: TRANSFER_RAIL_STEPS, current: s === "dispatched" ? "dispatched" : "draft", stopped: false };
}
