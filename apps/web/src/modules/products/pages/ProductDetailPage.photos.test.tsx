import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { Product, ProductMedia } from "@/modules/products/types";

// T-03: panel foto produk (tab Foto di detail produk): render, unggah, ganti
// foto utama, hapus. Aturan berkas (jenis/ukuran) ditolak di klien.
const detail = vi.fn();
const listMedia = vi.fn();
const uploadMedia = vi.fn();
const setPrimaryMedia = vi.fn();
const deleteMedia = vi.fn();
vi.mock("@/modules/products/services/products.service", () => ({
  productsService: {
    detail: (...a: unknown[]) => detail(...a),
    listMedia: (...a: unknown[]) => listMedia(...a),
    uploadMedia: (...a: unknown[]) => uploadMedia(...a),
    setPrimaryMedia: (...a: unknown[]) => setPrimaryMedia(...a),
    deleteMedia: (...a: unknown[]) => deleteMedia(...a),
    stockBalances: vi.fn().mockResolvedValue([]),
    archive: vi.fn(),
  },
}));

vi.setConfig({ testTimeout: 60000 });

import ProductDetailPage from "@/modules/products/pages/ProductDetailPage";

const product: Product = {
  id: 5, code: "GULA-1", name: "Gula Pasir", categoryId: 1, categoryName: "Sembako", type: "goods",
  tracked: true, baseUom: "kg", purchaseUom: "karung", salesUom: "kg", purchaseFactor: 50, salesFactor: 1,
  purchasePrice: 600000, sellingPrice: 15000, minSellingPrice: 0, minStock: 0, status: "active",
  summary: "", attributes: {}, variants: [],
};
const photo = (id: number, primary: boolean): ProductMedia => ({
  id, originalName: `foto-${id}.jpg`, mime: "image/jpeg", sizeBytes: 1000, isPrimary: primary,
  url: `/media/${id}`, width: 800, height: 600,
});

beforeEach(() => {
  for (const m of [detail, listMedia, uploadMedia, setPrimaryMedia, deleteMedia]) m.mockReset();
  detail.mockResolvedValue(product);
  listMedia.mockResolvedValue([photo(1, true), photo(2, false)]);
  uploadMedia.mockResolvedValue({ message: "Foto diunggah", data: photo(3, false) });
  setPrimaryMedia.mockResolvedValue({ message: "Foto utama diganti", data: null });
  deleteMedia.mockResolvedValue({ message: "Foto dihapus", data: null });
  useAuthStore.setState({ permissions: ["products.view", "products.update"], ready: true });
});
afterEach(cleanup);

function renderPhotos() {
  return render(
    <MemoryRouter initialEntries={["/products/5?tab=foto"]}>
      <Routes>
        <Route path="/products/:id" element={<ProductDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("detail produk — panel foto", () => {
  it("menampilkan foto, penanda Utama, dimensi, dan aksi hanya pada foto bukan-utama", async () => {
    renderPhotos();
    expect(await screen.findByAltText("foto-1.jpg")).toBeInTheDocument();
    expect(screen.getByAltText("foto-2.jpg")).toBeInTheDocument();
    expect(screen.getAllByText("Utama")).toHaveLength(1);
    expect(screen.getAllByText(/800 × 600 px/)).toHaveLength(2);
    // "Jadikan utama" hanya untuk foto kedua; "Hapus" untuk keduanya.
    expect(screen.getAllByRole("button", { name: "Jadikan utama" })).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "Hapus" })).toHaveLength(2);
    expect(listMedia).toHaveBeenCalledWith(5);
  });

  it("tanpa foto: keadaan kosong dengan ajakan unggah", async () => {
    listMedia.mockResolvedValue([]);
    renderPhotos();
    expect(await screen.findByText("Belum ada foto")).toBeInTheDocument();
    expect(screen.getByText(/Unggah foto pertama/)).toBeInTheDocument();
  });

  it("gagal memuat daftar foto tidak menjatuhkan halaman produk", async () => {
    listMedia.mockRejectedValue(new Error("media down"));
    renderPhotos();
    expect(await screen.findByText("Belum ada foto")).toBeInTheDocument();
    expect(screen.getAllByText("Gula Pasir").length).toBeGreaterThan(0);
  });

  it("unggah foto sah memanggil server lalu memuat ulang daftar", async () => {
    const user = userEvent.setup();
    renderPhotos();
    await screen.findByAltText("foto-1.jpg");
    const file = new File(["x"], "baru.png", { type: "image/png" });
    await user.upload(screen.getByLabelText(/Seret foto ke sini/), file);
    await waitFor(() => expect(uploadMedia).toHaveBeenCalledWith(5, file));
    await waitFor(() => expect(listMedia.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it("berkas bukan gambar ditolak di klien dan tidak dikirim", async () => {
    const user = userEvent.setup({ applyAccept: false });
    renderPhotos();
    await screen.findByAltText("foto-1.jpg");
    await user.upload(screen.getByLabelText(/Seret foto ke sini/), new File(["x"], "dokumen.pdf", { type: "application/pdf" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/JPEG, PNG, atau WebP/);
    expect(uploadMedia).not.toHaveBeenCalled();
  });

  it("galat server saat unggah tampil sebagai pesan dan daftar tidak berubah", async () => {
    uploadMedia.mockRejectedValue({ status: 413, message: "Berkas terlalu besar" });
    const user = userEvent.setup();
    renderPhotos();
    await screen.findByAltText("foto-1.jpg");
    await user.upload(screen.getByLabelText(/Seret foto ke sini/), new File(["x"], "besar.jpg", { type: "image/jpeg" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Berkas terlalu besar");
  });

  it("Jadikan utama memanggil server dengan id foto yang benar", async () => {
    const user = userEvent.setup();
    renderPhotos();
    await screen.findByAltText("foto-2.jpg");
    await user.click(screen.getByRole("button", { name: "Jadikan utama" }));
    await waitFor(() => expect(setPrimaryMedia).toHaveBeenCalledWith(5, 2));
  });

  it("Hapus meminta konfirmasi: batal tidak menghapus, konfirmasi menghapus foto yang dipilih", async () => {
    const user = userEvent.setup();
    renderPhotos();
    await screen.findByAltText("foto-2.jpg");
    const second = screen.getByAltText("foto-2.jpg").closest("li") as HTMLElement;
    await user.click(within(second).getByRole("button", { name: "Hapus" }));
    let dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText(/foto-2\.jpg/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Batal" }));
    expect(deleteMedia).not.toHaveBeenCalled();

    await user.click(within(second).getByRole("button", { name: "Hapus" }));
    dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Hapus" }));
    await waitFor(() => expect(deleteMedia).toHaveBeenCalledWith(5, 2));
  });

  it("tanpa izin products.update: tidak ada dropzone maupun aksi foto", async () => {
    useAuthStore.setState({ permissions: ["products.view"], ready: true });
    renderPhotos();
    await screen.findByAltText("foto-1.jpg");
    expect(screen.queryByLabelText(/Seret foto ke sini/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Hapus" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Jadikan utama" })).not.toBeInTheDocument();
  });

  it("produk arsip: foto dikunci (tanpa unggah/ubah)", async () => {
    detail.mockResolvedValue({ ...product, status: "archived" });
    renderPhotos();
    await screen.findByAltText("foto-1.jpg");
    expect(screen.queryByLabelText(/Seret foto ke sini/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Hapus" })).not.toBeInTheDocument();
  });
});
