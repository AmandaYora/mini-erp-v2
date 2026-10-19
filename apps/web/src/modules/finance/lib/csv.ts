export type CsvCell = string | number | null | undefined;

function escapeCell(v: CsvCell): string {
  if (v === null || v === undefined) return "";
  const s = String(v);
  return /[",\r\n;]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** Susun CSV (pemisah koma, baris CRLF) dari matriks sel. Angka ditulis mentah (tanpa format rupiah). */
export function buildCsv(rows: CsvCell[][]): string {
  return rows.map((r) => r.map(escapeCell).join(",")).join("\r\n");
}

/** Unduh teks CSV sebagai berkas (BOM agar Excel membaca UTF-8). */
export function downloadCsv(filename: string, rows: CsvCell[][]): void {
  const blob = new Blob(["﻿", buildCsv(rows)], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
