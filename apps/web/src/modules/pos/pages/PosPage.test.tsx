import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";

// Hanya perangkaian halaman yang diuji (temuan 5): logika keranjang/katalog dan isi panel ditiru.
const h = vi.hoisted(() => ({ wide: true, pos: {} as Record<string, unknown> }));
vi.mock("@/modules/pos/hooks/use-is-wide", () => ({
  POS_LANDSCAPE_QUERY: "(orientation: landscape)",
  useIsWide: () => h.wide,
  useMediaQuery: () => false,
}));
vi.mock("@/modules/pos/hooks/use-pos-cart", () => ({ usePosCart: () => h.pos }));
vi.mock("@/modules/pos/hooks/use-pos-catalog", () => ({
  usePosCatalog: () => ({ productQuery: "", setProductQuery: vi.fn() }),
}));
vi.mock("@/modules/pos/components/ProductGrid", () => ({ ProductGrid: () => <section aria-label="Produk" /> }));
vi.mock("@/modules/pos/components/CartPanel", () => ({
  CartPanel: ({ variant = "aside", onBeforeCheckout }: { variant?: string; onBeforeCheckout?: () => void }) => (
    <aside aria-label="Keranjang" data-variant={variant}>
      <button type="button" onClick={onBeforeCheckout}>
        Bayar
      </button>
    </aside>
  ),
}));
vi.mock("@/modules/pos/components/CheckoutSheet", () => ({ CheckoutSheet: () => null }));
vi.mock("@/modules/pos/components/VariantSheet", () => ({ VariantSheet: () => null }));
vi.mock("@/modules/pos/components/ScanDialog", () => ({
  ScanDialog: ({ open }: { open: boolean }) => (open ? <div role="dialog" aria-label="Pindai" /> : null),
}));

import PosPage from "@/modules/pos/pages/PosPage";

function makePos(over: Partial<PosCartApi> = {}) {
  h.pos = {
    totals: { itemCount: 3, total: 45000 },
    started: false,
    readiness: { canCheckout: true },
    openCheckout: vi.fn(),
    variantChoice: null,
    setVariantChoice: vi.fn(),
    addProductById: vi.fn(),
    scanCode: vi.fn(),
    ...over,
  };
}

const renderPage = () =>
  render(
    <MemoryRouter>
      <PosPage />
    </MemoryRouter>,
  );

beforeEach(() => makePos());
afterEach(() => {
  cleanup();
  h.wide = true;
});

describe("PosPage — tata letak per lebar layar", () => {
  it("≥ 1024: keranjang berdampingan, tanpa dok", () => {
    renderPage();
    expect(screen.getByRole("complementary", { name: "Keranjang" })).toHaveAttribute("data-variant", "aside");
    expect(screen.queryByRole("navigation", { name: "Aksi kasir" })).toBeNull();
    expect(screen.getByRole("button", { name: "Pindai dengan kamera" })).toBeInTheDocument();
  });

  it("< 1024: dok bawah; keranjang dibuka sebagai sheet dan ditutup sebelum checkout", () => {
    h.wide = false;
    renderPage();
    expect(screen.queryByRole("complementary", { name: "Keranjang" })).toBeNull();
    const dock = screen.getByRole("navigation", { name: "Aksi kasir" });
    fireEvent.click(within(dock).getByRole("button", { name: /^Buka keranjang, 3 barang/ }));
    const sheet = screen.getByRole("dialog", { name: "Keranjang · 3 barang" });
    expect(within(sheet).getByRole("complementary", { name: "Keranjang" })).toHaveAttribute("data-variant", "sheet");
    fireEvent.click(within(sheet).getByRole("button", { name: "Bayar" }));
    expect(screen.queryByRole("dialog", { name: "Keranjang · 3 barang" })).toBeNull();
  });

  it("< 1024: Pindai di dok membuka pemindai; checkout yang sudah dimulai dilanjutkan langsung", () => {
    h.wide = false;
    makePos({ started: true } as Partial<PosCartApi>);
    renderPage();
    const dock = screen.getByRole("navigation", { name: "Aksi kasir" });
    fireEvent.click(within(dock).getByRole("button", { name: /Pindai/ }));
    expect(screen.getByRole("dialog", { name: "Pindai" })).toBeInTheDocument();
    fireEvent.click(within(dock).getByRole("button", { name: /^Lanjutkan pembayaran/ }));
    expect(h.pos.openCheckout).toHaveBeenCalledTimes(1);
  });
});
