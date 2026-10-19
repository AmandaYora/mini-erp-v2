import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const previewOpening = vi.fn();
vi.mock("@/modules/stock/services/stock.service", () => ({
  stockService: {
    previewOpening: (...a: unknown[]) => previewOpening(...a),
    commitOpening: vi.fn(),
    downloadOpeningTemplate: vi.fn(),
  },
}));

import OpeningPage from "@/modules/stock/pages/OpeningPage";

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ permissions: ["stock.adjust"], user: null, ready: true });
  previewOpening.mockResolvedValue({
    message: "ok",
    data: {
      validRows: 1,
      skipped: 1,
      errors: [],
      totalQty: 7,
      totalValue: 70000,
      rowsTruncated: false,
      warnings: ["1 baris dilewati karena jumlahnya kosong atau 0"],
      rows: [
        { row: 2, productCode: "P-1", productName: "Semen", variantCode: "", locationCode: "GDG", qty: 7, unitCost: 10000, value: 70000, status: "post", note: "" },
        { row: 3, productCode: "P-2", productName: "Paku", variantCode: "", locationCode: "GDG", qty: 0, unitCost: 5000, value: 0, status: "skip", note: "jumlah kosong atau 0" },
      ],
    },
  });
});
afterEach(() => cleanup());

describe("OpeningPage (ST-05)", () => {
  it("tidak menjanjikan format .xls", () => {
    const { container } = render(<OpeningPage />);
    expect(container.textContent).not.toMatch(/\.xls(?!x)/);
    expect(container.textContent).toMatch(/\.xlsx/);
    expect((container.querySelector('input[type="file"]') as HTMLInputElement).accept).toBe(".xlsx");
  });

  it("pratinjau menampilkan baris per barang, total jumlah dan nilai, dan peringatan", async () => {
    const { container } = render(<OpeningPage />);
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File(["x"], "stok.xlsx")] } });
    fireEvent.click(await screen.findByRole("button", { name: "Pratinjau" }));
    await waitFor(() => expect(previewOpening).toHaveBeenCalled());
    expect(await screen.findByText("Semen")).toBeTruthy();
    expect(screen.getByText("Paku")).toBeTruthy();
    expect(screen.getByText("Total jumlah")).toBeTruthy();
    expect(screen.getByText("Total nilai")).toBeTruthy();
    expect(screen.getAllByText(/jumlahnya kosong/).length).toBeGreaterThan(0);
  });
});
