import { userKey } from "@/shared/stores/ui-prefs.store";

// Preferensi daftar per pengguna di localStorage (PLAN redesain-ui U6-3, U11 — tanpa endpoint):
// - kolom tersembunyi per tabel (`DataTable columnsKey`)
// - "Tampilan" tersimpan per daftar: kombinasi query string filter + kolom tersembunyi.
// Semua akses dibungkus try/catch: mode privat / penyimpanan penuh tidak boleh merusak halaman.

const COLS_PREFIX = "mini-erp.cols.";
const VIEWS_PREFIX = "mini-erp.views.";
const COLS_EVENT = "mini-erp:columns-changed";

export const MAX_VIEWS = 12;
export const MAX_VIEW_NAME = 40;

/** Kolom tersembunyi tersimpan; null = belum pernah diatur (pakai default tabel). */
export function readHiddenColumns(tableKey: string): string[] | null {
  try {
    const raw = localStorage.getItem(userKey(COLS_PREFIX + tableKey));
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((x): x is string => typeof x === "string") : null;
  } catch {
    return null;
  }
}

/** Simpan kolom tersembunyi (null = kembali ke default) dan beri tahu tabel yang sedang tampil. */
export function writeHiddenColumns(tableKey: string, hidden: string[] | null): void {
  try {
    if (hidden === null) localStorage.removeItem(userKey(COLS_PREFIX + tableKey));
    else localStorage.setItem(userKey(COLS_PREFIX + tableKey), JSON.stringify(hidden));
  } catch {
    /* abaikan */
  }
  window.dispatchEvent(new CustomEvent<string>(COLS_EVENT, { detail: tableKey }));
}

/** Berlangganan perubahan kolom dari luar tabel (mis. menerapkan tampilan tersimpan). */
export function onHiddenColumnsChange(tableKey: string, cb: () => void): () => void {
  const handler = (e: Event) => {
    if ((e as CustomEvent<string>).detail === tableKey) cb();
  };
  window.addEventListener(COLS_EVENT, handler);
  return () => window.removeEventListener(COLS_EVENT, handler);
}

export interface SavedView {
  id: string;
  name: string;
  /** Query string tanpa `?` dan tanpa `page` (filter + sortir). */
  query: string;
  /** Kolom tersembunyi saat disimpan; null = default tabel / daftar tanpa pilihan kolom. */
  hiddenColumns: string[] | null;
}

function isSavedView(v: unknown): v is SavedView {
  if (!v || typeof v !== "object") return false;
  const o = v as Record<string, unknown>;
  return (
    typeof o.id === "string" &&
    typeof o.name === "string" &&
    typeof o.query === "string" &&
    (o.hiddenColumns === null || Array.isArray(o.hiddenColumns))
  );
}

export function readSavedViews(listKey: string): SavedView[] {
  try {
    const raw = localStorage.getItem(userKey(VIEWS_PREFIX + listKey));
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter(isSavedView) : [];
  } catch {
    return [];
  }
}

function writeSavedViews(listKey: string, views: SavedView[]): boolean {
  try {
    localStorage.setItem(userKey(VIEWS_PREFIX + listKey), JSON.stringify(views));
    return true;
  } catch {
    return false;
  }
}

/** Bentuk kanonik query tampilan: tanpa `page`, kunci terurut (perbandingan "tampilan aktif"). */
export function normalizeViewQuery(search: string): string {
  const params = new URLSearchParams(search.startsWith("?") ? search.slice(1) : search);
  params.delete("page");
  const entries = [...params.entries()].filter(([, v]) => v !== "");
  entries.sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
  return new URLSearchParams(entries).toString();
}

export type SaveViewResult = { ok: true; views: SavedView[] } | { ok: false; message: string };

/** Simpan tampilan baru; nama yang sama (tanpa beda huruf besar) menimpa tampilan lama. */
export function saveView(
  listKey: string,
  input: { name: string; query: string; hiddenColumns: string[] | null },
): SaveViewResult {
  const name = input.name.trim();
  if (!name) return { ok: false, message: "Nama tampilan wajib diisi." };
  if (name.length > MAX_VIEW_NAME) return { ok: false, message: `Nama tampilan maksimal ${MAX_VIEW_NAME} karakter.` };
  const views = readSavedViews(listKey);
  const existing = views.find((v) => v.name.toLowerCase() === name.toLowerCase());
  if (!existing && views.length >= MAX_VIEWS) {
    return { ok: false, message: `Maksimal ${MAX_VIEWS} tampilan per daftar. Hapus salah satu dulu.` };
  }
  const view: SavedView = {
    id: existing?.id ?? `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`,
    name,
    query: normalizeViewQuery(input.query),
    hiddenColumns: input.hiddenColumns,
  };
  const next = existing ? views.map((v) => (v.id === existing.id ? view : v)) : [...views, view];
  if (!writeSavedViews(listKey, next)) {
    return { ok: false, message: "Penyimpanan peramban tidak tersedia; tampilan tidak bisa disimpan." };
  }
  return { ok: true, views: next };
}

export function deleteView(listKey: string, id: string): SavedView[] {
  const next = readSavedViews(listKey).filter((v) => v.id !== id);
  writeSavedViews(listKey, next);
  return next;
}
