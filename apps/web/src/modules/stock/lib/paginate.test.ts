import { describe, expect, it } from "vitest";
import { clampPage, paginate } from "@/modules/stock/lib/paginate";

describe("paginate", () => {
  const rows = Array.from({ length: 7 }, (_, i) => i + 1);
  it("memotong per halaman", () => {
    expect(paginate(rows, 1, 3)).toEqual([1, 2, 3]);
    expect(paginate(rows, 3, 3)).toEqual([7]);
  });
  it("menjepit halaman di luar jangkauan", () => {
    expect(paginate(rows, 9, 3)).toEqual([7]);
    expect(paginate(rows, 0, 3)).toEqual([1, 2, 3]);
    expect(clampPage(5, 0, 10)).toBe(1);
  });
});
