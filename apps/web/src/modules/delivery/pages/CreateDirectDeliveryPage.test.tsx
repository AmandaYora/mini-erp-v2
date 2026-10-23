import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const createDirect = vi.fn();
const openDirect = vi.fn();
const listAddresses = vi.fn();
vi.mock("@/modules/delivery/services/delivery.service", () => ({
  deliveryService: {
    locationOptions: vi.fn().mockResolvedValue([{ id: 3, code: "GD-A", name: "Gudang A", status: "active" }]),
    createDirect: (...a: unknown[]) => createDirect(...a),
    openDirect: (...a: unknown[]) => openDirect(...a),
  },
}));
vi.mock("@/modules/party/services/party.service", () => ({
  customerService: { listAddresses: (...a: unknown[]) => listAddresses(...a) },
}));
vi.mock("@/modules/party/components/PartySearchSelect", () => ({
  PartySearchSelect: ({ onChange }: { onChange: (id: number | null, party?: unknown) => void }) => (
    <button type="button" onClick={() => onChange(5, { id: 5, code: "C-5", name: "PT Maju", type: "customer" })}>
      pilih pelanggan
    </button>
  ),
}));
vi.mock("@/modules/products/components/ProductSearchSelect", () => ({
  ProductSearchSelect: ({ onChange }: { onChange: (id: number | null, product?: unknown) => void }) => (
    <>
      <button type="button" onClick={() => onChange(10, semen)}>
        pilih semen
      </button>
      <button type="button" onClick={() => onChange(20, ongkir)}>
        pilih ongkir
      </button>
      <button type="button" onClick={() => onChange(30, cat)}>
        pilih cat
      </button>
    </>
  ),
}));

import CreateDirectDeliveryPage from "@/modules/delivery/pages/CreateDirectDeliveryPage";

const semen = {
  id: 10,
  code: "SMN",
  name: "Semen",
  sellingPrice: 0,
  baseUom: "sak",
  purchaseUom: "sak",
  salesUom: "sak",
  purchaseFactor: 1,
  salesFactor: 1,
  purchasePrice: 0,
  minSellingPrice: 0,
  type: "barang",
  tracked: true,
  status: "active",
  variants: [{ id: 11, code: "SMN", name: "Semen", barcode: "", isDefault: true, status: "active", isHidden: true, sortOrder: 0, attributes: {} }],
};
const ongkir = { ...semen, id: 20, code: "ONGKIR", name: "Ongkos kirim", type: "jasa", tracked: false, variants: [{ ...semen.variants[0], id: 21 }] };

// Produk cat sesudah variantfix: "Standar" bawaan tersembunyi + warna terlihat (temuan 2).
const cat = {
  ...semen,
  id: 30,
  code: "CAT",
  name: "Cat Tembok",
  variants: [
    { ...semen.variants[0], id: 31, code: "CAT", name: "Standar" },
    { ...semen.variants[0], id: 33, code: "CAT-002", name: "Merah", isDefault: false, isHidden: false, sortOrder: 2 },
    { ...semen.variants[0], id: 32, code: "CAT-001", name: "Putih", isDefault: false, isHidden: false, sortOrder: 1 },
  ],
};

function Where() {
  const loc = useLocation();
  return <div data-testid="where">{loc.pathname}</div>;
}

function open() {
  return render(
    <MemoryRouter initialEntries={["/deliveries/new-direct"]}>
      <Routes>
        <Route path="/deliveries/new-direct" element={<CreateDirectDeliveryPage />} />
        <Route path="/deliveries/:id" element={<div>detail surat jalan</div>} />
      </Routes>
      <Where />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  listAddresses.mockResolvedValue([
    { id: 40, label: "Gudang proyek", recipient: "Pak Anton", phone: "0812", text: "Jl. Proyek 1", isPrimary: true, sortOrder: 0, status: "active" },
  ]);
  openDirect.mockResolvedValue({ items: [], count: 0, oldestDate: "" });
});
afterEach(() => cleanup());

describe("halaman Buat SJ Langsung (W-02)", () => {
  it("pelanggan dan minimal satu baris wajib; tanpa kolom harga", async () => {
    open();
    fireEvent.click(screen.getByRole("button", { name: "Simpan surat jalan" }));
    // Notice di atas form + galat pada kolom pelanggan.
    expect((await screen.findAllByText("Pelanggan wajib dipilih")).length).toBeGreaterThan(0);
    expect(createDirect).not.toHaveBeenCalled();
    expect(screen.queryByText(/Harga/)).toBeNull();
  });

  it("jasa/non-stok ditolak di klien dengan penjelasan; barang berstok menjadi baris", async () => {
    open();
    fireEvent.click(screen.getByText("pilih ongkir"));
    expect(screen.queryByText("ONGKIR — Ongkos kirim")).toBeNull();
    fireEvent.click(screen.getByText("pilih semen"));
    expect(await screen.findByText("SMN — Semen")).toBeTruthy();
    expect((screen.getByLabelText("Qty kirim SMN — Semen") as HTMLInputElement).value).toBe("1");
  });

  it("berhasil: mengirim partyId, alamat tersimpan, baris, dan clientRef lalu membuka detail", async () => {
    createDirect.mockResolvedValue({ message: "Surat jalan langsung dibuat", data: { id: 77, number: "SJ-0077", warnings: ["Stok belum ditahan"] } });
    open();
    fireEvent.click(screen.getByText("pilih pelanggan"));
    await waitFor(() => expect(listAddresses).toHaveBeenCalledWith(5));
    fireEvent.click(screen.getByText("pilih semen"));
    await screen.findByText("SMN — Semen");
    fireEvent.change(screen.getByLabelText("Qty kirim SMN — Semen"), { target: { value: "12" } });
    fireEvent.change(screen.getByPlaceholderText("Nama supir"), { target: { value: "Andi" } });
    fireEvent.change(screen.getByPlaceholderText("Nama petugas gudang"), { target: { value: "Sari" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan surat jalan" }));
    await waitFor(() => expect(createDirect).toHaveBeenCalledTimes(1));
    const body = createDirect.mock.calls[0][0] as Record<string, unknown>;
    expect(body.partyId).toBe(5);
    expect(body.driverName).toBe("Andi");
    expect(body.warehouseStaffName).toBe("Sari");
    expect(typeof body.clientRef).toBe("string");
    expect((body.clientRef as string).length).toBeGreaterThan(8);
    expect(body.items).toEqual([{ productId: 10, variantId: 11, locationId: undefined, uom: "sak", qty: 12 }]);
    expect("price" in body).toBe(false);
    expect(await screen.findByText("detail surat jalan")).toBeTruthy();
  });

  it("produk ber-warna: varian wajib dipilih, tanpa 'Standar' tersembunyi", async () => {
    createDirect.mockResolvedValue({ message: "ok", data: { id: 78, number: "SJ-0078", warnings: [] } });
    open();
    fireEvent.click(screen.getByText("pilih pelanggan"));
    fireEvent.click(screen.getByText("pilih cat"));
    const select = (await screen.findByLabelText("Varian CAT — Cat Tembok")) as HTMLSelectElement;
    expect(select.value).toBe("0");
    expect([...select.options].map((o) => o.textContent)).toEqual(["Pilih varian…", "CAT-001 — Putih", "CAT-002 — Merah"]);
    fireEvent.change(screen.getByPlaceholderText("Nama supir"), { target: { value: "Andi" } });
    fireEvent.change(screen.getByPlaceholderText("Nama petugas gudang"), { target: { value: "Sari" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan surat jalan" }));
    expect((await screen.findAllByText("Varian wajib dipilih")).length).toBeGreaterThan(0);
    expect(createDirect).not.toHaveBeenCalled();
    fireEvent.change(select, { target: { value: "33" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan surat jalan" }));
    await waitFor(() => expect(createDirect).toHaveBeenCalledTimes(1));
    expect((createDirect.mock.calls[0][0] as { items: { variantId: number }[] }).items[0].variantId).toBe(33);
  });

  it("galat per kolom dari server tampil di kolomnya (pesan server tidak diganti)", async () => {
    createDirect.mockRejectedValue({
      status: 400,
      message: "Validasi gagal",
      errors: [{ field: "items", message: "baris 1: produk Semen bukan barang berstok" }],
    });
    open();
    fireEvent.click(screen.getByText("pilih pelanggan"));
    fireEvent.click(screen.getByText("pilih semen"));
    await screen.findByText("SMN — Semen");
    fireEvent.click(screen.getByRole("button", { name: "Simpan surat jalan" }));
    await waitFor(() => expect(createDirect).toHaveBeenCalled());
    expect(await screen.findByText("baris 1: produk Semen bukan barang berstok")).toBeTruthy();
    expect(screen.getAllByText(/Validasi gagal/).length).toBeGreaterThan(0);
  });

  it("menampilkan peringatan bila pelanggan masih punya SJ Langsung belum diorder", async () => {
    openDirect.mockResolvedValue({ items: [], count: 2, oldestDate: "2026-10-01" });
    open();
    fireEvent.click(screen.getByText("pilih pelanggan"));
    expect(await screen.findByText(/masih punya surat jalan langsung yang belum diorder/)).toBeTruthy();
    expect(openDirect).toHaveBeenCalledWith({ partyId: 5, limit: 0 }, expect.anything());
  });
});
