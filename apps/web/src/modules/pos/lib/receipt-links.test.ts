import { describe, expect, it } from "vitest";
import { hideDiscountOption, invoiceHref, paymentProofHref, posReceiptHref } from "@/modules/pos/lib/receipt-links";

describe("tautan cetak POS", () => {
  it("struk membawa paymentId dan hideDiscount sesuai pilihan checkout", () => {
    expect(posReceiptHref({ orderId: 7, hideDiscount: false })).toBe("/print/pos/7");
    expect(posReceiptHref({ orderId: 7, paymentId: 3, hideDiscount: false })).toBe("/print/pos/7?paymentId=3");
    expect(posReceiptHref({ orderId: 7, paymentId: 3, hideDiscount: true })).toBe("/print/pos/7?paymentId=3&hideDiscount=1");
    expect(posReceiptHref({ orderId: 7, hideDiscount: true })).toBe("/print/pos/7?hideDiscount=1");
  });
  it("nota ke /print/sales dan bukti bayar hanya bila ada pembayaran", () => {
    expect(invoiceHref({ orderId: 7, hideDiscount: true })).toBe("/print/sales/7?hideDiscount=1");
    expect(invoiceHref({ orderId: 7, hideDiscount: false })).toBe("/print/sales/7");
    expect(paymentProofHref({ orderId: 7, paymentId: 3, hideDiscount: false })).toBe("/print/payments/3");
    expect(paymentProofHref({ orderId: 7, paymentId: null, hideDiscount: false })).toBeNull();
  });
});

describe("opsi sembunyikan diskon di layar sukses POS (D12)", () => {
  const paid = { hasDiscount: true, payLater: false, paymentPending: false, change: 0 };

  it("tampil hanya bila ada diskon", () => {
    expect(hideDiscountOption({ ...paid, hasDiscount: false })).toEqual({ visible: false, allowed: false });
    expect(hideDiscountOption(paid)).toEqual({ visible: true, allowed: true });
  });

  it("nonaktif bila transaksi tidak lunas", () => {
    expect(hideDiscountOption({ ...paid, payLater: true }).allowed).toBe(false);
    expect(hideDiscountOption({ ...paid, paymentPending: true }).allowed).toBe(false);
    expect(hideDiscountOption({ ...paid, change: -5000 }).allowed).toBe(false);
    expect(hideDiscountOption({ ...paid, change: 15000 }).allowed).toBe(true);
  });
});
