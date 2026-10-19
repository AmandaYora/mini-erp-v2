import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const list = vi.fn();
const openParties = vi.fn();
const balance = vi.fn();
const ledger = vi.fn();
const create = vi.fn();
vi.mock("@/modules/payment/services/payment.service", () => ({
  paymentService: {
    list: (...a: unknown[]) => list(...a),
    openParties: (...a: unknown[]) => openParties(...a),
    balance: (...a: unknown[]) => balance(...a),
    ledger: (...a: unknown[]) => ledger(...a),
    create: (...a: unknown[]) => create(...a),
  },
}));
vi.mock("@/modules/party/components/PartySearchSelect", () => ({
  PartySearchSelect: () => <div data-testid="party-select" />,
}));
vi.mock("@/modules/sales/components/SalesOrderSearchSelect", () => ({ SalesOrderSearchSelect: () => null }));
vi.mock("@/modules/purchasing/components/PurchaseOrderSearchSelect", () => ({ PurchaseOrderSearchSelect: () => null }));
vi.mock("@/modules/payment/components/PaymentFormModal", () => ({
  PaymentFormModal: ({ initialParty }: { initialParty?: { id: number; type: string } }) => (
    <div data-testid="pay-form">{initialParty ? `${initialParty.type}:${initialParty.id}` : "kosong"}</div>
  ),
}));

import PaymentsPage from "@/modules/payment/pages/PaymentsPage";

const page = (items: unknown[]) => ({
  items,
  total: items.length,
  totalOutstanding: 350000,
  page: 1,
  limit: 10,
  totalPages: 1,
});
const budi = { partyId: 4, code: "C-4", name: "Budi", side: "receivable", orderCount: 3, oldestDate: "2026-08-01", outstanding: 250000 };
const ani = { partyId: 5, code: "C-5", name: "Ani", side: "receivable", orderCount: 1, oldestDate: "2026-09-01", outstanding: 100000 };

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ permissions: ["payment.view", "payment.create"], user: null, ready: true });
  list.mockResolvedValue({ items: [], meta: { page: 1, limit: 20, total: 0, totalPages: 0 } });
  openParties.mockResolvedValue(page([budi, ani]));
  balance.mockResolvedValue({ partyId: 4, partyName: "Budi", totalBilled: 0, totalPaid: 0, totalReturned: 0, totalRefunded: 0, outstanding: 250000, orders: [], returns: [] });
  ledger.mockResolvedValue({ items: [], meta: { page: 1, limit: 10, total: 0, totalPages: 0 } });
});
afterEach(() => cleanup());

function open() {
  return render(
    <MemoryRouter initialEntries={["/payments?tab=saldo"]}>
      <PaymentsPage />
    </MemoryRouter>,
  );
}

describe("PaymentsPage — tab Saldo pihak: daftar pihak bersaldo (PY-01)", () => {
  it("menampilkan semua pihak bersaldo dengan jumlah dokumen, tanggal terlama, sisa, dan total", async () => {
    open();
    expect(await screen.findByText("Budi")).toBeTruthy();
    expect(screen.getByText("Ani")).toBeTruthy();
    expect(openParties).toHaveBeenCalledWith({ side: "receivable", search: "", page: 1, limit: 10 });
    expect(screen.getByText((_, el) => el?.tagName === "P" && /2\s*pihak/.test(el.textContent ?? ""))).toBeTruthy();
    expect(screen.getAllByRole("button", { name: "Catat bayar" })).toHaveLength(2);
  });

  it("pencarian dan pergantian sisi dikirim ke server", async () => {
    open();
    await screen.findByText("Budi");
    fireEvent.change(screen.getByLabelText("Cari pihak bersaldo"), { target: { value: "0812" } });
    await waitFor(() => expect(openParties).toHaveBeenLastCalledWith(expect.objectContaining({ search: "0812" })), { timeout: 3000 });
    fireEvent.click(screen.getByRole("radio", { name: "Hutang supplier" }));
    await waitFor(() => expect(openParties).toHaveBeenLastCalledWith(expect.objectContaining({ side: "payable" })));
  });

  it("Catat bayar membuka form pembayaran dengan pihak terisi", async () => {
    open();
    await screen.findByText("Budi");
    fireEvent.click(screen.getAllByRole("button", { name: "Catat bayar" })[0]);
    expect((await screen.findByTestId("pay-form")).textContent).toBe("customer:4");
  });

  it("tanpa izin payment.create tombol Catat bayar tidak ada", async () => {
    useAuthStore.setState({ permissions: ["payment.view"] });
    open();
    await screen.findByText("Budi");
    expect(screen.queryByRole("button", { name: "Catat bayar" })).toBeNull();
  });

  it("baris diklik memuat saldo rinci pihak itu", async () => {
    open();
    fireEvent.click(await screen.findByText("Budi"));
    await waitFor(() => expect(balance).toHaveBeenCalledWith(4));
  });
});
