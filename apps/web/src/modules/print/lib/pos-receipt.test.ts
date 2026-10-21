import { describe, expect, it } from "vitest";
import { receiptPayments, receiptSummary, type ReceiptPayment } from "@/modules/print/lib/pos-receipt";
import type { DocOrder } from "@/modules/print/lib/sales-document";

// Perilaku struk pos-thermal-receipt-print-page.tsx sistem lama.

function order(over: Partial<DocOrder & { orderDate: string; paymentTerms: string }> = {}) {
  return {
    grandTotal: 180000,
    discountTotal: 20000,
    taxTotal: 0,
    dueDate: "",
    orderDate: "2026-10-01",
    paymentTerms: "cod",
    paidAmount: 180000,
    outstandingAmount: 0,
    items: [{ qty: 2, unitPrice: 100000, discountPct: 0, discountNominal: 20000, lineTotal: 180000 }],
    ...over,
  };
}

function pay(over: Partial<ReceiptPayment> = {}): ReceiptPayment {
  return {
    id: 1,
    number: "PAY-1",
    method: "cash",
    paidAt: "2026-10-01T10:00:00+07:00",
    status: "active",
    amount: 180000,
    amountTendered: 200000,
    ...over,
  };
}

describe("riwayat pembayaran struk", () => {
  it("tanpa yang batal, urut waktu lalu id", () => {
    const out = receiptPayments([
      pay({ id: 3, paidAt: "2026-10-02T09:00:00+07:00" }),
      pay({ id: 2, status: "cancelled" }),
      pay({ id: 1 }),
    ]);
    expect(out.map((p) => p.id)).toEqual([1, 3]);
  });
});

describe("ringkasan struk", () => {
  it("lunas tunai: LUNAS, tunai & kembalian dari pembayaran terakhir", () => {
    const s = receiptSummary(order(), [pay()], false);
    expect(s).toMatchObject({
      statusLabel: "LUNAS",
      paymentNumber: "PAY-1",
      methods: "Tunai",
      subtotal: 200000,
      discount: 20000,
      total: 180000,
      paid: 180000,
      tendered: 200000,
      change: 20000,
      outstanding: null,
    });
  });

  it("piutang tanpa pembayaran: PIUTANG, No Bayar '-', metode Tempo", () => {
    const s = receiptSummary(order({ paymentTerms: "net", paidAmount: 0, outstandingAmount: 180000 }), [], false);
    expect(s).toMatchObject({ statusLabel: "PIUTANG", paymentNumber: "-", methods: "Tempo", paid: 0, outstanding: 180000 });
    expect(s.receiptDate).toBe("2026-10-01");
  });

  it("metode digabung; tunai diterima hanya dari pembayaran tunai terakhir", () => {
    const s = receiptSummary(
      order(),
      [pay({ id: 1, amount: 80000, amountTendered: 100000 }), pay({ id: 2, number: "PAY-2", method: "transfer", amount: 100000, amountTendered: 0, paidAt: "2026-10-01T11:00:00+07:00" })],
      false,
    );
    expect(s.methods).toBe("Tunai, Transfer Bank");
    expect(s.paymentNumber).toBe("PAY-2");
    expect(s.tendered).toBeNull();
  });

  it("sembunyikan diskon: tanpa baris diskon, total & dibayar harga penuh, kembalian dari total tercetak", () => {
    const s = receiptSummary(order(), [pay()], true);
    expect(s).toMatchObject({ discount: null, subtotal: 200000, total: 200000, paid: 200000, change: 0, outstanding: null });
  });
});
