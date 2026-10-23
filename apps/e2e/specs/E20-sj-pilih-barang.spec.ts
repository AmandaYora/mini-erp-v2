import { test, expect, type Page } from "@playwright/test";
import { confirmDelivery, confirmSO, createSO, ok, partyByCode, productByCode, today } from "../helpers/api";
import { collectErrors, loginAs } from "../helpers/ui";

// Form Surat Jalan: checkbox "Kirim" per barang (padanan "Pilih Item yang Dibawa" legacy). SJ pertama
// membawa sebagian barang; barang yang dilepas tetap menjadi sisa SO dan dibawa SJ kedua, lalu SO
// selesai setelah keduanya dikonfirmasi.
async function saveDelivery(page: Page, soId: number, uncheck: string[]): Promise<number> {
  await page.goto(`/deliveries/new?from=${soId}`);
  await expect(page.getByRole("checkbox", { name: "Pilih semua barang" })).toBeVisible();
  for (const name of uncheck) await page.getByRole("checkbox", { name: `Kirim ${name}` }).click();
  await page.getByPlaceholder("Nama supir").fill("Sopir E20");
  await page.getByPlaceholder("Nama petugas gudang").fill("Gudang E20");
  await page.getByRole("button", { name: "Simpan surat jalan" }).click();
  await page.waitForURL(/\/deliveries\/\d+$/);
  return Number(new URL(page.url()).pathname.split("/").pop());
}

test("E20 SJ sebagian barang lewat checkbox, SJ kedua membawa sisanya", async ({ page }) => {
  const garam = await productByCode("P017");
  const telur = await productByCode("P018");
  const customer = await partyByCode("customers", "C003");
  const so = await createSO("admin", customer.id, [{ p: garam, qty: 3 }, { p: telur, qty: 2 }], { paymentTerms: "net", dueDate: today(14) });
  await confirmSO("admin", so.id);

  await loginAs(page, "admin");
  const errors = collectErrors(page);

  // Semua dilepas: ditolak di klien dengan pesan jelas.
  await page.goto(`/deliveries/new?from=${so.id}`);
  await page.getByRole("checkbox", { name: "Pilih semua barang" }).click();
  await expect(page.getByLabel(`Qty kirim ${telur.name}`)).toBeDisabled();
  await page.getByRole("button", { name: "Simpan surat jalan" }).click();
  await expect(page.getByText("Pilih minimal 1 barang yang dikirim")).toBeVisible();

  const first = await saveDelivery(page, so.id, [telur.name]);
  const sj1 = await ok<any>("admin", "GET", `/deliveries/${first}`);
  expect(sj1.items.map((i: any) => [i.productId, i.qty])).toEqual([[garam.id, 3]]);

  // Form berikutnya hanya memuat sisa: telur 2 (garam sudah dibuatkan draf).
  await page.goto(`/deliveries/new?from=${so.id}`);
  await expect(page.getByRole("checkbox", { name: `Kirim ${telur.name}` })).toBeVisible();
  await expect(page.getByRole("checkbox", { name: `Kirim ${garam.name}` })).toHaveCount(0);
  const second = await saveDelivery(page, so.id, []);
  const sj2 = await ok<any>("admin", "GET", `/deliveries/${second}`);
  expect(sj2.items.map((i: any) => [i.productId, i.qty])).toEqual([[telur.id, 2]]);

  for (const id of [first, second]) await confirmDelivery("admin", id, { recipientName: "Penerima E20", recipientSignatureStatus: "signed" });
  const after = await ok<any>("admin", "GET", `/sales-orders/${so.id}`);
  expect(after.status).toBe("completed");
  expect(errors).toEqual([]);
});
