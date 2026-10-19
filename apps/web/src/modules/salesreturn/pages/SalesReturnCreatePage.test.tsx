import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SalesReturnPreview } from "@/modules/salesreturn/types";

const context = vi.fn();
const preview = vi.fn();
vi.mock("@/modules/salesreturn/services/sales-return.service", () => ({
  salesReturnService: {
    context: (...a: unknown[]) => context(...a),
    preview: (...a: unknown[]) => preview(...a),
    create: vi.fn(),
  },
  salesReturnLookupService: {
    locationOptions: vi.fn().mockResolvedValue([{ id: 3, code: "GD-1", name: "Gudang Utama", status: "active" }]),
    productOptions: vi.fn().mockResolvedValue([]),
    productDetail: vi.fn(),
    sourceOrderDetail: vi.fn().mockResolvedValue({ id: 7, number: "SO-0007", partyName: "Budi", items: [] }),
  },
}));
vi.mock("@/modules/sales/components/SalesOrderSearchSelect", () => ({
  SalesOrderSearchSelect: () => <div>pilih order</div>,
}));

import SalesReturnCreatePage from "@/modules/salesreturn/pages/SalesReturnCreatePage";

function previewData(over: Partial<SalesReturnPreview> = {}): SalesReturnPreview {
  return {
    mode: "return_only",
    subtotal: 100000,
    discountTotal: 0,
    taxTotal: 0,
    total: 100000,
    items: [],
    replacementItems: [],
    replacementTotal: 0,
    difference: 0,
    owedToCustomer: 100000,
    chargeToCustomer: 0,
    orderOutstanding: 60000,
    receivableReduction: 60000,
    externalRemaining: 40000,
    allowedSettlementTypes: ["refund", "customer_credit"],
    hasParty: true,
    ...over,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  context.mockResolvedValue({
    salesOrderId: 7,
    orderNumber: "SO-0007",
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
        delivered: 4,
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
    <MemoryRouter initialEntries={["/sales-returns/new?from=7"]}>
      <Routes>
        <Route path="/sales-returns/new" element={<SalesReturnCreatePage />} />
      </Routes>
    </MemoryRouter>,
  );
  await screen.findByText("Semen");
  await userEvent.selectOptions(screen.getByLabelText("Lokasi masuk Semen"), "3");
  await userEvent.click(screen.getByRole("button", { name: "Lanjut" }));
  await userEvent.type(await screen.findByPlaceholderText("Mis. barang cacat / salah kirim"), "Barang cacat");
}

describe("halaman Buat Retur Jual: langkah Tinjau menampilkan penyelesaian (K-04d)", () => {
  it("pratinjau menampilkan selisih terutang, pengurang piutang otomatis, dan jenis yang sah", async () => {
    preview.mockResolvedValue({ message: "ok", data: previewData() });
    await toReviewStep();
    await userEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    const list = await screen.findByRole("list", { name: "Penyelesaian nilai retur" });
    await waitFor(() => expect(list.querySelectorAll("li").length).toBeGreaterThan(0));
    const text = list.textContent ?? "";
    expect(text).toContain("Nilai terutang ke pelanggan");
    expect(text).toMatch(/Mengurangi piutang otomatis saat dikonfirmasi: Rp\s?60\.000/);
    expect(text).toMatch(/Sisa Rp\s?40\.000 dapat diselesaikan sebagai refund atau saldo pelanggan/);
    // Body pratinjau memakai payload retur yang sama dengan buat.
    expect(preview.mock.calls[0][0]).toMatchObject({ salesOrderId: 7, returnMode: "return_only" });
  });

  it("retur yang seluruhnya diserap tagihan: tak ada yang direfund", async () => {
    preview.mockResolvedValue({
      message: "ok",
      data: previewData({ receivableReduction: 100000, orderOutstanding: 150000, externalRemaining: 0, allowedSettlementTypes: [] }),
    });
    await toReviewStep();
    await userEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    const list = await screen.findByRole("list", { name: "Penyelesaian nilai retur" });
    expect(within(list).getByText(/Seluruh nilai retur mengurangi piutang/)).toBeTruthy();
    expect(list.textContent).not.toContain("dapat diselesaikan sebagai");
  });

  it("tukar lebih mahal: selisih ditagih, bukan direfund", async () => {
    preview.mockResolvedValue({
      message: "ok",
      data: previewData({
        mode: "exchange",
        replacementTotal: 130000,
        difference: 30000,
        owedToCustomer: 0,
        chargeToCustomer: 30000,
        receivableReduction: 0,
        externalRemaining: 0,
        allowedSettlementTypes: [],
      }),
    });
    await toReviewStep();
    await userEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    const list = await screen.findByRole("list", { name: "Penyelesaian nilai retur" });
    expect(list.textContent).toMatch(/Pelanggan membayar selisih Rp\s?30\.000/);
    expect(list.textContent).not.toContain("refund");
  });
});
