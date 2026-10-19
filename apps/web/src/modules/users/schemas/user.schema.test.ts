import { describe, expect, it } from "vitest";
import { createUserSchema, editUserSchema } from "@/modules/users/schemas/user.schema";

const base = { username: "kasir01", fullName: "Kasir Satu" };

describe("user schema (phone & displayTitle)", () => {
  it("phone dan displayTitle opsional", () => {
    const res = editUserSchema.safeParse(base);
    expect(res.success && res.data.phone).toBe("");
    expect(res.success && res.data.displayTitle).toBe("");
  });

  it("phone maksimal 30, displayTitle maksimal 100", () => {
    expect(editUserSchema.safeParse({ ...base, phone: "1".repeat(30) }).success).toBe(true);
    expect(editUserSchema.safeParse({ ...base, phone: "1".repeat(31) }).success).toBe(false);
    expect(editUserSchema.safeParse({ ...base, displayTitle: "t".repeat(100) }).success).toBe(true);
    expect(editUserSchema.safeParse({ ...base, displayTitle: "t".repeat(101) }).success).toBe(false);
  });

  it("create tetap mewajibkan password 8 karakter", () => {
    expect(createUserSchema.safeParse({ ...base, password: "short" }).success).toBe(false);
    expect(createUserSchema.safeParse({ ...base, password: "longenough" }).success).toBe(true);
  });
});
