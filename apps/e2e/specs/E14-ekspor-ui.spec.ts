import { test, expect, type Page } from "@playwright/test";
import { writeFileSync } from "node:fs";
import {
  call, ok, productByCode, partyByCode, createSO, confirmSO, createDelivery, confirmDelivery, createPO, confirmPO,
  receiveBody, locationByCode, pay, today, drainPosting,
} from "../helpers/api";
import { readXlsx, sheetByName, type SheetData } from "../helpers/xlsx";
import { loginAs } from "../helpers/ui";

// T-05: ekspor lewat UI (unduh sungguhan di browser), baca isi berkas, cek sheet/kolom selaras
// SL-04 (ekspor SO), PO-02 (ekspor PO) dan FN-04 (paket pajak). Dokumen uji dibuat lewat API lebih dulu
// (nilai diketahui persis), lalu baris dokumen itu dicari di berkas hasil unduhan.
const SO_SHEETS = ["Ringkasan", "Daftar Order", "Detail Item"];
const SO_COLS = [
  "Nomor Order", "Tanggal Order", "Customer", "Kanal", "Status Order", "Termin", "Jatuh Tempo", "Subtotal", "Diskon", "DPP", "Pajak",
  "Total Order", "Penyesuaian Retur", "Total Bersih", "Dibayar", "Sisa", "Status Bayar", "Status Finansial", "Metode Bayar", "Kasir",
  "Dibuat Oleh", "Penanggung Jawab", "Sopir", "No Plat", "No Faktur Pajak", "Tgl Faktur Pajak", "Persetujuan Kredit", "Catatan",
];
const SO_ITEM_COLS = ["Nomor Order", "Kode Produk", "Nama Produk", "Qty", "Satuan", "Harga Satuan", "DPP Baris", "Pajak Baris", "Total Baris"];
const PO_COLS = [
  "Nomor Order", "Tanggal Order", "Supplier", "Status Order", "Termin", "Jatuh Tempo", "Subtotal", "Diskon", "DPP", "Pajak", "Total Order",
  "Dibayar", "Sisa", "Status Bayar", "Status Finansial", "Metode Bayar", "Status Retur", "No Faktur Supplier", "Tgl Faktur Supplier",
  "No Faktur Pajak", "Tgl Faktur Pajak", "Dibuat Oleh", "Penanggung Jawab", "Catatan",
];
const PO_ITEM_COLS = ["Nomor Order", "Kode Produk", "Nama Produk", "Qty", "Satuan", "Harga Satuan", "Tarif Pajak %", "DPP Baris", "Pajak Baris", "Total Baris"];
const BALANCE_SHEET = "Posisi Harta Hutang"; // neraca
const ACTUAL_SHEETS = ["Ringkasan", "Untung Rugi", BALANCE_SHEET, "Cek Saldo Akun", "Rincian PPN", "PPN Masukan", "Rekonsiliasi Fiskal", "Buku Besar"];
const CAPPED_EXTRA = ["Peredaran Bruto", "Transaksi Dikecualikan"];

function dump(label: string, sheets: SheetData[]) {
  if (process.env.E2E_DUMP) writeFileSync(`${process.env.E2E_DUMP}-${label}.json`, JSON.stringify(sheets.map((s) => ({ n: s.name, r: s.rows.slice(0, 40) })), null, 1));
}

const col = (header: (string | number | boolean | Date | null)[], name: string) => {
  const i = header.indexOf(name);
  expect(i, `kolom "${name}" ada`).toBeGreaterThanOrEqual(0);
  return i;
};

/** Baris data (bukan header) dari sheet tabel yang kolom `key`-nya = value. */
function rowsWhere(sheet: SheetData, key: string, value: string) {
  const k = col(sheet.rows[0], key);
  return sheet.rows.slice(1).filter((r) => String(r[k]) === value);
}

async function download(page: Page, click: () => Promise<void>) {
  const [dl] = await Promise.all([page.waitForEvent("download", { timeout: 60_000 }), click()]);
  expect(dl.suggestedFilename()).toMatch(/\.xlsx$/);
  const p = await dl.path();
  expect(p).toBeTruthy();
  return { sheets: await readXlsx(p!), filename: dl.suggestedFilename() };
}

test.describe.serial("E14 ekspor lewat UI (SO, PO, paket pajak)", () => {
  let so: any;
  let po: any;
  let custName = "";
  let supName = "";
  const soPaid = 40_000;
  const poPaid = 30_000;
  let ppnOutBefore = 0;
  let ppnInBefore = 0;
  const ym = (() => {
    const d = new Date(new Date().toLocaleString("en-US", { timeZone: "Asia/Jakarta" }));
    return `year=${d.getFullYear()}&month=${d.getMonth() + 1}`;
  })();

  test("data uji: SO ber-PPN berfaktur + PO ber-PPN berfaktur, sebagian dibayar, terkirim/diterima, diposting", async () => {
    const p1 = await productByCode("P008");
    const p2 = await productByCode("P009");
    const cust = await partyByCode("customers", "C004");
    const sup = await partyByCode("suppliers", "S002");
    const loc = await locationByCode("HQ-GA");
    await drainPosting();
    const t0 = (await call("akuntan", "GET", `/finance/reports/tax-summary?${ym}`)).data;
    custName = cust.name;
    supName = sup.name;
    ppnOutBefore = t0.ppnOut;
    ppnInBefore = t0.ppnIn;

    so = await createSO("admin", cust.id, [{ p: p1, qty: 10, unitPrice: 4000 }, { p: p2, qty: 5, unitPrice: 6000 }], {
      paymentTerms: "net", dueDate: today(14), taxType: "exclude", taxRate: 11,
      taxInvoiceNumber: "010.000-26.00000014", taxInvoiceDate: today(),
    });
    await confirmSO("admin", so.id);
    await confirmDelivery("gudang", (await createDelivery("gudang", so.id, [{ p: p1, qty: 10 }, { p: p2, qty: 5 }])).id);
    await pay("akuntan", cust.id, soPaid, [{ orderType: "sales", orderId: so.id, amount: soPaid }], "transfer");

    po = await createPO("admin", sup.id, [{ p: p1, qty: 20, unitPrice: 2500 }, { p: p2, qty: 10, unitPrice: 4000 }], {
      taxType: "exclude", taxRate: 11, taxInvoiceNumber: "010.000-26.00000099", taxInvoiceDate: today(),
      supplierInvoiceNumber: "FS-E2E-0099", supplierInvoiceDate: today(),
    });
    await confirmPO("admin", po.id);
    await ok("gudang", "POST", "/goods-receipts", receiveBody(po.id, loc.id, [{ p: p1, qty: 20 }, { p: p2, qty: 10 }]));
    await pay("akuntan", sup.id, poPaid, [{ orderType: "purchase", orderId: po.id, amount: poPaid }], "transfer");
    await drainPosting();

    expect(so.grandTotal).toBe(Math.round(70_000 * 1.11)); // 10x4000 + 5x6000 = 70.000 + PPN 11%
    expect(po.grandTotal).toBe(Math.round(90_000 * 1.11)); // 20x2500 + 10x4000 = 90.000 + PPN 11%
  });

  test("SO: unduh XLSX dari halaman Order Jual; 3 sheet, 28 kolom, nilai baris cocok (SL-04)", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto("/sales-orders");
    await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
    const { sheets } = await download(page, async () => {
      await page.getByRole("button", { name: /^Ekspor/ }).click();
      await page.getByRole("menuitem", { name: "Ekspor XLSX" }).click();
    });
    dump("so", sheets);
    expect(sheets.map((s) => s.name)).toEqual(SO_SHEETS);

    const list = sheetByName(sheets, "Daftar Order")!;
    expect(list.rows[0]).toEqual(SO_COLS);
    expect(list.rows[0].length).toBeGreaterThanOrEqual(22);
    const [row] = rowsWhere(list, "Nomor Order", so.number);
    expect(row, `baris ${so.number} ada di Daftar Order`).toBeTruthy();
    const v = (name: string) => row[col(list.rows[0], name)];
    expect(Number(v("DPP"))).toBe(70_000);
    expect(Number(v("Pajak"))).toBe(so.grandTotal - 70_000);
    expect(Number(v("Total Order"))).toBe(so.grandTotal);
    expect(Number(v("Dibayar"))).toBe(soPaid);
    expect(Number(v("Sisa"))).toBe(so.grandTotal - soPaid);
    expect(String(v("No Faktur Pajak"))).toBe("010.000-26.00000014");
    expect(String(v("Status Bayar"))).not.toBe("");
    expect(String(v("Status Finansial"))).not.toBe("");
    expect(String(v("Metode Bayar")).toLowerCase()).toMatch(/transfer/);
    expect(rowsWhere(list, "Nomor Order", so.number)).toHaveLength(1);

    const items = sheetByName(sheets, "Detail Item")!;
    for (const c of SO_ITEM_COLS) expect(items.rows[0]).toContain(c);
    const lines = rowsWhere(items, "Nomor Order", so.number);
    expect(lines).toHaveLength(2);
    expect(lines.map((r) => String(r[col(items.rows[0], "Kode Produk")])).sort()).toEqual(["P008", "P009"]);
    expect(lines.reduce((a, r) => a + Number(r[col(items.rows[0], "DPP Baris")]), 0)).toBe(70_000);
    expect(lines.reduce((a, r) => a + Number(r[col(items.rows[0], "Total Baris")]), 0)).toBe(so.grandTotal);

    const cover = sheetByName(sheets, "Ringkasan")!;
    const text = cover.rows.flat().map(String).join("|").toLowerCase();
    for (const k of ["total", "dibayar", "sisa", "pajak"]) expect(text, `ringkasan memuat "${k}"`).toContain(k);
  });

  test("PO: unduh XLSX dari halaman Order Beli; 3 sheet, kolom Dibayar/Sisa/DPP/Status Finansial/Metode, nilai cocok (PO-02)", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto("/purchase-orders");
    await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
    const { sheets } = await download(page, () => page.getByRole("button", { name: "Ekspor XLSX" }).click());
    dump("po", sheets);
    expect(sheets.map((s) => s.name)).toEqual(SO_SHEETS);

    const list = sheetByName(sheets, "Daftar Order")!;
    expect(list.rows[0]).toEqual(PO_COLS);
    const [row] = rowsWhere(list, "Nomor Order", po.number);
    expect(row, `baris ${po.number} ada di Daftar Order`).toBeTruthy();
    const v = (name: string) => row[col(list.rows[0], name)];
    expect(Number(v("DPP"))).toBe(90_000);
    expect(Number(v("Pajak"))).toBe(po.grandTotal - 90_000);
    expect(Number(v("Total Order"))).toBe(po.grandTotal);
    expect(Number(v("Dibayar"))).toBe(poPaid);
    expect(Number(v("Sisa"))).toBe(po.grandTotal - poPaid);
    expect(String(v("No Faktur Pajak"))).toBe("010.000-26.00000099");
    expect(String(v("Metode Bayar")).toLowerCase()).toMatch(/transfer/);
    expect(String(v("Status Finansial"))).not.toBe("");

    const items = sheetByName(sheets, "Detail Item")!;
    for (const c of PO_ITEM_COLS) expect(items.rows[0]).toContain(c);
    const lines = rowsWhere(items, "Nomor Order", po.number);
    expect(lines).toHaveLength(2);
    expect(lines.reduce((a, r) => a + Number(r[col(items.rows[0], "DPP Baris")]), 0)).toBe(90_000);
    expect(lines.reduce((a, r) => a + Number(r[col(items.rows[0], "Total Baris")]), 0)).toBe(po.grandTotal);

    const cover = sheetByName(sheets, "Ringkasan")!;
    const text = cover.rows.flat().map(String).join("|").toLowerCase();
    for (const k of ["total", "dibayar", "sisa"]) expect(text, `ringkasan memuat "${k}"`).toContain(k);
  });

  test("paket pajak (UI Rincian PPN): data riil dan peredaran terbatas; sheet PPN Masukan/Buku Besar, total = layar (FN-04)", async ({ page }) => {
    await loginAs(page, "akuntan");
    await page.goto("/finance/reports/tax-detail");
    await expect(page.getByRole("heading", { level: 1, name: "Rincian PPN" })).toBeVisible();

    const sum = (await call("akuntan", "GET", `/finance/reports/tax-summary?${ym}`)).data;
    const ppnOutDelta = sum.ppnOut - ppnOutBefore;
    const ppnInDelta = sum.ppnIn - ppnInBefore;
    expect(ppnOutDelta).toBe(so.grandTotal - 70_000);
    expect(ppnInDelta).toBe(po.grandTotal - 90_000);

    const actual = await download(page, () => page.getByRole("button", { name: "Paket data riil" }).first().click());
    dump("tax-actual", actual.sheets);
    const names = actual.sheets.map((s) => s.name);
    for (const n of ACTUAL_SHEETS) expect(names, `paket riil memuat sheet ${n}`).toContain(n);
    checkPackage(actual.sheets, sum);

    const capped = await download(page, () => page.getByRole("button", { name: "Paket peredaran terbatas" }).first().click());
    dump("tax-capped", capped.sheets);
    const cnames = capped.sheets.map((s) => s.name);
    for (const n of CAPPED_EXTRA) expect(cnames, `paket terbatas memuat sheet ${n}`).toContain(n);
    // paket terbatas memakai sheet yang sama dengan data riil (commit 78a15f9) ditambah dua sheet plafon
    for (const n of ACTUAL_SHEETS) expect(cnames, `paket terbatas memuat sheet ${n}`).toContain(n);
    expect(capped.filename).not.toBe(actual.filename);
    checkPackage(capped.sheets, sum);
  });

  function checkPackage(sheets: SheetData[], sum: any) {
    // PPN Masukan: baris PO uji ada dan total sheet = ringkasan pajak bulan ini.
    const masukan = sheetByName(sheets, "PPN Masukan")!;
    const hdr = masukan.rows[0];
    const ppnCol = col(hdr, "PPN Masukan");
    const poRows = masukan.rows.slice(1).filter((r) => r.some((c) => String(c) === po.number));
    expect(poRows.length, "baris PO uji di sheet PPN Masukan").toBeGreaterThan(0);
    expect(poRows.reduce((a, r) => a + Number(r[ppnCol] || 0), 0)).toBe(po.grandTotal - 90_000);
    // pada sisi beli "faktur" = faktur pemasok; pemasok harus terisi namanya (N-51) dan tidak ditandai tanpa faktur
    expect(poRows[0][col(hdr, "Pemasok")], "nama pemasok di sheet PPN Masukan").toBe(supName);
    expect(poRows[0][col(hdr, "No. Faktur Pemasok")]).toBe("FS-E2E-0099");
    expect(poRows[0][col(hdr, "Tanpa faktur")]).toBe("Tidak");
    const isDataRow = (r: SheetData["rows"][number]) => /^\d{4}-\d{2}-\d{2}/.test(String(r[0]));
    const total = masukan.rows.slice(1).filter(isDataRow).reduce((a, r) => a + Number(r[ppnCol] || 0), 0);
    expect(total, "total PPN Masukan di sheet = ringkasan pajak layar").toBe(sum.ppnIn);

    const rincian = sheetByName(sheets, "Rincian PPN")!;
    const outCol = col(rincian.rows[0], "PPN Keluaran");
    const soRows = rincian.rows.slice(1).filter((r) => r.some((c) => String(c) === "010.000-26.00000014"));
    expect(soRows.length, "baris SO uji di sheet Rincian PPN").toBeGreaterThan(0);
    expect(soRows.reduce((a, r) => a + Number(r[outCol] || 0), 0)).toBe(so.grandTotal - 70_000);
    expect(soRows[0][col(rincian.rows[0], "Pelanggan")], "nama pelanggan di sheet Rincian PPN").toBe(custName);
    const outTotal = rincian.rows.slice(1).filter(isDataRow).reduce((a, r) => a + Number(r[outCol] || 0), 0);
    expect(outTotal, "total PPN Keluaran di sheet = ringkasan pajak layar").toBe(sum.ppnOut);


    const gl = sheetByName(sheets, "Buku Besar")!;
    expect(gl.rows.length).toBeGreaterThan(1);
    const cover = sheetByName(sheets, "Ringkasan")!;
    expect(cover.rows.flat().map(String).join("|")).toContain("PPN Masukan");
  }
});
