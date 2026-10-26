import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";
import type { PosCatalogApi } from "@/modules/pos/hooks/use-pos-catalog";
import type { PosProductListItem } from "@/modules/pos/types";

vi.mock("@/modules/pos/services/pos.service", () => ({ posService: {} }));
import { ProductGrid } from "@/modules/pos/components/ProductGrid";

// Temuan 5 lanjutan: stok tersedia di tile + info produk tanpa keranjang.
const item = (id: number, over: Partial<PosProductListItem> = {}): PosProductListItem => ({
  id, code: `P${id}`, name: `Produk ${id}`, categoryId: null, categoryName: null, sellingPrice: 1000, status: "active",
  tracked: true, stockAvailable: 12, stockUom: "pcs", belowMinStock: false, ...over,
});
const variant = (id: number, over: Record<string, unknown> = {}) => ({ id, code: `V${id}`, name: `V${id}`, barcode: "", isDefault: false, status: "active", isHidden: false, ...over });

function show(items: PosProductListItem[]) {
  const pos = {
    cart: [], holds: { lineState: {} }, addingId: null, barcode: "", setBarcode: vi.fn(), submitBarcode: vi.fn(), scanNotice: null,
    addProductById: vi.fn(),
  } as unknown as PosCartApi;
  const catalog = {
    items, loadingFirst: false, loadingMore: false, hasMore: false, error: null, generation: 1, page: 1, lastPageCount: items.length,
    categories: [], activeCategoryId: null, setActiveCategoryId: vi.fn(), loadMore: vi.fn(), retry: vi.fn(),
  } as unknown as PosCatalogApi;
  const onInfo = vi.fn();
  render(<ProductGrid pos={pos} catalog={catalog} onInfo={onInfo} />);
  return { pos, onInfo };
}
const tile = (name: string) => screen.getByRole("button", { name: `Tambah ${name}` });
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("tile katalog — stok tersedia", () => {
  it("normal, menipis, habis, desimal, tidak dilacak; jumlah varian terlihat", () => {
    show([
      item(1),
      item(2, { stockAvailable: 3, belowMinStock: true }),
      item(3, { stockAvailable: 0 }),
      item(4, { stockAvailable: 0.5, stockUom: "m" }),
      item(5, { tracked: false, stockAvailable: undefined }),
      item(6, { variants: [variant(60, { isHidden: true, isDefault: true }), variant(61), variant(62), variant(63, { status: "archived" })] }),
    ]);
    expect(within(tile("Produk 1")).getByText("Stok 12 pcs")).toBeInTheDocument();
    expect(within(tile("Produk 2")).getByTitle("Stok menipis: 3 pcs")).toHaveTextContent("Stok menipis: 3 pcs");
    expect(within(tile("Produk 3")).getByText("Stok habis")).toBeInTheDocument();
    expect(within(tile("Produk 4")).getByText("Stok 0,5 m")).toBeInTheDocument();
    expect(within(tile("Produk 5")).queryByText(/Stok|Menipis/)).toBeNull();
    expect(within(tile("Produk 6")).getByText("2 varian")).toBeInTheDocument();
    expect(within(tile("Produk 1")).queryByText(/varian/)).toBeNull();
  });

  it("ketuk = tambah ke keranjang; tombol info = info tanpa menambah", () => {
    const { pos, onInfo } = show([item(1)]);
    fireEvent.click(tile("Produk 1"));
    expect(pos.addProductById).toHaveBeenCalledWith(1);
    fireEvent.click(screen.getByRole("button", { name: "Info Produk 1" }));
    expect(onInfo).toHaveBeenCalledWith(1);
    expect(pos.addProductById).toHaveBeenCalledTimes(1);
  });

  it("tekan-lama / klik kanan membuka info; klik yang menyusul tidak menambah", () => {
    vi.useFakeTimers();
    const { pos, onInfo } = show([item(1)]);
    const t = tile("Produk 1");
    fireEvent.pointerDown(t, { pointerType: "touch", clientX: 10, clientY: 10 });
    act(() => vi.advanceTimersByTime(500));
    fireEvent.pointerUp(t);
    fireEvent.click(t);
    expect(onInfo).toHaveBeenCalledTimes(1);
    expect(pos.addProductById).not.toHaveBeenCalled();
    // Tekan singkat lalu geser (menggulir): bukan tekan-lama.
    fireEvent.pointerDown(t, { pointerType: "touch", clientX: 10, clientY: 10 });
    fireEvent.pointerMove(t, { clientX: 10, clientY: 40 });
    act(() => vi.advanceTimersByTime(600));
    expect(onInfo).toHaveBeenCalledTimes(1);
    fireEvent.contextMenu(t);
    expect(onInfo).toHaveBeenCalledTimes(2);
  });
});

describe("tile katalog — foto sebagai elemen utama (analysis/003)", () => {
  it("foto di atas selebar kartu; stok habis meredupkan foto; tanpa foto = inisial", () => {
    show([
      item(1, { primaryImageUrl: "/media/semen.webp" }),
      item(2, { primaryImageUrl: "/media/habis.webp", stockAvailable: 0 }),
      item(3, { name: "Pipa Rucika" }),
    ]);
    const first = tile("Produk 1");
    const img = first.querySelector("img");
    expect(img?.getAttribute("src")).toBe("/media/semen.webp");
    // Foto anak pertama tombol (di atas teks), kotak 4:3.
    expect(first.firstElementChild?.className).toContain("aspect-[4/3]");
    expect(tile("Produk 2").querySelector("img")?.className).toContain("grayscale");
    expect(within(tile("Produk 2")).getByText("Stok habis")).toBeInTheDocument();
    expect(tile("Pipa Rucika").firstElementChild?.textContent).toBe("PR");
  });
});

describe("tile katalog — pilihan tampilan Foto besar / Ringkas", () => {
  it("bawaan foto besar; Ringkas mengecilkan foto dan tersimpan per perangkat", () => {
    localStorage.removeItem("pos.catalogView");
    show([item(1, { primaryImageUrl: "/media/semen.webp" })]);
    expect(screen.getByRole("radio", { name: "Foto besar" })).toHaveAttribute("aria-checked", "true");
    expect(tile("Produk 1").firstElementChild?.className).toContain("aspect-[4/3]");
    fireEvent.click(screen.getByRole("radio", { name: "Ringkas" }));
    expect(screen.getByRole("radio", { name: "Ringkas" })).toHaveAttribute("aria-checked", "true");
    expect(tile("Produk 1").querySelector(".aspect-\\[4\\/3\\]")).toBeNull();
    expect(tile("Produk 1").querySelector("img")?.parentElement?.className).toContain("size-14");
    expect(localStorage.getItem("pos.catalogView")).toBe("compact");
    cleanup();
    show([item(1)]);
    expect(screen.getByRole("radio", { name: "Ringkas" })).toHaveAttribute("aria-checked", "true");
    localStorage.removeItem("pos.catalogView");
  });
});

