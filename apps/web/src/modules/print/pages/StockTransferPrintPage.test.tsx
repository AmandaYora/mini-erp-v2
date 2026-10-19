import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resolveIdentity } from "@/modules/print/document";
import { CONTINUOUS_PAPER_DEFAULTS, resolvePaperProfile } from "@/modules/print/paper";

const profileState = {
  profile: { continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS } },
  paper: resolvePaperProfile(undefined),
  identity: resolveIdentity({ continuousNota: { ...CONTINUOUS_PAPER_DEFAULTS } }, { companyName: "Toko Uji", companyCity: "Jakarta" }),
  corrupt: false,
  warning: null as string | null,
  loading: false,
};
vi.mock("@/modules/print/hooks/usePrintProfile", () => ({ usePrintProfile: () => profileState }));

const getTransfer = vi.fn();
vi.mock("@/modules/stock/services/stock.service", () => ({
  stockService: {
    getTransfer: (...a: unknown[]) => getTransfer(...a),
    listBranches: vi.fn().mockResolvedValue([
      { id: 1, code: "PST", name: "Pusat", status: "active" },
      { id: 2, code: "BDG", name: "Bandung", status: "active" },
    ]),
    listLocations: vi.fn().mockResolvedValue([{ id: 11, code: "GDG", name: "Gudang", branchId: 1 }]),
    getProduct: vi.fn().mockResolvedValue({ id: 10, code: "P-10", name: "Semen", baseUom: "sak", variants: [{ id: 11, code: "V1", name: "Std" }] }),
  },
}));

import StockTransferPrintPage from "@/modules/print/pages/StockTransferPrintPage";

beforeEach(() => {
  getTransfer.mockResolvedValue({
    id: 5, number: "TRF-0005", fromBranchId: 1, toBranchId: 2, fromLocationId: 11, toLocationId: 11, status: "dispatched",
    notes: "Hati-hati", driverName: "Joko", vehiclePlate: "B 1234 XY", transferDate: "2026-10-12",
    items: [{ productId: 10, variantId: 11, qty: 8, notes: "", toLocationId: 0 }],
  });
});
afterEach(() => cleanup());

describe("StockTransferPrintPage (ST-02)", () => {
  it("mencetak surat transfer: dokumen, sopir, kendaraan, tanggal, barang, dan tiga tanda tangan", async () => {
    render(
      <MemoryRouter initialEntries={["/print/transfers/5"]}>
        <Routes>
          <Route path="/print/transfers/:id" element={<StockTransferPrintPage />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(await screen.findByText("Surat Transfer Barang")).toBeTruthy();
    expect(screen.getByText("TRF-0005")).toBeTruthy();
    expect(screen.getAllByText("Joko").length).toBeGreaterThan(0);
    expect(screen.getByText("B 1234 XY")).toBeTruthy();
    expect(screen.getByText("PST — Pusat")).toBeTruthy();
    expect(screen.getByText("BDG — Bandung")).toBeTruthy();
    expect(screen.getByText("Semen")).toBeTruthy();
    expect(screen.getByText("Gudang Asal", { selector: ".sj-sign-role" })).toBeTruthy();
    expect(screen.getByText("Sopir", { selector: ".sj-sign-role" })).toBeTruthy();
    expect(screen.getByText("Gudang Tujuan", { selector: ".sj-sign-role" })).toBeTruthy();
  });
});
