import type { DescriptionItem } from "./description-list";
import { formatDateTime } from "@/shared/lib/format";

/** Jejak pembatalan sebuah dokumen (kolom cancel_reason/cancelled_at/cancelled_by di backend). */
export interface CancellationTrail {
  cancelReason?: string;
  cancelledAt?: string;
  cancelledBy?: number;
}

/**
 * Baris DescriptionList untuk dokumen yang dibatalkan: kapan & oleh siapa, lalu alasannya.
 * Kosong bila dokumen tidak dibatalkan atau dibatalkan sebelum alasan dicatat.
 */
export function cancellationItems(doc: CancellationTrail, label = "Dibatalkan"): DescriptionItem[] {
  if (!doc.cancelledAt && !doc.cancelReason) return [];
  const by = doc.cancelledBy ? ` · #${doc.cancelledBy}` : "";
  return [
    { label, value: doc.cancelledAt ? `${formatDateTime(doc.cancelledAt)}${by}` : null },
    { label: "Alasan batal", value: doc.cancelReason || null, wide: true },
  ];
}
