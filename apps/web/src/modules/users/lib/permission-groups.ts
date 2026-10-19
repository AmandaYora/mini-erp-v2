import type { PermissionItem } from "@/modules/users/types";

export interface PermissionGroup {
  /** Prefiks kunci izin sebelum titik pertama, mis. `sales` dari `sales.create`. */
  module: string;
  items: PermissionItem[];
}

/** Kelompokkan katalog per modul dari prefiks kunci izin; modul alfabetis, izin mengikuti urutan katalog. */
export function groupPermissions(catalog: PermissionItem[]): PermissionGroup[] {
  const map = new Map<string, PermissionItem[]>();
  for (const p of catalog) {
    const dot = p.code.indexOf(".");
    const module = dot > 0 ? p.code.slice(0, dot) : "lainnya";
    const list = map.get(module);
    if (list) list.push(p);
    else map.set(module, [p]);
  }
  return Array.from(map, ([module, items]) => ({ module, items })).sort((a, b) =>
    a.module.localeCompare(b.module),
  );
}
