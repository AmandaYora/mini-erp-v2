import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const createAdjustment = vi.fn();
vi.mock("@/modules/stock/services/stock.service", () => ({
  stockService: {
    adjustAlwaysApprove: vi.fn().mockResolvedValue(false),
    listLocations: vi.fn().mockResolvedValue([{ id: 5, name: "Gudang", code: "G" }]),
    balancesByProduct: vi.fn().mockResolvedValue([{ productId: 10, variantId: 11, locationId: 5, onHand: 100, reserved: 0, available: 100 }]),
    createAdjustment: (...a: unknown[]) => createAdjustment(...a),
  },
}));
vi.mock("@/modules/stock/components/ProductSelect", () => ({
  ProductSelect: ({ onChange }: { onChange: (n: number) => void }) => (
    <button type="button" onClick={() => onChange(10)}>
      pilih produk
    </button>
  ),
}));
vi.mock("@/modules/stock/components/VariantSelect", () => ({
  VariantSelect: ({ onChange }: { onChange: (n: number) => void }) => (
    <button type="button" onClick={() => onChange(11)}>
      pilih varian
    </button>
  ),
}));
vi.mock("@/modules/stock/components/StockLocationSelect", () => ({
  StockLocationSelect: ({ onChange }: { onChange: (n: number) => void }) => (
    <button type="button" onClick={() => onChange(5)}>
      pilih lokasi
    </button>
  ),
}));

import AdjustmentsPage from "@/modules/stock/pages/AdjustmentsPage";

function open() {
  return render(
    <MemoryRouter>
      <AdjustmentsPage />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  createAdjustment.mockResolvedValue({ message: "ok", data: null });
});
afterEach(() => cleanup());

describe("AdjustmentsPage — tujuan koreksi (ADR-0024)", () => {
  it("menyembunyikan koreksi saldo awal tanpa izin stock.adjust_opening", async () => {
    useAuthStore.setState({ permissions: ["stock.view", "stock.adjust"], user: null, ready: true });
    open();
    await screen.findByLabelText("Mode koreksi");
    expect(screen.queryByLabelText("Tujuan koreksi")).toBeNull();
  });

  it("mengirim purpose opening_correction bila dipilih", async () => {
    useAuthStore.setState({ permissions: ["stock.view", "stock.adjust", "stock.adjust_opening"], user: null, ready: true });
    open();
    fireEvent.change(await screen.findByLabelText("Tujuan koreksi"), { target: { value: "opening_correction" } });
    expect(screen.getByText(/tidak memengaruhi laba/)).toBeTruthy();
    fireEvent.click(screen.getByText("pilih produk"));
    fireEvent.click(screen.getByText("pilih varian"));
    fireEvent.click(screen.getByText("pilih lokasi"));
    fireEvent.change(screen.getByLabelText("Qty fisik baris 1"), { target: { value: "2" } });
    fireEvent.change(screen.getByLabelText(/Alasan/), { target: { value: "stok awal impor salah" } });
    fireEvent.click(screen.getByText("Simpan koreksi"));
    await waitFor(() => expect(createAdjustment).toHaveBeenCalledTimes(1));
    expect(createAdjustment.mock.calls[0][0]).toMatchObject({ mode: "set", qtyAfter: 2, purpose: "opening_correction" });
  });
});
