import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { SalesReturn } from "@/modules/salesreturn/types";

const getReturn = vi.fn();
const addSettlement = vi.fn();
const confirmReplacement = vi.fn();
const findReplacementDelivery = vi.fn();
vi.mock("@/modules/salesreturn/services/sales-return.service", () => ({
  salesReturnService: {
    get: (...a: unknown[]) => getReturn(...a),
    addSettlement: (...a: unknown[]) => addSettlement(...a),
    confirmReplacement: (...a: unknown[]) => confirmReplacement(...a),
    findReplacementDelivery: (...a: unknown[]) => findReplacementDelivery(...a),
    confirm: vi.fn(),
    cancel: vi.fn(),
    dispatchReplacement: vi.fn(),
  },
}));

import SalesReturnDetailPage from "@/modules/salesreturn/pages/SalesReturnDetailPage";

function ret(over: Partial<SalesReturn> = {}): SalesReturn {
  return {
    id: 5,
    number: "RJ-0005",
    branchId: 1,
    salesOrderId: 7,
    orderNumber: "SO-0007",
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
    replacementSubtotal: 0,
    replacementTaxTotal: 0,
    replacementTotal: 0,
    differenceAmount: 0,
    receivableReduction: 60000,
    returnMode: "return_only",
    replacementDeliveryStatus: "not_required",
    items: [],
    replacementItems: [],
    settlements: [],
    settledTotal: 0,
    unsettledTotal: 100000,
    externalRemaining: 40000,
    chargeRemaining: 0,
    allowedSettlementTypes: ["refund", "customer_credit"],
    ...over,
  } as SalesReturn;
}

const ALL = ["salesreturn.view", "salesreturn.create", "salesreturn.archive", "salesreturn.refund", "payment.create"];

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/sales-returns/5"]}>
      <Routes>
        <Route path="/sales-returns/:id" element={<SalesReturnDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ permissions: ALL, user: null, ready: true });
});
afterEach(() => cleanup());

describe("detail retur jual: penyelesaian menurut arah selisih (K-04c)", () => {
  it("retur murni menawarkan Refund & Saldo pelanggan, TIDAK Tagih pembayaran / Kurangi piutang", async () => {
    getReturn.mockResolvedValue(ret());
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Catat penyelesaian" }));
    const select = await screen.findByRole("combobox");
    expect(within(select).getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Pilih…",
      "Refund",
      "Saldo pelanggan (uang muka)",
    ]);
  });

  it("memilih jenis mengisi nominal dengan batas sisa eksternal dan mengirimnya", async () => {
    getReturn.mockResolvedValue(ret());
    addSettlement.mockResolvedValue({ message: "ok", data: ret({ externalRemaining: 0, allowedSettlementTypes: [] }) });
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Catat penyelesaian" }));
    await userEvent.selectOptions(await screen.findByRole("combobox"), "customer_credit");
    expect((screen.getByPlaceholderText("Nominal (Rp)") as HTMLInputElement).value).toBe("40000");
    expect(screen.getByText(/Maksimal: Rp\s?40\.000/)).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(addSettlement).toHaveBeenCalled());
    expect(addSettlement.mock.calls[0][1]).toMatchObject({ type: "customer_credit", amount: 40000 });
  });

  it("tukar lebih mahal hanya menawarkan Tagih pembayaran sebesar selisih terbuka", async () => {
    getReturn.mockResolvedValue(
      ret({
        returnMode: "exchange",
        replacementDeliveryStatus: "confirmed",
        replacementTotal: 130000,
        differenceAmount: 30000,
        receivableReduction: 0,
        externalRemaining: 0,
        chargeRemaining: 30000,
        allowedSettlementTypes: ["collect_payment"],
      }),
    );
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Catat penyelesaian" }));
    const select = await screen.findByRole("combobox");
    expect(within(select).getAllByRole("option").map((o) => o.textContent)).toEqual(["Pilih…", "Tagih pembayaran"]);
    await userEvent.selectOptions(select, "collect_payment");
    expect((screen.getByPlaceholderText("Nominal (Rp)") as HTMLInputElement).value).toBe("30000");
  });

  it("retur yang seluruhnya diserap tagihan tidak punya tombol penyelesaian", async () => {
    getReturn.mockResolvedValue(ret({ receivableReduction: 100000, externalRemaining: 0, allowedSettlementTypes: [] }));
    renderPage();
    await screen.findByRole("heading", { name: "RJ-0005" });
    expect(screen.queryByRole("button", { name: "Catat penyelesaian" })).toBeNull();
    expect(screen.getByText(/Tidak ada nilai yang perlu diselesaikan/)).toBeTruthy();
  });
});

describe("konfirmasi terima pengganti: tagih selisih saat serah (K-04e)", () => {
  const exchange = () =>
    ret({
      returnMode: "exchange",
      replacementDeliveryStatus: "dispatched",
      replacementTotal: 130000,
      differenceAmount: 30000,
      receivableReduction: 0,
      externalRemaining: 0,
      chargeRemaining: 30000,
      allowedSettlementTypes: ["collect_payment"],
    });

  async function openConfirm() {
    getReturn.mockResolvedValue(exchange());
    findReplacementDelivery.mockResolvedValue({ id: 88, number: "SJ-88", salesOrderId: 7, status: "draft", documentKind: "replacement", salesReturnId: 5 });
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Konfirmasi terima pengganti" }));
    await screen.findByText(/Konfirmasi penerimaan SJ-88/);
  }

  it("mencentang tagih sekarang mengirim settlement pay_now bersama konfirmasi", async () => {
    confirmReplacement.mockResolvedValue({ message: "ok", data: { id: 88, number: "SJ-88", status: "confirmed" } });
    await openConfirm();
    await userEvent.click(screen.getByRole("checkbox", { name: /Tagih selisih sekarang/ }));
    expect((screen.getByDisplayValue("30000") as HTMLInputElement).value).toBe("30000");
    await userEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));
    await waitFor(() => expect(confirmReplacement).toHaveBeenCalled());
    expect(confirmReplacement.mock.calls[0][0]).toBe(5);
    expect(confirmReplacement.mock.calls[0][1]).toBe(88);
    expect(confirmReplacement.mock.calls[0][2]).toMatchObject({
      settlement: { mode: "pay_now", amount: 30000, method: "cash" },
    });
  });

  it("nominal melebihi selisih ditolak di klien dan tidak memanggil server", async () => {
    await openConfirm();
    await userEvent.click(screen.getByRole("checkbox", { name: /Tagih selisih sekarang/ }));
    const amount = screen.getByDisplayValue("30000");
    await userEvent.clear(amount);
    await userEvent.type(amount, "30001");
    await userEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));
    expect(await screen.findByText(/Nominal melebihi selisih yang harus dibayar/)).toBeTruthy();
    expect(confirmReplacement).not.toHaveBeenCalled();
  });

  it("tanpa mencentang, konfirmasi tetap polos (tanpa settlement)", async () => {
    confirmReplacement.mockResolvedValue({ message: "ok", data: { id: 88, number: "SJ-88", status: "confirmed" } });
    await openConfirm();
    await userEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));
    await waitFor(() => expect(confirmReplacement).toHaveBeenCalled());
    expect(confirmReplacement.mock.calls[0][2]).toBeUndefined();
  });

  it("tanpa izin payment.create opsi tagih sekarang tidak tampil", async () => {
    useAuthStore.setState({ permissions: ["salesreturn.view", "salesreturn.create"], user: null, ready: true });
    await openConfirm();
    expect(screen.queryByRole("checkbox", { name: /Tagih selisih sekarang/ })).toBeNull();
  });
});
