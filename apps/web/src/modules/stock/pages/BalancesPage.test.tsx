import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const listBalances = vi.fn();
const balancesByProduct = vi.fn();
const scan = vi.fn();
vi.mock("@/modules/stock/services/stock.service", () => ({
  stockService: {
    listLocations: vi.fn().mockResolvedValue([]),
    listBalances: (...a: unknown[]) => listBalances(...a),
    balancesByProduct: (...a: unknown[]) => balancesByProduct(...a),
    scan: (...a: unknown[]) => scan(...a),
    getProduct: vi.fn().mockResolvedValue({
      id: 10, code: "P-10", name: "Semen", status: "active", variants: [{ id: 11, code: "V", name: "Std" }],
      baseUom: "pcs", purchaseUom: "dus", salesUom: "pcs", purchaseFactor: 12, salesFactor: 1,
    }),
  },
}));
vi.mock("@/modules/stock/components/ProductSelect", () => ({ ProductSelect: () => <div /> }));
vi.mock("@/modules/stock/components/VariantSelect", () => ({ VariantSelect: () => <div /> }));

import BalancesPage from "@/modules/stock/pages/BalancesPage";

beforeEach(() => {
  vi.clearAllMocks();
  listBalances.mockResolvedValue({
    items: [{ branchId: 1, productId: 10, variantId: 11, locationId: 5, onHand: 17, reserved: 0, available: 17, productCode: "P-10", productName: "Semen" }],
    meta: { page: 1, limit: 25, total: 1, totalPages: 1 },
  });
  balancesByProduct.mockResolvedValue([]);
  scan.mockResolvedValue({ status: "ok", product: { id: 10, code: "P-10", name: "Semen" }, variant: { id: 11, code: "V", name: "Std" } });
});
afterEach(() => cleanup());

describe("BalancesPage (ST-06)", () => {
  it("menampilkan label dual-satuan di samping angka satuan dasar", async () => {
    render(
      <MemoryRouter>
        <BalancesPage />
      </MemoryRouter>,
    );
    expect((await screen.findAllByText("1 dus 5 pcs")).length).toBeGreaterThan(0);
  });

  it("pindai barcode lalu Enter menyaring saldo ke produk itu", async () => {
    render(
      <MemoryRouter>
        <BalancesPage />
      </MemoryRouter>,
    );
    await screen.findAllByText("1 dus 5 pcs");
    const input = screen.getByLabelText(/Pindai barcode/);
    fireEvent.change(input, { target: { value: "8991234" } });
    fireEvent.keyDown(input, { key: "Enter" });
    await waitFor(() => expect(scan).toHaveBeenCalledWith("8991234"));
    await waitFor(() => expect(balancesByProduct).toHaveBeenCalledWith(10));
  });
});
