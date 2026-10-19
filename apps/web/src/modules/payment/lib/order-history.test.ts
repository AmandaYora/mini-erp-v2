import { describe, expect, it } from "vitest";
import {
  accumulateProgress,
  deliveryEvents,
  hasFulfilledDocument,
  paymentEvents,
  remainingQty,
  returnEvents,
  sortHistory,
} from "@/modules/payment/lib/order-history";

describe("riwayat order", () => {
  it("pembayaran: hanya yang beralokasi ke order ini, nominal = alokasi", () => {
    const ev = paymentEvents(
      [
        {
          id: 1, number: "PAY-1", paidAt: "2026-10-01", status: "active", method: "cash", amount: 150,
          allocations: [
            { orderType: "sales", orderId: 7, amount: 100 },
            { orderType: "sales", orderId: 8, amount: 50 },
          ],
        },
        { id: 2, number: "PAY-2", paidAt: "2026-10-02", status: "active", method: "cash", amount: 5, allocations: [{ orderType: "purchase", orderId: 7, amount: 5 }] },
      ],
      "sales",
      7,
    );
    expect(ev).toHaveLength(1);
    expect(ev[0].amount).toBe(100);
    expect(ev[0].href).toBe("/payments/1");
  });

  it("urutan: terbaru dulu, id besar dulu pada tanggal sama", () => {
    const events = [
      ...deliveryEvents([
        { id: 1, number: "SJ-1", deliveryDate: "2026-10-01", status: "confirmed", items: [] },
        { id: 2, number: "SJ-2", deliveryDate: "2026-10-01", status: "draft", items: [] },
      ]),
      ...returnEvents("sales", [{ id: 5, number: "RJ-5", returnDate: "2026-10-03", status: "draft", total: 10 }]),
    ];
    expect(sortHistory(events).map((e) => e.number)).toEqual(["RJ-5", "SJ-2", "SJ-1"]);
  });

  it("SJ pengganti dibedakan; retur beli menaut ke rute retur beli", () => {
    const ev = deliveryEvents([{ id: 3, number: "SJ-3", deliveryDate: "2026-10-01", status: "draft", documentKind: "replacement", items: [] }]);
    expect(ev[0].kind).toBe("replacement");
    expect(returnEvents("purchase", [{ id: 1, number: "RB-1", returnDate: "2026-10-01", status: "draft", total: 1 }])[0].href).toBe("/purchase-returns/1");
  });

  it("dokumen terpenuhi", () => {
    expect(hasFulfilledDocument(deliveryEvents([{ id: 1, number: "a", deliveryDate: "d", status: "draft", items: [] }]))).toBe(false);
    expect(hasFulfilledDocument(deliveryEvents([{ id: 1, number: "a", deliveryDate: "d", status: "confirmed", items: [] }]))).toBe(true);
  });
});

describe("progres per baris", () => {
  const items = (qty: number) => [{ productId: 1, variantId: 2, qty }];

  it("menjumlah confirmed dan draf terpisah, mengabaikan batal dan pengganti", () => {
    const m = accumulateProgress(
      [
        { status: "confirmed", items: items(3) },
        { status: "confirmed", items: items(2) },
        { status: "draft", items: items(4) },
        { status: "cancelled", items: items(9) },
        { status: "confirmed", documentKind: "replacement", items: items(7) },
      ],
      "confirmed",
      "draft",
    );
    expect(m.get("1/2")).toEqual({ done: 5, pending: 4 });
  });

  it("sisa tidak negatif dan toleran pecahan", () => {
    expect(remainingQty(10, { done: 5, pending: 4 })).toBe(1);
    expect(remainingQty(10, { done: 12, pending: 0 })).toBe(0);
    expect(remainingQty(10, undefined)).toBe(10);
    expect(remainingQty(1, { done: 0.7, pending: 0.1 })).toBeCloseTo(0.2, 6);
  });
});
