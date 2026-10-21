import { describe, expect, it } from "vitest";
import {
  correctionReasonSchema,
  issueMap,
  priceCorrectionSchema,
  purchaseHeaderSchema,
} from "@/modules/purchasing/schemas/purchase-correction.schema";

const header = {
  dueDate: "",
  paymentTermDays: 0,
  notes: "",
  supplierInvoiceNumber: "",
  supplierInvoiceDate: "",
  taxInvoiceNumber: "",
  taxInvoiceDate: "",
  assignedUserId: 0,
  reason: "",
};

describe("purchase-correction schemas", () => {
  it("alasan batal/tutup/koreksi wajib dan maksimal 500 karakter", () => {
    expect(correctionReasonSchema.safeParse("   ").success).toBe(false);
    expect(correctionReasonSchema.safeParse("x".repeat(501)).success).toBe(false);
    expect(correctionReasonSchema.safeParse(" salah input ").data).toBe("salah input");
  });

  it("info dokumen: tanggal YYYY-MM-DD, masa tempo 0..3650, alasan opsional", () => {
    expect(purchaseHeaderSchema.safeParse(header).success).toBe(true);
    expect(purchaseHeaderSchema.safeParse({ ...header, dueDate: "31/12/2026" }).success).toBe(false);
    expect(purchaseHeaderSchema.safeParse({ ...header, paymentTermDays: 4000 }).success).toBe(false);
  });

  it("koreksi harga: rupiah bulat tidak negatif, diskon 0..100, minimal satu baris", () => {
    const ok = { lineId: 1, unitPrice: 12000, discountPct: 0, discountNominal: 0 };
    expect(priceCorrectionSchema.safeParse({ lines: [ok], reason: "harga faktur" }).success).toBe(true);
    expect(priceCorrectionSchema.safeParse({ lines: [], reason: "harga faktur" }).success).toBe(false);
    const bad = priceCorrectionSchema.safeParse({
      lines: [{ ...ok, unitPrice: -1, discountPct: 120, discountNominal: 0.5 }],
      reason: "",
    });
    expect(bad.success).toBe(false);
    if (!bad.success) {
      const m = issueMap(bad.error);
      expect(Object.keys(m)).toEqual(
        expect.arrayContaining(["lines.0.unitPrice", "lines.0.discountPct", "lines.0.discountNominal", "reason"]),
      );
    }
  });
});
