import { describe, expect, it } from "vitest";
import {
  receiptOrderRef,
  receiptPartySide,
  receiptSummary,
  type ReceiptPaymentLike,
} from "@/modules/print/lib/payment-receipt";

// Ringkasan bukti pembayaran payment-receipt-print-page.tsx sistem lama.

const ref = { orderType: "sales", orderId: 5, amount: 50000 };

function p(id: number, paidAt: string, amount: number, status = "active"): ReceiptPaymentLike {
  return { id, paidAt, status, allocations: [{ orderType: "sales", orderId: 5, amount }] };
}

describe("ringkasan bukti pembayaran", () => {
  it("terbayar = pembayaran order sampai dan termasuk yang ini; yang batal tak dihitung", () => {
    const first = p(1, "2026-10-01T09:00:00+07:00", 30000);
    const cancelled = p(2, "2026-10-01T10:00:00+07:00", 99000, "cancelled");
    const self = p(3, "2026-10-02T09:00:00+07:00", 50000);
    const later = p(4, "2026-10-03T09:00:00+07:00", 100000);
    expect(receiptSummary(self, ref, 180000, [later, cancelled, first, self])).toEqual({
      totalAmount: 180000,
      thisAmount: 50000,
      paidUntilThis: 80000,
      outstandingAfter: 100000,
    });
  });

  it("pembayaran ini ikut dihitung walau riwayat tak terbaca; pelunasan -> sisa 0", () => {
    const self = p(3, "2026-10-02T09:00:00+07:00", 180000);
    expect(receiptSummary(self, { ...ref, amount: 180000 }, 180000, []).outstandingAfter).toBe(0);
  });

  it("riwayat dari daftar tanpa alokasi: pembayaran ini tetap dihitung dari data detailnya", () => {
    // Bentuk respons daftar lama: allocations kosong; tanggal "YYYY-MM-DD hh:mm:ss".
    const self = { id: 3, paidAt: "2026-10-21 02:18:20", status: "active", allocations: [{ ...ref, amount: 125000 }] };
    const listed = { ...self, allocations: [] };
    const s = receiptSummary(self, { ...ref, amount: 125000 }, 125000, [listed]);
    expect(s.paidUntilThis).toBe(125000);
    expect(s.outstandingAfter).toBe(0);
  });

  it("waktu sama: urutan id", () => {
    const a = p(1, "2026-10-02T09:00:00+07:00", 10000);
    const b = p(2, "2026-10-02T09:00:00+07:00", 20000);
    expect(receiptSummary(a, ref, 100000, [a, b]).paidUntilThis).toBe(10000);
    expect(receiptSummary(b, ref, 100000, [a, b]).paidUntilThis).toBe(30000);
  });
});

describe("order & pihak", () => {
  it("order hanya bila alokasi tunggal", () => {
    expect(receiptOrderRef({ allocations: [ref] })).toEqual(ref);
    expect(receiptOrderRef({ allocations: [] })).toBeNull();
    expect(receiptOrderRef({ allocations: [ref, { ...ref, orderId: 6 }] })).toBeNull();
  });

  it("Customer untuk tagihan jual, Supplier untuk beli", () => {
    expect(receiptPartySide("sales", "", "in")).toBe("customer");
    expect(receiptPartySide("purchase", "", "out")).toBe("supplier");
    expect(receiptPartySide(undefined, "supplier", "in")).toBe("supplier");
    expect(receiptPartySide(undefined, "", "out")).toBe("supplier");
  });
});
