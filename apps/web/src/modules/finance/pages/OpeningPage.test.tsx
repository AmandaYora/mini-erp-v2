import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const openingStatus = vi.fn();
const openingInventory = vi.fn();
const accounts = vi.fn();
const previewOpening = vi.fn();
const postOpening = vi.fn();
const completeOpening = vi.fn();
vi.mock("@/modules/finance/services/finance.service", () => ({
  financeService: {
    openingStatus: (...a: unknown[]) => openingStatus(...a),
    openingInventory: (...a: unknown[]) => openingInventory(...a),
    accounts: (...a: unknown[]) => accounts(...a),
    previewOpening: (...a: unknown[]) => previewOpening(...a),
    postOpening: (...a: unknown[]) => postOpening(...a),
    completeOpening: (...a: unknown[]) => completeOpening(...a),
  },
}));

import OpeningPage from "@/modules/finance/pages/OpeningPage";

const inventory = {
  total: 1_234_000,
  positions: 3,
  estimated: 1,
  openingPosted: false,
  missingCost: [{ productId: 9, variantId: 90, productCode: "NOMODAL", productName: "Barang Tanpa Modal", qty: 4, suggestedCost: 7000 }],
};

beforeEach(() => {
  for (const m of [openingStatus, openingInventory, accounts, previewOpening, postOpening, completeOpening]) m.mockReset();
  openingStatus.mockResolvedValue(null);
  openingInventory.mockResolvedValue(inventory);
  accounts.mockResolvedValue([]);
  useAuthStore.setState({ permissions: ["finance.view", "finance.manage"], ready: true });
});
afterEach(cleanup);

function renderPage() {
  return render(
    <MemoryRouter>
      <OpeningPage />
    </MemoryRouter>,
  );
}

describe("saldo awal: persediaan terhitung server", () => {
  it("menampilkan total persediaan terhitung dan produk tanpa harga modal pada langkah susun", async () => {
    renderPage();
    expect(await screen.findByText("Persediaan terhitung otomatis")).toBeInTheDocument();
    expect(screen.getByText(/1\.234\.000/)).toBeInTheDocument();
    expect(screen.getByText(/1 produk berstok tanpa harga modal/)).toBeInTheDocument();
  });

  it("jurnal hanya persediaan: baris kosong boleh ditinjau dan tidak mengirim baris manual", async () => {
    previewOpening.mockResolvedValue({
      message: "Pratinjau saldo awal",
      data: {
        id: 0,
        number: "",
        date: "2026-10-18",
        memo: "Saldo awal",
        status: "posted",
        inventory,
        lines: [
          { accountId: 1, accountCode: "1200", accountName: "Persediaan", debit: 1_234_000, credit: 0, description: "Persediaan 3 produk (dihitung server dari stok awal)" },
          { accountId: 2, accountCode: "3000", accountName: "Modal Awal", debit: 0, credit: 1_234_000 },
        ],
      },
    });
    renderPage();
    const btn = await screen.findByRole("button", { name: "Tinjau saldo awal" });
    await waitFor(() => expect(btn).toBeEnabled());
    fireEvent.click(btn);
    await waitFor(() => expect(previewOpening).toHaveBeenCalled());
    expect(previewOpening.mock.calls[0][0].lines).toEqual([]);
    expect(await screen.findByText("Tinjau saldo awal")).toBeInTheDocument();
    expect(screen.getByText(/Persediaan 3 produk/)).toBeInTheDocument();
  });

  it("setelah dikunci: daftar produk tanpa modal dan tombol Lengkapi memanggil jalur lengkapi", async () => {
    openingStatus.mockResolvedValue({ id: 5, number: "JV-1", date: "2026-10-01", memo: "Saldo awal", lines: [] });
    openingInventory.mockResolvedValue({ ...inventory, openingPosted: true });
    completeOpening.mockResolvedValue({ message: "Harga modal saldo awal dilengkapi", data: { id: 6, number: "JV-2", total: 28000 } });
    renderPage();
    expect(await screen.findByText("Barang Tanpa Modal")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Lengkapi dari harga beli \(1\)/ }));
    await waitFor(() => expect(completeOpening).toHaveBeenCalledWith({ items: [{ productId: 9, variantId: 90 }] }));
  });
});
