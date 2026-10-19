import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const list = vi.fn();
vi.mock("@/modules/purchasing/services/purchase-order.service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/modules/purchasing/services/purchase-order.service")>();
  return {
    ...actual,
    purchaseOrderService: { list: (...a: unknown[]) => list(...a), downloadExport: vi.fn() },
  };
});
vi.mock("@/modules/party/components/PartySearchSelect", () => ({
  PartySearchSelect: () => <div data-testid="party-picker" />,
}));

import PurchaseOrdersPage from "@/modules/purchasing/pages/PurchaseOrdersPage";

const row = {
  id: 3,
  number: "PO-0003",
  partyId: 1,
  partyName: "Pemasok Satu",
  orderDate: "2026-10-01",
  dueDate: "2026-10-30",
  paymentTerms: "net",
  grandTotal: 200000,
  status: "completed",
  paidAmount: 50000,
  outstandingAmount: 150000,
  paymentStatus: "partial",
  isOverdue: false,
  createdByName: "Rani Pembuat",
  returnSummary: "partial",
};

beforeEach(() => {
  list.mockReset();
  list.mockResolvedValue({ items: [row], meta: { page: 1, limit: 20, total: 1, totalPages: 1 } });
  useAuthStore.setState({ permissions: ["purchasing.view"], ready: true });
});
afterEach(cleanup);

describe("daftar order beli (PO-01)", () => {
  it("menampilkan dibayar, sisa, status bayar, pembuat, dan badge retur", async () => {
    render(
      <MemoryRouter initialEntries={["/purchase-orders"]}>
        <PurchaseOrdersPage />
      </MemoryRouter>,
    );
    expect(await screen.findByText("PO-0003")).toBeInTheDocument();
    expect(screen.getByText("Bayar sebagian")).toBeInTheDocument();
    expect(screen.getByText("Rani Pembuat")).toBeInTheDocument();
    expect(screen.getByText("Diretur sebagian")).toBeInTheDocument();
    expect(screen.getAllByText(/150\.000/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/50\.000/).length).toBeGreaterThan(1);
  });
});
