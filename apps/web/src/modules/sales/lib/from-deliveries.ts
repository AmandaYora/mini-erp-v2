// Util murni mode "order dari surat jalan" (SJ Langsung, ADR-0022): parsing
// query ?fromDeliveries=1,2, tautan, umur SJ, dan clientRef idempoten yang
// disimpan di sessionStorage per gabungan id SJ — ulang setelah gagal jaringan
// mengirim clientRef yang sama sehingga server melanjutkan, bukan menggandakan.

import { ROUTE_PATHS } from "@/app/routes/route-paths";

export const FROM_DELIVERIES_PARAM = "fromDeliveries";
/** Batas server: deliveryIds 1–20 per order. */
export const MAX_FROM_DELIVERIES = 20;

/** "1,2,2,x,0" → [1, 2] (unik, positif, urut naik, maksimal 20). */
export function parseFromDeliveriesParam(raw: string | null | undefined): number[] {
  if (!raw) return [];
  const ids = new Set<number>();
  for (const part of raw.split(",")) {
    const n = Number(part.trim());
    if (Number.isInteger(n) && n > 0) ids.add(n);
  }
  return [...ids].sort((a, b) => a - b).slice(0, MAX_FROM_DELIVERIES);
}

/** Tautan form order mode "dari SJ". */
export function fromDeliveriesPath(ids: readonly number[]): string {
  return `${ROUTE_PATHS.salesOrders}/new?${FROM_DELIVERIES_PARAM}=${ids.join(",")}`;
}

/** Umur dokumen dalam hari (0 bila tanggal kosong/tidak sah atau di masa depan). */
export function ageDays(date: string, today: string): number {
  if (!date || !today) return 0;
  const a = Date.parse(`${date}T00:00:00Z`);
  const b = Date.parse(`${today}T00:00:00Z`);
  if (Number.isNaN(a) || Number.isNaN(b)) return 0;
  return Math.max(0, Math.round((b - a) / 86_400_000));
}

/** Semua SJ terpilih milik satu pelanggan? (server menolak gabungan lintas pelanggan). */
export function sameCustomer(partyIds: readonly number[]): boolean {
  return new Set(partyIds).size <= 1;
}

const CLIENT_REF_PREFIX = "mini-erp:sjl-client-ref:";

type RefStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

function defaultStorage(): RefStorage | null {
  try {
    return typeof sessionStorage === "undefined" ? null : sessionStorage;
  } catch {
    return null;
  }
}

export function clientRefKey(ids: readonly number[]): string {
  return `${CLIENT_REF_PREFIX}${[...ids].sort((a, b) => a - b).join(",")}`;
}

export function newClientRef(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `sjl-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

/**
 * clientRef untuk gabungan SJ ini: pakai yang tersimpan bila ada (percobaan
 * sebelumnya gagal di tengah), selain itu buat baru dan simpan. Tanpa
 * sessionStorage tetap mengembalikan UUID baru (idempoten hanya dalam sesi React).
 */
export function clientRefFor(ids: readonly number[], storage: RefStorage | null = defaultStorage()): string {
  const key = clientRefKey(ids);
  try {
    const existing = storage?.getItem(key);
    if (existing && existing.length <= 64) return existing;
    const fresh = newClientRef();
    storage?.setItem(key, fresh);
    return fresh;
  } catch {
    return newClientRef();
  }
}

/** Dipanggil setelah order berhasil dibuat: percobaan berikutnya memakai clientRef baru. */
export function clearClientRef(ids: readonly number[], storage: RefStorage | null = defaultStorage()): void {
  try {
    storage?.removeItem(clientRefKey(ids));
  } catch {
    // storage tidak tersedia — tidak ada yang perlu dibersihkan
  }
}
