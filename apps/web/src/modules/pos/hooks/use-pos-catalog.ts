import { useEffect, useState } from "react";
import { toast } from "@/shared/stores/toast.store";
import { posService } from "@/modules/pos/services/pos.service";
import { readApiError } from "@/modules/pos/lib/api-error";
import type { PosCategory, PosProductOption } from "@/modules/pos/types";

/**
 * Katalog POS: kategori (drilldown) + pencarian produk. Debounce 350 ms; kategori aktif
 * memfilter katalog (null = semua). Tidak ada auto-focus: input cari baru fokus saat diketuk.
 */
export function usePosCatalog() {
  const [categories, setCategories] = useState<PosCategory[]>([]);
  const [activeCategoryId, setActiveCategoryId] = useState<number | null>(null);
  const [productQuery, setProductQuery] = useState("");
  const [productOptions, setProductOptions] = useState<PosProductOption[]>([]);
  const [searching, setSearching] = useState(false);

  useEffect(() => {
    posService
      .categories()
      .then((list) => setCategories(list ?? []))
      .catch((err) => {
        setCategories([]);
        toast.warning("Kategori produk tidak termuat", readApiError(err).message);
      });
  }, []);

  useEffect(() => {
    const t = window.setTimeout(() => {
      setSearching(true);
      const req =
        activeCategoryId === null
          ? posService.searchProducts(productQuery)
          : posService
              .products({
                search: productQuery,
                categoryId: activeCategoryId,
                limit: 20,
              })
              .then((res) => res.items);
      req
        .then((list) => setProductOptions(list ?? []))
        .catch((err) => {
          toast.danger("Gagal mencari produk", readApiError(err).message);
          setProductOptions([]);
        })
        .finally(() => setSearching(false));
    }, 350);
    return () => window.clearTimeout(t);
  }, [productQuery, activeCategoryId]);

  return {
    categories,
    activeCategoryId,
    setActiveCategoryId,
    productQuery,
    setProductQuery,
    productOptions,
    searching,
  };
}

export type PosCatalogApi = ReturnType<typeof usePosCatalog>;
