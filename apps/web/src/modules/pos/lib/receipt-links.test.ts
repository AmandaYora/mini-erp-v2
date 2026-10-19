import { describe, expect, it } from "vitest";
import { invoiceHref, paymentProofHref, posReceiptHref } from "@/modules/pos/lib/receipt-links";

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
