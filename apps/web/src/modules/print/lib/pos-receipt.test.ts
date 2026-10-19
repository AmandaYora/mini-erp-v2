import { describe, expect, it } from "vitest";
import { receiptPaymentLines } from "@/modules/print/lib/pos-receipt";

describe("receiptPaymentLines", () => {
  it("tunai dengan uang diterima: tendered dan kembalian dari pembayaran tersimpan", () => {
    const l = receiptPaymentLines({ method: "cash", amount: 85000, amountTendered: 100000, change: 15000 }, 85000, "cod");
    expect(l).toEqual({ paid: 85000, tendered: 100000, change: 15000, outstanding: null, method: "Tunai" });
  });

  it("kembalian diturunkan bila server tidak mengirim", () => {
    const l = receiptPaymentLines({ method: "cash", amount: 85000, amountTendered: 100000, change: 0 }, 85000, "cod");
    expect(l.change).toBe(15000);
  });

  it("tunai tanpa uang diterima tercatat: tidak mengarang angka", () => {
    const l = receiptPaymentLines({ method: "cash", amount: 85000, amountTendered: 0, change: 0 }, 85000, "cod");
    expect(l.tendered).toBeNull();
    expect(l.change).toBeNull();
  });

  it("non-tunai tidak punya tendered", () => {
    const l = receiptPaymentLines({ method: "transfer", amount: 50000, amountTendered: 0, change: 0 }, 85000, "cod");
    expect(l.tendered).toBeNull();
    expect(l.outstanding).toBe(35000);
    expect(l.method).toBe("Transfer");
  });

  it("tanpa pembayaran: tempo dilabeli", () => {
    expect(receiptPaymentLines(null, 1000, "net").method).toBe("Tempo");
    expect(receiptPaymentLines(null, 1000, "cod").paid).toBeNull();
  });
});

import { receiptView } from "@/modules/print/lib/pos-receipt";

describe("receiptView — sembunyikan diskon (SL-05)", () => {
  const order = {
    subtotal: 90000,
    discountTotal: 10000,
    grandTotal: 90000,
    items: [{ qty: 2, unitPrice: 50000, lineTotal: 90000 }],
  };
  const pay = { method: "cash", amount: 90000, amountTendered: 100000, change: 10000 };

  it("default: angka netto server dan baris diskon dicetak", () => {
    const v = receiptView(order, pay, false);
    expect(v.discount).toBe(10000);
    expect(v.total).toBe(90000);
    expect(v.lineAmount(order.items[0])).toBe(90000);
  });

  it("disembunyikan: harga penuh, tanpa diskon, total dan bayar naik sehingga tak ada sisa palsu", () => {
    const v = receiptView(order, pay, true);
    expect(v.discount).toBeNull();
    expect(v.subtotal).toBe(100000);
    expect(v.total).toBe(100000);
    expect(v.lineAmount(order.items[0])).toBe(100000);
    expect(v.payment?.amount).toBe(100000);
    expect(receiptPaymentLines(v.payment, v.total, "cod").outstanding).toBeNull();
  });

  it("belum lunas: pembayaran tidak dinaikkan, sisa tetap riil", () => {
    const part = { method: "cash", amount: 40000, amountTendered: 40000, change: 0 };
    const v = receiptView(order, part, true);
    expect(v.payment?.amount).toBe(40000);
    expect(receiptPaymentLines(v.payment, v.total, "cod").outstanding).toBe(60000);
  });
});
