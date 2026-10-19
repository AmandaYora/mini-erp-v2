import { describe, expect, it } from "vitest";
import {
  discountCell,
  documentTitle,
  documentTotals,
  dueDateCell,
  lineAmount,
  readHideDiscount,
  withHideDiscountParam,
} from "@/modules/print/lib/sales-document";
import type { DocOrder } from "@/modules/print/lib/sales-document";

const money = (n: number) => `Rp${n}`;
const num = (n: number) => String(n);

const order: DocOrder = {
  status: "confirmed",
  grandTotal: 180000,
  paymentTerms: "net",
  paidAmount: 50000,
  outstandingAmount: 130000,
  paymentStatus: "partial",
  effectiveDueDate: "2026-11-01",
  isOverdue: false,
  dueDate: "2026-11-01",
  subtotal: 200000,
  discountTotal: 20000,
  taxTotal: 0,
  items: [
    { qty: 2, unitPrice: 100000, discountPct: 0, discountNominal: 20000, lineTotal: 180000 },
  ],
};

describe("judul dan total nota", () => {
  it("judul menurut status bayar", () => {
    expect(documentTitle(order)).toBe("Nota (Sebagian)");
    expect(documentTitle({ ...order, paidAmount: 0, outstandingAmount: 180000, paymentStatus: "unpaid" })).toBe("Tagihan");
    expect(documentTitle({ ...order, paidAmount: 180000, outstandingAmount: 0, paymentStatus: "paid" })).toBe("Nota");
  });

  it("total normal: diskon tampil, dibayar & sisa riil", () => {
    expect(documentTotals(order, false)).toEqual({
      subtotal: 200000,
      discount: 20000,
      tax: null,
      total: 180000,
      paid: 50000,
      outstanding: 130000,
    });
  });

  it("sembunyikan diskon: jumlah naik ke bruto, baris diskon hilang, total dan sisa tetap riil", () => {
    const t = documentTotals(order, true);
    expect(t.discount).toBeNull();
    expect(t.subtotal).toBe(200000);
    expect(t.total).toBe(180000);
    expect(t.outstanding).toBe(130000);
    expect(lineAmount(order.items[0], true)).toBe(200000);
    expect(lineAmount(order.items[0], false)).toBe(180000);
    expect(discountCell(order.items[0], true, money, num)).toBe("-");
    expect(discountCell(order.items[0], false, money, num)).toBe("Rp20000");
  });

  it("tanpa paid: baris Sudah Dibayar tidak dicetak", () => {
    expect(documentTotals({ ...order, paidAmount: 0, outstandingAmount: 180000, paymentStatus: "unpaid" }, false).paid).toBeNull();
  });

  it("jatuh tempo: '-' (null) bila lunas atau tanpa tempo", () => {
    expect(dueDateCell(order)).toBe("2026-11-01");
    expect(dueDateCell({ ...order, paidAmount: 180000, outstandingAmount: 0, paymentStatus: "paid" })).toBeNull();
    expect(dueDateCell({ ...order, paymentTerms: "cod", effectiveDueDate: "", dueDate: "" })).toBeNull();
  });
});

describe("param sembunyikan diskon", () => {
  it("baca dan tulis", () => {
    expect(readHideDiscount(new URLSearchParams("hideDiscount=1"))).toBe(true);
    expect(readHideDiscount(new URLSearchParams(""))).toBe(false);
    expect(withHideDiscountParam(new URLSearchParams("a=1"), true).toString()).toBe("a=1&hideDiscount=1");
    expect(withHideDiscountParam(new URLSearchParams("hideDiscount=1"), false).toString()).toBe("");
  });
});
