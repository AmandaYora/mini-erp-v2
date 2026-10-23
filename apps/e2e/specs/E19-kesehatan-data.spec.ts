import { test, expect } from "@playwright/test";
import { call, confirmDelivery, confirmSO, createSO, ok, partyByCode, productByCode, today, uuid } from "../helpers/api";
import { collectErrors, loginAs } from "../helpers/ui";

// Alert kesehatan data (ADR-0025, spesifikasi §8.4). Seed: P021 harga minimum > harga jual (A8),
// P022/P023 nama sama "Paku Kayu 5cm" (A9). Spec ini menambah satu SJ reguler bersopir yang
// dikonfirmasi tanpa status tanda tangan (A11).
//
// Hasil cek di-cache per cabang (lapis cepat 60 dtk); `?refresh=1` membuangnya tetapi dibatasi
// 1×/30 dtk per cabang, jadi asersi "sudah hilang" memakai expect.poll.
const A8 = "master.min_price_above_selling";
const A9 = "master.duplicate_product_name";
const A11 = "delivery.unsigned";

async function healthIds(): Promise<string[]> {
  const page = await ok<any>("admin", "GET", "/data-health?refresh=1");
  return page.areas.flatMap((a: any) => a.items.map((i: any) => i.id));
}

test.describe.serial("E19 kesehatan data: kartu dasbor, halaman, tunda, izin", () => {
  test.beforeAll(async () => {
    const p = await productByCode("P015");
    const customer = await partyByCode("customers", "C005");
    const so = await createSO("admin", customer.id, [{ p, qty: 1 }], { paymentTerms: "net", dueDate: today(14) });
    await confirmSO("admin", so.id);
    const dn = await ok<any>("admin", "POST", "/deliveries", {
      salesOrderId: so.id, clientRef: uuid(), driverName: "Sopir E19", vehiclePlate: "B 19 E",
      items: [{ productId: p.id, variantId: p.variantId, uom: "pcs", qty: 1 }],
    });
    await confirmDelivery("admin", dn.id, { recipientName: "Penerima E19" });
    await expect.poll(healthIds, { timeout: 45_000, intervals: [1_000, 5_000] }).toEqual(expect.arrayContaining([A8, A9, A11]));
  });

  test("kartu Perlu tindakan memuat A8; aksinya membuka daftar produk terfilter", async ({ page }) => {
    await loginAs(page, "admin");
    const errors = collectErrors(page);
    await page.goto("/dashboard");
    const card = page.locator("section", { has: page.getByRole("heading", { name: /^Perlu tindakan/ }) });
    const row = card.getByRole("link", { name: /Perlu perhatian: Harga minimum di atas harga jual/ });
    await expect(row).toBeVisible();
    // Info (nama ganda, SJ ≤ 3 hari) tidak pernah masuk kartu.
    await expect(card.getByRole("link", { name: /Nama produk ganda/ })).toHaveCount(0);
    await row.click();
    await expect(page).toHaveURL(/\/products\?priceIssue=min_above_selling/);
    await expect(page.getByText("Harga minimum di atas harga jual").first()).toBeVisible();
    await expect(page.getByText("P021").first()).toBeVisible();
    await expect(page.getByText("P001", { exact: true })).toHaveCount(0);
    expect(errors).toEqual([]);
  });

  test("halaman: A9 dan A11 tampil; tunda A9 3 hari lalu aktifkan lagi, keduanya teraudit", async ({ page }) => {
    await loginAs(page, "admin");
    const errors = collectErrors(page);
    await page.goto("/data-health");
    await expect(page.getByRole("heading", { level: 1, name: "Kesehatan Data" })).toBeVisible();
    await expect(page.getByRole("heading", { level: 3, name: "SJ terkirim belum bertanda tangan" })).toBeVisible();
    await expect(page.getByRole("status")).toContainText("perhatian");

    await page.getByRole("button", { name: "Tunda: Nama produk ganda" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel(/^3 hari/).check();
    await dialog.getByRole("textbox").fill("Menunggu pemilik memilih kode yang dipakai");
    await dialog.getByRole("button", { name: "Tunda", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole("heading", { level: 3, name: "Nama produk ganda" })).toHaveCount(0);

    await page.getByRole("tab", { name: /Ditunda/ }).click();
    await expect(page.getByText(/Ditunda oleh Admin E2E/)).toBeVisible();
    await page.getByRole("button", { name: "Aktifkan lagi: Nama produk ganda" }).click();
    await expect(page.getByText("Tidak ada alert yang ditunda")).toBeVisible();
    await page.getByRole("tab", { name: /Aktif/ }).click();
    await expect(page.getByRole("heading", { level: 3, name: "Nama produk ganda" })).toBeVisible();

    const audit = await ok<any[]>("admin", "GET", "/audit-logs?action=data_health_snoozes.&limit=10");
    const actions = audit.map((a) => a.action ?? a.actionKey);
    expect(actions).toEqual(expect.arrayContaining(["data_health_snoozes.create", "data_health_snoozes.delete"]));
    expect(errors).toEqual([]);
  });

  test("menyelesaikan A8 (harga minimum diturunkan) menghilangkan alertnya", async () => {
    const p = await productByCode("P021");
    const d = await ok<any>("admin", "GET", `/products/${p.id}`);
    await ok("admin", "PUT", `/products/${p.id}`, {
      code: d.code, name: d.name, categoryId: d.categoryId, type: d.type, tracked: d.tracked,
      baseUom: d.baseUom, purchaseUom: d.purchaseUom, salesUom: d.salesUom,
      purchaseFactor: d.purchaseFactor, salesFactor: d.salesFactor, purchasePrice: d.purchasePrice,
      sellingPrice: d.sellingPrice, minSellingPrice: 0, minStock: d.minStock, status: d.status,
    });
    await expect.poll(healthIds, { timeout: 45_000, intervals: [1_000, 5_000] }).not.toContain(A8);
  });

  test("kasir: tanpa kartu alert, endpoint 403, halaman /403", async ({ page }) => {
    expect((await call("kasir", "GET", "/dashboard/alerts")).status).toBe(403);
    expect((await call("kasir", "GET", "/data-health")).status).toBe(403);
    expect((await call("kasir", "POST", `/data-health/${A9}/snooze`, { days: 1, reason: "coba tunda" })).status).toBe(403);
    await loginAs(page, "kasir");
    await page.goto("/dashboard");
    await expect(page.getByText("Pesanan jatuh tempo dan stok kritis yang perlu ditangani.")).toBeVisible();
    await page.goto("/data-health");
    await page.waitForURL(/\/403/);
  });

  test("GET /dashboard/alerts hangat < 300 ms", async () => {
    await ok("admin", "GET", "/dashboard/alerts?limit=5");
    const t0 = Date.now();
    const r = await call("admin", "GET", "/dashboard/alerts?limit=5");
    const ms = Date.now() - t0;
    expect(r.status).toBe(200);
    expect(r.data.items.length).toBeLessThanOrEqual(5);
    expect(ms).toBeLessThan(300);
  });
});
