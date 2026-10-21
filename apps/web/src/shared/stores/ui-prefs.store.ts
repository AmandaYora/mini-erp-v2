import { create } from "zustand";

export type Density = "compact" | "default" | "comfortable";

const DENSITY_KEY = "mini-erp.density";
const RECENT_KEY = "mini-erp.recent";
const RECENT_MAX = 8;

// Pemilik preferensi per pengguna (PLAN U11: "terakhir dibuka", kolom tabel, dan tampilan
// tersimpan disimpan di localStorage per pengguna). Diisi store auth setiap kali user berganti.
let prefsOwner = "anon";

export function setPrefsOwner(userId: number | string | null | undefined): void {
  prefsOwner = userId === null || userId === undefined || userId === "" ? "anon" : String(userId);
}

/** Kunci localStorage yang dibatasi pengguna aktif: `<base>@<userId>`. */
export function userKey(base: string): string {
  return `${base}@${prefsOwner}`;
}

function readDensity(): Density {
  try {
    const v = localStorage.getItem(DENSITY_KEY);
    if (v === "compact" || v === "comfortable") return v;
  } catch {
    /* penyimpanan privat */
  }
  return "default";
}

/** Pasang atribut kepadatan di <html> (dibaca CSS: --row-py). */
export function applyDensity(d: Density): void {
  if (typeof document === "undefined") return;
  if (d === "default") delete document.documentElement.dataset.density;
  else document.documentElement.dataset.density = d;
}

interface UiPrefsState {
  density: Density;
  setDensity: (d: Density) => void;
}

export const useUiPrefs = create<UiPrefsState>((set) => ({
  density: readDensity(),
  setDensity: (density) => {
    try {
      localStorage.setItem(DENSITY_KEY, density);
    } catch {
      /* abaikan */
    }
    applyDensity(density);
    set({ density });
  },
}));

export interface RecentDoc {
  path: string;
  label: string;
}

export function readRecent(): RecentDoc[] {
  try {
    const raw = localStorage.getItem(userKey(RECENT_KEY));
    return raw ? (JSON.parse(raw) as RecentDoc[]) : [];
  } catch {
    return [];
  }
}

/** Catat dokumen yang baru dibuka (untuk command palette "Terakhir dibuka"). */
export function pushRecent(doc: RecentDoc): void {
  try {
    const next = [doc, ...readRecent().filter((d) => d.path !== doc.path)].slice(0, RECENT_MAX);
    localStorage.setItem(userKey(RECENT_KEY), JSON.stringify(next));
  } catch {
    /* abaikan */
  }
}
