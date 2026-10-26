import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { draftKey, saveDraft } from "@/shared/lib/draft-storage";

// T-03: alur inti form order jual yang belum dicakup SalesOrderFormPage.test.tsx
// (pajak, alamat kirim, pelanggan baru, pindai sudah ada di sana): simpan tanpa
// baris ditolak, payload dasar, galat server, dan penjaga izin.
const create = vi.fn();
vi.mock("@/modules/sales/services/sales.service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/modules/sales/services/sales.service")>();
  return {
    ...actual,
    salesService: { create: (...a: unknown[]) => create(...a), get: vi.fn(), update: vi.fn() },
  };
});
vi.mock("@/modules/party/components/PartySearchSelect", () => ({
  PartySearchSelect: () => <div data-testid="party-picker" />,
}));
vi.mock("@/modules/products/services/products.service", () => ({
  productsService: { scanLookup: vi.fn(), detail: vi.fn() },
}));
vi.mock("@/modules/pos/components/ScanDialog", () => ({ ScanDialog: () => null }));
vi.mock("@/modules/party/services/party.service", () => ({
  customerService: {
    listAddresses: vi.fn().mockResolvedValue([]),
    get: vi.fn(),
    create: vi.fn(),
    createAddress: vi.fn(),
  },
  memberTypeService: { list: vi.fn().mockResolvedValue([]) },
}));

vi.setConfig({ testTimeout: 30000 });

import SalesOrderFormPage from "@/modules/sales/pages/SalesOrderFormPage";

const line = {
  key: 1, productId: 5, productLabel: "BRG-1 — Barang Uji", variantId: 9, uom: "pcs", qty: 3,
  discountPct: 0, discountNominal: 0, catalogPrice: 50000, minPrice: 0, manualPrice: "",
  discountReason: "", note: "", taxable: true, pricingSource: "", uoms: ["pcs"],
  variants: [{ id: 9, code: "D", name: "Default" }],
};

function seedDraft(lines: unknown[]) {
  saveDraft(draftKey(1, "sales-order-new")!, {
    partyId: null, pickedCustomer: null, orderDate: "2026-10-18", paymentTerms: "cod", dueDate: "",
    notes: "", shipToAddressId: null, taxInvoiceNumber: "", taxInvoiceDate: "", lines,
  });
}

beforeEach(() => {
  create.mockReset().mockResolvedValue({ message: "Order dibuat", data: { id: 3 } });
  localStorage.clear();
  useAuthStore.setState({
    permissions: ["sales.create", "sales.view", "products.view"],
    ready: true,
    user: { id: 1, username: "u", fullName: "U" },
  } as never);
});
afterEach(cleanup);

function renderPage() {
  return render(
    <MemoryRouter>
      <SalesOrderFormPage />
    </MemoryRouter>,
  );
}

describe("form order jual — inti", () => {
  it("render: tombol Simpan order tersedia dan baris draf dipulihkan", async () => {
    seedDraft([line]);
    renderPage();
    expect(await screen.findByText(/BRG-1/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Simpan order" })).toBeInTheDocument();
  });

  it("tanpa baris barang: simpan ditolak di klien dan server tidak dipanggil", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Simpan order" }));
    expect((await screen.findAllByText(/Minimal 1 baris item/)).length).toBeGreaterThan(0);
    expect(create).not.toHaveBeenCalled();
  });

  it("simpan sah: payload membawa baris (produk, varian, satuan, qty) dan syarat COD", async () => {
    seedDraft([line]);
    renderPage();
    await screen.findByText(/BRG-1/);
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    const body = create.mock.calls[0][0];
    expect(body).toMatchObject({ paymentTerms: "cod", orderDate: "2026-10-18" });
    expect(body.items).toEqual([expect.objectContaining({ productId: 5, variantId: 9, uom: "pcs", qty: 3 })]);
  });

  it("galat server tidak menghapus baris: pengguna bisa mencoba lagi", async () => {
    create.mockRejectedValueOnce({ status: 409, message: "Stok tidak mencukupi" });
    seedDraft([line]);
    renderPage();
    await screen.findByText(/BRG-1/);
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(screen.getByText(/BRG-1/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(2));
  });
});
