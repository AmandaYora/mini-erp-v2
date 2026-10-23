import { test, expect } from "@playwright/test";
import { call, ok, productByCode, stockQty } from "../helpers/api";
import { collectErrors, loginAs } from "../helpers/ui";

// Temuan 7 (2026-10-23): "input 0.8 di POS ga bisa". Di tablet kasir qty diketik 0,8 (keyboard fisik dengan
// koma, seperti id-ID); order, SJ serah langsung, dan stok membawa 0,8 — bukan dibulatkan ke 1.
test.use({ viewport: { width: 1280, height: 800 }, hasTouch: true });

test("E24 POS: jual qty 0,8 lalu bayar tunai → order, SJ, dan stok 0,8", async ({ page }) => {
  const p = await productByCode("P009"); // Sabun Mandi 5.500
  const before = await stockQty(p.id);
  await loginAs(page, "kasir");
  const errors = collectErrors(page);
  await page.goto("/pos");
  await page.getByLabel("Cari Produk").fill(p.code);
  await page.getByRole("button", { name: `Tambah ${p.name}` }).first().click();

  const cart = page.getByRole("complementary", { name: "Keranjang" });
  await cart.getByRole("button", { name: `Ubah qty ${p.name}` }).click();
  const pad = page.getByRole("group", { name: "Papan angka qty" });
  await expect(pad).toBeVisible();
  for (const key of ["0", ",", "8", "Enter"]) await pad.press(key);
  await expect(cart.getByRole("button", { name: `Ubah qty ${p.name}` })).toHaveText("0,8");
  await expect(cart.getByRole("button", { name: "Kurangi qty" })).toBeDisabled();
  // round(0,8 × 5.500) = 4.400
  await expect(cart.getByRole("button", { name: /^Bayar ·/ })).toContainText("4.400");

  await cart.getByRole("button", { name: /^Bayar ·/ }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Tunai Diterima").fill("5000");
  await dialog.getByRole("button", { name: "Proses Bayar" }).click();
  await expect(dialog.getByText("Kembalian").first()).toBeVisible();
  await expect(dialog.getByText(`${p.name} × 0,8`)).toBeVisible();

  const orders = await call("admin", "GET", "/sales-orders?channel=pos&limit=1");
  const so = await ok<any>("admin", "GET", `/sales-orders/${orders.data[0].id}`);
  expect(so.items.map((i: any) => [i.productId, i.qty])).toEqual([[p.id, 0.8]]);
  expect(so.grandTotal).toBe(4400);
  const notes = await ok<any[]>("admin", "GET", `/deliveries?salesOrderId=${so.id}`);
  expect(notes).toHaveLength(1);
  const dn = await ok<any>("admin", "GET", `/deliveries/${notes[0].id}`);
  expect(dn.status).toBe("confirmed");
  expect(dn.items.map((i: any) => i.qty)).toEqual([0.8]);
  expect(await stockQty(p.id)).toBeCloseTo(before - 0.8, 6);
  expect(errors).toEqual([]);
});
