import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "@/shared/stores/toast.store";
import { isAbort } from "@/shared/services/http-client";
import { posService } from "@/modules/pos/services/pos.service";
import { readApiError } from "@/modules/pos/lib/api-error";
import type { PosCategory, PosProductListItem } from "@/modules/pos/types";

/** Produk per halaman katalog (dimuat berikutnya saat grid di-scroll). */
export const PAGE_SIZE = 48;
/** Penyegaran stok sesudah checkout membaca ulang paling banyak halaman ini (240 produk teratas). */
export const REFRESH_PAGES = 5;

/**
 * Katalog POS: kategori (drilldown) + pencarian produk, dimuat per halaman (infinite scroll).
 * Debounce 350 ms; kategori aktif memfilter katalog (null = semua). Tidak ada auto-focus: input
 * cari baru fokus saat diketuk.
 *
 * Ganti kata kunci/kategori = generasi baru: request lama dibatalkan, halaman 1 dimuat, dan item
 * lama tetap tampil (redup) sampai halaman 1 tiba. Respons hanya diterapkan bila generasinya masih
 * sama (pola seqRef + AbortController di async-search-select) — abort mengurus biaya, seq mengurus
 * kebenaran.
 */
export function usePosCatalog() {
  const [categories, setCategories] = useState<PosCategory[]>([]);
  const [activeCategoryId, setActiveCategoryId] = useState<number | null>(null);
  const [productQuery, setProductQuery] = useState("");

  const [items, setItems] = useState<PosProductListItem[]>([]);
  // Halaman terakhir yang termuat (0 = belum ada).
  const [page, setPage] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [total, setTotal] = useState(0);
  // true sejak awal: halaman 1 pasti dimuat setelah debounce (tanpa kilasan "Tidak ada hasil").
  const [loadingFirst, setLoadingFirst] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  // Galat halaman >1 (halaman 1 memakai toast). Item yang sudah tampil tidak dihapus.
  const [error, setError] = useState<string | null>(null);
  // Bertambah tiap halaman 1 diterapkan — grid kembali ke atas.
  const [generation, setGeneration] = useState(0);
  // Jumlah produk baru dari halaman terakhir (pengumuman pembaca layar).
  const [lastPageCount, setLastPageCount] = useState(0);

  // Ref untuk pengaman sinkron: IntersectionObserver bisa memanggil loadMore dua kali
  // sebelum render berikutnya.
  const seqRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);
  const paramsRef = useRef<{ search: string; categoryId: number | null }>({ search: "", categoryId: null });
  const idsRef = useRef<Set<number>>(new Set());
  const pageRef = useRef(0);
  const hasMoreRef = useRef(false);
  const inflightRef = useRef(false);
  const errorRef = useRef(false);

  useEffect(() => {
    posService
      .categories()
      .then((list) => setCategories(list ?? []))
      .catch((err) => {
        setCategories([]);
        toast.warning("Kategori produk tidak termuat", readApiError(err).message);
      });
  }, []);

  const fetchPage = useCallback((pageNo: number) => {
    const gen = seqRef.current;
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    inflightRef.current = true;
    errorRef.current = false;
    setError(null);
    // Halaman 1 juga menggantikan muat-berikutnya generasi lama yang baru saja dibatalkan.
    setLoadingFirst(pageNo === 1);
    setLoadingMore(pageNo > 1);

    posService
      .products({ ...paramsRef.current, page: pageNo, limit: PAGE_SIZE }, controller.signal)
      .then((res) => {
        if (seqRef.current !== gen) return;
        const ids = pageNo === 1 ? new Set<number>() : new Set(idsRef.current);
        const fresh: PosProductListItem[] = [];
        for (const p of res.items) {
          if (ids.has(p.id)) continue;
          ids.add(p.id);
          fresh.push(p);
        }
        // Pengaman: halaman yang tak menambah id baru = akhir katalog (hindari muat tanpa henti).
        const more = res.meta.page < res.meta.totalPages && fresh.length > 0;
        idsRef.current = ids;
        pageRef.current = pageNo;
        hasMoreRef.current = more;
        setItems((prev) => (pageNo === 1 ? fresh : [...prev, ...fresh]));
        setPage(pageNo);
        setHasMore(more);
        setTotal(res.meta.total);
        setLastPageCount(fresh.length);
        if (pageNo === 1) setGeneration((g) => g + 1);
      })
      .catch((err) => {
        if (seqRef.current !== gen || isAbort(err)) return;
        const message = readApiError(err).message;
        if (pageNo === 1) {
          toast.danger("Gagal mencari produk", message);
          idsRef.current = new Set();
          pageRef.current = 0;
          hasMoreRef.current = false;
          setItems([]);
          setPage(0);
          setHasMore(false);
          setTotal(0);
          setLastPageCount(0);
        } else {
          errorRef.current = true;
          setError(message);
        }
      })
      .finally(() => {
        if (seqRef.current !== gen || abortRef.current !== controller) return;
        inflightRef.current = false;
        abortRef.current = null;
        setLoadingFirst(false);
        setLoadingMore(false);
      });
  }, []);

  useEffect(() => {
    const t = window.setTimeout(() => {
      seqRef.current += 1;
      paramsRef.current = { search: productQuery, categoryId: activeCategoryId };
      fetchPage(1);
    }, 350);
    return () => window.clearTimeout(t);
  }, [productQuery, activeCategoryId, fetchPage]);

  // Keluar dari POS: batalkan request yang masih berjalan dan abaikan hasilnya.
  useEffect(
    () => () => {
      seqRef.current += 1;
      abortRef.current?.abort();
    },
    [],
  );

  /** Muat halaman berikutnya; no-op saat memuat, galat, atau katalog habis. */
  const loadMore = useCallback(() => {
    if (inflightRef.current || errorRef.current || !hasMoreRef.current) return;
    fetchPage(pageRef.current + 1);
  }, [fetchPage]);

  /** Ulangi halaman yang gagal (halaman >1). */
  const retry = useCallback(() => {
    if (inflightRef.current || !errorRef.current) return;
    fetchPage(pageRef.current + 1);
  }, [fetchPage]);

  /**
   * Segarkan stok (dan harga) produk yang sudah termuat tanpa mengganggu kisi: baca ulang halaman
   * 1..n diam-diam (n ≤ REFRESH_PAGES), lalu ganti hanya field stok/harga per id — urutan, posisi
   * gulir, dan item tidak berubah. Dipakai sesudah checkout agar angka stok tile tidak basi.
   */
  const refreshStock = useCallback(async () => {
    const gen = seqRef.current;
    const pages = Math.min(pageRef.current, REFRESH_PAGES);
    const fresh = new Map<number, PosProductListItem>();
    for (let p = 1; p <= pages; p++) {
      try {
        const res = await posService.products({ ...paramsRef.current, page: p, limit: PAGE_SIZE });
        for (const it of res.items) fresh.set(it.id, it);
      } catch {
        return; // penyegaran bonus: gagal = angka lama tetap tampil
      }
      if (seqRef.current !== gen) return;
    }
    if (fresh.size === 0) return;
    setItems((prev) =>
      prev.map((it) => {
        const f = fresh.get(it.id);
        return f
          ? { ...it, stockAvailable: f.stockAvailable, belowMinStock: f.belowMinStock, sellingPrice: f.sellingPrice }
          : it;
      }),
    );
  }, []);

  return {
    categories,
    activeCategoryId,
    setActiveCategoryId,
    productQuery,
    setProductQuery,
    items,
    page,
    hasMore,
    total,
    loadingFirst,
    loadingMore,
    error,
    generation,
    lastPageCount,
    loadMore,
    retry,
    refreshStock,
  };
}

export type PosCatalogApi = ReturnType<typeof usePosCatalog>;
