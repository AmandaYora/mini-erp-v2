import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

// posService tiruan: hanya detail produk yang relevan; panggilan lain mengembalikan nilai kosong.
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

describe("usePosCart — qty desimal (temuan 7)", () => {
  async function cartWithCable() {
    svc.fns.productDetail = vi.fn().mockResolvedValue({
      ...product([variant(1, "KBL", { isDefault: true, isHidden: true })]),
      code: "KBL", name: "Kabel per meter", sellingPrice: 12345, minSellingPrice: 10000,
    });
    const hook = renderHook(() => usePosCart());
    await act(async () => {
      await hook.result.current.addProductById(7);
    });
    await waitFor(() => expect(hook.result.current.cart).toHaveLength(1));
    return hook;
  }

  it("qty 0,8 diterima; total dibulatkan ke rupiah; qty ≤ 0 diabaikan", async () => {
    const { result } = await cartWithCable();
    const key = result.current.cart[0].key;
    act(() => result.current.setQty(key, 0.8));
    expect(result.current.cart[0].qty).toBe(0.8);
    expect(result.current.totals.total).toBe(9876); // round(0,8 × 12.345)
    act(() => result.current.setQty(key, 0));
    act(() => result.current.setQty(key, -2));
    act(() => result.current.setQty(key, Number.NaN));
    expect(result.current.cart[0].qty).toBe(0.8);
    act(() => result.current.setQty(key, 1.23456));
    expect(result.current.cart[0].qty).toBe(1.2346); // 4 desimal
  });

  it("tombol ±1 dari 0,8 tanpa galat floating point; tidak pernah ≤ 0", async () => {
    const { result } = await cartWithCable();
    const key = result.current.cart[0].key;
    act(() => result.current.setQty(key, 0.8));
    act(() => result.current.changeQty(key, 1));
    expect(result.current.cart[0].qty).toBe(1.8);
    act(() => result.current.changeQty(key, -1));
    expect(result.current.cart[0].qty).toBe(0.8);
    act(() => result.current.changeQty(key, -1));
    expect(result.current.cart[0].qty).toBe(0.8);
  });

  it("ketuk/pindai produk yang sama menambah 1 ke qty desimal; diskon dijepit harga minimum × qty", async () => {
    const { result } = await cartWithCable();
    const key = result.current.cart[0].key;
    act(() => result.current.setQty(key, 0.8));
    await act(async () => {
      await result.current.addProductById(7);
    });
    await waitFor(() => expect(result.current.cart[0].qty).toBe(1.8));
    // Batas potongan: round(12.345 × 1,8) − round(10.000 × 1,8) = 22.221 − 18.000.
    act(() => result.current.setLineDiscount(key, { discountNominal: 999999 }));
    expect(result.current.cart[0].discountNominal).toBe(4221);
  });
});
