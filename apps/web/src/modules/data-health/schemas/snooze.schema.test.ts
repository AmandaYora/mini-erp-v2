import { describe, expect, it } from "vitest";
import { snoozeSchema } from "@/modules/data-health/schemas/snooze.schema";

describe("snoozeSchema", () => {
  it("durasi 1/3/7/14 dan batas alert kritis 7 hari", () => {
    expect(snoozeSchema(14).safeParse({ days: 14, reason: "Sedang dikerjakan" }).success).toBe(true);
    expect(snoozeSchema(7).safeParse({ days: 14, reason: "Sedang dikerjakan" }).success).toBe(false);
    expect(snoozeSchema(14).safeParse({ days: 5, reason: "Sedang dikerjakan" }).success).toBe(false);
  });

  it("alasan dipangkas, 5–255 karakter", () => {
    expect(snoozeSchema(14).safeParse({ days: 1, reason: "  abcd  " }).success).toBe(false);
    const ok = snoozeSchema(14).safeParse({ days: 1, reason: "  abcde  " });
    expect(ok.success && ok.data.reason).toBe("abcde");
    expect(snoozeSchema(14).safeParse({ days: 1, reason: "x".repeat(256) }).success).toBe(false);
  });
});
