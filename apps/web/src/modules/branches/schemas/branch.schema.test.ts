import { describe, expect, it } from "vitest";
import { branchSchema } from "@/modules/branches/schemas/branch.schema";

const base = { code: "JKT01", name: "Jakarta" };

describe("branchSchema", () => {
  it("kode hingga 50 karakter diterima, 51 ditolak", () => {
    expect(branchSchema.safeParse({ ...base, code: "A".repeat(50) }).success).toBe(true);
    expect(branchSchema.safeParse({ ...base, code: "A".repeat(51) }).success).toBe(false);
  });

  it("kota maksimal 120 karakter", () => {
    expect(branchSchema.safeParse({ ...base, city: "k".repeat(120) }).success).toBe(true);
    expect(branchSchema.safeParse({ ...base, city: "k".repeat(121) }).success).toBe(false);
  });

  it("defaultLocationLabel opsional, maksimal 150 karakter", () => {
    const ok = branchSchema.safeParse(base);
    expect(ok.success && ok.data.defaultLocationLabel).toBe("");
    expect(branchSchema.safeParse({ ...base, defaultLocationLabel: "g".repeat(150) }).success).toBe(true);
    const bad = branchSchema.safeParse({ ...base, defaultLocationLabel: "g".repeat(151) });
    expect(bad.success).toBe(false);
  });
});
