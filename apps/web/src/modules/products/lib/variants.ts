// Varian yang boleh ditawarkan saat menjual (POS, form order): aktif dan tidak tersembunyi,
// urut sort_order lalu kode — paritas legacy (pos-shared: !isHidden && active). Varian
// "Standar" tersembunyi adalah bawaan produk ber-varian; ia hanya dipakai bila tidak ada
// varian terlihat sama sekali (produk tanpa varian nyata).
export interface SellableVariantLike {
  id: number;
  code: string;
  status: string;
  isDefault: boolean;
  isHidden?: boolean;
  sortOrder?: number;
}

export function sellableVariants<T extends SellableVariantLike>(variants: T[]): T[] {
  const active = variants.filter((v) => v.status === "active" || v.status === "");
  const visible = active.filter((v) => !v.isHidden);
  const pool = visible.length > 0 ? visible : active.length > 0 ? active : variants;
  return [...pool].sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0) || a.code.localeCompare(b.code));
}

/** Varian yang otomatis dipakai bila pengguna tidak perlu memilih: tepat satu varian layak jual. */
export function soleSellableVariant<T extends SellableVariantLike>(variants: T[]): T | null {
  const list = sellableVariants(variants);
  return list.length === 1 ? list[0] : null;
}
