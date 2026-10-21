import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { draftKey, saveDraft } from "@/shared/lib/draft-storage";

const create = vi.fn();
const getOrder = vi.fn();
const update = vi.fn();
vi.mock("@/modules/sales/services/sales.service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/modules/sales/services/sales.service")>();
  return {
    ...actual,
    salesService: {
      create: (...a: unknown[]) => create(...a),
      get: (...a: unknown[]) => getOrder(...a),
      update: (...a: unknown[]) => update(...a),
    },
    productOptionService: { search: vi.fn().mockResolvedValue([]) },
  };
});
vi.mock("@/modules/party/components/PartySearchSelect", () => ({
  PartySearchSelect: ({ value, selectedParty }: { value: number | null; selectedParty?: { name: string } | null }) => (
    <div data-testid="party-picker" data-value={value ?? ""}>
      {selectedParty?.name ?? ""}
    </div>
  ),
}));
const scanLookup = vi.fn();
const productDetail = vi.fn();
vi.mock("@/modules/products/services/products.service", () => ({
  productsService: {
    scanLookup: (...a: unknown[]) => scanLookup(...a),
    detail: (...a: unknown[]) => productDetail(...a),
  },
}));
// Dialog kamera diganti tombol yang memanggil onScan, supaya jalur form yang diuji.
vi.mock("@/modules/pos/components/ScanDialog", () => ({
  ScanDialog: ({ open, onScan }: { open: boolean; onScan: (c: string) => Promise<{ kind: string; detail?: string; qty?: number }> }) =>
    open ? (
      <button
        type="button"
        onClick={() =>
          void onScan("8999").then((o) => {
            const el = document.createElement("p");
            el.dataset.testid = "scan-outcome";
            el.textContent = `${o.kind}:${o.qty ?? ""}${o.detail ?? ""}`;
            document.body.appendChild(el);
          })
        }
      >
        kirim-kode
      </button>
    ) : null,
}));
const createCustomer = vi.fn();
const createAddress = vi.fn();
vi.mock("@/modules/party/services/party.service", () => ({
  customerService: {
    listAddresses: vi.fn().mockResolvedValue([]),
    get: vi.fn(),
    create: (...a: unknown[]) => createCustomer(...a),
    createAddress: (...a: unknown[]) => createAddress(...a),
  },
  memberTypeService: { list: vi.fn().mockResolvedValue([]) },
}));

import SalesOrderFormPage from "@/modules/sales/pages/SalesOrderFormPage";

const line = {
  key: 1,
  productId: 5,
  productLabel: "BRG-1 — Barang Uji",
  variantId: 9,
  uom: "pcs",
  qty: 2,
  discountPct: 0,
  discountNominal: 0,
  catalogPrice: 50000,
  minPrice: 0,
  manualPrice: "",
  discountReason: "",
  note: "",
  taxable: true,
  pricingSource: "",
  uoms: ["pcs"],
  variants: [{ id: 9, code: "D", name: "Default" }],
};

function seedDraft(extra: Record<string, unknown> = {}) {
  const key = draftKey(1, "sales-order-new")!;
  saveDraft(key, {
    partyId: null,
    pickedCustomer: null,
    orderDate: "2026-10-18",
    paymentTerms: "cod",
    dueDate: "",
    notes: "",
    shipToAddressId: null,
    taxInvoiceNumber: "",
    taxInvoiceDate: "",
    lines: [line],
    ...extra,
  });
}

beforeEach(() => {
  create.mockReset();
  create.mockResolvedValue({ message: "Order dibuat", data: { id: 3 } });
  createCustomer.mockReset();
  scanLookup.mockReset();
  productDetail.mockReset();
  document.querySelectorAll("[data-testid=scan-outcome]").forEach((n) => n.remove());
  productDetail.mockResolvedValue({
    id: 5,
    code: "BRG-1",
    name: "Barang Uji",
    sellingPrice: 50000,
    minSellingPrice: 0,
    salesUom: "pcs",
    purchaseUom: "pcs",
    baseUom: "pcs",
    variants: [{ id: 9, code: "D", name: "Default", isDefault: true }],
  });
  createAddress.mockReset();
  createAddress.mockResolvedValue({ message: "ok", data: { id: 1 } });
  localStorage.clear();
  useAuthStore.setState({
    permissions: ["sales.create", "sales.view", "customers.create", "customers.update", "products.view"],
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

describe("form order jual — kontrol PPN (F-01)", () => {
  it("bawaan tanpa pajak: tidak ada ringkasan DPP/PPN dan payload membawa taxType none", async () => {
    seedDraft();
    renderPage();
    await screen.findByText(/BRG-1/);
    expect(screen.queryByTestId("tax-summary")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(create.mock.calls[0][0]).toMatchObject({ taxType: "none", taxRate: 0 });
  });

  it("kena pajak exclude: tarif bawaan 11%, ringkasan DPP/PPN/total sama dengan hitungan server", async () => {
    seedDraft();
    renderPage();
    await screen.findByText(/BRG-1/);
    fireEvent.click(screen.getByLabelText(/Kena pajak \(PPN\)/));
    const summary = await screen.findByTestId("tax-summary");
    // 2 x 50.000 = 100.000 → PPN 11% = 11.000 → total 111.000
    expect(summary.textContent).toMatch(/100\.000/);
    expect(summary.textContent).toMatch(/11\.000/);
    expect(screen.getByText(/Estimasi total/).parentElement?.textContent).toMatch(/111\.000/);
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(create.mock.calls[0][0]).toMatchObject({ taxType: "exclude", taxRate: 11 });
  });

  it("harga termasuk pajak: total tetap, DPP dipisah, payload include", async () => {
    seedDraft();
    renderPage();
    await screen.findByText(/BRG-1/);
    fireEvent.click(screen.getByLabelText(/Kena pajak \(PPN\)/));
    fireEvent.click(await screen.findByLabelText(/Harga sudah termasuk pajak/));
    const summary = await screen.findByTestId("tax-summary");
    // net 100.000 termasuk PPN 11% → DPP 90.090, PPN 9.910, total tetap 100.000
    expect(summary.textContent).toMatch(/90\.090/);
    expect(summary.textContent).toMatch(/9\.910/);
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(create.mock.calls[0][0]).toMatchObject({ taxType: "include", taxRate: 11 });
  });

  it("draf yang dipulihkan membawa pengaturan pajaknya", async () => {
    seedDraft({ taxType: "exclude", taxRate: 12 });
    renderPage();
    await screen.findByText(/BRG-1/);
    expect(screen.getByLabelText(/Kena pajak \(PPN\)/)).toBeChecked();
    expect(screen.getByLabelText(/Tarif/)).toHaveValue("12");
  });
});

describe("form order jual — alamat kirim sekali pakai (SL-01)", () => {
  async function openDetail() {
    await screen.findByText(/BRG-1/);
    fireEvent.click(screen.getByRole("button", { name: /Detail lain/ }));
  }

  it("walk-in: alamat ad-hoc dikirim sebagai snapshot dengan id alamat 0", async () => {
    seedDraft();
    renderPage();
    await openDetail();
    fireEvent.change(await screen.findByLabelText("Penerima"), { target: { value: "Pak Joko" } });
    fireEvent.change(screen.getByLabelText("Alamat kirim"), { target: { value: "Jl. Sementara 5" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(create.mock.calls[0][0]).toMatchObject({
      shipToAddressId: 0,
      shipToRecipient: "Pak Joko",
      shipToAddress: "Jl. Sementara 5",
    });
    expect(createAddress).not.toHaveBeenCalled();
  });

  it("penerima tanpa alamat ditolak di klien dan tidak menyimpan", async () => {
    seedDraft();
    renderPage();
    await openDetail();
    fireEvent.change(await screen.findByLabelText("Penerima"), { target: { value: "Pak Joko" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    expect(await screen.findByText(/Alamat wajib diisi/)).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it("pelanggan terdaftar + 'simpan ke buku alamat': order disimpan lalu alamat masuk buku alamat", async () => {
    seedDraft({ partyId: 7, pickedCustomer: { value: 7, label: "C7 — Budi" } });
    renderPage();
    await openDetail();
    fireEvent.click(await screen.findByLabelText(/Pakai alamat sekali pakai/));
    fireEvent.change(screen.getByLabelText("Alamat kirim"), { target: { value: "Jl. Mawar 1" } });
    fireEvent.click(screen.getByLabelText(/Simpan juga ke buku alamat/));
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(createAddress).toHaveBeenCalled());
    expect(create.mock.calls[0][0]).toMatchObject({ shipToAddressId: 0, shipToAddress: "Jl. Mawar 1" });
    expect(createAddress.mock.calls[0]).toEqual([7, { text: "Jl. Mawar 1", label: undefined, recipient: undefined, phone: undefined }]);
  });
});

describe("form order jual — pelanggan baru cepat (PR-01)", () => {
  it("membuat pelanggan dari form lalu otomatis terpilih tanpa kehilangan baris", async () => {
    createCustomer.mockResolvedValue({
      message: "Customer dibuat",
      data: { id: 42, code: "CUS-042", name: "Siti Baru", phone: "081234567", member: null },
    });
    seedDraft();
    renderPage();
    await screen.findByText(/BRG-1/);
    fireEvent.click(screen.getByRole("button", { name: /Pelanggan baru/ }));
    fireEvent.change(await screen.findByLabelText(/Nama/), { target: { value: "Siti Baru" } });
    fireEvent.change(screen.getByLabelText(/Telepon/), { target: { value: "081234567" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(createCustomer).toHaveBeenCalled());
    expect(createCustomer.mock.calls[0][0]).toMatchObject({ name: "Siti Baru", phone: "081234567" });
    await waitFor(() => expect(screen.getByTestId("party-picker")).toHaveAttribute("data-value", "42"));
    expect(screen.getByText(/BRG-1/)).toBeInTheDocument();
  });

  it("termin net: telepon wajib, pelanggan tidak dibuat tanpa telepon", async () => {
    seedDraft({ paymentTerms: "net", dueDate: "2026-11-01" });
    renderPage();
    await screen.findByText(/BRG-1/);
    fireEvent.click(screen.getByRole("button", { name: /Pelanggan baru/ }));
    fireEvent.change(await screen.findByLabelText(/Nama/), { target: { value: "Tanpa HP" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));
    expect(await screen.findByText(/wajib memakai pelanggan yang memiliki nomor telepon/)).toBeInTheDocument();
    expect(createCustomer).not.toHaveBeenCalled();
  });
});

describe("form order jual — pindai QR / barcode (SL-10)", () => {
  it("kode dikenal menambah baris; pindai ulang menambah qty pada baris yang sama", async () => {
    scanLookup.mockResolvedValue({ id: 5, code: "BRG-1", name: "Barang Uji", status: "active", via: "barcode", variantRequired: false, variantId: 9 });
    seedDraft({ lines: [] });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /Pindai/ }));
    fireEvent.click(await screen.findByRole("button", { name: "kirim-kode" }));
    await waitFor(() => expect(screen.getByTestId("scan-outcome").textContent).toBe("added:1"));
    expect(await screen.findByText(/BRG-1/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "kirim-kode" }));
    await waitFor(() => expect(document.querySelectorAll("[data-testid=scan-outcome]").length).toBe(2));
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    const items = create.mock.calls[0][0].items;
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({ productId: 5, variantId: 9, qty: 2 });
  });

  it("kode berbagai varian atau produk nonaktif ditolak tanpa menambah baris", async () => {
    scanLookup.mockResolvedValue({ id: 5, code: "BRG-1", name: "Barang Uji", status: "active", via: "product_code", variantRequired: true });
    seedDraft({ lines: [] });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /Pindai/ }));
    fireEvent.click(await screen.findByRole("button", { name: "kirim-kode" }));
    await waitFor(() => expect(screen.getByTestId("scan-outcome").textContent).toMatch(/^rejected:.*varian/));
    expect(productDetail).not.toHaveBeenCalled();
  });

  it("tombol Pindai disembunyikan tanpa izin products.view", async () => {
    useAuthStore.setState({ permissions: ["sales.create"], ready: true, user: { id: 1, username: "u", fullName: "U" } } as never);
    seedDraft();
    renderPage();
    await screen.findByText(/BRG-1/);
    expect(screen.queryByRole("button", { name: /Pindai/ })).not.toBeInTheDocument();
  });
});

describe("form order jual — koreksi order terkonfirmasi", () => {
  function confirmedOrder(editMode: string) {
    return {
      id: 7, number: "SO-0007", branchId: 1, partyId: 0, partyName: "Tunai", channel: "pos", memberCode: "",
      orderDate: "2026-10-01 10:00:00", dueDate: "", paymentTerms: "cod", taxType: "none", taxRate: 0,
      subtotal: 100000, discountTotal: 0, taxTotal: 0, grandTotal: 100000, status: "confirmed", notes: "",
      shipToAddressId: 0, shipToLabel: "", shipToRecipient: "", shipToPhone: "", shipToAddress: "",
      taxInvoiceNumber: "", taxInvoiceDate: "", editMode,
      items: [{
        id: 1, productId: 5, variantId: 9, productCode: "BRG-1", productName: "Barang Uji", uom: "pcs", uomFactor: 1,
        qty: 2, qtyBase: 2, unitPrice: 50000, discountPct: 0, discountNominal: 0, lineTotal: 100000,
        stockTracked: true, itemType: "barang", variantName: "Default", variantCode: "D", baseUom: "pcs",
        pricingSource: "catalog", basePrice: 50000, discountReason: "", note: "", taxable: true, taxBase: 100000, taxAmount: 0,
      }],
    };
  }

  function renderEdit() {
    useAuthStore.setState({ permissions: ["sales.update", "sales.view"] } as never);
    return render(
      <MemoryRouter initialEntries={["/sales-orders/7/edit"]}>
        <Routes>
          <Route path="/sales-orders/:id/edit" element={<SalesOrderFormPage />} />
          <Route path="/sales-orders/:id" element={<div>detail</div>} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it("tanpa dokumen turunan: form bisa disunting, kanal asli dan alasan ikut terkirim", async () => {
    getOrder.mockResolvedValue(confirmedOrder("full"));
    update.mockResolvedValue({ message: "Sales order disimpan", data: { id: 7 } });
    renderEdit();
    expect(await screen.findByText("Mengoreksi order terkonfirmasi")).toBeTruthy();
    expect(screen.queryByText(/hanya baca/)).toBeNull();
    fireEvent.change(screen.getByLabelText("Alasan koreksi"), { target: { value: "salah ketik qty" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(update).toHaveBeenCalled());
    expect(update.mock.calls[0][1]).toMatchObject({ channel: "pos", reason: "salah ketik qty" });
  });

  it("sudah ada dokumen turunan: form hanya baca dan menunjuk ke detail", async () => {
    getOrder.mockResolvedValue(confirmedOrder("header"));
    renderEdit();
    expect(await screen.findByText(/hanya baca/)).toBeTruthy();
    expect(screen.getByText(/ubah info order/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Simpan" })).toBeNull();
  });
});
