// Pembanding before/after untuk rincian audit (fungsi murni, mudah diuji).

export type DiffKind = "added" | "removed" | "changed" | "same";

export interface DiffRow {
  /** Jalur kunci bertitik, mis. "address.city". */
  path: string;
  before: unknown;
  after: unknown;
  kind: DiffKind;
}

function isPlainObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

/** Ratakan objek bersarang menjadi pasangan jalur -> nilai daun (array = daun). */
export function flatten(value: unknown, prefix = ""): Map<string, unknown> {
  const out = new Map<string, unknown>();
  if (isPlainObject(value)) {
    const keys = Object.keys(value);
    if (keys.length === 0 && prefix) out.set(prefix, value);
    for (const key of keys) {
      const path = prefix ? `${prefix}.${key}` : key;
      for (const [p, v] of flatten(value[key], path)) out.set(p, v);
    }
    return out;
  }
  out.set(prefix || "(nilai)", value);
  return out;
}

function same(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

/**
 * Beda antara `before` dan `after`. Hanya baris yang berbeda dikembalikan
 * kecuali `includeSame`. before kosong = entri baru (semua "added"),
 * after kosong = entri dihapus (semua "removed").
 */
export function diffRecords(before: unknown, after: unknown, includeSame = false): DiffRow[] {
  const b = before == null ? new Map<string, unknown>() : flatten(before);
  const a = after == null ? new Map<string, unknown>() : flatten(after);
  const paths = [...new Set([...b.keys(), ...a.keys()])].sort();
  const rows: DiffRow[] = [];
  for (const path of paths) {
    const hasB = b.has(path);
    const hasA = a.has(path);
    const bv = b.get(path);
    const av = a.get(path);
    let kind: DiffKind;
    if (hasB && !hasA) kind = "removed";
    else if (!hasB && hasA) kind = "added";
    else kind = same(bv, av) ? "same" : "changed";
    if (kind === "same" && !includeSame) continue;
    rows.push({ path, before: bv, after: av, kind });
  }
  return rows;
}

/** Nilai untuk ditampilkan: null/kosong "—", teks apa adanya, selain itu JSON. */
export function formatDiffValue(v: unknown): string {
  if (v === null || v === undefined) return "—";
  if (typeof v === "string") return v === "" ? "(kosong)" : v;
  return JSON.stringify(v);
}

/** Apakah rekaman punya rincian yang layak diperluas. */
export function hasDetails(rec: { before?: unknown; after?: unknown; metadata?: unknown }): boolean {
  return rec.before != null || rec.after != null || rec.metadata != null;
}
