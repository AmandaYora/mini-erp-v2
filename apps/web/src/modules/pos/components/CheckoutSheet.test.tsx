import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { CheckoutSheet } from "@/modules/pos/components/CheckoutSheet";

function fakePos(over: Record<string, unknown> = {}, completed: Record<string, unknown> = {}) {
  return {
    checkoutOpen: true,
    setCheckoutOpen: vi.fn(),
    processing: false,
    mode: "pay_now",
    started: false,
    estimateTotal: 90000,
    hideDiscount: false,
    setHideDiscount: vi.fn(),
    cart: [],
    displayUnitPrice: () => 0,
    memberSavings: 0,
    newTransaction: vi.fn(),
    completed: {
      orderId: 7,
      orderNumber: "POS-7",
      payLater: false,
      dueDate: "",
      grandTotal: 90000,
      method: "cash",
      tendered: 100000,
      change: 10000,
      paymentPending: false,
      paymentId: 3,
      ...completed,
    },
    ...over,
  } as never;
}

function renderSheet(pos: never) {
  return render(
    <MemoryRouter>
      <CheckoutSheet pos={pos} />
    </MemoryRouter>,
  );
}

const href = (name: RegExp | string) => screen.getByRole("link", { name }).getAttribute("href");

beforeEach(() => {
  useAuthStore.setState({ permissions: ["sales.view", "payment.view"], ready: true });
});
afterEach(cleanup);

describe("layar sukses POS — tombol cetak (SL-05, SL-06)", () => {
  it("struk, nota, dan bukti bayar muncul dengan tautan yang benar", () => {
    renderSheet(fakePos());
    expect(href("Cetak Struk")).toBe("/print/pos/7?paymentId=3");
    expect(href("Cetak Nota")).toBe("/print/sales/7");
    expect(href("Bukti Bayar")).toBe("/print/payments/3");
  });

  it("pilihan sembunyikan diskon ikut ke struk dan nota", () => {
    renderSheet(fakePos({ hideDiscount: true }));
    expect(href("Cetak Struk")).toBe("/print/pos/7?paymentId=3&hideDiscount=1");
    expect(href("Cetak Nota")).toBe("/print/sales/7?hideDiscount=1");
  });

  it("Bayar Nanti: Cetak Tagihan dan tanpa Bukti Bayar", () => {
    renderSheet(fakePos({}, { payLater: true, paymentId: null, dueDate: "2026-11-01" }));
    expect(href("Cetak Tagihan")).toBe("/print/sales/7");
    expect(screen.queryByRole("link", { name: "Bukti Bayar" })).not.toBeInTheDocument();
  });

  it("tanpa izin: nota dan bukti bayar disembunyikan, struk tetap ada", () => {
    useAuthStore.setState({ permissions: [], ready: true });
    renderSheet(fakePos());
    expect(screen.getByRole("link", { name: "Cetak Struk" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Cetak Nota" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Bukti Bayar" })).not.toBeInTheDocument();
  });
});

describe("form pembayaran — PPN opsional (K-03)", () => {
  function payForm(over: Record<string, unknown> = {}) {
    const setTaxed = vi.fn();
    const setTaxIncluded = vi.fn();
    const setTaxRate = vi.fn();
    const pos = fakePos({
      completed: null,
      mode: "pay_now",
      method: "transfer",
      tendered: "",
      changePreview: 0,
      taxType: "none",
      taxRate: 0,
      totals: { subtotal: 100000, discount: 0, base: 100000, tax: 0, total: 100000, itemCount: 2 },
      estimateTotal: 100000,
      notes: "",
      customerId: null,
      customerDetail: null,
      setTaxed,
      setTaxIncluded,
      setTaxRate,
      setMode: vi.fn(),
      setMethod: vi.fn(),
      setTendered: vi.fn(),
      setNotes: vi.fn(),
      setShipToAddressId: vi.fn(),
      handleCheckout: vi.fn(),
      ...over,
    });
    return { pos, setTaxed, setTaxIncluded, setTaxRate };
  }

  it("bawaan tanpa pajak: kotak PPN tidak dicentang dan tidak ada ringkasan DPP/PPN", () => {
    const { pos } = payForm();
    renderSheet(pos);
    expect(screen.getByLabelText(/Kena pajak \(PPN\)/)).not.toBeChecked();
    expect(screen.queryByTestId("pos-tax-summary")).not.toBeInTheDocument();
  });

  it("mencentang Kena pajak memanggil setTaxed(true)", () => {
    const { pos, setTaxed } = payForm();
    renderSheet(pos);
    fireEvent.click(screen.getByLabelText(/Kena pajak \(PPN\)/));
    expect(setTaxed).toHaveBeenCalledWith(true);
  });

  it("kena pajak exclude: tarif, ringkasan DPP/PPN/Total dari hitungan keranjang", () => {
    const { pos, setTaxIncluded } = payForm({
      taxType: "exclude",
      taxRate: 11,
      totals: { subtotal: 100000, discount: 0, base: 100000, tax: 11000, total: 111000, itemCount: 2 },
      estimateTotal: 111000,
    });
    renderSheet(pos);
    expect(screen.getByLabelText(/Kena pajak \(PPN\)/)).toBeChecked();
    expect(screen.getByLabelText("Tarif (%)")).toHaveValue("11");
    const sum = screen.getByTestId("pos-tax-summary").textContent ?? "";
    expect(sum).toMatch(/100\.000/);
    expect(sum).toMatch(/11\.000/);
    expect(sum).toMatch(/111\.000/);
    fireEvent.click(screen.getByLabelText(/Harga sudah termasuk pajak/));
    expect(setTaxIncluded).toHaveBeenCalledWith(true);
  });

  it("kontrol pajak terkunci setelah order dibuat (checkout berjalan)", () => {
    const { pos } = payForm({ started: true, taxType: "exclude", taxRate: 11 });
    renderSheet(pos);
    expect(screen.getByLabelText(/Kena pajak \(PPN\)/)).toBeDisabled();
    expect(screen.getByLabelText("Tarif (%)")).toBeDisabled();
  });
});
