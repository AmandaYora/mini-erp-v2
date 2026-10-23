import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test, expect, devices, type Page } from "@playwright/test";
import { confirmSO, createDelivery, createSO, partyByCode, productByCode } from "../helpers/api";
import { loginAs } from "../helpers/ui";

// Temuan 3 (2026-10-23): surat jalan dipecah 8 barang per lembar; setiap lembar dokumen lengkap dengan
// "Halaman X dari Y". Diukur di PDF Chromium (ukuran @page dihormati): jumlah halaman PDF harus sama
// dengan jumlah lembar — lembar yang meluap menambah halaman dan menggagalkan tes (brief §8).

let deliveryId = 0;
let orderId = 0;

test.beforeAll(async () => {
  const p = await productByCode("P001");
  const c = await partyByCode("customers", "C001");
  const so = await createSO("admin", c.id, [{ p, qty: 1 }]);
  await confirmSO("admin", so.id);
  const dn = await createDelivery("admin", so.id, [{ p, qty: 1 }]);
  deliveryId = dn.id;
  orderId = so.id;
});

const LONG = "Pipa PVC Rucika AW 3 inch panjang 4 meter tebal standar SNI untuk instalasi air bersih bertekanan tinggi";

interface Scenario {
  rows: number;
  longNames?: number[]; // indeks baris bernama panjang (±110 karakter)
  notes?: string;
}

/** Ganti isi SJ dan order dengan `rows` barang sintetis (data asli dipakai sebagai cetakan). */
async function mockLines(page: Page, s: Scenario) {
  await page.route(`**/api/v1/deliveries/${deliveryId}`, async (route) => {
    const res = await route.fetch();
    const body = await res.json();
    const base = body.data.items[0];
    body.data.items = Array.from({ length: s.rows }, (_, i) => ({ ...base, id: 9000 + i, productId: 7000 + i, qty: i + 1 }));
    if (s.notes) body.data.notes = s.notes;
    await route.fulfill({ response: res, json: body });
  });
  await page.route(`**/api/v1/sales-orders/${orderId}`, async (route) => {
    const res = await route.fetch();
    const body = await res.json();
    const base = body.data.items[0];
    body.data.items = Array.from({ length: s.rows }, (_, i) => ({
      ...base,
      id: 8000 + i,
      productId: 7000 + i,
      productName: s.longNames?.includes(i) ? `${LONG} #${i + 1}` : `Barang uji ${i + 1}`,
    }));
    await route.fulfill({ response: res, json: body });
  });
}

function pdfPageCount(buf: Buffer): number {
  return (buf.toString("latin1").match(/\/Type\s*\/Page(?![a-zA-Z])/g) ?? []).length;
}

async function printToPdf(page: Page, paper: "continuous" | "a4", s: Scenario) {
  await mockLines(page, s);
  await page.goto(`/print/deliveries/${deliveryId}?paper=${paper}`);
  await expect(page.locator(".sj-sheet").first()).toBeVisible();
  const sheets = await page.locator(".sj-sheet").count();
  const pdf = await page.pdf({ preferCSSPageSize: true, printBackground: true });
  return { sheets, pages: pdfPageCount(pdf), pdf };
}

const PLAIN = [1, 8, 9, 20, 41];

for (const paper of ["continuous", "a4"] as const) {
  test.describe(`E25 SJ dipecah per 8 — kertas ${paper}`, () => {
    for (const rows of PLAIN) {
      test(`${rows} barang → ${Math.max(1, Math.ceil(rows / 8))} halaman PDF, tiap lembar lengkap`, async ({ page }) => {
        await loginAs(page, "admin");
        const { sheets, pages } = await printToPdf(page, paper, { rows });
        const want = Math.max(1, Math.ceil(rows / 8));
        expect(sheets).toBe(want);
        expect(pages).toBe(want);
        for (let i = 0; i < want; i++) {
          const sheet = page.locator(".sj-sheet").nth(i);
          await expect(sheet.getByText("SURAT JALAN")).toBeVisible();
          await expect(sheet.locator(".sj-sign-box")).toHaveCount(3);
          await expect(sheet.getByText(`Halaman ${i + 1} dari ${want}`)).toBeVisible();
          await expect(sheet.locator("tbody tr")).toHaveCount(8);
        }
      });
    }

    test("nama panjang + catatan SJ dua baris: lembar tetap tidak meluap", async ({ page }) => {
      await loginAs(page, "admin");
      const notes = "Barang diturunkan di gudang belakang, hubungi Pak Joko sebelum tiba. Semen jangan ditumpuk lebih dari 10 sak; pipa disusun di rak samping pagar.";
      const { sheets, pages } = await printToPdf(page, paper, { rows: 20, longNames: [0, 2, 5, 9, 12, 18], notes });
      expect(pages).toBe(sheets);
      expect(sheets).toBeGreaterThanOrEqual(3);
      await expect(page.getByText(`Halaman ${sheets} dari ${sheets}`)).toBeVisible();
    });
  });
}

// Profil Pixel 7 tanpa defaultBrowserType (tidak boleh di dalam describe; browser tetap Chromium).
const { defaultBrowserType: _browser, ...pixel7 } = devices["Pixel 7"];
void _browser;

test.describe("E25 SJ di perangkat Android (viewport tidak mengubah lembar)", () => {
  test.use(pixel7);
  test("20 barang kontinu = 3 halaman", async ({ page }) => {
    await loginAs(page, "admin");
    const { sheets, pages } = await printToPdf(page, "continuous", { rows: 20 });
    expect([sheets, pages]).toEqual([3, 3]);
  });
});

// Bukti visual (opsional, SJ_SHOTS=dir): PDF + lembar 1, tengah, terakhir untuk 20 barang di media cetak.
// Satu halaman browser per jenis kertas: pdf() berulang di halaman yang sama setelah emulateMedia tidak
// mencerminkan cetak sungguhan.
for (const paper of ["continuous", "a4"] as const) {
  test(`E25 screenshot lembar 20 barang (${paper}, media cetak)`, async ({ page }) => {
    const dir = process.env.SJ_SHOTS;
    test.skip(!dir, "SJ_SHOTS tidak diset");
    await loginAs(page, "admin");
    const { pdf, pages } = await printToPdf(page, paper, { rows: 20 });
    expect(pages).toBe(3);
    fs.writeFileSync(path.join(dir ?? os.tmpdir(), `sj-paging-${paper}-20.pdf`), pdf);
    await page.emulateMedia({ media: "print" });
    for (const i of [0, 1, 2]) {
      await page.locator(".sj-sheet").nth(i).screenshot({ path: path.join(dir ?? os.tmpdir(), `sj-paging-${paper}-20-hal${i + 1}.png`) });
    }
  });
}
