import { describe, expect, it } from "vitest";
import { buildCategoryRows, parentIds } from "@/modules/products/lib/category-tree";
import type { ProductCategory } from "@/modules/products/types";

const cat = (id: number, code: string, parentId: number | null): ProductCategory => ({
  id,
  code,
  name: `Kategori ${code}`,
  parentId,
  sortOrder: 0,
  status: "active",
});

const ITEMS = [cat(3, "B-1", 2), cat(1, "A", null), cat(2, "B", null), cat(4, "A-1", 1), cat(5, "A-1-1", 4)];

describe("buildCategoryRows", () => {
  it("meratakan pohon: induk sebelum anak, berurut kode, dengan kedalaman", () => {
    const rows = buildCategoryRows(ITEMS);
    expect(rows.map((r) => [r.category.code, r.depth])).toEqual([
      ["A", 0],
      ["A-1", 1],
      ["A-1-1", 2],
      ["B", 0],
      ["B-1", 1],
    ]);
    expect(rows.find((r) => r.category.code === "A")?.hasChildren).toBe(true);
    expect(rows.find((r) => r.category.code === "A-1-1")?.hasChildren).toBe(false);
  });

  it("cabang yang dilipat tidak ikut diratakan tetapi induknya tetap ada", () => {
    const rows = buildCategoryRows(ITEMS, new Set([1]));
    expect(rows.map((r) => r.category.code)).toEqual(["A", "B", "B-1"]);
    expect(rows[0].hasChildren).toBe(true);
  });

  it("induk yang tidak ada di daftar menjadikan kategori itu akar (tidak hilang)", () => {
    const rows = buildCategoryRows([cat(9, "X", 777)]);
    expect(rows).toHaveLength(1);
    expect(rows[0].depth).toBe(0);
  });

  it("memotong siklus data supaya tidak berulang tanpa henti", () => {
    const rows = buildCategoryRows([cat(1, "A", 2), cat(2, "B", 1), cat(3, "C", 3)]);
    // A dan B saling menjadi induk: tidak ada akar sah, tetapi C (induk dirinya) menjadi akar.
    expect(rows.map((r) => r.category.code)).toEqual(["C"]);
  });

  it("saudara berurut menurut sortOrder dulu, baru kode (PR-06)", () => {
    const items = [
      { ...cat(1, "A", null), sortOrder: 5 },
      { ...cat(2, "B", null), sortOrder: 1 },
      { ...cat(3, "C", null), sortOrder: 1 },
      { ...cat(4, "A-1", 1), sortOrder: 2 },
      { ...cat(5, "A-2", 1), sortOrder: 0 },
    ];
    expect(buildCategoryRows(items).map((r) => r.category.code)).toEqual(["B", "C", "A", "A-2", "A-1"]);
  });

  it("daftar kosong menghasilkan baris kosong", () => {
    expect(buildCategoryRows([])).toEqual([]);
  });
});

describe("parentIds", () => {
  it("mengembalikan id yang punya anak", () => {
    expect(parentIds(ITEMS).sort()).toEqual([1, 2, 4]);
  });
});
