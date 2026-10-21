import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const get = vi.fn();
const proofs = vi.fn();
const salesOrderDetail = vi.fn();
const uploadProof = vi.fn();
const confirm = vi.fn();
const cancel = vi.fn();
const productOption = vi.fn();
vi.mock("@/modules/delivery/services/delivery.service", () => ({
  deliveryService: {
    get: (...a: unknown[]) => get(...a),
    proofs: (...a: unknown[]) => proofs(...a),
    salesOrderDetail: (...a: unknown[]) => salesOrderDetail(...a),
    uploadProof: (...a: unknown[]) => uploadProof(...a),
    confirm: (...a: unknown[]) => confirm(...a),
    cancel: (...a: unknown[]) => cancel(...a),
    productOption: (...a: unknown[]) => productOption(...a),
  },
}));

import DeliveryDetailPage from "@/modules/delivery/pages/DeliveryDetailPage";

const note = {
  id: 5,
  number: "SJ-0005",
  branchId: 1,
  salesOrderId: 7,
  orderNumber: "SO-0007",
  deliveryDate: "2026-10-10 00:00:00",
  status: "draft",
  notes: "",
  driverName: "Budi",
  vehiclePlate: "B 1 X",
  warehouseStaffName: "Sari",
  recipientName: "",
  recipientSignatureStatus: "",
  recipientSignatureMissingReason: "",
  dropLocationNote: "",
  dispatchedAt: null,
  dispatchedBy: null,
  confirmedAt: null,
  confirmedBy: null,
  documentKind: "order",
  items: [{ id: 1, productId: 10, variantId: 11, locationId: 3, uom: "sak", uomFactor: 1, qty: 2, qtyBase: 2 }],
};
const so = {
  id: 7,
  number: "SO-0007",
  partyName: "Budi",
  channel: "regular",
  paymentTerms: "net",
  status: "confirmed",
  items: [{ id: 1, productId: 10, variantId: 11, productCode: "P-10", productName: "Semen", uom: "sak", uomFactor: 1, qty: 2, qtyBase: 2 }],
};
const photo = { id: 1, originalName: "lokasi.jpg", mime: "image/jpeg", sizeBytes: 10, url: "/m/1", purpose: "delivery_location_photo" };
const signed = { id: 2, originalName: "sj-ttd.jpg", mime: "image/jpeg", sizeBytes: 10, url: "/m/2", purpose: "delivery_signed_document" };

// SJ Langsung (ADR-0022) terkonfirmasi yang belum diorder: pelanggan & alamat dari SJ.
const directNote = {
  ...note,
  id: 9,
  number: "SJ-0009",
  salesOrderId: 0,
  orderNumber: "",
  status: "confirmed",
  confirmedAt: "2026-10-11 10:00:00",
  confirmedBy: 2,
  origin: "direct",
  partyId: 5,
  partyName: "PT Maju",
  shipToLabel: "Proyek",
  shipToRecipient: "Pak Anton",
  shipToPhone: "0812",
  shipToAddress: "Jl. Proyek 1",
  linkedAt: "",
  linkedBy: 0,
};
const semenOption = {
  id: 10,
  code: "SMN",
  name: "Semen",
  variants: [{ id: 11, code: "SMN", name: "Semen", isHidden: true, isDefault: true, status: "active" }],
};

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ permissions: ["delivery.view", "delivery.create", "delivery.archive"], user: null, ready: true });
  get.mockResolvedValue(note);
  salesOrderDetail.mockResolvedValue(so);
  proofs.mockResolvedValue([]);
  productOption.mockResolvedValue(semenOption);
});
afterEach(() => cleanup());

function open() {
  return render(
    <MemoryRouter initialEntries={["/deliveries/5"]}>
      <Routes>
        <Route path="/deliveries/:id" element={<DeliveryDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("DeliveryDetailPage — bukti kirim bertipe (DL-01) dan peringatan stok (DL-04)", () => {
  it("memisahkan dua zona unggah: surat jalan fisik (wajib) dan foto lokasi (opsional)", async () => {
    proofs.mockResolvedValue([photo, signed]);
    open();
    expect(await screen.findByText(/Surat jalan fisik bertanda tangan \(1\)/)).toBeTruthy();
    expect(screen.getByText(/Foto lokasi pengiriman \(1\)/)).toBeTruthy();
    expect(screen.getByText("sj-ttd.jpg")).toBeTruthy();
    expect(screen.getByText("lokasi.jpg")).toBeTruthy();
    expect(screen.getByText("Unggah scan/foto surat jalan")).toBeTruthy();
    expect(screen.getByText("Unggah foto lokasi (opsional)")).toBeTruthy();
  });

  it("foto lokasi saja tidak memenuhi syarat konfirmasi: daftar kelengkapan menandai dokumen belum ada", async () => {
    proofs.mockResolvedValue([photo]);
    open();
    fireEvent.click(await screen.findByRole("button", { name: "Konfirmasi" }));
    expect(await screen.findByText(/Foto\/scan surat jalan fisik bertanda tangan \(minimal satu\)/)).toBeTruthy();
    expect(screen.getAllByText("Belum").length).toBeGreaterThan(0);
  });

  it("dengan scan SJ terunggah, syarat dokumen terpenuhi", async () => {
    proofs.mockResolvedValue([signed]);
    open();
    fireEvent.click(await screen.findByRole("button", { name: "Konfirmasi" }));
    await screen.findByText(/Foto\/scan surat jalan fisik bertanda tangan \(minimal satu\)/);
    // sopir + petugas + dokumen terpenuhi; hanya penerima+tanda tangan yang belum
    expect(screen.getAllByText("Terpenuhi").length).toBe(3);
  });

  it("SJ draf menampilkan peringatan stok belum ditahan", async () => {
    open();
    expect(await screen.findByText("Stok belum ditahan oleh surat jalan draf")).toBeTruthy();
  });

  it("galat konfirmasi (stok kurang) tampil utuh di dialog agar bisa dicoba ulang", async () => {
    proofs.mockResolvedValue([signed]);
    confirm.mockRejectedValue(
      Object.assign(new Error("x"), {
        isAxiosError: true,
        response: { status: 409, data: { success: false, message: "Surat jalan SJ-0005 belum bisa dikonfirmasi: Stok tersedia tidak mencukupi. Surat jalan tetap draf", errors: [] } },
      }),
    );
    open();
    fireEvent.click(await screen.findByRole("button", { name: "Konfirmasi" }));
    fireEvent.change(await screen.findByPlaceholderText("Nama penerima"), { target: { value: "Ani" } });
    fireEvent.change(screen.getByDisplayValue("Pilih status…"), { target: { value: "signed" } });
    const buttons = screen.getAllByRole("button", { name: "Konfirmasi" });
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => expect(confirm).toHaveBeenCalled());
    expect((await screen.findAllByText(/Surat jalan tetap draf/)).length).toBeGreaterThan(0);
  });
});

describe("DeliveryDetailPage — batal dengan alasan (koreksi salah input)", () => {
  it("dialog batal mewajibkan alasan lalu mengirimkannya", async () => {
    cancel.mockResolvedValue({ message: "Surat jalan dibatalkan", data: { ...note, status: "cancelled" } });
    open();
    await screen.findAllByText("SJ-0005");
    fireEvent.pointerDown(screen.getByRole("button", { name: "Aksi lainnya" }), { button: 0, ctrlKey: false });
    fireEvent.click(await screen.findByRole("menuitem", { name: "Batalkan surat jalan" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Batalkan surat jalan" }));
    expect(await within(dialog).findByText("Alasan wajib diisi")).toBeTruthy();
    expect(cancel).not.toHaveBeenCalled();
    fireEvent.change(within(dialog).getByLabelText(/Alasan/), { target: { value: "jumlah salah" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Batalkan surat jalan" }));
    await waitFor(() => expect(cancel).toHaveBeenCalledWith(5, "jumlah salah"));
  });

  it("SJ batal menampilkan alasan dan waktu batal", async () => {
    get.mockResolvedValue({
      ...note,
      status: "cancelled",
      cancelReason: "sopir salah",
      cancelledAt: "2026-10-21 09:30:00",
      cancelledBy: 4,
    });
    open();
    expect(await screen.findByText("sopir salah")).toBeTruthy();
    expect(screen.getByText("Alasan batal")).toBeTruthy();
  });
});

describe("DeliveryDetailPage — SJ Langsung (ADR-0022, W-03)", () => {
  it("SJ order biasa tidak berubah: tanpa lencana SJ Langsung dan tanpa langkah Diorder", async () => {
    open();
    await screen.findAllByText("SJ-0005");
    expect(screen.queryByText("SJ Langsung")).toBeNull();
    expect(screen.queryByText("Belum diorder")).toBeNull();
    expect(screen.queryByText("Diorder")).toBeNull();
    expect(salesOrderDetail).toHaveBeenCalledWith(7);
  });

  it("SJ Langsung belum diorder: lencana, langkah Diorder, pelanggan & alamat dari SJ, nama produk dari master", async () => {
    get.mockResolvedValue(directNote);
    open();
    await screen.findAllByText("SJ-0009");
    expect(screen.getByText("SJ Langsung")).toBeTruthy();
    // Lencana di header + nilai "Order jual" di Info dokumen.
    expect(screen.getAllByText("Belum diorder").length).toBe(2);
    expect(screen.getByText("Diorder")).toBeTruthy();
    expect(screen.getAllByText(/PT Maju/).length).toBeGreaterThan(0);
    expect(screen.getByText(/Jl\. Proyek 1/)).toBeTruthy();
    expect(screen.getByText("Semen")).toBeTruthy();
    expect(salesOrderDetail).not.toHaveBeenCalled();
    expect(productOption).toHaveBeenCalledWith(10);
    expect(screen.queryByText("Dipesan (SO)")).toBeNull();
  });

  it("tombol Buat order dari SJ ini hanya dengan sales.create, menuju form order ?fromDeliveries=", async () => {
    get.mockResolvedValue(directNote);
    open();
    await screen.findAllByText("SJ-0009");
    expect(screen.queryByRole("link", { name: "Buat order dari SJ ini" })).toBeNull();
    cleanup();
    useAuthStore.setState({ permissions: ["delivery.view", "sales.create"], user: null, ready: true });
    open();
    await screen.findAllByText("SJ-0009");
    const link = screen.getByRole("link", { name: "Buat order dari SJ ini" });
    expect(link.getAttribute("href")).toBe("/sales-orders/new?fromDeliveries=9");
  });

  it("SJ Langsung terkonfirmasi belum diorder masih bisa dibatalkan dengan alasan (stok dikembalikan)", async () => {
    get.mockResolvedValue(directNote);
    cancel.mockResolvedValue({ message: "Surat jalan dibatalkan, stok dikembalikan", data: { ...directNote, status: "cancelled" } });
    open();
    await screen.findAllByText("SJ-0009");
    fireEvent.pointerDown(screen.getByRole("button", { name: "Aksi lainnya" }), { button: 0, ctrlKey: false });
    fireEvent.click(await screen.findByRole("menuitem", { name: "Batalkan surat jalan" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/stok yang sudah keluar dikembalikan/)).toBeTruthy();
    fireEvent.change(within(dialog).getByLabelText(/Alasan/), { target: { value: "salah pelanggan" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Batalkan surat jalan" }));
    await waitFor(() => expect(cancel).toHaveBeenCalledWith(5, "salah pelanggan"));
  });

  it("SJ order terkonfirmasi tetap tidak menawarkan batal (aturan lama)", async () => {
    get.mockResolvedValue({ ...note, status: "confirmed" });
    open();
    await screen.findAllByText("SJ-0005");
    expect(screen.queryByRole("button", { name: "Aksi lainnya" })).toBeNull();
  });

  it("dialog konfirmasi SJ Langsung draf tidak memuat pelunasan COD dan tidak mengirim settlement", async () => {
    get.mockResolvedValue({ ...directNote, status: "draft", confirmedAt: null, confirmedBy: null });
    proofs.mockResolvedValue([signed]);
    confirm.mockResolvedValue({ message: "Surat jalan dikonfirmasi", data: { ...directNote } });
    open();
    fireEvent.click(await screen.findByRole("button", { name: "Konfirmasi" }));
    expect(await screen.findByText(/tanpa pelunasan; order dibuat belakangan/)).toBeTruthy();
    expect(screen.queryByText("Pelunasan order COD")).toBeNull();
    fireEvent.change(screen.getByPlaceholderText("Nama penerima"), { target: { value: "Ani" } });
    fireEvent.change(screen.getByDisplayValue("Pilih status…"), { target: { value: "signed" } });
    const buttons = screen.getAllByRole("button", { name: "Konfirmasi" });
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => expect(confirm).toHaveBeenCalled());
    const body = confirm.mock.calls[0][1] as Record<string, unknown>;
    expect(body.settlement).toBeUndefined();
    expect(body.recipientName).toBe("Ani");
  });
});
