import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const revaluationPositions = vi.fn();
const revaluationPreview = vi.fn();
const revaluationCommit = vi.fn();
const listRevaluations = vi.fn();
vi.mock("@/modules/finance/services/corrections.service", () => ({
  correctionsService: {
    revaluationPositions: (...a: unknown[]) => revaluationPositions(...a),
    revaluationPreview: (...a: unknown[]) => revaluationPreview(...a),
    revaluationCommit: (...a: unknown[]) => revaluationCommit(...a),
    listRevaluations: (...a: unknown[]) => listRevaluations(...a),
  },
}));
// Pemilih produk asli memanggil server; di sini cukup tombol yang memilih produk #5.
vi.mock("@/modules/products/components/ProductSearchSelect", () => ({
  ProductSearchSelect: ({ onChange }: { onChange?: (id: number | null) => void }) => (
    <button type="button" onClick={() => onChange?.(5)}>
      Pilih REVAL
    </button>
  ),
}));

import InventoryRevaluationPage from "@/modules/finance/pages/InventoryRevaluationPage";

const line = {
  productId: 5,
  variantId: 6,
  productCode: "REVAL",
  productName: "Barang Revaluasi",
  qty: 10,
  unitCostBefore: 7000,
  unitCostAfter: 9000,
  valueBefore: 70000,
  valueAfter: 90000,
  delta: 20000,
};

beforeEach(() => {
  for (const m of [revaluationPositions, revaluationPreview, revaluationCommit, listRevaluations]) m.mockReset();
  revaluationPositions.mockResolvedValue([
    { productId: 5, variantId: 6, productCode: "REVAL", productName: "Barang Revaluasi", variantName: "", baseUom: "pcs", qty: 10, value: 70000, averageCost: 7000, lastReceiptCost: 0, catalogCost: 8500 },
  ]);
  revaluationPreview.mockResolvedValue({ message: "ok", data: { branchId: 1, date: "2026-10-21", reason: "", totalDelta: 20000, lines: [line], applied: false } });
  revaluationCommit.mockResolvedValue({
    message: "Harga modal direvaluasi",
    data: { id: 3, branchId: 1, date: "2026-10-21", reason: "salah ketik", totalDelta: 20000, lines: [line], applied: true, entryId: 44, entryNumber: "JV-TST-2026-0009" },
  });
  listRevaluations.mockResolvedValue({ items: [], meta: { page: 1, limit: 20, total: 0, totalPages: 0 } });
  useAuthStore.setState({ permissions: ["finance.view", "finance.manage", "finance.close.force"], ready: true });
});
afterEach(cleanup);

describe("InventoryRevaluationPage", () => {
  it("pilih produk → harga baru → pratinjau → bukukan dengan alasan", async () => {
    render(
      <MemoryRouter>
        <InventoryRevaluationPage />
      </MemoryRouter>,
    );
    expect(await screen.findByText("Belum ada revaluasi")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Pilih REVAL" }));
    expect(await screen.findByText(/Master Rp/)).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/Harga modal baru/), { target: { value: "9.000" } });
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    await waitFor(() =>
      expect(revaluationPreview).toHaveBeenCalledWith(expect.objectContaining({ productId: 5, variantId: 6, newUnitCost: 9000 })),
    );
    expect(await screen.findByText("Selisih dibukukan")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Bukukan revaluasi" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(screen.getByLabelText(/Alasan koreksi/), { target: { value: "salah ketik" } });
    fireEvent.click(dialog.querySelector('button[type="submit"]') as HTMLButtonElement);
    await waitFor(() =>
      expect(revaluationCommit).toHaveBeenCalledWith(
        expect.objectContaining({ productId: 5, variantId: 6, newUnitCost: 9000, reason: "salah ketik", clientRef: expect.any(String) }),
      ),
    );
    expect(await screen.findByText("JV-TST-2026-0009")).toBeInTheDocument();
  });

  it("tanpa finance.close.force tidak ada tombol bukukan", async () => {
    useAuthStore.setState({ permissions: ["finance.view", "finance.manage"], ready: true });
    render(
      <MemoryRouter>
        <InventoryRevaluationPage />
      </MemoryRouter>,
    );
    fireEvent.click(await screen.findByRole("button", { name: "Pilih REVAL" }));
    await screen.findByText(/Master Rp/);
    fireEvent.change(screen.getByLabelText(/Harga modal baru/), { target: { value: "9000" } });
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    await screen.findByText("Selisih dibukukan");
    expect(screen.queryByRole("button", { name: "Bukukan revaluasi" })).toBeNull();
  });
});
