import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const salesOrderDetail = vi.fn();
const list = vi.fn();
const create = vi.fn();
vi.mock("@/modules/delivery/services/delivery.service", () => ({
  deliveryService: {
    locationOptions: vi.fn().mockResolvedValue([]),
    salesOrderDetail: (...a: unknown[]) => salesOrderDetail(...a),
    list: (...a: unknown[]) => list(...a),
    create: (...a: unknown[]) => create(...a),
  },
}));
vi.mock("@/modules/sales/components/SalesOrderSearchSelect", () => ({
  SalesOrderSearchSelect: ({ onChange }: { onChange: (id: number | null) => void }) => (
    <button type="button" onClick={() => onChange(7)}>
      pilih order
    </button>
  ),
}));

import CreateDeliveryPage from "@/modules/delivery/pages/CreateDeliveryPage";

const so = {
  id: 7,
  number: "SO-0007",
  partyId: 1,
  partyName: "Budi",
  channel: "regular",
  orderDate: "2026-10-01",
  status: "confirmed",
  items: [
    { id: 1, productId: 10, variantId: 11, productCode: "P-10", productName: "Semen", uom: "sak", uomFactor: 1, qty: 10, qtyBase: 10 },
    { id: 2, productId: 20, variantId: 21, productCode: "P-20", productName: "Paku", uom: "kg", uomFactor: 1, qty: 5, qtyBase: 5 },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
  salesOrderDetail.mockResolvedValue(so);
  // 4 sak sudah dikirim, 2 sak masih draf; paku belum tersentuh.
  list.mockResolvedValue({
    items: [
      { status: "confirmed", items: [{ productId: 10, variantId: 11, qty: 4 }] },
      { status: "draft", items: [{ productId: 10, variantId: 11, qty: 2 }] },
      { status: "cancelled", items: [{ productId: 20, variantId: 21, qty: 5 }] },
    ],
    meta: {},
  });
});
afterEach(() => cleanup());

function Where() {
  const loc = useLocation();
  return <div data-testid="where">{loc.pathname + loc.search}</div>;
}

function open(path = "/deliveries/new?from=7") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/deliveries/new" element={<CreateDeliveryPage />} />
        <Route path="/deliveries/:id" element={<div>detail surat jalan</div>} />
      </Routes>
      <Where />
    </MemoryRouter>,
  );
}

describe("halaman Buat Surat Jalan (?from=)", () => {
  it("order terkunci dari ?from= dan baris terisi sisa kirim (bukan qty SO penuh)", async () => {
    open();
    expect(await screen.findByDisplayValue("SO-0007 — Budi")).toBeTruthy();
    await screen.findByText("Semen");
    const qtyInputs = screen.getAllByRole("spinbutton") as HTMLInputElement[];
    expect(qtyInputs.map((i) => i.value)).toEqual(["4", "5"]);
    expect(list).toHaveBeenCalledWith({ salesOrderId: 7, page: 1, limit: 100 });
  });

  it("tanpa ?from= memilih order lewat pencarian lalu memuat baris dan menyinkronkan URL", async () => {
    open("/deliveries/new");
    expect(await screen.findByText(/Pilih order jual untuk memuat baris barang/)).toBeTruthy();
    fireEvent.click(screen.getByText("pilih order"));
    await screen.findByText("Semen");
    expect(screen.getByTestId("where")).toHaveTextContent("/deliveries/new?from=7");
  });

  it("menjelaskan dokumen wajib dan menandai sopir/petugas wajib", async () => {
    open();
    await screen.findByText("Semen");
    expect(screen.getByText(/wajib mencantumkan nama sopir dan petugas gudang/)).toBeTruthy();
    expect(screen.getByPlaceholderText("Nama supir")).toBeTruthy();
    expect(screen.getByPlaceholderText("Nama petugas gudang")).toBeTruthy();
  });

  it("galat per kolom dari server tampil di kolomnya", async () => {
    create.mockRejectedValue({
      status: 400,
      message: "Dokumen surat jalan belum lengkap",
      errors: [
        { field: "driverName", message: "nama sopir wajib diisi" },
        { field: "warehouseStaffName", message: "nama petugas gudang wajib diisi" },
      ],
    });
    open();
    await screen.findByText("Semen");
    fireEvent.click(screen.getByRole("button", { name: "Simpan surat jalan" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByText("nama sopir wajib diisi")).toBeTruthy());
    expect(screen.getByText("nama petugas gudang wajib diisi")).toBeTruthy();
    expect(screen.getAllByText(/Dokumen surat jalan belum lengkap/).length).toBeGreaterThan(0);
  });

  it("berhasil: membuat dengan clientRef lalu membuka detail", async () => {
    create.mockResolvedValue({ message: "ok", data: { id: 55, number: "SJ-55" } });
    open();
    await screen.findByText("Semen");
    fireEvent.change(screen.getByPlaceholderText("Nama supir"), { target: { value: "Andi" } });
    fireEvent.change(screen.getByPlaceholderText("Nama petugas gudang"), { target: { value: "Rudi" } });
    fireEvent.click(screen.getByRole("button", { name: "Simpan surat jalan" }));
    await screen.findByText("detail surat jalan");
    const body = create.mock.calls[0][0];
    expect(body).toMatchObject({ salesOrderId: 7, driverName: "Andi", warehouseStaffName: "Rudi" });
    expect(typeof body.clientRef).toBe("string");
    expect(body.items).toHaveLength(2);
  });
});
