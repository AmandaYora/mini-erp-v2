import { test, expect } from "@playwright/test";
import { confirmPO, locationByCode, ok, partyByCode, uuid, today } from "../helpers/api";
import { collectErrors, loginAs } from "../helpers/ui";

// Temuan 1 (2026-10-23): retur beli atas PO ber-DUS (isi 12) bisa dipilih per PCS. Form menampilkan
// sisa 2 dus; ganti satuan ke pcs → batas 24; retur 6 pcs tersimpan sebagai 6 pcs bernilai pro-rata.
test("E22 retur beli PO ber-DUS: pilih satuan PCS, batas dan nilai mengikuti satuan", async ({ page }) => {
  const code = `DUS-E22-${uuid().slice(0, 6)}`.toUpperCase();
  const prod = await ok<any>("admin", "POST", "/products", {
    code, name: `Pipa Dus ${code.slice(-6)}`, type: "barang", tracked: true,
    baseUom: "pcs", purchaseUom: "dus", salesUom: "pcs", purchaseFactor: 12, salesFactor: 1,
    purchasePrice: 120000, sellingPrice: 15000,
  });
  const variantId = prod.variants[0].id;
  const supplier = await partyByCode("suppliers", "S002");
  const loc = await locationByCode("HQ-GA");
  const po = await ok<any>("admin", "POST", "/purchase-orders", {
    partyId: supplier.id, paymentTerms: "net", dueDate: today(30), taxType: "none",
    items: [{ productId: prod.id, variantId, uom: "dus", qty: 2, unitPrice: 120000, discountPct: 0, discountNominal: 0 }],
  });
  await confirmPO("admin", po.id);
  await ok("admin", "POST", "/goods-receipts", {
    purchaseOrderId: po.id, clientRef: uuid(),
    items: [{ productId: prod.id, variantId, locationId: loc.id, uom: "dus", qty: 2 }],
  });

  await loginAs(page, "admin");
  const errors = collectErrors(page);
  await page.goto(`/purchase-returns/new?from=${po.id}`);
  const qty = page.getByLabel(`Qty retur ${prod.name}`);
  await expect(qty).toHaveValue("2");
  await page.getByLabel(`Satuan retur ${prod.name}`).selectOption("pcs");
  await expect(qty).toHaveValue("24");
  await qty.fill("6");
  await page.getByLabel(`Lokasi keluar ${prod.name}`).selectOption(String(loc.id));
  await page.getByRole("button", { name: "Lanjut" }).click();
  await page.getByPlaceholder("Mis. barang cacat dari supplier").fill("Sebagian isi dus pecah");
  await page.getByRole("button", { name: "Pratinjau" }).click();
  await expect(page.getByText("Pratinjau — belum disimpan")).toBeVisible();
  await page.getByRole("button", { name: "Simpan draf" }).click();
  await page.waitForURL(/\/purchase-returns\/\d+$/);
  const id = Number(new URL(page.url()).pathname.split("/").pop());
  const ret = await ok<any>("admin", "GET", `/purchase-returns/${id}`);
  expect(ret.items.map((i: any) => [i.uom, i.qty, i.qtyBase])).toEqual([["pcs", 6, 6]]);
  expect(ret.total).toBe(60000); // 6/12 dari Rp120.000 per dus
  expect(errors).toEqual([]);
});
