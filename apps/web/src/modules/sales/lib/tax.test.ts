import { describe, expect, it } from "vitest";
import { estimateTax, lineTax, normalizeTaxType } from "@/modules/sales/lib/tax";

describe("pajak order jual (cermin server)", () => {
  it("exclude: PPN ditambahkan di atas net", () => {
    expect(lineTax(100000, "exclude", 11)).toEqual({ base: 100000, tax: 11000, total: 111000 });
  });
  it("include: DPP dipisah dari harga, total tetap", () => {
    expect(lineTax(111000, "include", 11)).toEqual({ base: 100000, tax: 11000, total: 111000 });
  });
  it("none dan tarif 0 tidak menambah pajak", () => {
    expect(lineTax(5000, "none", 11)).toEqual({ base: 5000, tax: 0, total: 5000 });
    expect(lineTax(5000, "include", 0)).toEqual({ base: 5000, tax: 0, total: 5000 });
  });
  it("baris tidak kena pajak dikecualikan", () => {
    const t = estimateTax(
      [
        { net: 100000, taxable: true },
        { net: 50000, taxable: false },
      ],
      "exclude",
      11,
    );
    expect(t).toEqual({ base: 150000, tax: 11000, total: 161000 });
  });
  it("normalizeTaxType menolak nilai asing", () => {
    expect(normalizeTaxType("x")).toBe("none");
    expect(normalizeTaxType("include")).toBe("include");
    expect(normalizeTaxType(undefined)).toBe("none");
  });
});
