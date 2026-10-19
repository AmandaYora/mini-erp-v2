import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

const create = vi.fn();
const uploadMedia = vi.fn();
vi.mock("@/modules/products/services/products.service", () => ({
  productsService: {
    listCategories: vi.fn().mockResolvedValue([]),
    create: (...a: unknown[]) => create(...a),
    uploadMedia: (...a: unknown[]) => uploadMedia(...a),
    detail: vi.fn(),
    update: vi.fn(),
  },
}));

// Mengetik banyak isian lewat userEvent lambat di mesin bersama.
vi.setConfig({ testTimeout: 60000 });

import ProductFormPage from "@/modules/products/pages/ProductFormPage";

beforeEach(() => {
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      observe() {}
      disconnect() {}
      unobserve() {}
    },
  );
  create.mockReset().mockResolvedValue({ message: "Produk dibuat", data: { id: 77 } });
  uploadMedia.mockReset().mockResolvedValue({ message: "ok", data: {} });
  useAuthStore.setState({ permissions: ["products.view", "products.create"], ready: true });
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

async function fillAndSubmit(photo?: File) {
  const user = userEvent.setup();
  render(
    <MemoryRouter initialEntries={["/products/new"]}>
      <Routes>
        <Route path="/products/new" element={<ProductFormPage />} />
        <Route path="/products/:id" element={<p>detail terbuka</p>} />
      </Routes>
    </MemoryRouter>,
  );
  await user.type(screen.getByLabelText(/^Kode/), "FOTO-1");
  await user.type(screen.getByLabelText(/^Nama/), "Produk Foto");
  await user.type(screen.getByLabelText(/Satuan dasar/), "pcs");
  await user.type(screen.getByLabelText(/Satuan beli/), "pcs");
  await user.type(screen.getByLabelText(/Satuan jual/), "pcs");
  const buy = screen.getByLabelText("Harga beli");
  await user.clear(buy);
  await user.type(buy, "1000");
  const sell = screen.getByLabelText("Harga jual");
  await user.clear(sell);
  await user.type(sell, "2000");
  if (photo) await user.upload(screen.getByLabelText(/Seret foto ke sini/), photo);
  await user.click(screen.getByRole("button", { name: /Simpan|Buat/ }));
  return user;
}

describe("form tambah produk — foto utama", () => {
  it("produk disimpan dulu, lalu foto diunggah sebagai foto utama", async () => {
    await fillAndSubmit(new File(["x"], "depan.jpg", { type: "image/jpeg" }));
    await waitFor(() => expect(uploadMedia).toHaveBeenCalledTimes(1));
    expect(create).toHaveBeenCalledTimes(1);
    expect(uploadMedia).toHaveBeenCalledWith(77, expect.objectContaining({ name: "depan.jpg" }), { isPrimary: true, sortOrder: 0 });
    expect(await screen.findByText("detail terbuka")).toBeInTheDocument();
  });

  it("unggah foto gagal tidak membatalkan produk: tetap membuka detail", async () => {
    uploadMedia.mockRejectedValue(new Error("gagal"));
    await fillAndSubmit(new File(["x"], "depan.jpg", { type: "image/jpeg" }));
    expect(await screen.findByText("detail terbuka")).toBeInTheDocument();
    expect(create).toHaveBeenCalledTimes(1);
  });

  it("tanpa foto tidak ada unggahan", async () => {
    await fillAndSubmit();
    expect(await screen.findByText("detail terbuka")).toBeInTheDocument();
    expect(uploadMedia).not.toHaveBeenCalled();
  });

  it("berkas bukan gambar ditolak di klien dan tidak dikirim", async () => {
    const user = userEvent.setup({ applyAccept: false });
    render(
      <MemoryRouter initialEntries={["/products/new"]}>
        <Routes>
          <Route path="/products/new" element={<ProductFormPage />} />
        </Routes>
      </MemoryRouter>,
    );
    await user.upload(screen.getByLabelText(/Seret foto ke sini/), new File(["x"], "dok.pdf", { type: "application/pdf" }));
    expect(await screen.findByText(/JPEG, PNG, atau WebP/, { selector: "p[role=alert]" })).toBeInTheDocument();
  });
});
