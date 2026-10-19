import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const listMovements = vi.fn();
vi.mock("@/modules/stock/services/stock.service", () => ({
  stockService: {
    listMovements: (...a: unknown[]) => listMovements(...a),
    getProduct: vi.fn().mockResolvedValue({ id: 10, code: "P-10", name: "Semen", variants: [{ id: 11, code: "V", name: "Std" }] }),
  },
}));
vi.mock("@/modules/stock/components/ProductSelect", () => ({ ProductSelect: () => <div /> }));

import { StockMovementTab } from "@/modules/stock/components/StockMovementTab";

const mv = (over: object) => ({
  id: 1, productId: 10, variantId: 11, locationId: 5, direction: "out", type: "out", qty: 3, refType: "adjustment", refId: 9,
  notes: "koreksi opname", createdBy: 1, createdAt: "2026-10-10T03:00:00Z", balanceBefore: 20, balanceAfter: 17, movedAt: "2026-10-10T03:00:00Z", ...over,
});

beforeEach(() => {
  vi.clearAllMocks();
  listMovements.mockResolvedValue({
    items: [mv({}), mv({ id: 2, direction: "in", balanceBefore: null, balanceAfter: null, notes: "" })],
    meta: { page: 1, limit: 20, total: 2, totalPages: 1 },
  });
});
afterEach(() => cleanup());

describe("StockMovementTab (ST-04)", () => {
  it("menampilkan saldo sebelum/sesudah, catatan, dan tipe mutasi", async () => {
    render(<StockMovementTab locations={[{ id: 5, branchId: 1, code: "GDG", name: "Gudang", parentId: null, isSystem: true, kind: "normal", status: "active", isPickingArea: false, isPrimary: false, sortOrder: 0 }]} />);
    expect(await screen.findByText("koreksi opname")).toBeTruthy();
    expect(screen.getAllByText("Keluar").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Masuk").length).toBeGreaterThan(0);
    expect(screen.getByText("Saldo")).toBeTruthy();
    expect(screen.getByText("Catatan")).toBeTruthy();
  });

  it("memuat tanpa filter tipe secara default", async () => {
    render(<StockMovementTab locations={[]} />);
    await waitFor(() => expect(listMovements).toHaveBeenCalled());
    expect(listMovements.mock.calls[0][0].type).toBeUndefined();
  });
});
