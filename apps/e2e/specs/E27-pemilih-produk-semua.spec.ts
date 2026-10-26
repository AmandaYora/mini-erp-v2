import { test, expect } from "@playwright/test";
import { ok, productByCode, partyByCode, createSO, confirmSO, createDelivery, confirmDelivery } from "../helpers/api";
import { collectErrors, loginAs } from "../helpers/ui";

// Temuan pengguna 2026-10-26: pemilih produk tidak menampilkan semua barang master. search-options
// dulu maks. 20–30 baris dari offset 0; pemilih barang pengganti retur bahkan memuat 20 produk sekali
// lalu menyaring di browser. Kini search-options berhalaman (`page`) dan pemilih memuat halaman
// berikutnya saat digulir: produk ke-31+ yang cocok bisa dipilih.
test.describe.serial("E27 pemilih produk menjangkau semua barang", () => {
  const N = 35;
  const code = (i: number) => `E27-${String(i).padStart(3, "0")}`;

  test.beforeAll(async () => {
    for (let i = 1; i <= N; i++) {
      await ok("admin", "POST", "/products", {
        code: code(i), name: `Pilih Semua ${String(i).padStart(3, "0")}`, type: "barang", tracked: true,
        baseUom: "pcs", purchaseUom: "pcs", salesUom: "pcs", purchaseFactor: 1, salesFactor: 1,
        purchasePrice: 5000, sellingPrice: 8000,
      });
    }
  });

  test("search-options berhalaman: halaman 2 berisi kecocokan ke-31..35", async () => {
    const p1 = await ok<any[]>("admin", "GET", "/products/search-options?q=E27-&limit=30&page=1");
    const p2 = await ok<any[]>("admin", "GET", "/products/search-options?q=E27-&limit=30&page=2");
    expect(p1).toHaveLength(30);
    expect(p2.map((r) => r.code)).toEqual([code(31), code(32), code(33), code(34), code(35)]);
    // Tanpa page: hanya baris pertama, seperti sebelumnya.
    const legacy = await ok<any[]>("admin", "GET", "/products/search-options?q=E27-&limit=30");
    expect(legacy).toHaveLength(30);
  });

  test("retur tukar: barang pengganti di luar halaman pertama bisa dipilih dengan menggulir", async ({ page }) => {
    const p = await productByCode("P011");
    const customer = await partyByCode("customers", "C004");
    const so = await createSO("admin", customer.id, [{ p, qty: 2 }]);
    await confirmSO("admin", so.id);
    const d = await createDelivery("admin", so.id, [{ p, qty: 2 }]);
    await confirmDelivery("admin", d.id, { settlement: { mode: "pay_now", method: "cash", amountTendered: so.grandTotal } });

    const errors = collectErrors(page);
    await loginAs(page, "admin");
    await page.goto(`/sales-returns/new?from=${so.id}`);
    await page.getByLabel(`Qty retur ${p.name}`).fill("1");
    await page.getByRole("radio", { name: "Tukar barang" }).click();
    await page.getByRole("button", { name: "Tambah barang pengganti" }).click();
    await page.getByRole("button", { name: "Produk pengganti baris 1" }).click();
    await page.getByPlaceholder("Kode atau nama produk…").fill("E27-");
    const list = page.getByRole("listbox");
    await expect(list.getByText(`${code(30)} — Pilih Semua 030`)).toBeVisible();
    await expect(list.getByText(`${code(35)} — Pilih Semua 035`)).toHaveCount(0);
    // Gulir ke bawah daftar: halaman berikutnya dimuat otomatis.
    await list.evaluate((el) => {
      el.scrollTop = el.scrollHeight;
    });
    const last = list.getByText(`${code(35)} — Pilih Semua 035`);
    await expect(last).toBeVisible();
    await last.click();
    await expect(page.getByRole("button", { name: "Produk pengganti baris 1" })).toContainText(code(35));
    expect(errors).toEqual([]);
  });

  test("SJ Langsung (ProductSearchSelect): produk ke-31+ terjangkau dengan menggulir", async ({ page }) => {
    const errors = collectErrors(page);
    await loginAs(page, "admin");
    await page.goto("/deliveries/new-direct");
    await page.locator("#f-product").click();
    await page.getByPlaceholder("Ketik kode atau nama produk…").fill("E27-");
    const list = page.getByRole("listbox");
    await expect(list.getByText(`${code(1)} — Pilih Semua 001`)).toBeVisible();
    await list.evaluate((el) => {
      el.scrollTop = el.scrollHeight;
    });
    await expect(list.getByText(`${code(35)} — Pilih Semua 035`)).toBeVisible();
    expect(errors).toEqual([]);
  });
});
