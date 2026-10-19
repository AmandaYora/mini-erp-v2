/** Nilai pilihan select (string|number|null) → id numerik atau null. */
export function toId(v: string | number | null | undefined): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}
