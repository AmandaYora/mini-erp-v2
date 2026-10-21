import { describe, expect, it } from "vitest";
import { cancelPaymentSchema, paymentCorrectSchema } from "@/modules/payment/schemas/payment.schema";
import { isCorrectable } from "@/modules/payment/types";

describe("cancelPaymentSchema", () => {
  it("alasan wajib dan dipangkas, maks 255", () => {
    expect(cancelPaymentSchema.safeParse({ reason: "   " }).success).toBe(false);
    const r = cancelPaymentSchema.safeParse({ reason: "  dobel input  " });
    expect(r.success && r.data.reason).toBe("dobel input");
    expect(cancelPaymentSchema.safeParse({ reason: "a".repeat(256) }).success).toBe(false);
  });
});

describe("paymentCorrectSchema", () => {
  const base = { method: "transfer", amount: "150000", reason: "salah pilih metode" };

  it("menerima koreksi lengkap dan mengubah nominal jadi angka", () => {
    const r = paymentCorrectSchema.safeParse({ ...base, paidAt: "2026-10-21", reference: "TRF-1" });
    expect(r.success).toBe(true);
    expect(r.success && r.data.amount).toBe(150000);
  });

  it("alasan minimal 5 karakter setelah dipangkas", () => {
    const r = paymentCorrectSchema.safeParse({ ...base, reason: "  abc  " });
    expect(r.success).toBe(false);
    expect(!r.success && r.error.flatten().fieldErrors.reason?.[0]).toMatch(/minimal 5/);
  });

  it("metode offset tidak bisa dipilih", () => {
    expect(paymentCorrectSchema.safeParse({ ...base, method: "offset" }).success).toBe(false);
  });

  it("uang diterima hanya tunai dan tidak kurang dari nominal", () => {
    expect(paymentCorrectSchema.safeParse({ ...base, amountTendered: "200000" }).success).toBe(false);
    expect(
      paymentCorrectSchema.safeParse({ ...base, method: "cash", amountTendered: "100000" }).success,
    ).toBe(false);
    expect(
      paymentCorrectSchema.safeParse({ ...base, method: "cash", amountTendered: "200000" }).success,
    ).toBe(true);
  });

  it("nominal harus bilangan bulat positif", () => {
    expect(paymentCorrectSchema.safeParse({ ...base, amount: "0" }).success).toBe(false);
    expect(paymentCorrectSchema.safeParse({ ...base, amount: "10.5" }).success).toBe(false);
  });
});

describe("isCorrectable", () => {
  const p = {
    status: "active",
    method: "cash",
    pairId: 0,
    creditSourceType: "",
    allocations: [{ orderType: "sales", orderId: 1, orderNumber: "SO-1", amount: 1000 }],
  };

  it("pembayaran aktif biasa bisa dikoreksi", () => {
    expect(isCorrectable(p)).toBe(true);
  });

  it("batal, offset, pakai saldo, berpasangan, atau tanpa alokasi tidak bisa", () => {
    expect(isCorrectable({ ...p, status: "cancelled" })).toBe(false);
    expect(isCorrectable({ ...p, method: "offset" })).toBe(false);
    expect(isCorrectable({ ...p, method: "advance" })).toBe(false);
    expect(isCorrectable({ ...p, pairId: 9 })).toBe(false);
    expect(isCorrectable({ ...p, creditSourceType: "sales_return" })).toBe(false);
    expect(isCorrectable({ ...p, allocations: [] })).toBe(false);
  });
});
