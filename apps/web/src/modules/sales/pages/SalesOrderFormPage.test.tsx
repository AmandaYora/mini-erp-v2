import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { draftKey, saveDraft } from "@/shared/lib/draft-storage";

const create = vi.fn();
const getOrder = vi.fn();
const update = vi.fn();
const openDirect = vi.fn();
const previewFromDeliveries = vi.fn();
const createFromDeliveries = vi.fn();
vi.mock("@/modules/sales/services/sales.service", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/modules/sales/services/sales.service")>();
  return {
    ...actual,
    salesService: {
      create: (...a: unknown[]) => create(...a),
      get: (...a: unknown[]) => getOrder(...a),
      update: (...a: unknown[]) => update(...a),
      openDirectDeliveries: (...a: unknown[]) => openDirect(...a),
      previewFromDeliveries: (...a: unknown[]) => previewFromDeliveries(...a),
      createFromDeliveries: (...a: unknown[]) => createFromDeliveries(...a),
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
  openDirect.mockReset();
  openDirect.mockResolvedValue({ items: [], count: 0, oldestDate: "" });
  previewFromDeliveries.mockReset();
  createFromDeliveries.mockReset();
  sessionStorage.clear();
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

  it("produk ber-varian: baris menunggu pilihan varian dan tidak tersimpan sebelum dipilih (temuan 2)", async () => {
    seedDraft({
      partyId: 3,
      lines: [{ ...line, variantId: 0, variants: [{ id: 11, code: "CAT-001", name: "Putih" }, { id: 12, code: "CAT-002", name: "Merah" }] }],
    });
    renderPage();
    const select = await screen.findByLabelText("Varian BRG-1 — Barang Uji");
    expect((select as HTMLSelectElement).value).toBe("");
    expect(screen.getByRole("option", { name: "Pilih varian…" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    expect(await screen.findAllByText(/Varian wajib dipilih/)).not.toHaveLength(0);
    expect(create).not.toHaveBeenCalled();
    fireEvent.change(select, { target: { value: "12" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(create.mock.calls[0][0].items[0]).toMatchObject({ productId: 5, variantId: 12 });
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

describe("form order jual — mode dari surat jalan (ADR-0022 W-06)", () => {
  const previewItem = {
    productId: 5, variantId: 9, productCode: "BRG-1", productName: "Barang Uji", variantName: "Default", variantCode: "D",
    uom: "pcs", uomFactor: 1, qty: 4, qtyBase: 4, unitPrice: 50000, discountPct: 0, discountNominal: 0, lineTotal: 200000,
    stockTracked: true, itemType: "barang", pricingSource: "catalog", basePrice: 50000, discountReason: "", note: "",
    taxable: true, taxBase: 200000, taxAmount: 0,
  };
  const preview = {
    partyId: 3, partyName: "Budi", shipToLabel: "Gudang", shipToRecipient: "Pak Budi", shipToPhone: "0812", shipToAddress: "Jl. Mawar 1",
    deliveryDate: "2026-10-18",
    deliveries: [{ id: 11, number: "SJ-0011", deliveryDate: "2026-10-17" }, { id: 12, number: "SJ-0012", deliveryDate: "2026-10-18" }],
    items: [previewItem], subtotal: 200000, discountTotal: 0, taxTotal: 0, grandTotal: 200000,
    warnings: ["Masa faktur pajak berbeda dengan masa SJ"],
  };

  function renderFrom(path = "/sales-orders/new?fromDeliveries=12,11") {
    return render(
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/sales-orders/new" element={<SalesOrderFormPage />} />
          <Route path="/sales-orders/:id" element={<div>detail</div>} />
        </Routes>
      </MemoryRouter>,
    );
  }

  beforeEach(() => {
    previewFromDeliveries.mockResolvedValue(preview);
    createFromDeliveries.mockResolvedValue({ message: "Order dibuat dari surat jalan dan selesai", data: { id: 99, number: "SO-0099", status: "completed" } });
    useAuthStore.setState({
      permissions: ["sales.create", "sales.view", "payment.create", "sales.approve"],
      ready: true,
      user: { id: 1, username: "u", fullName: "U" },
    } as never);
  });

  it("pratinjau mengunci pelanggan dan qty: tanpa pemilih pelanggan, tanpa isian qty, harga bisa diisi", async () => {
    renderFrom();
    expect(await screen.findByTestId("locked-customer")).toHaveTextContent("Budi");
    expect(previewFromDeliveries.mock.calls[0][0]).toMatchObject({ deliveryIds: [11, 12], paymentTerms: "cod" });
    expect(screen.queryByTestId("party-picker")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Qty Barang Uji")).not.toBeInTheDocument();
    expect(screen.getByText("SJ-0011")).toBeInTheDocument();
    expect(screen.getByText("SJ-0012")).toBeInTheDocument();
    expect(screen.getByLabelText("Harga manual Barang Uji")).toBeInTheDocument();
    expect(screen.getByTestId("preview-warnings")).toHaveTextContent("Masa faktur pajak");
    expect(screen.getByTestId("preview-summary")).toHaveTextContent("200.000");
    // Tanpa pilihan prabayar.
    expect(screen.queryByRole("option", { name: /Prabayar/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Simpan & selesaikan" })).toBeEnabled();
  });

  it("order tunai wajib memilih cara pelunasan sebelum disimpan", async () => {
    renderFrom();
    await screen.findByTestId("locked-customer");
    fireEvent.click(screen.getByRole("button", { name: "Simpan & selesaikan" }));
    expect(await screen.findByText(/Pilih cara pelunasan/)).toBeInTheDocument();
    expect(createFromDeliveries).not.toHaveBeenCalled();
    // Jadikan tempo tanpa jatuh tempo juga ditolak.
    fireEvent.change(screen.getByLabelText(/^Cara pelunasan/), { target: { value: "switch_to_net" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan & selesaikan" }));
    expect(await screen.findByText("Jatuh tempo wajib diisi")).toBeInTheDocument();
    expect(createFromDeliveries).not.toHaveBeenCalled();
  });

  it("simpan mengirim clientRef dan memakai ulang clientRef yang sama saat ulang setelah gagal", async () => {
    createFromDeliveries
      .mockRejectedValueOnce({ status: 500, message: "Server sibuk" })
      .mockResolvedValueOnce({ message: "Order dibuat dari surat jalan dan selesai", data: { id: 99, number: "SO-0099" } });
    renderFrom();
    await screen.findByTestId("locked-customer");
    fireEvent.change(screen.getByLabelText(/^Cara pelunasan/), { target: { value: "pay_now" } });
    fireEvent.change(screen.getByLabelText("Harga manual Barang Uji"), { target: { value: "55000" } });
    // Harga manual memicu pratinjau ulang (debounce) — tunggu sampai tombol aktif kembali.
    await waitFor(() => expect(previewFromDeliveries).toHaveBeenCalledTimes(2));
    expect(previewFromDeliveries.mock.calls[1][0].lines).toEqual([
      expect.objectContaining({ productId: 5, variantId: 9, uom: "pcs", unitPrice: 55000 }),
    ]);
    await waitFor(() => expect(screen.getByRole("button", { name: "Simpan & selesaikan" })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: "Simpan & selesaikan" }));
    await waitFor(() => expect(createFromDeliveries).toHaveBeenCalledTimes(1));
    const first = createFromDeliveries.mock.calls[0][0];
    expect(first).toMatchObject({
      deliveryIds: [11, 12],
      paymentTerms: "cod",
      settlement: { mode: "pay_now", method: "cash" },
      lines: [expect.objectContaining({ unitPrice: 55000 })],
    });
    expect(first.clientRef).toMatch(/.+/);
    expect(await screen.findByText("Server sibuk")).toBeInTheDocument();
    // Ulang: clientRef sama → server melanjutkan, bukan menggandakan.
    fireEvent.click(screen.getByRole("button", { name: "Simpan & selesaikan" }));
    await waitFor(() => expect(createFromDeliveries).toHaveBeenCalledTimes(2));
    expect(createFromDeliveries.mock.calls[1][0].clientRef).toBe(first.clientRef);
    expect(await screen.findByText("detail")).toBeInTheDocument();
    // Sukses membersihkan clientRef tersimpan.
    expect(sessionStorage.getItem("mini-erp:sjl-client-ref:11,12")).toBeNull();
  });

  it("termin tempo: jatuh tempo wajib, tanpa blok pelunasan", async () => {
    renderFrom();
    await screen.findByTestId("locked-customer");
    fireEvent.change(screen.getByLabelText(/^Termin/), { target: { value: "net" } });
    expect(screen.queryByTestId("settlement")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Simpan & selesaikan" }));
    expect(await screen.findByText(/Jatuh tempo wajib diisi untuk termin tempo/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/^Jatuh tempo/), { target: { value: "2026-11-20" } });
    await waitFor(() => expect(screen.getByRole("button", { name: "Simpan & selesaikan" })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: "Simpan & selesaikan" }));
    await waitFor(() => expect(createFromDeliveries).toHaveBeenCalledTimes(1));
    expect(createFromDeliveries.mock.calls[0][0]).toMatchObject({ paymentTerms: "net", dueDate: "2026-11-20" });
    expect(createFromDeliveries.mock.calls[0][0].settlement).toBeUndefined();
  });

  it("kena pajak menyarankan tanggal faktur pajak = tanggal SJ", async () => {
    renderFrom();
    await screen.findByTestId("locked-customer");
    fireEvent.click(screen.getByLabelText(/Kena pajak \(PPN\)/));
    fireEvent.click(screen.getByRole("button", { name: /Detail lain/ }));
    expect(await screen.findByLabelText("Tgl. faktur pajak")).toHaveValue("2026-10-18");
  });

  it("pratinjau gagal: galat dengan pesan server dan tombol coba lagi", async () => {
    previewFromDeliveries.mockRejectedValueOnce({ status: 409, message: "Surat jalan sudah diorder" });
    renderFrom();
    expect(await screen.findByText("Surat jalan sudah diorder")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Coba lagi/ }));
    expect(await screen.findByTestId("locked-customer")).toBeInTheDocument();
  });
});

describe("form order jual biasa — banner SJ belum diorder (ADR-0022 W-07)", () => {
  it("pelanggan dengan SJ Langsung terbuka: banner tidak memblokir dan membuka pemilih", async () => {
    useAuthStore.setState({
      permissions: ["sales.create", "sales.view", "delivery.view"],
      ready: true,
      user: { id: 1, username: "u", fullName: "U" },
    } as never);
    openDirect.mockResolvedValue({ items: [], count: 2, oldestDate: "2026-10-10" });
    seedDraft({ partyId: 7, pickedCustomer: { value: 7, label: "C7 — Budi" } });
    renderPage();
    expect(await screen.findByText(/Pelanggan ini punya 2 SJ belum diorder/)).toBeInTheDocument();
    expect(openDirect).toHaveBeenCalledWith(7, 0);
    // Tidak memblokir: simpan biasa tetap jalan.
    fireEvent.click(screen.getByRole("button", { name: "Simpan order" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
  });

  it("tanpa izin delivery.view tidak memanggil ringkasan SJ", async () => {
    seedDraft({ partyId: 7, pickedCustomer: { value: 7, label: "C7 — Budi" } });
    renderPage();
    await screen.findByText(/BRG-1/);
    expect(openDirect).not.toHaveBeenCalled();
    expect(screen.queryByText(/SJ belum diorder/)).not.toBeInTheDocument();
  });
});
