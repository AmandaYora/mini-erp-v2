import { test, expect } from "@playwright/test";
import { productByCode, stockQty } from "../helpers/api";
import { collectErrors, loginAs } from "../helpers/ui";

// Temuan 5 (2026-10-23): Mode Kasir dibuka di HP. Tanpa ajakan "putar tablet"; cari → ketuk tile →
// dok menunjukkan jumlah + total → keranjang di sheet bawah → Bayar → tunai → selesai.
test.describe("E23 POS di HP (tata letak ringkas < 1024px)", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });

  test("tegak: transaksi tunai lewat dok dan sheet keranjang", async ({ page }) => {
    const p = await productByCode("P009"); // Sabun Mandi 5.500
    const before = await stockQty(p.id);
    await loginAs(page, "kasir");
    const errors = collectErrors(page);
    await page.goto("/pos");

    await expect(page.getByText("Putar tablet ke posisi mendatar")).toHaveCount(0);
    await expect(page.getByLabel("Pindai QR / kode")).toBeHidden();
    const dock = page.getByRole("navigation", { name: "Aksi kasir" });
    await expect(dock.getByRole("button", { name: "Keranjang kosong" })).toBeDisabled();

    await page.getByLabel("Cari Produk").fill(p.code);
    const tile = page.getByRole("button", { name: `Tambah ${p.name}` }).first();
    await tile.tap();
    await tile.tap();
    await dock.getByRole("button", { name: /^Buka keranjang, 2 barang, total Rp\s?11\.000/ }).tap();

    const sheet = page.getByRole("dialog", { name: "Keranjang · 2 barang" });
    await expect(sheet.getByText(p.name)).toBeVisible();
    await sheet.getByRole("button", { name: /^Bayar/ }).tap();
    await expect(sheet).toHaveCount(0);

    const pay = page.getByRole("dialog");
    await pay.getByLabel("Tunai Diterima").fill("20000");
    await pay.getByRole("button", { name: "Proses Bayar" }).tap();
    await expect(pay.getByText("Kembalian").first()).toBeVisible();
    await pay.getByRole("button", { name: "Transaksi Baru" }).tap();
    await expect(dock.getByRole("button", { name: "Keranjang kosong" })).toBeVisible();

    expect(await stockQty(p.id)).toBe(before - 2);
    expect(errors).toEqual([]);
  });

  test("mendatar: pencarian naik ke topbar, dok tetap ada", async ({ page }) => {
    await page.setViewportSize({ width: 844, height: 390 });
    await loginAs(page, "kasir");
    await page.goto("/pos");
    const search = page.getByLabel("Cari Produk");
    await expect(search).toBeVisible();
    const header = page.locator("header");
    await expect(header.getByLabel("Cari Produk")).toBeVisible();
    await expect(page.getByRole("navigation", { name: "Aksi kasir" })).toBeVisible();
  });
});
