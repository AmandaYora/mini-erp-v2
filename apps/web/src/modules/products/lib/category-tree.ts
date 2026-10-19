// Logika murni tabel pohon kategori (tanpa React). Kategori hanya membawa `parentId`;
// di sini diratakan menjadi baris berurutan (induk sebelum anak) dengan kedalaman,
// dan cabang yang dilipat tidak ikut diratakan. Diuji di category-tree.test.ts.

import type { ProductCategory } from "@/modules/products/types";

export interface CategoryTreeRow {
  category: ProductCategory;
  depth: number;
  /** Punya anak di daftar (menentukan chevron). */
  hasChildren: boolean;
}

/** Urutan saudara: `sortOrder` naik (PR-06), lalu kode agar urutan stabil. */
function byOrder(a: ProductCategory, b: ProductCategory): number {
  const diff = (a.sortOrder ?? 0) - (b.sortOrder ?? 0);
  return diff !== 0 ? diff : a.code.localeCompare(b.code, "id", { numeric: true });
}

/**
 * Ratakan kategori menjadi pohon. Akar = tanpa induk, atau induknya tidak ada di daftar
 * (mis. tidak ikut termuat) agar tidak ada kategori yang hilang. Saudara diurutkan menurut
 * `sortOrder`, lalu kode. Siklus data (induk saling menunjuk) dipotong supaya tidak berulang tanpa henti.
 */
export function buildCategoryRows(
  items: readonly ProductCategory[],
  collapsed: ReadonlySet<number> = new Set(),
): CategoryTreeRow[] {
  const ids = new Set(items.map((c) => c.id));
  const children = new Map<number, ProductCategory[]>();
  const roots: ProductCategory[] = [];
  for (const c of items) {
    if (c.parentId === null || !ids.has(c.parentId) || c.parentId === c.id) {
      roots.push(c);
    } else {
      const list = children.get(c.parentId) ?? [];
      list.push(c);
      children.set(c.parentId, list);
    }
  }

  const out: CategoryTreeRow[] = [];
  const seen = new Set<number>();
  function walk(c: ProductCategory, depth: number) {
    if (seen.has(c.id)) return;
    seen.add(c.id);
    const kids = (children.get(c.id) ?? []).slice().sort(byOrder);
    out.push({ category: c, depth, hasChildren: kids.length > 0 });
    if (collapsed.has(c.id)) return;
    for (const k of kids) walk(k, depth + 1);
  }
  for (const r of roots.slice().sort(byOrder)) walk(r, 0);
  return out;
}

/** Id semua kategori yang punya anak (untuk "Lipat semua"). */
export function parentIds(items: readonly ProductCategory[]): number[] {
  const ids = new Set(items.map((c) => c.id));
  const parents = new Set<number>();
  for (const c of items) {
    if (c.parentId !== null && c.parentId !== c.id && ids.has(c.parentId)) parents.add(c.parentId);
  }
  return [...parents];
}
