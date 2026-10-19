import { describe, expect, it } from "vitest";
import { parseQty, physicalDelta, resultingQty } from "@/modules/stock/lib/adjustment";

describe("adjustment", () => {
  it("parseQty", () => {
    expect(parseQty("")).toBeNull();
    expect(parseQty(" 5,5 ")).toBe(5.5);
    expect(parseQty("-1")).toBeNull();
    expect(parseQty("abc")).toBeNull();
    expect(parseQty("0")).toBe(0);
  });
  it("physicalDelta", () => {
    expect(physicalDelta(10, 7)).toBe(-3);
    expect(physicalDelta(null, 7)).toBeNull();
    expect(physicalDelta(0.1, 0.3)).toBe(0.2);
  });
  it("resultingQty", () => {
    expect(resultingQty(10, 4, "in")).toBe(14);
    expect(resultingQty(10, 4, "out")).toBe(6);
    expect(resultingQty(null, 4, "in")).toBeNull();
    expect(resultingQty(10, null, "out")).toBeNull();
  });
});
