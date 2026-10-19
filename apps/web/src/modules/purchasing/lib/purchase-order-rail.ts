import type { RelatedDocument, StepperStep } from "@/shared/components/patterns";
import { statusLabel, statusTone } from "@/shared/lib/format";

// Definisi Rel Dokumen order beli (PLAN §4.5): Draf → Terkonfirmasi → Selesai.
// "Selesai" terjadi lewat penerimaan barang (modul goodsreceipt), bukan aksi di PO.
// Dokumen terkait hanya dari riwayat yang SUDAH dimuat halaman detail.

/** Subset HistoryEvent (modul payment) yang dibutuhkan rel — tipe struktural, tanpa impor lintas modul. */
export interface RailEvent {
  kind: string;
  number: string;
  status: string;
  href: string;
}

const KIND_LABEL: Record<string, string> = {
  receipt: "Penerimaan",
  payment: "Pembayaran",
  return: "Retur beli",
};

const STEPS: StepperStep[] = [
  { key: "draft", label: "Draf" },
  { key: "confirmed", label: "Terkonfirmasi" },
  { key: "completed", label: "Selesai" },
];

const STEPS_CANCELLED: StepperStep[] = [
  { key: "draft", label: "Draf" },
  { key: "cancelled", label: "Dibatalkan" },
];

export interface PurchaseOrderRail {
  steps: StepperStep[];
  current: string;
  stopped: boolean;
  related: RelatedDocument[];
}

export function purchaseOrderRail(status: string, events: RailEvent[]): PurchaseOrderRail {
  const cancelled = status === "cancelled";
  return {
    steps: cancelled ? STEPS_CANCELLED : STEPS,
    current: cancelled ? "cancelled" : status,
    stopped: cancelled,
    related: events
      .filter((e) => e.href !== "" && e.number !== "")
      .map((e) => ({
        label: e.number,
        kind: KIND_LABEL[e.kind] ?? e.kind,
        to: e.href,
        status: { label: statusLabel(e.status), tone: statusTone(e.status) },
      })),
  };
}
