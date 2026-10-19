/** Id kesalahan singkat yang bisa dibacakan pengguna ke admin (dicatat juga di console). */
export function createErrorId(): string {
  const t = Date.now().toString(36).toUpperCase().slice(-5);
  const r = Math.random().toString(36).toUpperCase().slice(2, 5).padEnd(3, "0");
  return `ERR-${t}${r}`;
}
