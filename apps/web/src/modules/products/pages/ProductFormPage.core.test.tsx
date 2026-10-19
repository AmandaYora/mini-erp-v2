import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";

// T-03: alur inti form produk (render, validasi wajib, payload, galat server).
// Foto utama diuji di ProductFormPage.test.tsx.
const create = vi.fn();
vi.mock("@/modules/products/services/products.service", () => ({
  productsService: {
    listCategories: vi.fn().mockResolvedValue([]),
    create: (...a: unknown[]) => create(...a),
    uploadMedia: vi.fn(),
    detail: vi.fn(),
    update: vi.fn(),
  },
}));

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
  useAuthStore.setState({ permissions: ["products.view", "products.create"], ready: true });
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function renderForm() {
  return render(
    <MemoryRouter initialEntries={["/products/new"]}>
      <Routes>
        <Route path="/products/new" element={<ProductFormPage />} />
        <Route path="/products/:id" element={<p>detail terbuka</p>} />
      </Routes>
    </MemoryRouter>,
  );
}

async function fillRequired(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/^Kode/), "GULA-1");
  await user.type(screen.getByLabelText(/^Nama/), "Gula Pasir");
  await user.type(screen.getByLabelText(/Satuan dasar/), "kg");
  await user.type(screen.getByLabelText(/Satuan beli/), "karung");
  await user.type(screen.getByLabelText(/Satuan jual/), "kg");
  const buy = screen.getByLabelText("Harga beli");
  await user.clear(buy);
  await user.type(buy, "600000");
  const sell = screen.getByLabelText("Harga jual");
  await user.clear(sell);
  await user.type(sell, "15000");
}

describe("form tambah produk — inti", () => {
  it("render: bagian identitas, satuan, dan harga tersedia", () => {
    renderForm();
    for (const label of [/^Kode/, /^Nama/, /Satuan dasar/, /Satuan beli/, /Satuan jual/, "Harga beli", "Harga jual"]) {
      expect(screen.getByLabelText(label)).toBeInTheDocument();
    }
  });

  it("submit kosong: wajib-isi ditandai dan server tidak dipanggil", async () => {
    const user = userEvent.setup();
    renderForm();
    await user.click(screen.getByRole("button", { name: /Simpan|Buat/ }));
    expect((await screen.findAllByText(/wajib/i)).length).toBeGreaterThan(0);
    expect(create).not.toHaveBeenCalled();
  });

  it("isian sah: payload membawa kode, satuan, dan harga sebagai angka lalu membuka detail", async () => {
    const user = userEvent.setup();
    renderForm();
    await fillRequired(user);
    await user.click(screen.getByRole("button", { name: /Simpan|Buat/ }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0]).toMatchObject({
      code: "GULA-1", name: "Gula Pasir", baseUom: "kg", purchaseUom: "karung", salesUom: "kg",
      purchasePrice: 600000, sellingPrice: 15000,
    });
    expect(await screen.findByText("detail terbuka")).toBeInTheDocument();
  });

  it("kode duplikat dari server: pesan tampil dan pengguna tetap di form dengan isian utuh", async () => {
    // Bentuk asli server: Conflict polos tanpa daftar field (service.go).
    create.mockRejectedValue({ status: 409, message: "Kode produk 'GULA-1' sudah digunakan" });
    const user = userEvent.setup();
    renderForm();
    await fillRequired(user);
    await user.click(screen.getByRole("button", { name: /Simpan|Buat/ }));
    expect((await screen.findAllByText(/Kode produk .*sudah digunakan/)).length).toBeGreaterThan(0);
    expect(screen.queryByText("detail terbuka")).not.toBeInTheDocument();
    expect(screen.getByLabelText(/^Kode/)).toHaveValue("GULA-1");
  });
});
