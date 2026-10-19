import { MENU_GROUPS } from "@/app/routes/registry";

/**
 * Izin yang dibutuhkan untuk sebuah path berdasarkan menu terpanjang yang
 * cocok (rute detail mewarisi menu induknya). null = tak ada izin khusus.
 */
export function requiredPermForPath(pathname: string): string | null {
  let best: { len: number; perm: string } | null = null;
  for (const group of MENU_GROUPS) {
    for (const item of group.items) {
      if (pathname === item.path || pathname.startsWith(`${item.path}/`)) {
        if (!best || item.path.length > best.len) {
          best = { len: item.path.length, perm: item.perm };
        }
      }
    }
  }
  return best?.perm ?? null;
}

/** Apakah path masih boleh dibuka dengan izin saat ini (dipakai setelah ganti role). */
export function isPathPermitted(pathname: string, can: (perm: string) => boolean): boolean {
  const perm = requiredPermForPath(pathname);
  return perm === null || can(perm);
}
