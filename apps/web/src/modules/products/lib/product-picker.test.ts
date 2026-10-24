import { describe, expect, it, vi } from "vitest";

const { list } = vi.hoisted(() => ({ list: vi.fn() }));
vi.mock("@/modules/products/services/products.service", () => ({ productsService: { list } }));

import { loadActiveProductPage, PRODUCT_PICKER_PAGE_SIZE } from "@/modules/products/lib/product-picker";

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
