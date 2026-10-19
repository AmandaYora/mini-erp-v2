import { describe, expect, it } from "vitest";
import { changesLabel, countChanged } from "./dirty";

describe("countChanged", () => {
  it("0 bila semua nilai sama", () => {
    expect(countChanged({ a: "x", b: [1, 2] }, { a: "x", b: [1, 2] })).toBe(0);
  });

  it("membandingkan nilai bersarang, bukan referensi", () => {
    expect(countChanged({ a: "x", b: [1, 3] }, { a: "y", b: [1, 2] })).toBe(2);
  });

  it("kunci yang hanya ada di salah satu sisi dihitung berubah", () => {
    expect(countChanged({ a: 1 } as Record<string, number>, { a: 1, b: 2 } as Record<string, number>)).toBe(1);
  });
});

describe("changesLabel", () => {
  it("menyebut jumlah perubahan", () => {
    expect(changesLabel(3)).toBe("3 perubahan belum disimpan");
  });
});
