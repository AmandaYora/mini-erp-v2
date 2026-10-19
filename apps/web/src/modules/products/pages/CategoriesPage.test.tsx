import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { ProductCategory } from "@/modules/products/types";

const listCategories = vi.fn();
const createCategory = vi.fn();
const updateCategory = vi.fn();
vi.mock("@/modules/products/services/products.service", () => ({
  productsService: {
    listCategories: (...a: unknown[]) => listCategories(...a),
    createCategory: (...a: unknown[]) => createCategory(...a),
    updateCategory: (...a: unknown[]) => updateCategory(...a),
    archiveCategory: vi.fn(),
  },
}));

import CategoriesPage from "@/modules/products/pages/CategoriesPage";

const cat = (id: number, code: string, sortOrder: number): ProductCategory => ({
  id, code, name: `Kategori ${code}`, parentId: null, sortOrder, status: "active",
});

beforeEach(() => {
  for (const m of [listCategories, createCategory, updateCategory]) m.mockReset();
  listCategories.mockResolvedValue([cat(1, "A", 5), cat(2, "B", 1), cat(3, "C", 3)]);
  createCategory.mockResolvedValue({ message: "Kategori dibuat", data: {} });
  updateCategory.mockResolvedValue({ message: "Kategori disimpan", data: {} });
  useAuthStore.setState({ permissions: ["products.view", "product_categories.manage"], ready: true });
});
afterEach(cleanup);

const renderPage = () =>
  render(
    <MemoryRouter>
      <CategoriesPage />
    </MemoryRouter>,
  );

describe("halaman kategori — urutan tampil", () => {
  it("baris mengikuti sortOrder (bukan kode) dan kolom Urutan tampil", async () => {
    renderPage();
    await screen.findByText("Kategori A");
    const names = screen.getAllByRole("row").slice(1).map((r) => within(r).getAllByRole("cell")[0].textContent);
    expect(names).toEqual(["Kategori B", "Kategori C", "Kategori A"]);
    expect(screen.getByRole("columnheader", { name: "Urutan" })).toBeInTheDocument();
  });

  it("form tambah mengirim sortOrder", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Kategori A");
    await user.click(screen.getByRole("button", { name: "Tambah kategori" }));
    await user.type(screen.getByLabelText(/Kode/), "NEW");
    await user.type(screen.getByLabelText(/^Nama/), "Baru");
    const sort = screen.getByLabelText("Urutan tampil");
    await user.clear(sort);
    await user.type(sort, "7");
    await user.click(screen.getByRole("button", { name: "Buat kategori" }));
    expect(createCategory).toHaveBeenCalledWith({ code: "NEW", name: "Baru", parentId: null, sortOrder: 7 });
  });

  it("urutan di luar 0-9999 ditolak di klien", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Kategori A");
    await user.click(screen.getByRole("button", { name: "Tambah kategori" }));
    await user.type(screen.getByLabelText(/Kode/), "NEW");
    await user.type(screen.getByLabelText(/^Nama/), "Baru");
    const sort = screen.getByLabelText("Urutan tampil");
    await user.clear(sort);
    await user.type(sort, "10000");
    await user.click(screen.getByRole("button", { name: "Buat kategori" }));
    expect(await screen.findByText("Urutan maksimal 9999")).toBeInTheDocument();
    expect(createCategory).not.toHaveBeenCalled();
  });
});
