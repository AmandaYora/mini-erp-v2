import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { PurchaseReturn } from "@/modules/purchasereturn/types";

const getReturn = vi.fn();
const addSettlement = vi.fn();
vi.mock("@/modules/purchasereturn/services/purchase-return.service", () => ({
  purchaseReturnService: {
    get: (...a: unknown[]) => getReturn(...a),
    addSettlement: (...a: unknown[]) => addSettlement(...a),
    confirm: vi.fn(),
    cancel: vi.fn(),
  },
}));

import PurchaseReturnDetailPage from "@/modules/purchasereturn/pages/PurchaseReturnDetailPage";

function ret(over: Partial<PurchaseReturn> = {}): PurchaseReturn {
  return {
    id: 5,
    number: "RB-0005",
    branchId: 1,
    purchaseOrderId: 9,
    orderNumber: "PO-0009",
    returnDate: "2026-10-10",
    subtotal: 100000,
    discountTotal: 0,
    taxTotal: 0,
    taxType: "none",
    taxRate: 0,
    total: 100000,
    status: "confirmed",
    notes: "",
    reason: "Barang cacat",
    payableReduction: 60000,
    externalRemaining: 40000,
    allowedSettlementTypes: ["refund", "supplier_credit"],
    items: [],
    settlements: [],
    settledTotal: 0,
    unsettledTotal: 100000,
    ...over,
  };
}

const ALL = ["purchasereturn.view", "purchasereturn.create", "purchasereturn.archive", "purchasereturn.refund"];

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/purchase-returns/5"]}>
      <Routes>
        <Route path="/purchase-returns/:id" element={<PurchaseReturnDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ permissions: ALL, user: null, ready: true });
});
afterEach(() => cleanup());

describe("detail retur beli: penyelesaian memakai kamus pembelian (K-04b/c)", () => {
  it("sheet penyelesaian menawarkan Refund dari supplier & Kredit supplier, TIDAK 'Terima pembayaran'", async () => {
    getReturn.mockResolvedValue(ret());
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Catat penyelesaian" }));
    const select = await screen.findByRole("combobox");
    const labels = within(select).getAllByRole("option").map((o) => o.textContent);
    expect(labels).toEqual(["Pilih…", "Refund dari supplier", "Kredit supplier"]);
    expect(screen.queryByText(/Terima pembayaran/)).toBeNull();
    expect(screen.queryByText(/Kredit pelanggan/)).toBeNull();
    expect(screen.queryByText(/Kurangi utang/)).toBeNull();
  });

  it("nilai customer_credit/reduce_receivable lama dari data lama tidak memunculkan label penjualan", async () => {
    getReturn.mockResolvedValue(
      ret({
        settlements: [
          { id: 1, type: "supplier_credit", date: "2026-10-11", amount: 10000, paymentMethod: "", referenceNumber: "", notes: "" },
          { id: 2, type: "refund", date: "2026-10-12", amount: 5000, paymentMethod: "cash", referenceNumber: "", notes: "" },
        ],
        externalRemaining: 25000,
      }),
    );
    renderPage();
    expect((await screen.findAllByText("Kredit supplier")).length).toBeGreaterThan(0);
    expect(screen.getByText("Refund dari supplier")).toBeTruthy();
    expect(screen.queryByText("Kredit pelanggan")).toBeNull();
  });

  it("nominal awal = kelebihan di atas sisa hutang, dan simpan mengirim jenis pembelian", async () => {
    getReturn.mockResolvedValue(ret());
    addSettlement.mockResolvedValue({ message: "ok", data: ret({ externalRemaining: 0, allowedSettlementTypes: [] }) });
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Catat penyelesaian" }));
    expect(await screen.findByText(/Kelebihan yang bisa diselesaikan/)).toBeTruthy();
    await userEvent.selectOptions(await screen.findByRole("combobox"), "supplier_credit");
    const amount = screen.getByPlaceholderText("Nominal (Rp)") as HTMLInputElement;
    await userEvent.clear(amount);
    await userEvent.type(amount, "40000");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(addSettlement).toHaveBeenCalled());
    expect(addSettlement.mock.calls[0][0]).toBe(5);
    expect(addSettlement.mock.calls[0][1]).toMatchObject({ type: "supplier_credit", amount: 40000 });
  });

  it("retur yang seluruhnya diserap tagihan tidak punya tombol penyelesaian dan menjelaskannya", async () => {
    getReturn.mockResolvedValue(ret({ payableReduction: 100000, externalRemaining: 0, allowedSettlementTypes: [] }));
    renderPage();
    await screen.findByRole("heading", { name: "RB-0005" });
    expect(screen.queryByRole("button", { name: "Catat penyelesaian" })).toBeNull();
    expect(screen.getByText(/sudah mengurangi hutang/)).toBeTruthy();
  });

  it("tanpa izin refund pilihan Refund disembunyikan", async () => {
    useAuthStore.setState({ permissions: ["purchasereturn.view", "purchasereturn.create"], user: null, ready: true });
    getReturn.mockResolvedValue(ret());
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Catat penyelesaian" }));
    const select = await screen.findByRole("combobox");
    expect(within(select).getAllByRole("option").map((o) => o.textContent)).toEqual(["Pilih…", "Kredit supplier"]);
  });
});
