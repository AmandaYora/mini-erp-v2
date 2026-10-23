import { describe, expect, it } from "vitest";
import { acceptQtyDraft, canDecrease, formatQty, normalizeQty, parseQtyInput } from "@/modules/pos/lib/qty";

describe("qty POS desimal (temuan 7)", () => {
  it("menerima koma dan titik, maksimal 4 desimal, > 0", () => {
    expect(parseQtyInput("0,8")).toBe(0.8);
    expect(parseQtyInput("0.8")).toBe(0.8);
    expect(parseQtyInput("12")).toBe(12);
    expect(parseQtyInput("1,2345")).toBe(1.2345);
    expect(parseQtyInput("1,23456")).toBeNull();
    expect(parseQtyInput("0")).toBeNull();
    expect(parseQtyInput("0,0000")).toBeNull();
    expect(parseQtyInput("-1")).toBeNull();
    expect(parseQtyInput("abc")).toBeNull();
    expect(parseQtyInput("")).toBeNull();
  });

  it("membuang galat floating point dan menolak ≤ 0", () => {
    expect(normalizeQty(0.8 + 1)).toBe(1.8);
    expect(normalizeQty(1.8 - 1)).toBe(0.8);
    expect(normalizeQty(0.1 + 0.2)).toBe(0.3);
    expect(normalizeQty(0)).toBeNull();
    expect(normalizeQty(0.00004)).toBeNull();
    expect(normalizeQty(Number.NaN)).toBeNull();
  });

  it("tombol kurang langkah 1 hanya bila hasilnya tetap > 0", () => {
    expect(canDecrease(2)).toBe(true);
    expect(canDecrease(1.5)).toBe(true);
    expect(canDecrease(1)).toBe(false);
    expect(canDecrease(0.8)).toBe(false);
  });

  it("format id-ID sampai 4 desimal; draf Numpad dibatasi 4 desimal", () => {
    expect(formatQty(0.8)).toBe("0,8");
    expect(formatQty(1.2345)).toBe("1,2345");
    expect(formatQty(1500)).toBe("1.500");
    expect(acceptQtyDraft("0.1234")).toBe(true);
    expect(acceptQtyDraft("0.12345")).toBe(false);
    expect(acceptQtyDraft("12")).toBe(true);
  });
});
