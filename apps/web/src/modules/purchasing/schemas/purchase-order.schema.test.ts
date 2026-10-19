import { describe, expect, it } from "vitest";
import { purchaseOrderSchema } from "@/modules/purchasing/schemas/purchase-order.schema";

const line = { productId: 1, variantId: 1, uom: "pcs", qty: 1, unitPrice: 1000, discountPct: 0, discountNominal: 0 };
const base = { partyId: 1, paymentTerms: "net", taxType: "none", taxRate: 0, lines: [line] };

describe("purchaseOrderSchema", () => {
  it("net butuh dueDate, atau masa tempo + tanggal faktur supplier", () => {
    expect(purchaseOrderSchema.safeParse(base).success).toBe(false);
    expect(purchaseOrderSchema.safeParse({ ...base, dueDate: "2026-11-01" }).success).toBe(true);
    expect(purchaseOrderSchema.safeParse({ ...base, paymentTermDays: 30 }).success).toBe(false);
    expect(
      purchaseOrderSchema.safeParse({ ...base, paymentTermDays: 30, supplierInvoiceDate: "2026-10-01" }).success,
    ).toBe(true);
  });

  it("prepaid dan cod tidak butuh dueDate", () => {
    expect(purchaseOrderSchema.safeParse({ ...base, paymentTerms: "prepaid" }).success).toBe(true);
    expect(purchaseOrderSchema.safeParse({ ...base, paymentTerms: "cod" }).success).toBe(true);
    expect(purchaseOrderSchema.safeParse({ ...base, paymentTerms: "xyz" }).success).toBe(false);
  });

  it("paymentTermDays 0..3650 bulat", () => {
    const ok = { ...base, dueDate: "2026-11-01" };
    expect(purchaseOrderSchema.safeParse({ ...ok, paymentTermDays: 3651 }).success).toBe(false);
    expect(purchaseOrderSchema.safeParse({ ...ok, paymentTermDays: -1 }).success).toBe(false);
    expect(purchaseOrderSchema.safeParse({ ...ok, paymentTermDays: 1.5 }).success).toBe(false);
  });

  it("baris: alasan diskon/catatan maks 255, taxable default true", () => {
    const r = purchaseOrderSchema.safeParse({ ...base, dueDate: "2026-11-01" });
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.lines[0].taxable).toBe(true);
    expect(
      purchaseOrderSchema.safeParse({
        ...base,
        dueDate: "2026-11-01",
        lines: [{ ...line, note: "x".repeat(256) }],
      }).success,
    ).toBe(false);
  });
});
