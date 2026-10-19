import { describe, expect, it } from "vitest";
import {
  compactParams,
  dateRangeError,
  fromCsv,
  positiveId,
  sortParams,
  toCsv,
  toggleCsv,
} from "./list-query";

describe("compactParams", () => {
  it("membuang nilai kosong dan men-trim string", () => {
    expect(
      compactParams({ a: " x ", b: "", c: "   ", d: undefined, e: null, f: 0, g: Number.NaN, h: false }),
    ).toEqual({ a: "x", f: 0, h: false });
  });
});

describe("positiveId", () => {
  it("hanya id positif yang lolos", () => {
    expect(positiveId(5)).toBe(5);
    expect(positiveId(0)).toBeUndefined();
    expect(positiveId(-1)).toBeUndefined();
    expect(positiveId(null)).toBeUndefined();
    expect(positiveId(Number.NaN)).toBeUndefined();
  });
});

describe("CSV helpers", () => {
  it("toCsv membuang duplikat/kosong; kosong -> undefined", () => {
    expect(toCsv(["a", "b", "a", " "])).toBe("a,b");
    expect(toCsv([])).toBeUndefined();
    expect(toCsv(undefined)).toBeUndefined();
  });
  it("fromCsv memecah, men-trim, dan membuang duplikat", () => {
    expect(fromCsv("a, b,,a")).toEqual(["a", "b"]);
    expect(fromCsv("")).toEqual([]);
  });
  it("toggleCsv menambah dan melepas nilai", () => {
    expect(toggleCsv("", "draft")).toBe("draft");
    expect(toggleCsv("draft", "confirmed")).toBe("draft,confirmed");
    expect(toggleCsv("draft,confirmed", "draft")).toBe("confirmed");
  });
});

describe("dateRangeError", () => {
  it("menolak rentang terbalik saja", () => {
    expect(dateRangeError("2026-10-10", "2026-10-01")).not.toBeNull();
    expect(dateRangeError("2026-10-01", "2026-10-10")).toBeNull();
    expect(dateRangeError("2026-10-01", "2026-10-01")).toBeNull();
    expect(dateRangeError("", "2026-10-01")).toBeNull();
  });
});

describe("sortParams", () => {
  it("hanya mengirim urutan bila ada pilihan eksplisit", () => {
    expect(sortParams("", "")).toEqual({});
    expect(sortParams("total", "asc")).toEqual({ sort: "total", dir: "asc" });
    expect(sortParams("total", "")).toEqual({ sort: "total", dir: undefined });
  });
});
