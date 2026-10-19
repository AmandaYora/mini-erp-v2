import { MAX_BATCH } from "@/modules/finance/types";
import type { BatchItem, PostingSource } from "@/modules/finance/types";

/** Kunci unik satu item antrean. */
export function queueKey(q: Pick<PostingSource, "docType" | "docId">): string {
  return `${q.docType}:${q.docId}`;
}

/**
 * Item yang boleh dipilih untuk posting massal: dokumen di periode tertutup
 * tetap tampil sebagai peringatan tetapi tidak bisa diposting (backend
 * mengecualikannya dari batch dan tutup harian).
 */
export function selectableQueue(queue: PostingSource[]): PostingSource[] {
  return queue.filter((q) => !q.periodClosed && !q.ignored);
}

/** Dokumen yang sengaja diabaikan (FN-01): tampil terpisah, bisa dipulihkan. */
export function ignoredQueue(queue: PostingSource[]): PostingSource[] {
  return queue.filter((q) => q.ignored);
}

/** Dokumen di periode tertutup yang belum diabaikan. */
export function closedQueue(queue: PostingSource[]): PostingSource[] {
  return queue.filter((q) => q.periodClosed && !q.ignored);
}

/** Susun item batch dari pilihan; dipotong pada MAX_BATCH. */
export function buildBatch(
  queue: PostingSource[],
  selected: ReadonlySet<string>,
  action: BatchItem["action"] = "post",
): BatchItem[] {
  return selectableQueue(queue)
    .filter((q) => selected.has(queueKey(q)))
    .slice(0, MAX_BATCH)
    .map((q) => ({ docType: q.docType, docId: q.docId, action }));
}

/**
 * Pesan penolakan posting karena harga modal belum ada
 * (builders.go: "Harga modal produk X belum tersedia").
 */
export function isMissingCostError(message: string | undefined | null): boolean {
  return !!message && /harga modal/i.test(message) && /belum tersedia/i.test(message);
}

/** Nama produk dari "Harga modal produk X belum tersedia"; null bila tak ada. */
export function missingCostProduct(message: string): string | null {
  const m = /Harga modal produk (.+) belum tersedia/i.exec(message);
  const name = m?.[1]?.trim();
  return name && name !== "pada retur" && !name.startsWith("pada ") ? name : null;
}

/**
 * Item batch "susulan": dokumen di periode tertutup dijurnalkan ke periode
 * terbuka (ditandai Susulan). Hanya untuk item periodClosed; null bila item
 * masih bisa diposting biasa.
 */
export function buildCatchup(q: PostingSource): BatchItem | null {
  if (!q.periodClosed) return null;
  return { docType: q.docType, docId: q.docId, action: "catchup" };
}

/** Izin yang dibutuhkan backend untuk aksi catchup (post-batch menolak tanpanya). */
export const CATCHUP_PERMISSION = "finance.close.force";

/**
 * Bersihkan pilihan tabel: hanya kunci yang ada di antrean dan bukan periode tertutup,
 * dipotong pada MAX_BATCH (satu panggilan batch tak boleh melewati batas backend).
 */
export function sanitizeSelection(
  queue: PostingSource[],
  keys: ReadonlySet<string | number>,
): Set<string> {
  const allowed = new Set(selectableQueue(queue).map(queueKey));
  const out = new Set<string>();
  for (const k of keys) {
    const key = String(k);
    if (!allowed.has(key)) continue;
    if (out.size >= MAX_BATCH) break;
    out.add(key);
  }
  return out;
}

/** Dokumen yang ikut tutup harian: bukan periode tertutup dan bertanggal ≤ tanggal tutup (maks MAX_BATCH per jalan). */
export function closeDayCandidates(queue: PostingSource[], date: string): PostingSource[] {
  if (!date) return [];
  return selectableQueue(queue)
    .filter((q) => (q.date ?? "").slice(0, 10) <= date)
    .slice(0, MAX_BATCH);
}
