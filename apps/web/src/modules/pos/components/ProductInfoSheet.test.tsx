import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PosProductListItem } from "@/modules/pos/types";

// Semua fungsi posService dicatat: lembar info hanya boleh membaca (tanpa hold/suggest).
const svc = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  return { fns, proxy: new Proxy({} as Record<string, unknown>, { get: (_t, k: string) => (fns[k] ??= vi.fn().mockResolvedValue([])) }) };
});
vi.mock("@/modules/pos/services/pos.service", () => ({ posService: svc.proxy }));
import { ProductInfoSheet } from "@/modules/pos/components/ProductInfoSheet";

const tile: PosProductListItem = { id: 7, code: "CAT", name: "Cat Tembok", categoryId: null, categoryName: null, sellingPrice: 60000, status: "active" };
const variant = (id: number, over: Record<string, unknown> = {}) => ({ id, code: `CAT-${id}`, name: `Warna ${id}`, barcode: `899${id}`, isDefault: false, status: "active", isHidden: false, ...over });
const bal = (variantId: number, locationId: number, onHand: number, reserved = 0) => ({ branchId: 1, productId: 7, variantId, locationId, onHand, reserved, available: onHand - reserved });

beforeEach(() => {
  for (const k of Object.keys(svc.fns)) delete svc.fns[k];
  svc.fns.productDetail = vi.fn().mockResolvedValue({
    id: 7, code: "CAT", name: "Cat Tembok", baseUom: "kg", purchaseUom: "kg", salesUom: "kaleng", salesFactor: 5,
    sellingPrice: 60000, minSellingPrice: 11000, tracked: true, status: "active",
    variants: [variant(1, { isHidden: true, isDefault: true, name: "Standar" }), variant(3, { sortOrder: 2 }), variant(2, { sortOrder: 1 })],
  });
  svc.fns.productBalances = vi.fn().mockResolvedValue([bal(2, 10, 30, 5), bal(3, 10, 10), bal(2, 11, 15), bal(3, 99, 50)]);
  svc.fns.locations = vi.fn().mockResolvedValue([
    { id: 10, code: "GA", name: "Gudang A", kind: "normal", status: "active" },
    { id: 11, code: "TK", name: "Toko", kind: "normal", status: "active" },
    { id: 99, code: "RSK", name: "Rusak", kind: "damaged", status: "active" },
  ]);
});
afterEach(() => cleanup());

describe("ProductInfoSheet — cek harga & stok tanpa keranjang", () => {
  it("harga, harga minimum per satuan jual, stok tersedia per lokasi & varian (tanpa lokasi rusak)", async () => {
    render(<ProductInfoSheet product={tile} onClose={vi.fn()} onAdd={vi.fn()} />);
    const stock = await screen.findByRole("region", { name: "Stok tersedia" });
    expect(screen.getByText(/^Rp\s?60\.000$/)).toBeInTheDocument();
    expect(screen.getByText(/55\.000 \/ kaleng/)).toBeInTheDocument(); // 11.000/kg × 5
    // (25 + 10 + 15) kg ÷ 5 = 10 kaleng; 50 kg di lokasi rusak tidak dihitung.
    expect(within(stock).getByText("10 kaleng")).toBeInTheDocument();
    expect(within(stock).getByText("Gudang A").nextSibling).toHaveTextContent("7 kaleng");
    expect(within(stock).getByText("Toko").nextSibling).toHaveTextContent("3 kaleng");
    expect(within(stock).queryByText("Rusak")).toBeNull();
    const vars = screen.getByRole("region", { name: "Varian" });
    expect(within(vars).getAllByRole("listitem").map((li) => li.textContent)).toEqual([
      expect.stringMatching(/Warna 2.*8 kaleng/),
      expect.stringMatching(/Warna 3.*2 kaleng/),
    ]);
    expect(within(vars).queryByText("Standar")).toBeNull();
    const called = Object.keys(svc.fns).sort();
    expect(called).toEqual(["locations", "productBalances", "productDetail"]);
  });

  it("Tambah ke keranjang memakai jalur tile; stok tak terbaca tetap menampilkan harga", async () => {
    svc.fns.productBalances = vi.fn().mockRejectedValue({ status: 403, message: "Akses ditolak" });
    const onAdd = vi.fn();
    render(<ProductInfoSheet product={tile} onClose={vi.fn()} onAdd={onAdd} />);
    expect(await screen.findByText("Stok tidak dapat dibaca untuk akun ini.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Tambah ke keranjang" }));
    expect(onAdd).toHaveBeenCalledWith(7);
  });
});
