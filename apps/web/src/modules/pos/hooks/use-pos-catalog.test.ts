import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PageMeta } from "@/shared/services/http-client";
import type { PosProductListItem } from "@/modules/pos/types";

const svc = vi.hoisted(() => ({ products: vi.fn(), categories: vi.fn() }));
const toastMock = vi.hoisted(() => ({ danger: vi.fn(), warning: vi.fn() }));
vi.mock("@/modules/pos/services/pos.service", () => ({ posService: svc }));
vi.mock("@/shared/stores/toast.store", () => ({ toast: toastMock }));

import { PAGE_SIZE, usePosCatalog } from "@/modules/pos/hooks/use-pos-catalog";

type PageRes = { items: PosProductListItem[]; meta: PageMeta };
type Call = {
  params: { search?: string; categoryId?: number | null; page?: number; limit?: number };
  signal: AbortSignal;
  resolve: (r: PageRes) => void;
  reject: (e: unknown) => void;
};

// Tiap panggilan products() ditahan sampai tes menyelesaikannya.
let calls: Call[] = [];

const prod = (id: number): PosProductListItem => ({
  id,
  code: `P${id}`,
  name: `Produk ${id}`,
  categoryId: null,
  categoryName: null,
  sellingPrice: 1000,
  status: "active",
});
const pageOf = (ids: number[], page: number, totalPages: number, total = totalPages * PAGE_SIZE): PageRes => ({
  items: ids.map(prod),
  meta: { page, limit: PAGE_SIZE, total, totalPages },
});
const range = (from: number, n: number) => Array.from({ length: n }, (_, i) => from + i);

async function settle(call: Call, res: PageRes) {
  await act(async () => call.resolve(res));
}
async function fail(call: Call, err: unknown) {
  await act(async () => call.reject(err));
}
function debounce() {
  act(() => vi.advanceTimersByTime(350));
}
const ids = (items: PosProductListItem[]) => items.map((p) => p.id);

beforeEach(() => {
  vi.useFakeTimers();
  calls = [];
  svc.categories.mockResolvedValue([]);
  svc.products.mockImplementation(
    (params: Call["params"], signal: AbortSignal) =>
      new Promise<PageRes>((resolve, reject) => calls.push({ params, signal, resolve, reject })),
  );
  toastMock.danger.mockReset();
});
afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
});

describe("usePosCatalog — infinite scroll", () => {
  it("memuat halaman 1 setelah debounce, lalu loadMore menambah halaman 2", async () => {
    const { result } = renderHook(() => usePosCatalog());
    expect(result.current.loadingFirst).toBe(true);
    expect(calls).toHaveLength(0);
    debounce();
    expect(calls[0].params).toMatchObject({ search: "", categoryId: null, page: 1, limit: PAGE_SIZE });

    await settle(calls[0], pageOf(range(1, 48), 1, 3, 120));
    expect(result.current.items).toHaveLength(48);
    expect(result.current).toMatchObject({ page: 1, hasMore: true, total: 120, loadingFirst: false });

    act(() => result.current.loadMore());
    expect(result.current.loadingMore).toBe(true);
    expect(calls[1].params).toMatchObject({ page: 2, limit: PAGE_SIZE });
    await settle(calls[1], pageOf(range(49, 48), 2, 3, 120));
    expect(result.current.items).toHaveLength(96);
    expect(result.current).toMatchObject({ page: 2, hasMore: true, loadingMore: false, lastPageCount: 48 });
  });

  it("dedupe per id dan berhenti di halaman terakhir", async () => {
    const { result } = renderHook(() => usePosCatalog());
    debounce();
    await settle(calls[0], pageOf([1, 2, 3], 1, 2));
    act(() => result.current.loadMore());
    await settle(calls[1], pageOf([3, 4, 4, 5], 2, 2));
    expect(ids(result.current.items)).toEqual([1, 2, 3, 4, 5]);
    expect(result.current.hasMore).toBe(false);
    act(() => result.current.loadMore());
    expect(calls).toHaveLength(2);
  });

  it("halaman yang tak menambah id baru dianggap akhir katalog", async () => {
    const { result } = renderHook(() => usePosCatalog());
    debounce();
    await settle(calls[0], pageOf([1, 2], 1, 5));
    act(() => result.current.loadMore());
    await settle(calls[1], pageOf([1, 2], 2, 5));
    expect(result.current.hasMore).toBe(false);
    expect(ids(result.current.items)).toEqual([1, 2]);
  });

  it("loadMore ganda saat memuat hanya mengirim satu request", async () => {
    const { result } = renderHook(() => usePosCatalog());
    debounce();
    act(() => result.current.loadMore()); // halaman 1 masih dimuat: no-op
    expect(calls).toHaveLength(1);
    await settle(calls[0], pageOf([1, 2], 1, 3));
    act(() => {
      result.current.loadMore();
      result.current.loadMore();
    });
    expect(calls).toHaveLength(2);
  });

  it("ganti kata kunci: request lama dibatalkan, item lama tetap tampil sampai halaman 1 baru tiba", async () => {
    const { result } = renderHook(() => usePosCatalog());
    debounce();
    await settle(calls[0], pageOf([1, 2], 1, 3));
    act(() => result.current.loadMore());
    const stalePage2 = calls[1];

    act(() => result.current.setProductQuery("baut"));
    debounce();
    expect(stalePage2.signal.aborted).toBe(true);
    expect(calls[2].params).toMatchObject({ search: "baut", page: 1 });
    expect(result.current).toMatchObject({ loadingFirst: true, loadingMore: false });
    expect(ids(result.current.items)).toEqual([1, 2]);

    // Respons basi generasi lama diabaikan.
    await settle(stalePage2, pageOf([3, 4], 2, 3));
    expect(ids(result.current.items)).toEqual([1, 2]);

    const gen = result.current.generation;
    await settle(calls[2], pageOf([10, 11], 1, 1));
    expect(ids(result.current.items)).toEqual([10, 11]);
    expect(result.current).toMatchObject({ page: 1, hasMore: false, loadingFirst: false });
    expect(result.current.generation).toBe(gen + 1);
  });

  it("ganti kategori: halaman 1 per kategori, respons halaman 1 lama diabaikan", async () => {
    const { result } = renderHook(() => usePosCatalog());
    debounce();
    const oldFirst = calls[0];
    act(() => result.current.setActiveCategoryId(7));
    debounce();
    expect(oldFirst.signal.aborted).toBe(true);
    expect(calls[1].params).toMatchObject({ categoryId: 7, page: 1 });
    await settle(calls[1], pageOf([70], 1, 1));
    await settle(oldFirst, pageOf([1, 2, 3], 1, 1));
    expect(ids(result.current.items)).toEqual([70]);
  });

  it("galat halaman 2: error tanpa toast & item tetap; retry memuat ulang halaman 2", async () => {
    const { result } = renderHook(() => usePosCatalog());
    debounce();
    await settle(calls[0], pageOf([1, 2], 1, 2));
    act(() => result.current.loadMore());
    await fail(calls[1], { status: 503, message: "Server sibuk" });
    expect(result.current.error).toBe("Server sibuk");
    expect(ids(result.current.items)).toEqual([1, 2]);
    expect(toastMock.danger).not.toHaveBeenCalled();

    act(() => result.current.loadMore()); // galat: no-op sampai retry
    expect(calls).toHaveLength(2);
    act(() => result.current.retry());
    expect(result.current.error).toBeNull();
    expect(calls[2].params).toMatchObject({ page: 2 });
    await settle(calls[2], pageOf([3], 2, 2));
    expect(ids(result.current.items)).toEqual([1, 2, 3]);
    expect(result.current.hasMore).toBe(false);
  });

  it("galat halaman 1: toast pesan server dan daftar kosong", async () => {
    const { result } = renderHook(() => usePosCatalog());
    debounce();
    await fail(calls[0], { status: 500, message: "Basis data tidak tersedia" });
    expect(toastMock.danger).toHaveBeenCalledWith("Gagal mencari produk", "Basis data tidak tersedia");
    expect(result.current).toMatchObject({ items: [], hasMore: false, loadingFirst: false, error: null });
  });

  it("unmount membatalkan request berjalan", async () => {
    const { unmount } = renderHook(() => usePosCatalog());
    debounce();
    unmount();
    expect(calls[0].signal.aborted).toBe(true);
  });
});
