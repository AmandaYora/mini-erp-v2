import { describe, expect, it } from "vitest";
import { diffRecords, flatten, formatDiffValue, hasDetails } from "./diff";

describe("flatten", () => {
  it("meratakan objek bersarang dengan jalur bertitik dan memperlakukan array sebagai daun", () => {
    const m = flatten({ a: 1, b: { c: "x", d: [1, 2] } });
    expect(Object.fromEntries(m)).toEqual({ a: 1, "b.c": "x", "b.d": [1, 2] });
  });
});

describe("diffRecords", () => {
  it("hanya mengembalikan baris yang berbeda dengan jenis yang benar", () => {
    const rows = diffRecords(
      { name: "A", price: 100, old: true, nested: { x: 1 } },
      { name: "B", price: 100, fresh: 1, nested: { x: 2 } },
    );
    expect(rows).toEqual([
      { path: "fresh", before: undefined, after: 1, kind: "added" },
      { path: "name", before: "A", after: "B", kind: "changed" },
      { path: "nested.x", before: 1, after: 2, kind: "changed" },
      { path: "old", before: true, after: undefined, kind: "removed" },
    ]);
  });

  it("includeSame menyertakan baris tak berubah", () => {
    const rows = diffRecords({ a: 1, b: 2 }, { a: 1, b: 3 }, true);
    expect(rows.map((r) => [r.path, r.kind])).toEqual([
      ["a", "same"],
      ["b", "changed"],
    ]);
  });

  it("before kosong = semua added, after kosong = semua removed", () => {
    expect(diffRecords(null, { a: 1 }).map((r) => r.kind)).toEqual(["added"]);
    expect(diffRecords({ a: 1 }, null).map((r) => r.kind)).toEqual(["removed"]);
    expect(diffRecords(null, null)).toEqual([]);
  });

  it("membandingkan array berdasarkan isi", () => {
    expect(diffRecords({ t: [1, 2] }, { t: [1, 2] })).toEqual([]);
    expect(diffRecords({ t: [1, 2] }, { t: [1, 3] })).toHaveLength(1);
  });
});

describe("formatDiffValue / hasDetails", () => {
  it("memformat nilai untuk tampilan", () => {
    expect(formatDiffValue(null)).toBe("—");
    expect(formatDiffValue(undefined)).toBe("—");
    expect(formatDiffValue("")).toBe("(kosong)");
    expect(formatDiffValue("abc")).toBe("abc");
    expect(formatDiffValue(5)).toBe("5");
    expect(formatDiffValue({ a: 1 })).toBe('{"a":1}');
  });

  it("hasDetails true bila salah satu rincian ada", () => {
    expect(hasDetails({})).toBe(false);
    expect(hasDetails({ before: null, after: null })).toBe(false);
    expect(hasDetails({ after: { a: 1 } })).toBe(true);
    expect(hasDetails({ metadata: {} })).toBe(true);
  });
});
