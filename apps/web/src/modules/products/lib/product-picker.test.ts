import { describe, expect, it, vi } from "vitest";

const { list, searchOptions } = vi.hoisted(() => ({ list: vi.fn(), searchOptions: vi.fn() }));
vi.mock("@/modules/products/services/products.service", async () => {
  const actual = await vi.importActual<typeof import("@/modules/products/services/products.service")>(
    "@/modules/products/services/products.service",
  );
  return { ...actual, productsService: { list, searchOptions } };
});

import { loadActiveProductPage, PRODUCT_PICKER_PAGE_SIZE, searchOptionPage } from "@/modules/products/lib/product-picker";

describe("loadActiveProductPage", () => {
  it("meminta barang aktif per halaman dan memetakan opsi + hasMore dari meta", async () => {
    list.mockResolvedValueOnce({
      items: [{ id: 7, code: "BRG-7", name: "Barang Tujuh" }],
      meta: { page: 2, limit: PRODUCT_PICKER_PAGE_SIZE, total: 83, totalPages: 3 },
    });
    const signal = new AbortController().signal;
    const res = await loadActiveProductPage("kabel", signal, 2);
    expect(list).toHaveBeenCalledWith(
      { search: "kabel", status: "active", page: 2, limit: PRODUCT_PICKER_PAGE_SIZE },
      signal,
    );
    expect(res).toEqual({ options: [{ value: 7, label: "BRG-7 — Barang Tujuh" }], hasMore: true });
  });

  it("halaman terakhir: hasMore false; tanpa page = halaman 1", async () => {
    list.mockResolvedValueOnce({ items: [], meta: { page: 1, limit: 30, total: 0, totalPages: 0 } });
    const res = await loadActiveProductPage("");
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1 }), undefined);
    expect(res.hasMore).toBe(false);
  });
});

describe("searchOptionPage (search-options berhalaman)", () => {
  const rows = (n: number) => Array.from({ length: n }, (_, i) => ({ id: i + 1, code: `P${i + 1}`, name: `Barang ${i + 1}` }));

  it("mengirim page + limit 30 dan filter pemanggil; halaman penuh = masih ada lagi", async () => {
    searchOptions.mockResolvedValueOnce(rows(PRODUCT_PICKER_PAGE_SIZE));
    const signal = new AbortController().signal;
    const res = await searchOptionPage("cat", 2, { tracked: false }, signal);
    expect(searchOptions).toHaveBeenCalledWith({ tracked: false, q: "cat", page: 2, limit: PRODUCT_PICKER_PAGE_SIZE }, signal);
    expect(res.hasMore).toBe(true);
    expect(res.rows).toHaveLength(PRODUCT_PICKER_PAGE_SIZE);
  });

  it("halaman tidak penuh = habis; kata kunci kosong tidak dikirim", async () => {
    searchOptions.mockResolvedValueOnce(rows(5));
    const res = await searchOptionPage("", 3, {});
    expect(searchOptions).toHaveBeenLastCalledWith({ q: undefined, page: 3, limit: PRODUCT_PICKER_PAGE_SIZE }, undefined);
    expect(res.hasMore).toBe(false);
  });
});
