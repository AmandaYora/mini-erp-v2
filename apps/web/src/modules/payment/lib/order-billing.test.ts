import { describe, expect, it } from "vitest";
import {
  billingBadge,
  billingView,
  canCreateDelivery,
  canCreateReceipt,
  canCreateReturn,
  canRecordPayment,
  canSwitchToNet,
  defaultPaymentAmount,
  printButtonLabel,
  salesDocumentTitle,
} from "@/modules/payment/lib/order-billing";
import type { OrderBillingSource } from "@/modules/payment/lib/order-billing";

const base: OrderBillingSource = {
  status: "confirmed",
  grandTotal: 100000,
  paymentTerms: "cod",
  channel: "regular",
  paidAmount: 0,
  outstandingAmount: 100000,
  paymentStatus: "unpaid",
  effectiveDueDate: "",
  isOverdue: false,
};

describe("billingView / billingBadge", () => {
  it("tanpa ringkasan server = tidak tersedia", () => {
    const v = billingView({ status: "confirmed", grandTotal: 5 });
    expect(v.available).toBe(false);
    expect(canRecordPayment({ status: "confirmed", grandTotal: 5 })).toBe(false);
  });

  it("badge: lunas, tempo lewat, sebagian, belum", () => {
    expect(billingBadge(billingView({ ...base, paymentStatus: "paid", outstandingAmount: 0, paidAmount: 100000 })).label).toBe("Lunas");
    expect(billingBadge(billingView({ ...base, isOverdue: true })).tone).toBe("danger");
    expect(billingBadge(billingView({ ...base, paymentStatus: "partial", paidAmount: 1 })).label).toBe("Bayar sebagian");
    expect(billingBadge(billingView(base)).label).toBe("Belum bayar");
  });

  it("jatuh tempo dipotong ke tanggal", () => {
    expect(billingView({ ...base, effectiveDueDate: "2026-10-30T00:00:00Z" }).dueDate).toBe("2026-10-30");
  });
});

describe("aksi order", () => {
  it("bayar hanya bila ada sisa dan order terkonfirmasi/selesai", () => {
    expect(canRecordPayment(base)).toBe(true);
    expect(canRecordPayment({ ...base, status: "draft" })).toBe(false);
    expect(canRecordPayment({ ...base, outstandingAmount: 0, paymentStatus: "paid" })).toBe(false);
  });

  it("surat jalan: bukan POS dan bukan prabayar belum lunas", () => {
    expect(canCreateDelivery(base)).toBe(true);
    expect(canCreateDelivery({ ...base, channel: "pos" })).toBe(false);
    expect(canCreateDelivery({ ...base, paymentTerms: "prepaid" })).toBe(false);
    expect(
      canCreateDelivery({ ...base, paymentTerms: "prepaid", outstandingAmount: 0, paymentStatus: "paid", paidAmount: 100000 }),
    ).toBe(true);
    expect(canCreateDelivery({ ...base, status: "completed" })).toBe(false);
  });

  it("penerimaan mengikuti aturan prabayar", () => {
    expect(canCreateReceipt({ ...base, paymentTerms: "net" })).toBe(true);
    expect(canCreateReceipt({ ...base, paymentTerms: "prepaid" })).toBe(false);
  });

  it("retur: selesai, atau terkonfirmasi dengan dokumen barang terposting", () => {
    expect(canCreateReturn({ ...base, status: "completed" }, false)).toBe(true);
    expect(canCreateReturn(base, false)).toBe(false);
    expect(canCreateReturn(base, true)).toBe(true);
    expect(canCreateReturn({ ...base, status: "draft" }, true)).toBe(false);
  });

  it("setujui kredit untuk COD atau prabayar terkonfirmasi (SL-10)", () => {
    expect(canSwitchToNet(base)).toBe(true);
    expect(canSwitchToNet({ ...base, paymentTerms: "prepaid" })).toBe(true);
    expect(canSwitchToNet({ ...base, paymentTerms: "net" })).toBe(false);
    expect(canSwitchToNet({ ...base, status: "draft" })).toBe(false);
  });
});

describe("judul cetak", () => {
  it("Tagihan / Nota (Sebagian) / Nota", () => {
    expect(salesDocumentTitle(billingView(base))).toBe("Tagihan");
    expect(salesDocumentTitle(billingView({ ...base, paidAmount: 10, outstandingAmount: 90, paymentStatus: "partial" }))).toBe("Nota (Sebagian)");
    expect(salesDocumentTitle(billingView({ ...base, paidAmount: 100000, outstandingAmount: 0, paymentStatus: "paid" }))).toBe("Nota");
    expect(salesDocumentTitle(billingView({ status: "confirmed", grandTotal: 1 }))).toBe("Nota");
  });

  it("label tombol dan nominal awal", () => {
    expect(printButtonLabel(billingView(base))).toBe("Cetak Tagihan");
    expect(printButtonLabel(billingView({ ...base, outstandingAmount: 0, paymentStatus: "paid" }))).toBe("Cetak Nota");
    expect(defaultPaymentAmount(billingView(base))).toBe(100000);
  });
});
