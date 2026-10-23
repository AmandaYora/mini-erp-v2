import { test, expect } from "@playwright/test";
import { locationByCode, ok, uuid } from "../helpers/api";
import { PASSWORD } from "../helpers/env";
import { collectErrors, loginAs } from "../helpers/ui";

// Temuan 2 (2026-10-23): produk cat ber-warna. Ketuk tile di POS membuka "Pilih varian" (tanpa varian
// bawaan tersembunyi "Standar"), warna kedua masuk keranjang, dan tahanan stok jatuh ke warna itu.
test("E21 POS: produk multi-varian wajib pilih warna; warna terpilih yang ditahan", async ({ page }) => {
  const code = `CAT-E21-${uuid().slice(0, 6)}`.toUpperCase();
  const prod = await ok<any>("admin", "POST", "/products", {
    code, name: `Cat Tembok E21 ${code.slice(-6)}`, type: "barang", tracked: true,
    baseUom: "kg", purchaseUom: "kg", salesUom: "kg", purchaseFactor: 1, salesFactor: 1,
    purchasePrice: 40000, sellingPrice: 55000,
    variants: [
      { code, name: "Standar", isDefault: true, isHidden: true, sortOrder: 0 },
      { code: `${code}-001`, name: "Putih", sortOrder: 1 },
      { code: `${code}-002`, name: "Merah", sortOrder: 2 },
      { code: `${code}-003`, name: "Biru", sortOrder: 3 },
    ],
  });
  const variants: any[] = prod.variants;
  const merah = variants.find((v) => v.name === "Merah");
  expect(variants.find((v) => v.name === "Standar")?.isHidden).toBe(true);
  const ga = await locationByCode("HQ-GA");
  for (const v of variants.filter((x) => !x.isHidden)) {
    await ok("admin", "POST", "/stock/adjustments", {
      productId: prod.id, variantId: v.id, locationId: ga.id, mode: "in", qtyAfter: 0, qtyDelta: 5,
      reason: "stok uji E21", approver: "admin", approverPassword: PASSWORD,
    });
  }

  await loginAs(page, "kasir");
  const errors = collectErrors(page);
  await page.goto("/pos");
  await page.getByLabel("Cari Produk").fill(code);
  await page.getByRole("button", { name: `Tambah ${prod.name}` }).first().click();
  const sheet = page.getByRole("dialog", { name: "Pilih varian" });
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole("button", { name: /Standar/ })).toHaveCount(0);
  await expect(sheet.getByRole("button")).toContainText([/Putih/, /Merah/, /Biru/]);
  await sheet.getByRole("button", { name: /Merah/ }).click();
  await expect(sheet).toHaveCount(0);
  await expect(page.getByText("Merah").first()).toBeVisible();

  // Tahanan stok POS jatuh ke varian Merah (bukan "Standar").
  await expect
    .poll(async () => {
      const rows = await ok<any[]>("kasir", "GET", `/stock/balances?productId=${prod.id}`);
      return rows.filter((b) => b.variantId === merah.id).reduce((n, b) => n + b.reserved, 0);
    })
    .toBe(1);
  expect(errors).toEqual([]);
});
