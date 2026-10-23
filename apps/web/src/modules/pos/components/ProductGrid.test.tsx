import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PageMeta } from "@/shared/services/http-client";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";
import type { PosProductListItem } from "@/modules/pos/types";

const svc = vi.hoisted(() => ({ products: vi.fn(), categories: vi.fn() }));
vi.mock("@/modules/pos/services/pos.service", () => ({ posService: svc }));
vi.mock("@/shared/stores/toast.store", () => ({ toast: { danger: vi.fn(), warning: vi.fn() } }));

import { ProductGrid } from "@/modules/pos/components/ProductGrid";
import { usePosCatalog } from "@/modules/pos/hooks/use-pos-catalog";

// jsdom tak punya IntersectionObserver. Stub: seperti browser, observe() memicu callback awal
// dengan posisi sentinel saat ini (`sentinelVisible`).
let sentinelVisible = false;
type ObserverRec = {
  root: Element | null;
  rootMargin: string;
  disconnected: boolean;
  fire: (visible: boolean) => void;
};
const observers: ObserverRec[] = [];
class IOStub {
  private rec: (typeof observers)[number];
  constructor(
    private cb: (entries: { isIntersecting: boolean }[]) => void,
    opts: IntersectionObserverInit,
  ) {
    this.rec = {
      root: (opts.root as Element) ?? null,
      rootMargin: opts.rootMargin ?? "",
      disconnected: false,
      fire: (visible) => this.cb([{ isIntersecting: visible }]),
    };
    observers.push(this.rec);
  }
  observe() {
    queueMicrotask(() => {
      if (!this.rec.disconnected) this.cb([{ isIntersecting: sentinelVisible }]);
    });
  }
  disconnect() {
    this.rec.disconnected = true;
  }
  unobserve() {}
}

const prod = (id: number): PosProductListItem => ({
  id,
  code: `P${id}`,
  name: `Produk ${id}`,
  categoryId: null,
  categoryName: null,
  sellingPrice: 1000,
  status: "active",
});
/** Katalog palsu: `perPage` produk per halaman, `pages` halaman. */
function catalogOf(pages: number, perPage = 2) {
  svc.products.mockImplementation(async (params: { page: number }) => ({
    items: Array.from({ length: perPage }, (_, i) => prod((params.page - 1) * perPage + i + 1)),
    meta: { page: params.page, limit: 48, total: pages * perPage, totalPages: pages } satisfies PageMeta,
  }));
}

function makePos(over: Partial<PosCartApi> = {}): PosCartApi {
  return {
    cart: [],
    holds: { lineState: {} },
    addingId: null,
    addProductById: vi.fn().mockResolvedValue({ kind: "added" }),
    barcode: "",
    setBarcode: vi.fn(),
    submitBarcode: vi.fn(),
    scanNotice: null,
    ...over,
  } as unknown as PosCartApi;
}

function Harness({ pos }: { pos: PosCartApi }) {
  const catalog = usePosCatalog();
  return <ProductGrid pos={pos} catalog={catalog} />;
}

const tiles = () => screen.queryAllByRole("button", { name: /^Tambah Produk/ });
const grid = () => document.querySelector("[aria-busy]");

beforeEach(() => {
  sentinelVisible = false;
  observers.length = 0;
  vi.stubGlobal("IntersectionObserver", IOStub);
  svc.categories.mockResolvedValue([]);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("ProductGrid — infinite scroll", () => {
  it("observer ber-root kontainer scroll; halaman pendek memuat beruntun lalu berhenti", async () => {
    sentinelVisible = true;
    catalogOf(3);
    render(<Harness pos={makePos()} />);
    expect(await screen.findByText("Semua produk ditampilkan (6)")).toBeInTheDocument();
    expect(tiles()).toHaveLength(6);
    expect(svc.products).toHaveBeenCalledTimes(3);
    expect(svc.products.mock.calls.map((c) => c[0].page)).toEqual([1, 2, 3]);
    expect(observers[0].root).toHaveClass("overflow-y-auto");
    expect(observers[0].rootMargin).toBe("0px 0px 600px 0px");
    expect(screen.getByText("Semua produk ditampilkan")).toHaveClass("sr-only");
    expect(screen.queryByRole("button", { name: "Muat lagi" })).toBeNull();
  });

  it("sentinel belum terlihat: hanya halaman 1; tombol Muat lagi memuat halaman 2 (aria-busy selama memuat)", async () => {
    catalogOf(3);
    render(<Harness pos={makePos()} />);
    const more = await screen.findByRole("button", { name: "Muat lagi" });
    expect(tiles()).toHaveLength(2);
    expect(grid()).toHaveAttribute("aria-busy", "false");

    let release!: () => void;
    svc.products.mockImplementationOnce(
      (params: { page: number }) =>
        new Promise((resolve) => {
          release = () =>
            resolve({ items: [prod(3), prod(4)], meta: { page: params.page, limit: 48, total: 6, totalPages: 3 } });
        }),
    );
    fireEvent.click(more);
    await waitFor(() => expect(grid()).toHaveAttribute("aria-busy", "true"));
    expect(screen.getByText("Memuat produk berikutnya…")).toBeInTheDocument();
    release();
    await waitFor(() => expect(tiles()).toHaveLength(4));
    expect(grid()).toHaveAttribute("aria-busy", "false");
    expect(screen.getByText("2 produk lagi dimuat")).toBeInTheDocument();
  });

  it("sentinel terlihat setelah scroll -> loadMore", async () => {
    catalogOf(2);
    render(<Harness pos={makePos()} />);
    await screen.findByRole("button", { name: "Muat lagi" });
    expect(svc.products).toHaveBeenCalledTimes(1);
    // Seperti browser: sentinel masuk area pratinjau -> callback observer aktif. Observer dipasang
    // ulang di effect setelah render "Muat lagi", jadi tunggu sampai tepat satu yang hidup
    // (dulu dibaca seketika -> flaky bila effect belum jalan).
    await waitFor(() => expect(observers.filter((o) => !o.disconnected)).toHaveLength(1));
    const live = observers.filter((o) => !o.disconnected);
    sentinelVisible = true;
    act(() => live[0].fire(true));
    await screen.findByText("Semua produk ditampilkan (4)");
    expect(svc.products.mock.calls.map((c) => c[0].page)).toEqual([1, 2]);
    expect(observers.every((o) => o.disconnected)).toBe(true);
  });

  it("galat halaman berikutnya: pesan + Coba lagi tanpa menghapus produk", async () => {
    catalogOf(2);
    render(<Harness pos={makePos()} />);
    const more = await screen.findByRole("button", { name: "Muat lagi" });
    svc.products.mockRejectedValueOnce({ status: 503, message: "Server sibuk" });
    fireEvent.click(more);
    expect(await screen.findByText("Server sibuk")).toBeInTheDocument();
    expect(screen.getByText("Produk berikutnya gagal dimuat")).toBeInTheDocument();
    expect(tiles()).toHaveLength(2);
    fireEvent.click(screen.getByRole("button", { name: "Coba lagi" }));
    await screen.findByText("Semua produk ditampilkan (4)");
    expect(svc.products.mock.calls.map((c) => c[0].page)).toEqual([1, 2, 2]);
  });

  it("klik tile -> addProductById(id); badge ×qty dari keranjang", async () => {
    catalogOf(1);
    const pos = makePos({
      cart: [{ productId: 2, variantId: 20, qty: 3 }] as unknown as PosCartApi["cart"],
    });
    render(<Harness pos={pos} />);
    const tile = await screen.findByRole("button", { name: "Tambah Produk 2" });
    expect(tile).toHaveTextContent("×3");
    expect(screen.getByRole("button", { name: "Tambah Produk 1" })).not.toHaveTextContent("×");
    fireEvent.click(tile);
    expect(pos.addProductById).toHaveBeenCalledWith(2);
  });
});
