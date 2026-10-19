import { lazy, type ComponentType, type LazyExoticComponent } from "react";

// Pemulihan chunk-load (OP-01; padanan legacy `lib/chunk-load-recovery.ts`).
// Setelah deploy, nama berkas hasil build berganti; tab yang masih terbuka
// meminta chunk lama → 404 → "Failed to fetch dynamically imported module" →
// layar putih. Pemulihan: muat ulang halaman SATU KALI (index.html baru memuat
// chunk baru). Penjaga loop = cap waktu di sessionStorage: bila sudah memuat
// ulang dalam jendela cooldown dan masih gagal, galat dilempar ke
// errorElement (kartu galat) alih-alih memuat ulang tanpa akhir.

export const CHUNK_RELOAD_KEY = "mini-erp:chunk-reload-at";
export const CHUNK_RELOAD_COOLDOWN_MS = 30_000;

const CHUNK_PATTERNS = [
  /failed to fetch dynamically imported module/i,
  /error loading dynamically imported module/i,
  /importing a module script failed/i,
  /expected a javascript-or-wasm module script/i,
  /unable to preload css/i,
  /loading (css )?chunk [\w-]+ failed/i,
];

function messageOf(value: unknown): string {
  if (!value) return "";
  if (typeof value === "string") return value;
  if (value instanceof Error) return `${value.name} ${value.message}`;
  if (typeof value === "object") {
    const r = value as { message?: unknown; error?: unknown; reason?: unknown; payload?: unknown };
    return [r.message, r.error, r.reason, r.payload].map(messageOf).filter(Boolean).join(" ");
  }
  return String(value);
}

/** true bila galat berasal dari gagal memuat chunk/modul dinamis (bukan galat logika halaman). */
export function isChunkLoadError(value: unknown): boolean {
  const msg = messageOf(value);
  return CHUNK_PATTERNS.some((p) => p.test(msg));
}

export interface ReloadGuardDeps {
  storage?: Pick<Storage, "getItem" | "setItem"> | null;
  reload?: () => void;
  now?: number;
}

function defaultStorage(): Pick<Storage, "getItem" | "setItem"> | null {
  try {
    return typeof sessionStorage === "undefined" ? null : sessionStorage;
  } catch {
    return null;
  }
}

/**
 * Muat ulang halaman sekali. Mengembalikan true bila reload dipicu, false bila
 * penjaga menolak (sudah memuat ulang dalam jendela cooldown). Tanpa
 * sessionStorage, penjaga tidak bisa bekerja → TIDAK memuat ulang (lebih baik
 * kartu galat daripada loop).
 */
export function reloadOnceForChunkError(deps: ReloadGuardDeps = {}): boolean {
  const storage = deps.storage === undefined ? defaultStorage() : deps.storage;
  const now = deps.now ?? Date.now();
  if (!storage) return false;
  try {
    const prev = Number(storage.getItem(CHUNK_RELOAD_KEY));
    if (Number.isFinite(prev) && prev > 0 && now - prev < CHUNK_RELOAD_COOLDOWN_MS) return false;
    storage.setItem(CHUNK_RELOAD_KEY, String(now));
  } catch {
    return false;
  }
  (deps.reload ?? (() => window.location.reload()))();
  return true;
}

/**
 * Pengganti `React.lazy`: bila impor gagal karena chunk basi → muat ulang sekali
 * (promise dibiarkan menggantung selama halaman memuat ulang); bila penjaga
 * menolak atau galatnya bukan chunk → lempar, ditangkap errorElement.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- sama dengan batas tipe React.lazy
export function lazyWithRetry<T extends ComponentType<any>>(
  factory: () => Promise<{ default: T }>,
  deps: ReloadGuardDeps = {},
): LazyExoticComponent<T> {
  return lazy(async () => {
    try {
      return await factory();
    } catch (err) {
      if (isChunkLoadError(err) && reloadOnceForChunkError(deps)) {
        return new Promise<{ default: T }>(() => {});
      }
      throw err;
    }
  });
}

/** Pasang penangan `vite:preloadError` (dipicu Vite saat preload chunk/CSS gagal). */
export function installPreloadErrorHandler(win: Window = window, deps: ReloadGuardDeps = {}): () => void {
  const handler = (event: Event) => {
    if (reloadOnceForChunkError(deps)) event.preventDefault();
  };
  win.addEventListener("vite:preloadError", handler);
  return () => win.removeEventListener("vite:preloadError", handler);
}
