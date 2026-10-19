import { describe, expect, it } from "vitest";
import { loginSchema } from "@/modules/auth/schemas/login.schema";

describe("loginSchema", () => {
  it("menerima username maupun email sebagai identifier", () => {
    expect(loginSchema.safeParse({ identifier: "kasir01", password: "x" }).success).toBe(true);
    const email = loginSchema.safeParse({ identifier: "  a@b.id ", password: "x" });
    expect(email.success && email.data.identifier).toBe("a@b.id");
  });

  it("identifier kosong ditolak dengan pesan", () => {
    const res = loginSchema.safeParse({ identifier: "  ", password: "x" });
    expect(res.success).toBe(false);
    if (!res.success) expect(res.error.issues[0].path[0]).toBe("identifier");
  });

  it("password wajib", () => {
    expect(loginSchema.safeParse({ identifier: "a", password: "" }).success).toBe(false);
  });
});
