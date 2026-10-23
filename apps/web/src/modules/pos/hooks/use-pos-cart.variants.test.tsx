import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

// posService tiruan: hanya detail produk + saldo yang relevan; panggilan lain mengembalikan nilai kosong.
const svc = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  return {
    fns,
    proxy: new Proxy({} as Record<string, unknown>, {
      get: (_t, k: string) => (fns[k] ??= vi.fn().mockResolvedValue([])),
    }),
  };
});
vi.mock("@/modules/pos/services/pos.service", () => ({
  posService: svc.proxy,
  findPosPayment: vi.fn().mockResolvedValue(null),
}));
vi.mock("@/shared/stores/toast.store", () => ({
  toast: { danger: vi.fn(), warning: vi.fn(), success: vi.fn(), info: vi.fn(), fromServer: vi.fn() },
}));

import { usePosCart } from "@/modules/pos/hooks/use-pos-cart";

const variant = (id: number, code: string, over: Record<string, unknown> = {}) => ({
  id, code, name: code, barcode: "", isDefault: false, status: "active", isHidden: false, sortOrder: 0, ...over,
});
const product = (variants: ReturnType<typeof variant>[]) => ({
  id: 7, code: "PRG-TD", name: "Paragon TD 1KG", baseUom: "kg", purchaseUom: "kg", salesUom: "kg",
  sellingPrice: 50000, minSellingPrice: 0, status: "active", variants,
});

beforeEach(() => {
  for (const k of Object.keys(svc.fns)) delete svc.fns[k];
  try {
    localStorage.clear();
  } catch {
    // tanpa storage
  }
  useAuthStore.setState({ permissions: ["sales.create", "payment.create", "stock.view"], ready: true });
});

describe("usePosCart — produk multi-varian (temuan 2)", () => {
  it("ketuk produk ber-varian membuka pilihan (tanpa 'Standar' tersembunyi) beserta stok per varian", async () => {
    svc.fns.productDetail = vi.fn().mockResolvedValue(
      product([variant(1, "PRG-TD", { isDefault: true, isHidden: true, name: "Standar" }), variant(3, "PRG-TD-002", { sortOrder: 2 }), variant(2, "PRG-TD-001", { sortOrder: 1 })]),
    );
    svc.fns.productBalances = vi.fn().mockResolvedValue([
      { variantId: 2, available: 4 },
      { variantId: 2, available: 1 },
    ]);
    const { result } = renderHook(() => usePosCart());
    let r: unknown;
    await act(async () => {
      r = await result.current.addProductById(7);
    });
    expect(r).toEqual({ ok: false, reason: "choose" });
    expect(result.current.cart).toHaveLength(0);
    expect(result.current.variantChoice?.variants.map((v) => [v.id, v.available])).toEqual([
      [2, 5],
      [3, 0],
    ]);
    await act(async () => {
      await result.current.addProductById(7, 3);
    });
    await waitFor(() => expect(result.current.cart.map((l) => l.variantId)).toEqual([3]));
  });

  it("satu varian terlihat langsung masuk keranjang tanpa bertanya", async () => {
    svc.fns.productDetail = vi.fn().mockResolvedValue(
      product([variant(1, "PRG-TD", { isDefault: true, isHidden: true }), variant(2, "PRG-TD-001"), variant(4, "PRG-TD-OLD", { status: "archived" })]),
    );
    const { result } = renderHook(() => usePosCart());
    await act(async () => {
      await result.current.addProductById(7);
    });
    expect(result.current.variantChoice).toBeNull();
    await waitFor(() => expect(result.current.cart.map((l) => l.variantId)).toEqual([2]));
  });
});
