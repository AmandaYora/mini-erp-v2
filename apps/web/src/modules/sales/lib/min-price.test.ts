import { describe, expect, it } from "vitest";
import { belowMinPrice } from "@/modules/sales/lib/estimate";
import { splitItemErrors } from "@/modules/sales/lib/server-errors";

const base = { qty: 2, catalogPrice: 10000, discountPct: 0, discountNominal: 0, minPrice: 9000 };

describe("belowMinPrice", () => {
  it("harga katalog di atas minimum aman", () => {
    expect(belowMinPrice(base)).toBe(false);
  });
  it("harga manual di bawah minimum ditandai", () => {
    expect(belowMinPrice({ ...base, manualPrice: 8000 })).toBe(true);
    expect(belowMinPrice({ ...base, manualPrice: 9000 })).toBe(false);
  });
  it("diskon membuat net di bawah minimum", () => {
    expect(belowMinPrice({ ...base, discountNominal: 3000 })).toBe(true);
    expect(belowMinPrice({ ...base, discountPct: 20 })).toBe(true);
  });
  it("tanpa minimum atau harga member tidak dijaga", () => {
    expect(belowMinPrice({ ...base, minPrice: 0, manualPrice: 1 })).toBe(false);
    expect(belowMinPrice({ ...base, manualPrice: 1, memberPrice: true })).toBe(false);
  });
});

describe("splitItemErrors", () => {
  it("memetakan semua pesan per baris, tidak hanya yang pertama", () => {
    const r = splitItemErrors([
      { field: "items", message: "baris 1: harga Semen di bawah harga minimum Rp9000 per satuan dasar" },
      { field: "items", message: "baris 3: harga Paku di bawah harga minimum Rp500 per satuan dasar" },
      { field: "items", message: "wajib diisi" },
      { field: "partyId", message: "x" },
    ]);
    expect(Object.keys(r.byRow)).toEqual(["0", "2"]);
    expect(r.byRow[0]).toContain("Semen");
    expect(r.byRow[2]).toContain("Paku");
    expect(r.general).toEqual(["wajib diisi"]);
  });
  it("baris sama digabung", () => {
    const r = splitItemErrors([
      { field: "items", message: "baris 2: a" },
      { field: "items", message: "baris 2: b" },
    ]);
    expect(r.byRow[1]).toBe("a; b");
  });
});
