import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const damagedList = vi.fn();
const damagedMoveIn = vi.fn();
vi.mock("@/modules/stock/services/stock.service", () => ({
  stockService: {
    damagedList: (...a: unknown[]) => damagedList(...a),
    damagedMoveIn: (...a: unknown[]) => damagedMoveIn(...a),
    listLocations: vi.fn().mockResolvedValue([]),
    getProduct: vi.fn().mockResolvedValue({ id: 10, code: "P-10", name: "Semen", variants: [{ id: 11, code: "V", name: "Std" }] }),
  },
}));
vi.mock("@/modules/stock/components/ProductPicker", () => ({
  ProductPicker: ({ onProductChange, onVariantChange }: { onProductChange: (n: number) => void; onVariantChange: (n: number) => void }) => (
    <button
      type="button"
      onClick={() => {
        onProductChange(10);
        onVariantChange(11);
      }}
    >
      pilih produk
    </button>
  ),
}));
vi.mock("@/modules/stock/components/LocationSelect", () => ({ LocationSelect: () => <div data-testid="loc" /> }));

import DamagedPage from "@/modules/stock/pages/DamagedPage";

const page = {
  items: [{ branchId: 1, productId: 10, variantId: 11, locationId: 5, onHand: 4, reserved: 0, available: 4, estimatedUnitCost: 12500, estimatedLoss: 50000 }],
  total: 1,
  withStock: 1,
  empty: 0,
  totalEstimatedLoss: 350000,
  page: 1,
  limit: 20,
  totalPages: 1,
};

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ permissions: ["stock.view", "stock.adjust"], user: null, ready: true });
  damagedList.mockResolvedValue(page);
  damagedMoveIn.mockResolvedValue({ message: "ok", data: {} });
});
afterEach(() => cleanup());

function open() {
  return render(
    <MemoryRouter>
      <DamagedPage />
    </MemoryRouter>,
  );
}

describe("DamagedPage (ST-03, ST-06)", () => {
  it("menampilkan perkiraan rugi per baris dan potensi nilai total dari server", async () => {
    open();
    expect(await screen.findByText("P-10 — Semen")).toBeTruthy();
    expect(damagedList).toHaveBeenCalledWith({ page: 1, limit: 20, empty: false });
    expect(screen.getByText(/Potensi nilai/)).toBeTruthy();
    expect(screen.getAllByText(/50\.000/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/350\.000/).length).toBeGreaterThan(0);
  });

  it("catat rusak tanpa lokasi asal mengirim locationId 0", async () => {
    open();
    await screen.findByText("P-10 — Semen");
    fireEvent.click(screen.getByRole("button", { name: "Catat rusak" }));
    fireEvent.click(await screen.findByRole("button", { name: "pilih produk" }));
    fireEvent.change(screen.getByLabelText(/Jumlah/), { target: { value: "2" } });
    fireEvent.change(screen.getByLabelText(/Alasan/), { target: { value: "datang rusak" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(damagedMoveIn).toHaveBeenCalled());
    expect(damagedMoveIn).toHaveBeenCalledWith(expect.objectContaining({ productId: 10, variantId: 11, locationId: 0, qty: 2 }));
  });
});
