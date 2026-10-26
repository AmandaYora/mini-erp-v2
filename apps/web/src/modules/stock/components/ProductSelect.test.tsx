import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const searchOptions = vi.fn();
vi.mock("@/modules/products/services/products.service", async () => {
  const actual = await vi.importActual<typeof import("@/modules/products/services/products.service")>(
    "@/modules/products/services/products.service",
  );
  return { ...actual, productsService: { searchOptions: (...a: unknown[]) => searchOptions(...a) } };
});

import { ProductSelect } from "@/modules/stock/components/ProductSelect";
import { FakeIntersectionObserver, scrollPickerToEnd } from "@/test/fake-intersection-observer";

const row = (n: number) => ({ id: n, code: `STK-${String(n).padStart(3, "0")}`, name: `Barang ${n}`, variants: [] });

beforeEach(() => {
  searchOptions.mockReset();
  FakeIntersectionObserver.live = [];
  vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("ProductSelect stok berhalaman", () => {
  it("dulu berhenti di 20 baris; kini menggulir memuat halaman berikutnya sampai habis", async () => {
    const user = userEvent.setup();
    searchOptions.mockImplementation(async (p: { page?: number }) =>
      p.page === 1 ? Array.from({ length: 30 }, (_, i) => row(i + 1)) : p.page === 2 ? [row(31), row(32)] : [],
    );
    const onChange = vi.fn();
    render(<ProductSelect productId={null} onChange={onChange} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    expect(await screen.findByText("STK-030 — Barang 30")).toBeInTheDocument();
    scrollPickerToEnd();
    await user.click(await screen.findByText("STK-032 — Barang 32"));
    expect(onChange).toHaveBeenCalledWith(32);
    expect(searchOptions).toHaveBeenCalledTimes(2);
    expect(searchOptions.mock.calls[1][0]).toMatchObject({ page: 2, limit: 30 });
  });
});
