// Draf formulir panjang di localStorage agar sesi habis / tab tertutup tidak
// membuang isian. Semua akses dibungkus try/catch (mode privat, penyimpanan
// diblokir, kuota penuh) — tanpa penyimpanan, form tetap bekerja normal.
// Kunci mencakup pengguna: draf satu orang tidak muncul bagi orang lain di
// perangkat bersama. Draf kedaluwarsa otomatis dan dibersihkan saat logout.

const PREFIX = "mini-erp.draft:";
/** Draf lebih tua dari ini dianggap basi dan dibuang. */
export const DRAFT_MAX_AGE_MS = 24 * 60 * 60 * 1000;

interface Envelope<T> {
  v: 1;
  savedAt: number;
  data: T;
}

/** Kunci draf per pengguna + formulir; null bila pengguna belum diketahui. */
export function draftKey(userId: number | string | null | undefined, form: string): string | null {
  if (userId === null || userId === undefined || userId === "") return null;
  return `${PREFIX}${userId}:${form}`;
}

export function saveDraft<T>(key: string, data: T, now: number = Date.now()): boolean {
  try {
    const env: Envelope<T> = { v: 1, savedAt: now, data };
    localStorage.setItem(key, JSON.stringify(env));
    return true;
  } catch {
    return false;
  }
}

export function loadDraft<T>(key: string, now: number = Date.now(), maxAgeMs: number = DRAFT_MAX_AGE_MS): T | null {
  try {
    const raw = localStorage.getItem(key);
    if (raw === null) return null;
    const env = JSON.parse(raw) as Partial<Envelope<T>>;
    if (env.v !== 1 || typeof env.savedAt !== "number" || env.data === undefined) {
      localStorage.removeItem(key);
      return null;
    }
    if (now - env.savedAt > maxAgeMs) {
      localStorage.removeItem(key);
      return null;
    }
    return env.data;
  } catch {
    return null;
  }
}

export function clearDraft(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch {
    // diabaikan
  }
}

/** Hapus semua draf (logout eksplisit). Sesi habis TIDAK memanggil ini. */
export function clearAllDrafts(): void {
  try {
    const doomed: string[] = [];
    for (let i = 0; i < localStorage.length; i += 1) {
      const k = localStorage.key(i);
      if (k !== null && k.startsWith(PREFIX)) doomed.push(k);
    }
    for (const k of doomed) localStorage.removeItem(k);
  } catch {
    // diabaikan
  }
}
