import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import path from "node:path";
import { ROOT } from "../helpers/env";
import { confirmDelivery, confirmSO, createDelivery, createSO, partyByCode, productByCode, today } from "../helpers/api";
import { collectErrors, expectNoRuntimeErrors, loginAs } from "../helpers/ui";

// U7-3 (docs/plan/redesain-ui/PLAN.md §9 Fase 7): setiap template back-office pada 390 / 768 / 1366 / 1920
// dan Mode Kasir POS pada 1024×768 / 1280×800 / 1340×800 landscape — tanpa gulir menyamping di tingkat
// HALAMAN (tabel boleh menggulir di dalam kartunya sendiri).

const BACK_OFFICE = [
  { w: 390, h: 844 },
  { w: 768, h: 1024 },
  { w: 1366, h: 800 },
  { w: 1920, h: 1080 },
];
const POS = [
  { w: 1024, h: 768 },
  { w: 1280, h: 800 },
  { w: 1340, h: 800 },
];

// Semua rute menu (dibangkitkan dari registry, sama dengan V01) — POS punya uji sendiri di bawah.
const web = path.join(ROOT, "apps/web/src/app/routes");
const PATHS = Object.fromEntries(
  [...readFileSync(path.join(web, "route-paths.ts"), "utf8").matchAll(/^\s+(\w+): "([^"]+)",/gm)].map((m) => [m[1], m[2]]),
);
const MENU_ROUTES = [
  ...new Set(
    [...readFileSync(path.join(web, "registry.tsx"), "utf8").matchAll(/label: "[^"]+", path: ROUTE_PATHS\.(\w+)/g)]
      .map((m) => PATHS[m[1]])
      .filter((p): p is string => Boolean(p) && p !== "/pos"),
  ),
];

/** Lebar konten halaman yang melampaui viewport (px); 0 = tidak ada gulir menyamping. */
async function pageOverflow(page: Page): Promise<number> {
  return page.evaluate(() => {
    const el = document.scrollingElement ?? document.documentElement;
    return Math.max(0, el.scrollWidth - window.innerWidth);
  });
}

async function settle(page: Page) {
  await page.waitForLoadState("networkidle");
  await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
}

test.describe("V02 responsif", () => {
  let soId = 0;

  test.beforeAll(async () => {
    const p = await productByCode("P010");
    const c = await partyByCode("customers", "C003");
    // Data nyata (penjualan terkirim) agar dasbor/daftar berisi tabel: tata letak kosong tidak melebar,
    // tabel berisi bisa (dasbor 390 pernah melebar 268px karena grid tanpa kolom dasar).
    const sold = await createSO("admin", c.id, [{ p, qty: 2 }], { paymentTerms: "net", dueDate: today(14) });
    await confirmSO("admin", sold.id);
    const dn = await createDelivery("gudang", sold.id, [{ p, qty: 2 }]);
    await confirmDelivery("gudang", dn.id);
    soId = (await createSO("admin", c.id, [{ p, qty: 1 }])).id;
  });

  test("semua rute menu tanpa gulir menyamping di 390 dan 768 (dengan data)", async ({ page }) => {
    test.setTimeout(400_000);
    await loginAs(page, "admin");
    const routes = [...MENU_ROUTES, "/sales-orders/new", "/purchase-orders/new", "/stock/transfers/new", `/sales-orders/${soId}`];
    const found: string[] = [];
    for (const w of [390, 768]) {
      await page.setViewportSize({ width: w, height: 900 });
      for (const r of routes) {
        await page.goto(r);
        await page.waitForLoadState("networkidle");
        const over = await pageOverflow(page);
        if (over > 1) found.push(`${w} ${r}: halaman melebar ${over}px`);
      }
    }
    expect(found, found.join("\n")).toEqual([]);
  });

  test("template back-office tanpa gulir menyamping di 390/768/1366/1920", async ({ page }) => {
    test.setTimeout(240_000);
    const errors = collectErrors(page);
    await loginAs(page, "admin");
    // Satu rute per template (PLAN §7): dasbor, daftar, master (sheet), detail dokumen, form berbaris, laporan, pengaturan.
    const routes = [
      "/dashboard",
      "/sales-orders",
      "/customers",
      `/sales-orders/${soId}`,
      "/sales-orders/new",
      "/finance/reports/profit-loss",
      "/company",
    ];
    const found: string[] = [];
    for (const vp of BACK_OFFICE) {
      await page.setViewportSize({ width: vp.w, height: vp.h });
      for (const r of routes) {
        await page.goto(r);
        await settle(page);
        const over = await pageOverflow(page);
        if (over > 1) found.push(`${vp.w}×${vp.h} ${r}: halaman melebar ${over}px`);
      }
    }
    expect(found, found.join("\n")).toEqual([]);
    expectNoRuntimeErrors(page, errors);
  });

  test("390: baris barang form order jual tampil sebagai kartu, tanpa tabel yang menggulir", async ({ page }) => {
    const p = await productByCode("P011");
    await loginAs(page, "admin");
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/sales-orders/new");
    await settle(page);
    const picker = page.getByLabel("Tambah produk");
    await picker.click();
    await page.keyboard.type(p.code);
    await page.getByRole("option", { name: new RegExp(p.code) }).first().click();
    const qty = page.getByLabel(new RegExp(`^Qty .*${p.name}`)).first();
    await expect(qty).toBeVisible();
    // Mode kartu: isian Qty tidak berada di dalam <table>, dan halaman tidak melebar.
    expect(await qty.evaluate((el) => el.closest("table") === null)).toBe(true);
    await expect(page.getByText("Baris 1")).toBeVisible();
    expect(await pageOverflow(page)).toBeLessThanOrEqual(1);
  });

  test("POS landscape 1024/1280/1340: tanpa gulir menyamping dan tombol Bayar terlihat", async ({ page }) => {
    await loginAs(page, "kasir");
    for (const vp of POS) {
      await page.setViewportSize({ width: vp.w, height: vp.h });
      await page.goto("/pos");
      await page.waitForLoadState("networkidle");
      const pay = page.getByRole("button", { name: /^Bayar ·/ });
      await expect(pay).toBeVisible();
      const box = await pay.boundingBox();
      expect(box, `${vp.w}×${vp.h}: tombol Bayar tanpa kotak`).not.toBeNull();
      expect(box!.y + box!.height, `${vp.w}×${vp.h}: tombol Bayar di luar layar`).toBeLessThanOrEqual(vp.h);
      expect(await pageOverflow(page), `${vp.w}×${vp.h}: halaman POS melebar`).toBeLessThanOrEqual(1);
    }
  });
});
