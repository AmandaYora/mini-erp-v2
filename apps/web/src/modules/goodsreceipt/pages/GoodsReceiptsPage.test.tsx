import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const list = vi.fn();
vi.mock("@/modules/goodsreceipt/services/goodsreceipt.service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/modules/goodsreceipt/services/goodsreceipt.service")>();
  return { ...actual, goodsreceiptService: { list: (...a: unknown[]) => list(...a) } };
});
vi.mock("@/modules/purchasing/components/PurchaseOrderSearchSelect", () => ({
  PurchaseOrderSearchSelect: () => <div data-testid="po-picker" />,
}));

import GoodsReceiptsPage from "@/modules/goodsreceipt/pages/GoodsReceiptsPage";

beforeEach(() => {
  list.mockReset();
  list.mockResolvedValue({
    items: [
      {
        id: 9,
        branchId: 1,
        purchaseOrderId: 3,
        number: "GR-0009",
        status: "posted",
        supplierDeliveryNumber: "SJ-77",
        orderNumber: "PO-0003",
        receivedAt: "2026-10-02 00:00:00",
        notes: "",
        lineCount: 4,
        receivedBy: 2,
        receivedByName: "Petugas Gudang",
        items: [],
      },
    ],
    meta: { page: 1, limit: 20, total: 1, totalPages: 1 },
  });
  useAuthStore.setState({ permissions: ["goodsreceipt.view"], ready: true });
});
afterEach(cleanup);

describe("daftar penerimaan (GR-01)", () => {
  it("mengisi kolom No. PO, Baris, dan Diterima oleh dari server", async () => {
    render(
      <MemoryRouter initialEntries={["/goods-receipts"]}>
        <GoodsReceiptsPage />
      </MemoryRouter>,
    );
    expect(await screen.findByText("GR-0009")).toBeInTheDocument();
    expect(screen.getByText("PO-0003")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("Petugas Gudang")).toBeInTheDocument();
  });
});
