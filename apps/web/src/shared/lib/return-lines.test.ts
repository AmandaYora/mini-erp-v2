import { describe, expect, it } from "vitest";
import {
  activeLines,
  clampQty,
  parseQty,
  validateReason,
  validateReturnDate,
} from "@/shared/lib/return-lines";

describe("clampQty", () => {
  it("clamps to the max and floors at zero", () => {
    expect(clampQty("12", 5)).toBe("5");
    expect(clampQty("-3", 5)).toBe("0");
    expect(clampQty("2.5", 5)).toBe("2.5");
  });
  it("leaves blank and non-numeric input alone", () => {
    expect(clampQty("", 5)).toBe("");
    expect(clampQty("abc", 5)).toBe("abc");
  });
});

describe("parseQty / activeLines", () => {
  it("returns 0 for invalid or non-positive", () => {
    expect(parseQty("")).toBe(0);
    expect(parseQty("x")).toBe(0);
    expect(parseQty("-1")).toBe(0);
    expect(parseQty("1.5")).toBe(1.5);
  });
  it("keeps only lines with qty > 0", () => {
    const rows = [
      { qty: "1", remaining: 3 },
      { qty: "0", remaining: 3 },
      { qty: "", remaining: 3 },
    ];
    expect(activeLines(rows)).toHaveLength(1);
  });
});

describe("validateReason", () => {
  it("requires non-blank", () => {
    expect(validateReason("   ")).not.toBeNull();
    expect(validateReason("rusak")).toBeNull();
  });
  it("caps length at 255", () => {
    expect(validateReason("a".repeat(255))).toBeNull();
    expect(validateReason("a".repeat(256))).not.toBeNull();
  });
});

describe("validateReturnDate", () => {
  it("accepts blank, today and past", () => {
    expect(validateReturnDate("", "2026-10-09")).toBeNull();
    expect(validateReturnDate("2026-10-09", "2026-10-09")).toBeNull();
    expect(validateReturnDate("2026-01-01", "2026-10-09")).toBeNull();
  });
  it("rejects future and malformed dates", () => {
    expect(validateReturnDate("2026-10-10", "2026-10-09")).not.toBeNull();
    expect(validateReturnDate("10/10/2026", "2026-10-09")).not.toBeNull();
  });
});

describe("validateReturnDate (RT-04 earliest)", () => {
  it("refuses a date before the first receive date", () => {
    expect(validateReturnDate("2026-09-09", "2026-10-18", "2026-09-10 00:00:00")).toMatch(/lebih awal/);
    expect(validateReturnDate("2026-09-10", "2026-10-18", "2026-09-10")).toBeNull();
    expect(validateReturnDate("2026-09-09", "2026-10-18")).toBeNull();
  });
});
