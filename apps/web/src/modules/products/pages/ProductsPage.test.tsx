import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import type { Product } from "@/modules/products/types";

const list = vi.fn();
const listCategories = vi.fn();
const scanLookup = vi.fn();
vi.mock("@/modules/products/services/products.service", () => ({
  productsService: {
    list: (...a: unknown[]) => list(...a),
    listCategories: (...a: unknown[]) => listCategories(...a),
    scanLookup: (...a: unknown[]) => scanLookup(...a),
    downloadQrZip: vi.fn(),
  },
}));
vi.mock("@/modules/products/components/QrModal", () => ({ default: () => null }));

// Dialog kamera sebenarnya butuh @zxing + kamera; di sini hanya pemicu onScan.
vi.mock("@/modules/pos/components/ScanDialog", () => ({
  ScanDialog: ({
    open,
    onScan,
  }: {
    open: boolean;
    onScan: (code: string) => Promise<{ kind: string; detail?: string }>;
  }) =>
    open ? (
      <div role="dialog" aria-label="Pindai">
        <button type="button" onClick={() => void onScan("8991234")}>
          kirim kode
        </button>
        <button type="button" onClick={() => void onScan("TAK-ADA")}>
          kirim kode asing
        </button>
      </div>
    ) : null,
}));

import ProductsPage from "@/modules/products/pages/ProductsPage";

const product = (over: Partial<Product>): Product => ({
  id: 1,
  code: "BRG-001",
  name: "Gula Pasir",
  categoryId: null,
  categoryName: "",
  type: "barang",
  tracked: true,
  baseUom: "pcs",
  purchaseUom: "pcs",
  salesUom: "pak",
  purchaseFactor: 1,
  salesFactor: 4,
  purchasePrice: 8000,
  sellingPrice: 12000,
  minSellingPrice: 0,
  minStock: 0,
  status: "active",
  summary: "",
  attributes: {},
  variants: [],
  ...over,
});

const page = (items: Product[]) => ({ items, meta: { page: 1, limit: 20, total: items.length, totalPages: 1 } });

function renderPage(path = "/products") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/products" element={<ProductsPage />} />
        <Route path="/products/:id" element={<p>detail produk terbuka</p>} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  for (const m of [list, listCategories, scanLookup]) m.mockReset();
  listCategories.mockResolvedValue([]);
  list.mockResolvedValue(page([product({})]));
  useAuthStore.setState({ permissions: ["products.view"], ready: true });
});
afterEach(cleanup);

describe("daftar produk — filter bawaan Aktif + arsip", () => {
  it("tampilan awal hanya produk aktif (status=active) tanpa chip filter dan tanpa kolom arsip", async () => {
    renderPage();
    await screen.findByText("Gula Pasir");
    expect(list.mock.calls[0][0]).toMatchObject({ status: "active" });
    expect(screen.queryByText(/^Status:/)).not.toBeInTheDocument();
    expect(screen.queryByText("Diarsipkan pada")).not.toBeInTheDocument();
  });

  it("satu klik 'Tampilkan arsip' membuka arsip + kolom 'Diarsipkan pada'; klik lagi kembali ke aktif", async () => {
    const user = userEvent.setup();
    list.mockImplementation(async (p: { status: string }) =>
      page(
        p.status === "archived"
          ? [product({ id: 9, code: "OLD-1", name: "Barang Lama", status: "archived", archivedAt: "2026-09-30T05:00:00Z" })]
          : [product({})],
      ),
    );
    renderPage();
    await screen.findByText("Gula Pasir");
    await user.click(screen.getByRole("button", { name: "Tampilkan arsip" }));
    await screen.findByText("Barang Lama");
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ status: "archived" }), expect.anything());
    expect(screen.getByText("Diarsipkan pada")).toBeInTheDocument();
    expect(screen.getByText(/30 Sep 2026|30\/09\/2026|30 September 2026/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Kembali ke aktif" }));
    await screen.findByText("Gula Pasir");
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ status: "active" }), expect.anything());
  });

  it("pilihan eksplisit 'Semua status' tidak mengirim filter status (tetap di URL)", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("Gula Pasir");
    await user.selectOptions(screen.getByLabelText("Status"), "all");
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ status: "" }), expect.anything()));
    expect(screen.getByText("Status: Semua status")).toBeInTheDocument();
  });

  it("URL ?status=archived dapat dibagikan: langsung memuat arsip", async () => {
    renderPage("/products?status=archived");
    await screen.findByText("Gula Pasir");
    expect(list.mock.calls[0][0]).toMatchObject({ status: "archived" });
  });
});

describe("daftar produk — stok, Min, dan harga beli sesuai izin", () => {
  const rows = [
    product({ id: 1, code: "A-1", name: "Di bawah minimum", stockAvailable: 2.5, stockUom: "pak", belowMinStock: true, minStock: 40 }),
    product({ id: 2, code: "A-2", name: "Stok aman", stockAvailable: 30, stockUom: "pak", belowMinStock: false, minStock: 8 }),
    product({ id: 3, code: "J-1", name: "Servis", type: "jasa", tracked: false }),
  ];

  it("dengan stock.view: kolom Stok (satuan jual), 'Min X' (dalam satuan jual) dan penanda di bawah minimum", async () => {
    useAuthStore.setState({ permissions: ["products.view", "stock.view"], ready: true });
    list.mockResolvedValue(page(rows));
    renderPage();
    await screen.findByText("Di bawah minimum");
    expect(screen.getByRole("columnheader", { name: "Stok" })).toBeInTheDocument();
    const low = screen.getByText("Di bawah minimum").closest("tr")!;
    expect(within(low).getByText("2,5 pak")).toHaveClass("text-destructive");
    expect(within(low).getByText("Min 10")).toBeInTheDocument(); // 40 satuan stok / 4 per pak
    const ok = screen.getByText("Stok aman").closest("tr")!;
    expect(within(ok).getByText("30 pak")).not.toHaveClass("text-destructive");
    expect(within(ok).getByText("Min 2")).toBeInTheDocument();
    const svc = screen.getByText("Servis").closest("tr")!;
    expect(within(svc).queryByText(/pak/)).not.toBeInTheDocument(); // tak dipantau: tanpa angka
  });

  it("tanpa stock.view kolom Stok tidak ada", async () => {
    list.mockResolvedValue(page(rows));
    renderPage();
    await screen.findByText("Stok aman");
    expect(screen.queryByRole("columnheader", { name: "Stok" })).not.toBeInTheDocument();
  });

  it("harga beli hanya untuk peran biaya (products.update / purchasing.view)", async () => {
    list.mockResolvedValue(page([product({ purchasePrice: 8000 })]));
    const { unmount } = renderPage();
    await screen.findByText("Gula Pasir");
    expect(screen.queryByRole("columnheader", { name: "Harga beli" })).not.toBeInTheDocument();
    unmount();

    useAuthStore.setState({ permissions: ["products.view", "purchasing.view"], ready: true });
    renderPage();
    await screen.findByText("Gula Pasir");
    expect(screen.getByRole("columnheader", { name: "Harga beli" })).toBeInTheDocument();
  });

  it("server tidak mengirim purchasePrice (tanpa izin) → sel '–', bukan Rp 0", async () => {
    useAuthStore.setState({ permissions: ["products.view", "products.update"], ready: true });
    const noCost = product({});
    delete (noCost as Partial<Product>).purchasePrice;
    list.mockResolvedValue(page([noCost]));
    renderPage();
    await screen.findByText("Gula Pasir");
    const row = screen.getByText("Gula Pasir").closest("tr")!;
    const cells = within(row).getAllByRole("cell");
    const idx = Array.from(row.parentElement!.parentElement!.querySelectorAll("th")).findIndex((h) => h.textContent === "Harga beli");
    expect(cells[idx]).toHaveTextContent("–");
  });
});

describe("daftar produk — pindai barcode (sekali pakai)", () => {
  it("kode dikenal membuka detail produk", async () => {
    const user = userEvent.setup();
    scanLookup.mockResolvedValue({ id: 42, code: "BRG-042", name: "Teh", status: "active", via: "barcode", variantRequired: false });
    renderPage();
    await screen.findByText("Gula Pasir");
    await user.click(screen.getByRole("button", { name: /Pindai/ }));
    await user.click(screen.getByRole("button", { name: "kirim kode" }));
    expect(await screen.findByText("detail produk terbuka")).toBeInTheDocument();
    expect(scanLookup).toHaveBeenCalledWith("8991234");
  });

  it("kode tak dikenal (404) tidak menavigasi", async () => {
    const user = userEvent.setup();
    scanLookup.mockRejectedValue({ isAxiosError: true, response: { status: 404, data: { message: "Produk tidak ditemukan" } } });
    renderPage();
    await screen.findByText("Gula Pasir");
    await user.click(screen.getByRole("button", { name: /Pindai/ }));
    await user.click(screen.getByRole("button", { name: "kirim kode asing" }));
    await waitFor(() => expect(scanLookup).toHaveBeenCalledWith("TAK-ADA"));
    expect(screen.queryByText("detail produk terbuka")).not.toBeInTheDocument();
  });
});
