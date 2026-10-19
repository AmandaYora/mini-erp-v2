import { describe, expect, it } from "vitest";
import { cancelExpenseSchema, expenseFormSchema, ignorePostingSchema, reopenPeriodSchema } from "@/modules/finance/schemas/finance.schema";

describe("cancelExpenseSchema", () => {
  it("requires a non-blank reason", () => {
    expect(cancelExpenseSchema.safeParse({ reason: "   " }).success).toBe(false);
    expect(cancelExpenseSchema.safeParse({ reason: "salah input" }).success).toBe(true);
  });

  it("trims and caps at 255 characters", () => {
    const r = cancelExpenseSchema.safeParse({ reason: "  x  " });
    expect(r.success && r.data.reason).toBe("x");
    expect(cancelExpenseSchema.safeParse({ reason: "a".repeat(256) }).success).toBe(false);
    expect(cancelExpenseSchema.safeParse({ reason: "a".repeat(255) }).success).toBe(true);
  });
});

describe("expenseFormSchema pihak lawan (FN-10)", () => {
  const base = { expenseAccountId: 1, payAccountId: 2, amount: "5000" };
  it("pihak opsional", () => {
    expect(expenseFormSchema.safeParse(base).success).toBe(true);
    const r = expenseFormSchema.safeParse({ ...base, partyId: 9 });
    expect(r.success && r.data.partyId).toBe(9);
  });
  it("menolak id pihak tidak valid", () => {
    expect(expenseFormSchema.safeParse({ ...base, partyId: 0 }).success).toBe(false);
  });
});

describe("alasan wajib (FN-01, FN-05)", () => {
  it("buka ulang periode dan abaikan dokumen menolak alasan kosong dan mengepas 255", () => {
    for (const schema of [reopenPeriodSchema, ignorePostingSchema]) {
      expect(schema.safeParse({ reason: "  " }).success).toBe(false);
      expect(schema.safeParse({ reason: "a".repeat(256) }).success).toBe(false);
      const r = schema.safeParse({ reason: " ok " });
      expect(r.success && r.data.reason).toBe("ok");
    }
  });
});
