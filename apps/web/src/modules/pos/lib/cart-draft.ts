// Draf keranjang POS yang bertahan setelah reload. Inti: clientRef (UUID per
// keranjang) — kunci idempoten untuk SO, SJ, dan pembayaran. Disimpan di
// localStorage; semua akses dibungkus try/catch (jendela privat / diblokir
// = halaman tetap jalan tanpa persistensi).
import type { HeldMap } from "@/modules/pos/lib/holds";
import type { CartLine, CheckoutMode, PayMethod } from "@/modules/pos/types";
import { normalizeTaxType, type SalesTaxType } from "@/modules/sales/lib/tax";

export const CART_DRAFT_KEY = "pos.cartDraft.v1";
/** Batas server untuk clientRef (CreateOrder menolak > 64). */
export const CLIENT_REF_MAX = 64;

export interface DraftStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

/** Tahanan stok yang diketahui klien per baris (kunci produk:varian) — ikut
 * tersimpan agar reservationKey SJ tetap benar setelah reload. */
export type HeldSnapshot = Record<
  string,
  { productId: number; variantId: number; held: HeldMap }
>;

export interface CartDraft {
  /** clientRef keranjang ini. */
  ref: string;
  lines: CartLine[];
  customerId: number | null;
  /** true setelah order di server dibuat (atau mungkin dibuat): isi
   * keranjang dikunci sampai selesai / dibatalkan. */
  started: boolean;
  mode: CheckoutMode;
  method: PayMethod;
  dueDate: string;
  tendered: string;
  /** Catatan transaksi dan alamat kirim opt-in (SL-07); draf lama tanpa kunci ini tetap terbaca. */
  notes: string;
  shipToAddressId: number | null;
  /** PPN opsional (K-03); bawaan tanpa pajak. Draf lama tanpa kunci ini tetap terbaca. */
  taxType: SalesTaxType;
  taxRate: number;
  /** Rekening transfer pilihan kasir (ADR-0026); null = default. Draf lama tanpa kunci ini tetap terbaca. */
  bankAccountId?: number | null;
  held: HeldSnapshot;
}

/** UUID v4. Memakai crypto.randomUUID; fallback getRandomValues lalu
 * Math.random untuk konteks tak aman (HTTP di LAN) yang tak punya randomUUID. */
export function newClientRef(): string {
  const c = (globalThis as { crypto?: Crypto }).crypto;
  if (c && typeof c.randomUUID === "function") return c.randomUUID();
  const bytes = new Uint8Array(16);
  if (c && typeof c.getRandomValues === "function") {
    c.getRandomValues(bytes);
  } else {
    for (let i = 0; i < 16; i++) bytes[i] = Math.floor(Math.random() * 256);
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0"));
  return [
    hex.slice(0, 4).join(""),
    hex.slice(4, 6).join(""),
    hex.slice(6, 8).join(""),
    hex.slice(8, 10).join(""),
    hex.slice(10, 16).join(""),
  ].join("-");
}

export function browserStorage(): DraftStorage | null {
  try {
    return typeof window !== "undefined" ? window.localStorage : null;
  } catch {
    return null;
  }
}

export function emptyDraft(ref: string = newClientRef()): CartDraft {
  return {
    ref,
    lines: [],
    customerId: null,
    started: false,
    mode: "pay_now",
    method: "cash",
    dueDate: "",
    tendered: "",
    notes: "",
    shipToAddressId: null,
    taxType: "none",
    taxRate: 0,
    held: {},
  };
}

function isDraft(v: unknown): v is CartDraft {
  if (typeof v !== "object" || v === null) return false;
  const d = v as Record<string, unknown>;
  return (
    typeof d.ref === "string" &&
    d.ref.length > 0 &&
    d.ref.length <= CLIENT_REF_MAX &&
    Array.isArray(d.lines) &&
    typeof d.started === "boolean"
  );
}

/** Baca draf tersimpan. null bila tak ada / rusak / storage tak tersedia. */
export function loadDraft(storage: DraftStorage | null = browserStorage()): CartDraft | null {
  if (!storage) return null;
  try {
    const raw = storage.getItem(CART_DRAFT_KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!isDraft(parsed)) return null;
    const merged = { ...emptyDraft(parsed.ref), ...parsed };
    // Pengaturan pajak dari penyimpanan dinormalkan: nilai asing = tanpa pajak.
    merged.taxType = normalizeTaxType(merged.taxType);
    if (merged.taxType === "none" || !(merged.taxRate > 0 && merged.taxRate <= 100)) {
      merged.taxType = "none";
      merged.taxRate = 0;
    }
    return merged;
  } catch {
    return null;
  }
}

/** Draf tersimpan, atau draf baru dengan ref segar (tidak ditulis). */
export function loadOrCreateDraft(
  storage: DraftStorage | null = browserStorage(),
): CartDraft {
  return loadDraft(storage) ?? emptyDraft();
}

export function saveDraft(
  draft: CartDraft,
  storage: DraftStorage | null = browserStorage(),
): void {
  if (!storage) return;
  try {
    storage.setItem(CART_DRAFT_KEY, JSON.stringify(draft));
  } catch {
    /* kuota penuh / diblokir — abaikan */
  }
}

export function clearDraft(storage: DraftStorage | null = browserStorage()): void {
  if (!storage) return;
  try {
    storage.removeItem(CART_DRAFT_KEY);
  } catch {
    /* abaikan */
  }
}
