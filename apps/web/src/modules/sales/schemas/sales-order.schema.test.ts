import { describe, expect, it } from "vitest";
import { salesOrderSchema } from "@/modules/sales/schemas/sales-order.schema";
import { parseManualPrice, toLinePayload } from "@/modules/sales/lib/payload";

const line = {
  productId: 1,
  variantId: 2,
  uom: "pcs",
  qty: 3,
  discountPct: 0,
  discountNominal: 0,
  unitPrice: null,
  discountReason: "",
  note: "",
  taxable: true,
};

const base = {
  partyId: 5,
  orderDate: "2026-10-09",
  paymentTerms: "cod" as const,
  dueDate: "",
  notes: "",
  items: [line],
};

describe("salesOrderSchema", () => {
  it("accepts prepaid and cod", () => {
    expect(salesOrderSchema.safeParse({ ...base, paymentTerms: "prepaid" }).success).toBe(true);
    expect(salesOrderSchema.safeParse(base).success).toBe(true);
  });
  it("requires due date for net", () => {
    const r = salesOrderSchema.safeParse({ ...base, paymentTerms: "net" });
    expect(r.success).toBe(false);
    if (!r.success) expect(r.error.issues.some((i) => i.path[0] === "dueDate")).toBe(true);
  });
  it("rejects net without customer (walk-in)", () => {
    const r = salesOrderSchema.safeParse({
      ...base,
      partyId: null,
      paymentTerms: "net",
      dueDate: "2026-11-01",
    });
    expect(r.success).toBe(false);
    if (!r.success) expect(r.error.issues[0]?.message).toMatch(/tanpa customer/);
  });
  it("rejects duplicate product/variant/uom rows", () => {
    const r = salesOrderSchema.safeParse({ ...base, items: [line, { ...line }] });
    expect(r.success).toBe(false);
  });
  it("rejects negative manual price and overlong reason", () => {
    expect(
      salesOrderSchema.safeParse({ ...base, items: [{ ...line, unitPrice: -1 }] }).success,
    ).toBe(false);
    expect(
      salesOrderSchema.safeParse({ ...base, items: [{ ...line, discountReason: "x".repeat(256) }] })
        .success,
    ).toBe(false);
  });
  it("defaults new line fields", () => {
    const r = salesOrderSchema.safeParse({
      ...base,
      items: [{ productId: 1, variantId: 2, uom: "pcs", qty: 3 }],
    });
    expect(r.success).toBe(true);
    if (r.success) {
      expect(r.data.items[0].unitPrice).toBeNull();
      expect(r.data.items[0].taxable).toBe(true);
    }
  });
});

describe("toLinePayload / parseManualPrice", () => {
  it("omits unitPrice and empty texts when unset", () => {
    const p = toLinePayload(line);
    expect(p).not.toHaveProperty("unitPrice");
    expect(p).not.toHaveProperty("discountReason");
    expect(p.taxable).toBe(true);
  });
  it("includes manual price, reason, note and taxable=false", () => {
    const p = toLinePayload({
      ...line,
      unitPrice: 9000,
      discountReason: " promo ",
      note: "kirim pagi",
      taxable: false,
    });
    expect(p.unitPrice).toBe(9000);
    expect(p.discountReason).toBe("promo");
    expect(p.note).toBe("kirim pagi");
    expect(p.taxable).toBe(false);
  });
  it("parses manual price input", () => {
    expect(parseManualPrice("")).toBeNull();
    expect(parseManualPrice(" 12000 ")).toBe(12000);
    expect(Number.isNaN(parseManualPrice("abc"))).toBe(true);
  });
});
