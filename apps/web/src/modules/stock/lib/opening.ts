import type { OpeningPreview } from "@/modules/stock/types";

/** Ringkasan pratinjau saldo awal: valid, dilewati (sudah bermutasi), bermasalah. */
export function openingPreviewSummary(p: OpeningPreview): string {
  const parts = [`${p.validRows} baris valid`];
  const skipped = p.skipped ?? 0;
  if (skipped > 0) parts.push(`${skipped} dilewati (posisi sudah punya mutasi)`);
  parts.push(`${p.errors.length} baris bermasalah`);
  return parts.join(", ");
}
