import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ProductOptionRow } from "@/modules/products/types";

const searchOptions = vi.fn();
const detail = vi.fn();
vi.mock("@/modules/products/services/products.service", async () => {
  const actual = await vi.importActual<typeof import("@/modules/products/services/products.service")>(
    "@/modules/products/services/products.service",
  );
  return {
    ...actual,
    productsService: {
      searchOptions: (...a: unknown[]) => searchOptions(...a),
      detail: (...a: unknown[]) => detail(...a),
    },
  };
});

import { ProductSearchSelect } from "@/modules/products/components/ProductSearchSelect";

const row = (id: number, code: string, name: string): ProductOptionRow => ({
  id,
  code,
  name,
  sellingPrice: 1000,
  baseUom: "pcs",
  purchaseUom: "pcs",
  salesUom: "pcs",
  purchaseFactor: 1,
  salesFactor: 1,
  purchasePrice: 500,
  minSellingPrice: 0,
  type: "barang",
  tracked: true,
  status: "active",
  variants: [],
});

beforeEach(() => {
  searchOptions.mockReset();
  detail.mockReset();
});
afterEach(cleanup);

describe("ProductSearchSelect", () => {
  it("memulihkan pilihan tersimpan lewat search-options?productId — tanpa pencarian ulang dan tanpa detail penuh", async () => {
    searchOptions.mockResolvedValue([row(7, "BRG-007", "Teh Botol")]);
    const onResolve = vi.fn();
    render(<ProductSearchSelect value={7} onResolve={onResolve} />);
    expect(await screen.findByText("BRG-007 — Teh Botol")).toBeInTheDocument();
    expect(searchOptions).toHaveBeenCalledTimes(1);
    expect(searchOptions).toHaveBeenCalledWith({ productId: 7, limit: 1 });
    expect(detail).not.toHaveBeenCalled();
    expect(onResolve).toHaveBeenCalledWith(expect.objectContaining({ id: 7 }));
  });

  it("produk terpilih yang hilang dari server mengosongkan hasil resolve", async () => {
    searchOptions.mockResolvedValue([]);
    const onResolve = vi.fn();
    render(<ProductSearchSelect value={99} onResolve={onResolve} />);
    await waitFor(() => expect(onResolve).toHaveBeenCalledWith(undefined));
  });

  it("pencarian mengirim q dan meneruskan preferStock (parameter, bukan bawaan)", async () => {
    const user = userEvent.setup();
    searchOptions.mockResolvedValue([row(1, "AB", "Alpha")]);
    const { unmount } = render(<ProductSearchSelect value={null} />);
    await user.click(screen.getByRole("button", { expanded: false }));
    await waitFor(() => expect(searchOptions).toHaveBeenCalled());
    expect(searchOptions.mock.calls[0][0]).toMatchObject({ status: "active" });
    expect(searchOptions.mock.calls[0][0].preferStock).toBeFalsy();
    unmount();

    searchOptions.mockClear();
    render(<ProductSearchSelect value={null} preferStock />);
    await user.click(screen.getByRole("button", { expanded: false }));
    await waitFor(() => expect(searchOptions).toHaveBeenCalled());
    expect(searchOptions.mock.calls[0][0]).toMatchObject({ preferStock: true });
  });
});
