import { describe, expect, it } from "vitest";
import {
  adjustmentSchema,
  adjustmentSchemaFor,
  damagedMoveSchema,
  isStrictAdjust,
  locationUpdateSchema,
  moveLocationSchema,
} from "@/modules/stock/schemas/stock.schema";

const base = {
  productId: 1,
  variantId: 2,
  locationId: 3,
  mode: "in",
  qty: "5",
  reason: "opname",
  approver: "",
  approverPassword: "",
};

describe("adjustmentSchema", () => {
  it("tanpa kebijakan ketat, penyetuju boleh kosong", () => {
    expect(adjustmentSchema.safeParse(base).success).toBe(true);
  });
  it("kebijakan ketat mewajibkan penyetuju + sandi", () => {
    const r = adjustmentSchemaFor(true).safeParse(base);
    expect(r.success).toBe(false);
    if (!r.success) {
      const paths = r.error.issues.map((i) => i.path[0]);
      expect(paths).toContain("approver");
      expect(paths).toContain("approverPassword");
    }
    expect(
      adjustmentSchemaFor(true).safeParse({
        ...base,
        approver: "spv",
        approverPassword: "x",
      }).success,
    ).toBe(true);
  });
  it("mode in/out butuh qty > 0, set boleh 0", () => {
    expect(adjustmentSchema.safeParse({ ...base, qty: "0" }).success).toBe(false);
    expect(
      adjustmentSchema.safeParse({ ...base, mode: "set", qty: "0" }).success,
    ).toBe(true);
  });
});

describe("locationUpdateSchema", () => {
  it("menerima forceTransfer opsional", () => {
    const r = locationUpdateSchema.safeParse({
      name: "Rak A",
      parentId: 4,
      forceTransfer: true,
    });
    expect(r.success).toBe(true);
  });
});

describe("strict adjustment policy", () => {
  it("is strict unless the setting is explicitly false", () => {
    expect(isStrictAdjust(undefined)).toBe(true);
    expect(isStrictAdjust("")).toBe(true);
    expect(isStrictAdjust("true")).toBe(true);
    expect(isStrictAdjust(" FALSE ")).toBe(false);
  });
});

describe("damaged move reason", () => {
  const ok = { productId: 1, variantId: 1, locationId: 1, qty: 2 };
  it("requires a reason", () => {
    expect(damagedMoveSchema.safeParse({ ...ok, reason: "  " }).success).toBe(false);
    expect(damagedMoveSchema.safeParse({ ...ok, reason: "bocor" }).success).toBe(true);
  });
});

describe("per-item destination", () => {
  it("accepts an optional toLocationId on items", () => {
    const r = moveLocationSchema.safeParse({
      fromLocationId: 1,
      toLocationId: 2,
      notes: "",
      items: [{ productId: 1, variantId: 1, qty: 3, toLocationId: 5 }],
    });
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.items[0].toLocationId).toBe(5);
  });
});

describe("transferSchemaFor (ST-02, BR-20)", () => {
  const base = {
    toBranchId: 2,
    fromLocationId: 1,
    toLocationId: 2,
    items: [{ productId: 1, variantId: 1, qty: 3 }],
  };
  it("antar cabang wajib sopir", async () => {
    const { transferSchemaFor } = await import("@/modules/stock/schemas/stock.schema");
    const r = transferSchemaFor(false).safeParse({ ...base, driverName: " " });
    expect(r.success).toBe(false);
    expect(transferSchemaFor(false).safeParse({ ...base, driverName: "Joko" }).success).toBe(true);
  });
  it("dalam cabang sopir opsional; tanggal harus YYYY-MM-DD", async () => {
    const { transferSchemaFor } = await import("@/modules/stock/schemas/stock.schema");
    expect(transferSchemaFor(true).safeParse({ ...base }).success).toBe(true);
    expect(transferSchemaFor(true).safeParse({ ...base, transferDate: "12/10/2026" }).success).toBe(false);
    expect(transferSchemaFor(true).safeParse({ ...base, transferDate: "2026-10-12" }).success).toBe(true);
  });
});
