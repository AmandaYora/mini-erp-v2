import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

// Pemilih produk diganti tombol per produk; yang diuji hanya pilihan varian baris (temuan 2 + variantfix).
type Opt = { value: number; label: string };
vi.mock("@/shared/components/ui", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/components/ui")>();
  const { useEffect, useState } = await import("react");
  return {
    ...actual,
    AsyncSearchSelect: ({
      loadOptions,
      onChange,
    }: {
      loadOptions: (q: string, signal?: AbortSignal, page?: number) => Promise<{ options: Opt[] }>;
      onChange: (v: number) => void;
    }) => {
      const [options, setOptions] = useState<Opt[]>([]);
      useEffect(() => {
        void loadOptions("", undefined, 1).then((r) => setOptions(r.options));
      }, [loadOptions]);
      return (
        <div>
          {options.map((o) => (
            <button key={o.value} type="button" onClick={() => onChange(o.value)}>
              pilih {o.label}
            </button>
          ))}
        </div>
      );
    },
  };
});
vi.mock("@/modules/products/lib/product-picker", () => ({
  loadActiveProductPage: vi.fn().mockResolvedValue({
    options: [
      { value: 30, label: "CAT — Cat Tembok" },
      { value: 40, label: "KBL — Kabel Mika" },
    ],
    hasMore: false,
  }),
}));
vi.mock("@/modules/party/components/PartySearchSelect", () => ({ PartySearchSelect: () => <div /> }));
vi.mock("@/modules/users/components/UserSearchSelect", () => ({ UserSearchSelect: () => <div /> }));

const variant = (id: number, code: string, name: string, over: Record<string, unknown> = {}) => ({
  id, code, name, isDefault: false, status: "active", isHidden: false, sortOrder: 0, ...over,
});
const base = {
  baseUom: "kg", purchaseUom: "kg", salesUom: "kg", purchaseFactor: 1, salesFactor: 1, purchasePrice: 40000, type: "barang",
};
const details: Record<number, unknown> = {
  // Sesudah variantfix: "Standar" bawaan tersembunyi + warna terlihat.
  30: { ...base, id: 30, code: "CAT", name: "Cat Tembok", variants: [
    variant(31, "CAT", "Standar", { isDefault: true, isHidden: true }),
    variant(33, "CAT-002", "Merah", { sortOrder: 2 }),
    variant(32, "CAT-001", "Putih", { sortOrder: 1 }),
  ] },
  // Produk satu-varian: hanya "Standar" tersembunyi.
  40: { ...base, id: 40, code: "KBL", name: "Kabel Mika", variants: [variant(41, "KBL", "Standar", { isDefault: true, isHidden: true })] },
};
vi.mock("@/modules/purchasing/services/purchase-order.service", () => ({
  purchaseOrderService: { get: vi.fn(), create: vi.fn(), update: vi.fn() },
  purchasingLookupService: {
    productDetail: (id: number) => Promise.resolve(details[id]),
  },
}));

import PurchaseOrderFormPage from "@/modules/purchasing/pages/PurchaseOrderFormPage";

function open() {
  return render(
    <MemoryRouter initialEntries={["/purchase-orders/new"]}>
      <Routes>
        <Route path="/purchase-orders/new" element={<PurchaseOrderFormPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  try {
    localStorage.clear();
  } catch {
    // tanpa storage
  }
  useAuthStore.setState({ permissions: ["purchasing.create"], ready: true });
});
afterEach(() => cleanup());

describe("PurchaseOrderFormPage — varian baris", () => {
  it("produk ber-warna: hanya warna terlihat (urut), wajib dipilih; 'Standar' tersembunyi tidak ditawarkan", async () => {
    open();
    fireEvent.click((await screen.findAllByText("pilih CAT — Cat Tembok"))[0]);
    const select = (await screen.findByLabelText("Varian baris 1")) as HTMLSelectElement;
    expect(select.value).toBe("");
    expect([...select.options].map((o) => o.textContent)).toEqual(["Pilih varian…", "CAT-001 — Putih", "CAT-002 — Merah"]);
  });

  it("produk satu-varian dengan 'Standar' tersembunyi: varian terisi otomatis tanpa pemilih", async () => {
    open();
    fireEvent.click((await screen.findAllByText("pilih KBL — Kabel Mika"))[0]);
    await screen.findByText("pilih KBL — Kabel Mika");
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.queryByLabelText("Varian baris 1")).toBeNull();
  });
});
