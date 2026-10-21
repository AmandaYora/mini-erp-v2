import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { CheckoutSheet } from "@/modules/pos/components/CheckoutSheet";
import { PAY_LATER_NEEDS_CUSTOMER_MESSAGE } from "@/modules/pos/lib/checkout";

// K1: kasir tanpa izin payment.create tidak boleh memulai "bayar sekarang" (order akan tertinggal
// setengah jalan karena konfirmasi SJ tunai mewajibkan pembayaran). Bayar Nanti tetap bisa.
function formPos(over: Record<string, unknown> = {}) {
  return {
    checkoutOpen: true,
    setCheckoutOpen: vi.fn(),
    processing: false,
    started: false,
    completed: null,
    mode: "pay_now",
    setMode: vi.fn(),
    method: "cash",
    setMethod: vi.fn(),
    tendered: "100000",
    setTendered: vi.fn(),
    estimateTotal: 90000,
    changePreview: 10000,
    checkoutError: null,
    resumeStep: null,
    customerId: null,
    customerDetail: null,
    dueDate: "",
    setDueDate: vi.fn(),
    notes: "",
    setNotes: vi.fn(),
    shipToAddressId: null,
    setShipToAddressId: vi.fn(),
    taxType: "none",
    taxRate: 11,
    setTaxed: vi.fn(),
    setTaxIncluded: vi.fn(),
    setTaxRate: vi.fn(),
    totals: { subtotal: 90000, discount: 0, tax: 0, total: 90000 },
    cart: [],
    displayUnitPrice: () => 0,
    memberSavings: 0,
    hideDiscount: false,
    setHideDiscount: vi.fn(),
    newTransaction: vi.fn(),
    handleCheckout: vi.fn(),
    canPay: true,
    payLaterBlock: null,
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

beforeEach(() => useAuthStore.setState({ permissions: ["sales.create"], ready: true }));
afterEach(cleanup);

describe("checkout POS tanpa izin pembayaran (K1)", () => {
  it("bayar sekarang: tombol Proses Bayar nonaktif dengan penjelasan", () => {
    const pos = formPos({ canPay: false });
    renderSheet(pos);
    expect(screen.getByText("Bayar sekarang tidak tersedia untuk akun ini")).toBeInTheDocument();
    const btn = screen.getByRole("button", { name: "Proses Bayar" });
    expect(btn).toBeDisabled();
    fireEvent.click(btn);
    expect((pos as { handleCheckout: ReturnType<typeof vi.fn> }).handleCheckout).not.toHaveBeenCalled();
  });

  it("Bayar Nanti tetap bisa diproses tanpa izin pembayaran", () => {
    renderSheet(formPos({ canPay: false, mode: "pay_later" }));
    expect(screen.queryByText("Bayar sekarang tidak tersedia untuk akun ini")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Buat Piutang" })).toBeEnabled();
  });

  it("dengan izin: Proses Bayar aktif", () => {
    renderSheet(formPos());
    expect(screen.getByRole("button", { name: "Proses Bayar" })).toBeEnabled();
  });
});

describe("Bayar Nanti hanya untuk pelanggan terdaftar", () => {
  it("walk-in: opsi Bayar Nanti nonaktif dengan penjelasan", () => {
    const pos = formPos({ customerId: null, payLaterBlock: PAY_LATER_NEEDS_CUSTOMER_MESSAGE });
    renderSheet(pos);
    const payLater = screen.getByRole("button", { name: "Bayar Nanti" });
    expect(payLater).toBeDisabled();
    expect(payLater).toHaveAccessibleDescription(PAY_LATER_NEEDS_CUSTOMER_MESSAGE);
    fireEvent.click(payLater);
    expect((pos as { setMode: ReturnType<typeof vi.fn> }).setMode).not.toHaveBeenCalled();
  });

  it("dengan pelanggan: Bayar Nanti bisa dipilih", () => {
    const pos = formPos({ customerId: 7 });
    renderSheet(pos);
    const payLater = screen.getByRole("button", { name: "Bayar Nanti" });
    expect(payLater).toBeEnabled();
    fireEvent.click(payLater);
    expect((pos as { setMode: ReturnType<typeof vi.fn> }).setMode).toHaveBeenCalledWith("pay_later");
  });
});
