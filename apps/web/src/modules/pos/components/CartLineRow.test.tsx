import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { PosCartApi } from "@/modules/pos/hooks/use-pos-cart";
import type { CartLine } from "@/modules/pos/types";
import { CartLineRow } from "@/modules/pos/components/CartLineRow";

// Temuan 7: qty desimal (mis. 0,8 m kabel) lewat Numpad atau keyboard fisik.
const line = (qty: number): CartLine => ({
  key: 1, productId: 7, productCode: "KBL", productName: "Kabel", variantId: 1, variantCode: "KBL", variantName: "Standar",
  uom: "m", qty, unitPrice: 12345, minSellingPrice: 0, discountPct: 0, discountNominal: 0,
});
function makePos(): PosCartApi {
  return {
    started: false,
    canDiscount: false,
    holds: { lineState: {} },
    quoteLineErrors: new Map(),
    memberPriceOf: () => null,
    displayUnitPrice: (l: CartLine) => l.unitPrice,
    estimateOf: (l: CartLine) => Math.round(l.qty * l.unitPrice),
    discountCeilingOf: () => 0,
    setQty: vi.fn(),
    changeQty: vi.fn(),
    removeLine: vi.fn(),
    setLineDiscount: vi.fn(),
  } as unknown as PosCartApi;
}
const show = (qty: number, pos = makePos()) => {
  render(
    <ul>
      <CartLineRow line={line(qty)} pos={pos} />
    </ul>,
  );
  return pos;
};
afterEach(() => cleanup());

describe("CartLineRow — qty desimal", () => {
  it("Numpad punya tombol koma: 0 , 8 OK → setQty(0,8)", () => {
    const pos = show(1);
    fireEvent.click(screen.getByRole("button", { name: "Ubah qty Kabel" }));
    for (const name of ["0", "Koma", "8"]) fireEvent.click(screen.getByRole("button", { name }));
    expect(screen.getByText("0,8")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "OK" }));
    expect(pos.setQty).toHaveBeenCalledWith(1, 0.8);
  });

  it.each([",", "."])("keyboard fisik: 0%s8 Enter → setQty(0,8)", (sep) => {
    const pos = show(1);
    fireEvent.click(screen.getByRole("button", { name: "Ubah qty Kabel" }));
    const pad = screen.getByRole("group", { name: "Papan angka qty" });
    for (const key of ["0", sep, "8", "Enter"]) fireEvent.keyDown(pad, { key });
    expect(pos.setQty).toHaveBeenCalledWith(1, 0.8);
  });

  it("maksimal 4 desimal; 0 tidak dikirim", () => {
    const pos = show(1);
    fireEvent.click(screen.getByRole("button", { name: "Ubah qty Kabel" }));
    const pad = screen.getByRole("group", { name: "Papan angka qty" });
    for (const key of ["1", ",", "2", "3", "4", "5", "6", "Enter"]) fireEvent.keyDown(pad, { key });
    expect(pos.setQty).toHaveBeenCalledWith(1, 1.2345);
    fireEvent.click(screen.getByRole("button", { name: "Ubah qty Kabel" }));
    for (const key of ["0", "Enter"]) fireEvent.keyDown(screen.getByRole("group", { name: "Papan angka qty" }), { key });
    expect(pos.setQty).toHaveBeenCalledTimes(1);
  });

  it("qty tampil id-ID; tombol kurang nonaktif bila qty − 1 ≤ 0", () => {
    show(0.8);
    expect(screen.getByRole("button", { name: "Ubah qty Kabel" })).toHaveTextContent("0,8");
    expect(screen.getByRole("button", { name: "Kurangi qty" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Tambah qty" })).toBeEnabled();
    cleanup();
    show(1.5);
    expect(screen.getByRole("button", { name: "Kurangi qty" })).toBeEnabled();
  });
});
