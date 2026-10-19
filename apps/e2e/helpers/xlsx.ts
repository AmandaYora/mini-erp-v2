import ExcelJS from "exceljs";
import { session } from "./api";
import { BASE_URL } from "./env";

/** Bangun berkas xlsx (satu sheet atau lebih) dari baris-baris objek. */
export async function buildXlsx(sheets: { name: string; headers: string[]; rows: (string | number)[][] }[]): Promise<Buffer> {
  const wb = new ExcelJS.Workbook();
  for (const s of sheets) {
    const ws = wb.addWorksheet(s.name);
    ws.addRow(s.headers);
    for (const r of s.rows) ws.addRow(r);
  }
  return Buffer.from(await wb.xlsx.writeBuffer());
}

/** Unggah multipart (field "file") sebagai peran tertentu; mengembalikan envelope. */
export async function uploadXlsx(path: string, data: Buffer, role: "admin" | "gudang" = "admin") {
  const s = await session(role);
  const form = new FormData();
  form.append("file", new Blob([new Uint8Array(data)], { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" }), "impor.xlsx");
  const res = await fetch(`${BASE_URL}/api/v1${path}`, { method: "POST", headers: { Authorization: `Bearer ${s.access}` }, body: form });
  const j: any = await res.json();
  return { status: res.status, success: !!j.success, message: j.message as string, data: j.data, errors: j.errors };
}

export interface SheetData {
  name: string;
  /** Baris sebagai array nilai sel (indeks 0 = kolom A); sel kosong = "". */
  rows: (string | number | boolean | Date | null)[][];
}

/** Baca berkas xlsx (path dari unduhan Playwright atau Buffer) menjadi sheet -> baris. */
export async function readXlsx(src: string | Buffer): Promise<SheetData[]> {
  const wb = new ExcelJS.Workbook();
  if (typeof src === "string") await wb.xlsx.readFile(src);
  else await wb.xlsx.load(src as any);
  return wb.worksheets.map((ws) => {
    const rows: SheetData["rows"] = [];
    ws.eachRow({ includeEmpty: true }, (row) => {
      const vals = (row.values as any[]).slice(1).map((v) => {
        if (v === undefined || v === null) return "";
        if (typeof v === "object" && !(v instanceof Date)) return (v.result ?? v.text ?? v.richText?.map((t: any) => t.text).join("") ?? "") as any;
        return v;
      });
      rows.push(vals);
    });
    return { name: ws.name, rows };
  });
}

export const sheetByName = (sheets: SheetData[], name: string) => sheets.find((s) => s.name === name);
