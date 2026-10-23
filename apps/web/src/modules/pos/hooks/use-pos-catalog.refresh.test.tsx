import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const svc = vi.hoisted(() => ({ products: vi.fn(), categories: vi.fn().mockResolvedValue([]) }));
vi.mock("@/modules/pos/services/pos.service", () => ({ posService: svc }));
vi.mock("@/shared/stores/toast.store", () => ({ toast: { danger: vi.fn(), warning: vi.fn() } }));
import { usePosCatalog } from "@/modules/pos/hooks/use-pos-catalog";

const page = (stock: number) => ({
  items: [
    { id: 1, code: "A", name: "A", sellingPrice: 1000, stockAvailable: stock, belowMinStock: false },
    { id: 2, code: "B", name: "B", sellingPrice: 2000, stockAvailable: 5, belowMinStock: false },
  ],
  meta: { page: 1, limit: 48, total: 2, totalPages: 1 },
});

describe("usePosCatalog.refreshStock (sesudah checkout)", () => {
  it("mengganti angka stok per id tanpa mengubah urutan atau generasi kisi", async () => {
    svc.products.mockResolvedValueOnce(page(10));
    const { result } = renderHook(() => usePosCatalog());
    await waitFor(() => expect(result.current.items).toHaveLength(2), { timeout: 2000 });
    const generation = result.current.generation;
    svc.products.mockResolvedValueOnce(page(7));
    await act(async () => {
      await result.current.refreshStock();
    });
    expect(result.current.items.map((i) => [i.id, i.stockAvailable])).toEqual([[1, 7], [2, 5]]);
    expect(result.current.generation).toBe(generation);
  });
});
