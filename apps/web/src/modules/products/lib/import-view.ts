// Tampilan hasil impor produk (logika murni). Backend (import_plan.go):
// - gerbang seluruh-file: commit ditolak bila ada SATU error;
// - hanya .xlsx (".xls" ditolak backend dengan pesan "simpan ulang .xlsx");
// - issues[{severity, sheet, row, column, message}] dikelompokkan per sheet.

import type { ImportIssue, ImportPreview } from "@/modules/products/types";

export const IMPORT_ACCEPT = ".xlsx";

/** Pesan galat bila berkas bukan .xlsx; null bila boleh dipilih. */
export function validateImportFile(name: string): string | null {
  const lower = name.toLowerCase();
  if (lower.endsWith(".xlsx")) return null;
  if (lower.endsWith(".xls")) {
    return "Format .xls tidak didukung. Buka berkas di Excel lalu simpan ulang sebagai .xlsx.";
  }
  return "Berkas harus berformat Excel .xlsx.";
}

export interface IssueGroup {
  sheet: string;
  errors: number;
  warnings: number;
  issues: ImportIssue[];
}

/** Kelompokkan per sheet (urutan kemunculan pertama), baris terurut, error sebelum peringatan. */
export function groupIssuesBySheet(issues: readonly ImportIssue[]): IssueGroup[] {
  const order: string[] = [];
  const map = new Map<string, ImportIssue[]>();
  for (const it of issues) {
    const key = it.sheet || "-";
    if (!map.has(key)) {
      map.set(key, []);
      order.push(key);
    }
    map.get(key)?.push(it);
  }
  return order.map((sheet) => {
    const list = (map.get(sheet) ?? [])
      .slice()
      .sort((a, b) => {
        if (a.severity !== b.severity) return a.severity === "error" ? -1 : 1;
        return a.row - b.row;
      });
    return {
      sheet,
      issues: list,
      errors: list.filter((i) => i.severity === "error").length,
      warnings: list.filter((i) => i.severity === "warning").length,
    };
  });
}

/** Lokasi sel yang mudah dibaca: "baris 12, kolom Kode Produk". */
export function issueLocation(i: Pick<ImportIssue, "row" | "column">): string {
  const row = i.row > 0 ? `baris ${i.row}` : "";
  const col = i.column ? `kolom ${i.column}` : "";
  return [row, col].filter(Boolean).join(", ") || "-";
}

/** Commit hanya boleh bila server menyatakan siap DAN tak ada error (gerbang). */
export function canCommit(preview: ImportPreview | null): boolean {
  if (!preview) return false;
  const errorCount = preview.issues.filter((i) => i.severity === "error").length;
  return preview.okToCommit && preview.summary.errors === 0 && errorCount === 0;
}

export interface PlannedRowView {
  kind: "Kategori" | "Produk";
  row: number;
  code: string;
  name: string;
  action: "create" | "update";
}

/**
 * Pisahkan baris rencana (kategori + produk) menjadi valid dan bermasalah: baris yang
 * nomornya muncul sebagai error di sheet kategori / daftar produk dianggap bermasalah.
 */
export function splitPlannedRows(preview: ImportPreview): { valid: PlannedRowView[]; problem: PlannedRowView[] } {
  const errRows = (re: RegExp) =>
    new Set(preview.issues.filter((i) => i.severity === "error" && re.test(i.sheet)).map((i) => i.row));
  const catErr = errRows(/kategori/i);
  const prodErr = errRows(/daftar produk/i);
  const all: PlannedRowView[] = [
    ...preview.categories.map((r) => ({ kind: "Kategori" as const, ...r })),
    ...preview.products.map((r) => ({ kind: "Produk" as const, ...r })),
  ];
  const valid: PlannedRowView[] = [];
  const problem: PlannedRowView[] = [];
  for (const r of all) {
    if ((r.kind === "Kategori" ? catErr : prodErr).has(r.row)) problem.push(r);
    else valid.push(r);
  }
  return { valid, problem };
}
