/** Banyaknya isian yang berbeda dari kondisi tersimpan (perbandingan nilai, bukan referensi). */
export function countChanged<T extends object>(current: T, baseline: T): number {
  const keys = new Set([...Object.keys(current), ...Object.keys(baseline)]);
  let n = 0;
  for (const k of keys) {
    const a = (current as Record<string, unknown>)[k];
    const b = (baseline as Record<string, unknown>)[k];
    if (JSON.stringify(a) !== JSON.stringify(b)) n++;
  }
  return n;
}

/** Teks ringkasan footer pengaturan, mis. "3 perubahan belum disimpan". */
export function changesLabel(count: number): string {
  return `${count} perubahan belum disimpan`;
}
