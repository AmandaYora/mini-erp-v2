import { describe, expect, it } from "vitest";
import { canCommit, groupIssuesBySheet, issueLocation, validateImportFile } from "@/modules/products/lib/import-view";
import type { ImportIssue, ImportPreview } from "@/modules/products/types";

const issue = (over: Partial<ImportIssue>): ImportIssue => ({
  severity: "error",
  sheet: "Daftar Produk",
  row: 2,
  column: "Kode Produk",
  message: "kode wajib diisi",
  ...over,
});

const preview = (over: Partial<ImportPreview> = {}): ImportPreview => ({
  okToCommit: true,
  summary: {
    categoriesTotal: 0,
    categoriesToCreate: 0,
    categoriesToUpdate: 0,
    productsTotal: 1,
    productsToCreate: 1,
    productsToUpdate: 0,
    variantsTotal: 0,
    attributesTotal: 0,
    errors: 0,
    warnings: 0,
  },
  issues: [],
  categories: [],
  products: [],
  validProducts: 1,
  validVariants: 0,
  errors: [],
  ...over,
});

describe("validateImportFile", () => {
  it("hanya .xlsx", () => {
    expect(validateImportFile("produk.xlsx")).toBeNull();
    expect(validateImportFile("PRODUK.XLSX")).toBeNull();
  });
  it(".xls ditolak dengan petunjuk simpan ulang", () => {
    expect(validateImportFile("lama.xls")).toMatch(/simpan ulang sebagai \.xlsx/i);
  });
  it("format lain ditolak", () => {
    expect(validateImportFile("data.csv")).toMatch(/\.xlsx/);
  });
});

describe("groupIssuesBySheet", () => {
  it("kelompok per sheet (urutan muncul), error dahulu lalu baris", () => {
    const groups = groupIssuesBySheet([
      issue({ sheet: "Daftar Produk", row: 9 }),
      issue({ sheet: "Kategori Produk", row: 3 }),
      issue({ sheet: "Daftar Produk", row: 4, severity: "warning" }),
      issue({ sheet: "Daftar Produk", row: 5 }),
    ]);
    expect(groups.map((g) => g.sheet)).toEqual(["Daftar Produk", "Kategori Produk"]);
    expect(groups[0].errors).toBe(2);
    expect(groups[0].warnings).toBe(1);
    expect(groups[0].issues.map((i) => i.row)).toEqual([5, 9, 4]);
  });

  it("tanpa temuan = tanpa kelompok", () => {
    expect(groupIssuesBySheet([])).toEqual([]);
  });
});

describe("issueLocation", () => {
  it("baris dan kolom", () => {
    expect(issueLocation({ row: 12, column: "Kode Produk" })).toBe("baris 12, kolom Kode Produk");
    expect(issueLocation({ row: 0, column: "" })).toBe("-");
  });
});

describe("gerbang commit", () => {
  it("siap bila okToCommit dan tanpa error", () => {
    expect(canCommit(preview())).toBe(true);
  });
  it("peringatan saja tidak menghalangi", () => {
    expect(
      canCommit(preview({ issues: [issue({ severity: "warning" })], summary: { ...preview().summary, warnings: 1 } })),
    ).toBe(true);
  });
  it("satu error menolak seluruh berkas", () => {
    expect(canCommit(preview({ okToCommit: false, issues: [issue({})], summary: { ...preview().summary, errors: 1 } }))).toBe(false);
    // Server mengaku siap tetapi ada error dalam issues: tetap diblokir.
    expect(canCommit(preview({ issues: [issue({})] }))).toBe(false);
  });
  it("tanpa pratinjau tak bisa commit", () => {
    expect(canCommit(null)).toBe(false);
  });
});

describe("splitPlannedRows", () => {
  it("baris dengan error di sheet-nya masuk bermasalah", async () => {
    const { splitPlannedRows } = await import("@/modules/products/lib/import-view");
    const p = preview({
      okToCommit: false,
      issues: [issue({ row: 3 }), issue({ row: 2, sheet: "Kategori Produk" })],
      categories: [{ row: 2, code: "K1", name: "Kat", action: "create" }],
      products: [
        { row: 2, code: "P1", name: "A", action: "create" },
        { row: 3, code: "P2", name: "B", action: "update" },
      ],
    });
    const { valid, problem } = splitPlannedRows(p);
    expect(valid.map((r) => r.code)).toEqual(["P1"]);
    expect(problem.map((r) => r.code)).toEqual(["K1", "P2"]);
  });
});
