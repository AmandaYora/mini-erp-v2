import { describe, expect, it } from "vitest";
import {
  PARTY_ATTR_LIMITS,
  PRODUCT_ATTR_LIMITS,
  attrsToRows,
  rowsToAttrs,
  validateRows,
} from "@/shared/lib/key-value";

describe("attrsToRows / rowsToAttrs", () => {
  it("peta -> baris terurut kunci, dan kembali ke peta", () => {
    const rows = attrsToRows({ Bank: "BCA", NPWP: "01.234" });
    expect(rows).toEqual([
      { key: "Bank", value: "BCA" },
      { key: "NPWP", value: "01.234" },
    ]);
    expect(rowsToAttrs(rows)).toEqual({ Bank: "BCA", NPWP: "01.234" });
  });

  it("null/undefined = tanpa baris", () => {
    expect(attrsToRows(undefined)).toEqual([]);
    expect(attrsToRows(null)).toEqual([]);
  });

  it("baris kosong total dibuang dan nilai di-trim; kosong semua = {} (mengosongkan)", () => {
    expect(rowsToAttrs([{ key: " ", value: "" }])).toEqual({});
    expect(rowsToAttrs([{ key: " NPWP ", value: " 1 " }])).toEqual({ NPWP: "1" });
  });
});

describe("validateRows", () => {
  it("batas party: maksimal 20 baris", () => {
    const rows = Array.from({ length: 21 }, (_, i) => ({ key: `k${i}`, value: "v" }));
    expect(validateRows(rows, PARTY_ATTR_LIMITS).general).toMatch(/20/);
    expect(validateRows(rows.slice(0, 20), PARTY_ATTR_LIMITS).general).toBeNull();
  });

  it("batas produk: 30 baris", () => {
    const rows = Array.from({ length: 30 }, (_, i) => ({ key: `k${i}`, value: "v" }));
    expect(validateRows(rows, PRODUCT_ATTR_LIMITS).general).toBeNull();
  });

  it("kunci 1-60 dan isi 1-255", () => {
    expect(validateRows([{ key: "", value: "x" }], PARTY_ATTR_LIMITS).byRow[0]).toMatch(/Nama/);
    expect(validateRows([{ key: "a".repeat(61), value: "x" }], PARTY_ATTR_LIMITS).byRow[0]).toMatch(/Nama/);
    expect(validateRows([{ key: "a".repeat(60), value: "x" }], PARTY_ATTR_LIMITS).byRow).toEqual({});
    expect(validateRows([{ key: "a", value: "" }], PARTY_ATTR_LIMITS).byRow[0]).toMatch(/Isi/);
    expect(validateRows([{ key: "a", value: "x".repeat(256) }], PARTY_ATTR_LIMITS).byRow[0]).toMatch(/Isi/);
    expect(validateRows([{ key: "a", value: "x".repeat(255) }], PARTY_ATTR_LIMITS).byRow).toEqual({});
  });

  it("nama ganda ditolak pada baris kedua", () => {
    const r = validateRows(
      [
        { key: "NPWP", value: "1" },
        { key: "NPWP", value: "2" },
      ],
      PARTY_ATTR_LIMITS,
    );
    expect(r.byRow[1]).toMatch(/ganda/);
    expect(r.byRow[0]).toBeUndefined();
  });

  it("baris kosong total tidak dihitung sebagai galat", () => {
    expect(validateRows([{ key: "", value: "" }], PARTY_ATTR_LIMITS)).toEqual({ general: null, byRow: {} });
  });
});
