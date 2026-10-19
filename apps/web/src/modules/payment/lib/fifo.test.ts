import { describe, expect, it } from "vitest";
import { cashChange, previewFifo } from "@/modules/payment/lib/fifo";
import {
  cancelGuardHint,
  paymentKind,
  paymentKindLabel,
} from "@/modules/payment/types";

const orders = [
  { orderType: "sales", orderId: 1, number: "SO-1", grandTotal: 100, paid: 0, outstanding: 100 },
  { orderType: "sales", orderId: 2, number: "SO-2", grandTotal: 50, paid: 50, outstanding: 0 },
  { orderType: "sales", orderId: 3, number: "SO-3", grandTotal: 200, paid: 0, outstanding: 200 },
];

describe("previewFifo", () => {
  it("allocates oldest first and skips settled bills", () => {
    const p = previewFifo(orders, 150);
    expect(p.rows.map((r) => [r.orderId, r.amount])).toEqual([
      [1, 100],
      [3, 50],
    ]);
    expect(p.unallocated).toBe(0);
  });

  it("reports the part no open bill can absorb", () => {
    expect(previewFifo(orders, 400).unallocated).toBe(100);
  });

  it("returns nothing for non-positive amounts", () => {
    expect(previewFifo(orders, 0).rows).toHaveLength(0);
    expect(previewFifo(orders, Number.NaN).rows).toHaveLength(0);
  });
});

describe("cashChange", () => {
  it("never goes negative", () => {
    expect(cashChange(120000, 100000)).toBe(20000);
    expect(cashChange(50000, 100000)).toBe(0);
  });
});

describe("payment kind and cancel hint", () => {
  it("classifies offset, refund and bill", () => {
    expect(paymentKind({ method: "offset", allocations: [] })).toBe("offset");
    expect(
      paymentKind({
        method: "cash",
        allocations: [{ orderType: "sales_return", orderId: 1, orderNumber: "", amount: 1 }],
      }),
    ).toBe("refund");
    expect(paymentKindLabel({ method: "cash", allocations: [] })).toBe("Pembayaran tagihan");
  });

  it("points a completed COD guard error to Retur", () => {
    const msg =
      "Order SO-1 (cod) sudah selesai; membatalkan pembayaran ini membuatnya belum lunas. Ubah termin";
    expect(cancelGuardHint(msg)).toContain("Retur");
    expect(cancelGuardHint("lain")).toBeNull();
  });
});
