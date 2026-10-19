import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PurchaseReturnPreview } from "@/modules/purchasereturn/types";

const context = vi.fn();
const preview = vi.fn();
vi.mock("@/modules/purchasereturn/services/purchase-return.service", () => ({
  purchaseReturnService: {
    context: (...a: unknown[]) => context(...a),
    preview: (...a: unknown[]) => preview(...a),
    create: vi.fn(),
  },
  purchaseReturnLookupService: {
    locationOptions: vi.fn().mockResolvedValue([{ id: 3, code: "GD-1", name: "Gudang Utama", status: "active" }]),
    sourceOrderDetail: vi.fn().mockResolvedValue({ id: 9, number: "PO-0009", partyName: "Toko Maju", items: [] }),
  },
}));
vi.mock("@/modules/purchasing/components/PurchaseOrderSearchSelect", () => ({
  PurchaseOrderSearchSelect: () => <div>pilih order beli</div>,
}));

import PurchaseReturnCreatePage from "@/modules/purchasereturn/pages/PurchaseReturnCreatePage";

function previewData(over: Partial<PurchaseReturnPreview> = {}): PurchaseReturnPreview {
  return {
    subtotal: 100000,
    discountTotal: 0,
    taxTotal: 0,
    total: 100000,
    items: [],
    orderOutstanding: 60000,
    payableReduction: 60000,
    externalRemaining: 40000,
    allowedSettlementTypes: ["refund", "supplier_credit"],
    ...over,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  context.mockResolvedValue({
    purchaseOrderId: 9,
    orderNumber: "PO-0009",
    status: "confirmed",
    lines: [
      {
        productId: 10,
        variantId: 11,
        productCode: "P-10",
        productName: "Semen",
        uom: "sak",
        uomFactor: 1,
        orderedQty: 4,
        received: 4,
        returned: 0,
        remaining: 4,
      },
    ],
    locations: [{ id: 3, code: "GD-1", name: "Gudang Utama" }],
  });
});
afterEach(() => cleanup());

async function toReviewStep() {
  render(
    <MemoryRouter initialEntries={["/purchase-returns/new?from=9"]}>
      <Routes>
        <Route path="/purchase-returns/new" element={<PurchaseReturnCreatePage />} />
      </Routes>
    </MemoryRouter>,
  );
  await screen.findByText("Semen");
  await userEvent.selectOptions(screen.getByLabelText("Lokasi keluar Semen"), "3");
  await userEvent.click(screen.getByRole("button", { name: "Lanjut" }));
  await userEvent.type(await screen.findByPlaceholderText(/Mis\. barang cacat/), "Barang cacat");
}

describe("halaman Buat Retur Beli: langkah Tinjau menampilkan penyelesaian (K-04d)", () => {
  it("pratinjau menampilkan sisa hutang, pengurang hutang otomatis, dan kelebihan yang sah", async () => {
    preview.mockResolvedValue({ message: "ok", data: previewData() });
    await toReviewStep();
    await userEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    const list = await screen.findByRole("list", { name: "Penyelesaian nilai retur" });
    await waitFor(() => expect(list.querySelectorAll("li").length).toBeGreaterThan(0));
    const text = list.textContent ?? "";
    expect(text).toMatch(/Sisa hutang order sebelum retur: Rp\s?60\.000/);
    expect(text).toMatch(/Mengurangi hutang otomatis saat dikonfirmasi: Rp\s?60\.000/);
    expect(text).toMatch(/Kelebihan Rp\s?40\.000 dapat diselesaikan sebagai refund dari supplier atau kredit supplier/);
    expect(preview.mock.calls[0][0]).toMatchObject({ purchaseOrderId: 9 });
  });

  it("retur yang seluruhnya mengurangi hutang: tak ada refund/saldo supplier", async () => {
    preview.mockResolvedValue({
      message: "ok",
      data: previewData({ payableReduction: 100000, orderOutstanding: 150000, externalRemaining: 0, allowedSettlementTypes: [] }),
    });
    await toReviewStep();
    await userEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    const list = await screen.findByRole("list", { name: "Penyelesaian nilai retur" });
    expect(within(list).getByText(/Seluruh nilai retur mengurangi hutang/)).toBeTruthy();
  });
});
