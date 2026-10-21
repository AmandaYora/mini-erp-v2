import { describe, expect, it } from "vitest";
import {
  documentTotals,
  dueDateCell,
  effectiveHideDiscount,
  hasLineDiscount,
  hideDiscountAllowed,
  lineDiscountTotal,
  linePrintSubtotal,
  linePrintUnitPrice,
  lineSellingAmount,
  orderPrintDiscount,
  orderPrintSubtotal,
  readHideDiscount,
  salesDocumentTitle,
  withHideDiscountParam,
  type DocLine,
  type DocOrder,
} from "@/modules/print/lib/sales-document";

// Angka diharapkan dihitung dengan rumus sales-print-shared.tsx sistem lama.

const discounted: DocLine = { qty: 2, unitPrice: 100000, discountPct: 0, discountNominal: 20000, lineTotal: 180000 };
const plain: DocLine = { qty: 3, unitPrice: 10000, discountPct: 0, discountNominal: 0, lineTotal: 30000 };

function order(over: Partial<DocOrder> = {}): DocOrder {
  return {
    grandTotal: 210000,
    discountTotal: 20000,
    taxTotal: 0,
    dueDate: "2026-11-01",
    paymentTerms: "net",
    paidAmount: 50000,
    outstandingAmount: 160000,
    paymentStatus: "partial",
    items: [discounted, plain],
    ...over,
  };
}

describe("harga & diskon per baris (legacy)", () => {
  it("harga satuan tercetak = harga penuh; diskon nominal + persen", () => {
    expect(linePrintUnitPrice(discounted)).toBe(100000);
    expect(linePrintSubtotal(discounted)).toBe(200000);
    expect(lineDiscountTotal(discounted)).toBe(20000);
    const pct: DocLine = { qty: 4, unitPrice: 25000, discountPct: 10, discountNominal: 5000, lineTotal: 85000 };
    expect(lineDiscountTotal(pct)).toBe(15000);
    expect(lineDiscountTotal(plain)).toBe(0);
  });

  it("harga member tercetak dengan harga katalognya; selisihnya diskon", () => {
    const member: DocLine = {
      qty: 2,
      unitPrice: 90000,
      discountPct: 0,
      discountNominal: 0,
      lineTotal: 180000,
      pricingSource: "member_rule",
      basePrice: 100000,
    };
    expect(linePrintUnitPrice(member)).toBe(100000);
    expect(linePrintSubtotal(member)).toBe(200000);
    expect(lineDiscountTotal(member)).toBe(20000);
    // Katalog lebih murah dari harga member (aturan markup): tidak dianggap diskon.
    expect(linePrintUnitPrice({ ...member, basePrice: 80000 })).toBe(90000);
    // Harga manual tidak pernah memakai harga katalog.
    expect(linePrintUnitPrice({ ...member, pricingSource: "manual" })).toBe(90000);
  });

  it("Harga Jual = total baris; harga penuh bila diskon disembunyikan", () => {
    expect(lineSellingAmount(discounted, false)).toBe(180000);
    expect(lineSellingAmount(discounted, true)).toBe(200000);
    expect(hasLineDiscount([plain])).toBe(false);
    expect(hasLineDiscount([plain, discounted])).toBe(true);
  });

  it("diskon order tanpa diskon baris tetap tercetak di ringkasan", () => {
    expect(orderPrintDiscount(order({ items: [plain], discountTotal: 7000 }))).toBe(7000);
    expect(orderPrintSubtotal(order())).toBe(230000);
  });
});

describe("ringkasan & judul (legacy)", () => {
  it("bayar sebagian: Jumlah bruto, Diskon, Total, Sudah Dibayar, Sisa", () => {
    const t = documentTotals(order(), false);
    expect(t).toMatchObject({
      grossSubtotal: 230000,
      discount: 20000,
      tax: null,
      total: 210000,
      paid: 50000,
      outstanding: 160000,
    });
    expect(salesDocumentTitle(t)).toBe("Tagihan - Bayar Sebagian");
  });

  it("judul Tagihan Penjualan / Nota Penjualan Lunas", () => {
    expect(salesDocumentTitle(documentTotals(order({ paidAmount: 0, outstandingAmount: 210000 }), false))).toBe(
      "Tagihan Penjualan",
    );
    expect(salesDocumentTitle(documentTotals(order({ paidAmount: 210000, outstandingAmount: 0 }), false))).toBe(
      "Nota Penjualan Lunas",
    );
  });

  it("sembunyikan diskon pada nota lunas: total & dibayar naik ke harga penuh, sisa 0", () => {
    const t = documentTotals(order({ paidAmount: 210000, outstandingAmount: 0 }), true);
    expect(t).toMatchObject({ discount: null, total: 230000, paid: 230000, outstanding: 0 });
  });

  it("tagihan yang masih bersisa tidak pernah disembunyikan diskonnya (D12)", () => {
    // Bayar sebagian dan belum dibayar: permintaan diabaikan, angka tetap riil.
    expect(documentTotals(order(), true)).toEqual(documentTotals(order(), false));
    const unpaid = order({ paidAmount: 0, outstandingAmount: 210000 });
    expect(documentTotals(unpaid, true)).toMatchObject({ discount: 20000, total: 210000, outstanding: 210000 });
  });

  it("boleh menyembunyikan diskon hanya bila server menyatakan lunas", () => {
    expect(hideDiscountAllowed({ paidAmount: 210000, outstandingAmount: 0 })).toBe(true);
    expect(hideDiscountAllowed({ paidAmount: 50000, outstandingAmount: 160000 })).toBe(false);
    // Tanpa ringkasan bayar dari server lunas tidak bisa dipastikan.
    expect(hideDiscountAllowed({})).toBe(false);
    expect(effectiveHideDiscount(true, { paidAmount: 1, outstandingAmount: 0 })).toBe(true);
    expect(effectiveHideDiscount(false, { paidAmount: 1, outstandingAmount: 0 })).toBe(false);
  });

  it("retur yang mengurangi tagihan: sisa mengikuti server", () => {
    // Tagihan 210.000, retur 30.000, dibayar 50.000 -> sisa server 130.000.
    expect(documentTotals(order({ outstandingAmount: 130000 }), false).outstanding).toBe(130000);
  });

  it("baris Pajak hanya bila ada pajak", () => {
    expect(documentTotals(order({ taxTotal: 23100 }), false).tax).toBe(23100);
  });
});

describe("jatuh tempo & parameter", () => {
  it("Tgl. Jtp hanya selagi ada sisa tagihan", () => {
    expect(dueDateCell(order())).toBe("2026-11-01");
    expect(dueDateCell(order({ paidAmount: 210000, outstandingAmount: 0 }))).toBeNull();
    expect(dueDateCell(order({ dueDate: "", effectiveDueDate: "2026-10-30" }))).toBe("2026-10-30");
    expect(dueDateCell(order({ dueDate: "" }))).toBeNull();
  });

  it("parameter hideDiscount", () => {
    const p = withHideDiscountParam(new URLSearchParams("paper=a4"), true);
    expect(readHideDiscount(p)).toBe(true);
    expect(readHideDiscount(withHideDiscountParam(p, false))).toBe(false);
  });
});

describe("draf, batal, dan retur (audit pra-migrasi)", () => {
  it("draf menagih total - dibayar (server mengirim sisa 0 untuk draf), tidak pernah Lunas", () => {
    const draft = order({ status: "draft", paidAmount: 0, outstandingAmount: 0 });
    const t = documentTotals(draft, true);
    expect(t.outstanding).toBe(210000);
    expect(salesDocumentTitle(t, "draft")).toBe("Tagihan Penjualan");
    expect(hideDiscountAllowed(draft)).toBe(false);
  });

  it("order batal bercap batal dan tidak boleh disembunyikan diskonnya", () => {
    const cancelled = order({ status: "cancelled", paidAmount: 0, outstandingAmount: 0 });
    expect(salesDocumentTitle(documentTotals(cancelled, false), "cancelled")).toBe("Nota Dibatalkan");
    expect(hideDiscountAllowed(cancelled)).toBe(false);
  });

  it("setelah retur, total tercetak = total bersih (legacy summary.totalAmount)", () => {
    // 1.175.000 lunas, retur 125.000 dikembalikan: bersih 1.050.000, dibayar 1.050.000.
    const o = order({ grandTotal: 1175000, netTotal: 1050000, paidAmount: 1050000, outstandingAmount: 0, discountTotal: 0, items: [plain] });
    const t = documentTotals(o, false);
    expect(t.total).toBe(1050000);
    expect(t.outstanding).toBe(0);
    expect(salesDocumentTitle(t)).toBe("Nota Penjualan Lunas");
  });
});
