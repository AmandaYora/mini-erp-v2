// Tujuan login setelah sesi habis: halaman yang sedang dibuka (path + query)
// dibawa lewat ?next agar pengguna kembali ke sana setelah masuk lagi.

const PUBLIC_PATHS = new Set(["/login", "/select-branch", "/403"]);

/** null = jangan redirect (sudah di halaman publik). */
export function loginRedirectTarget(pathname: string, search: string): string | null {
  if (PUBLIC_PATHS.has(pathname)) return null;
  const next = encodeURIComponent(`${pathname}${search}`);
  return `/login?next=${next}`;
}

/** ?next hanya boleh path internal (cegah open redirect). */
export function safeNextPath(next: string | null | undefined, fallback: string): string {
  if (!next) return fallback;
  if (!next.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return fallback;
  return next;
}
