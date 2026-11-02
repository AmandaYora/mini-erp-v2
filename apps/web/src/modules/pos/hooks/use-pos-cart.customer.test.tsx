import { act, renderHook } from "@testing-library/react";
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
import { loadDraft, saveDraft } from "@/modules/pos/lib/cart-draft";

beforeEach(() => {
  for (const k of Object.keys(svc.fns)) delete svc.fns[k];
  svc.fns.customerDetail = vi.fn().mockResolvedValue({ id: 4, name: "Budi", addresses: [] });
  try {
    localStorage.clear();
  } catch {
    // tanpa storage
  }
  useAuthStore.setState({ permissions: ["sales.create", "payment.create", "stock.view"], ready: true });
});

describe("usePosCart — pelanggan kembali ke Umum", () => {
  it("transaksi baru mengosongkan pelanggan", () => {
    const { result } = renderHook(() => usePosCart());
    act(() => result.current.setCustomerId(4));
    expect(result.current.customerId).toBe(4);
    act(() => result.current.newTransaction());
    expect(result.current.customerId).toBeNull();
  });

  it("kembali ke halaman POS: pelanggan draf yang belum checkout tidak dipulihkan", () => {
    const first = renderHook(() => usePosCart());
    act(() => first.result.current.setCustomerId(4));
    first.unmount();
    expect(loadDraft()?.customerId).toBe(4);
    const { result } = renderHook(() => usePosCart());
    expect(result.current.customerId).toBeNull();
  });

  it("draf yang checkout-nya sudah dimulai tetap memakai pelanggannya", () => {
    const first = renderHook(() => usePosCart());
    act(() => first.result.current.setCustomerId(4));
    first.unmount();
    const d = loadDraft();
    if (!d) throw new Error("draf tidak tersimpan");
    saveDraft({ ...d, started: true });
    const { result } = renderHook(() => usePosCart());
    expect(result.current.customerId).toBe(4);
  });
});
