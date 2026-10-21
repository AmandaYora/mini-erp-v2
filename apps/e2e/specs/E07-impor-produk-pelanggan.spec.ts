import { test, expect } from "@playwright/test";
import { call, ok, products, productByCode, partyByCode, createSO, confirmSO, createDelivery, confirmDelivery } from "../helpers/api";
import { buildXlsx, uploadXlsx } from "../helpers/xlsx";
import { loginAs } from "../helpers/ui";

const HEAD = [
  "Kode Produk", "Nama Produk", "Kategori", "Tipe Produk", "Pantau Stok?", "Satuan Stok", "Satuan Beli", "Satuan Jual",
  "Isi per Satuan Beli", "Isi Jual per Satuan Stok", "Harga Beli", "Harga Jual", "Harga Jual Minimum", "Minimum Stok", "Status", "Catatan Produk",
];
const row = (code: string, name: string, buy: number, sell: number) =>
  [code, name, "SEMBAKO", "Barang", "Ya", "pcs", "pcs", "pcs", 1, 1, buy, sell, 0, 5, "Aktif", ""];

test.describe.serial("E07 impor produk + CRUD pelanggan", () => {
  test("impor produk: berkas salah ditolak utuh, berkas benar masuk, impor ulang tidak menggandakan", async () => {
    const before = (await products()).length;

    const bad = await buildXlsx([{ name: "Daftar Produk", headers: HEAD, rows: [row("IMP-001", "Impor Satu", 1000, 2000), row("IMP-002", "", 1000, 2000)] }]);
    const prev = await uploadXlsx("/product-imports/preview", bad);
    expect(prev.status).toBeLessThan(500);
    expect(JSON.stringify(prev)).toMatch(/IMP-002|Nama|baris|row/i);
    const rejected = await uploadXlsx("/product-imports/commit", bad);
    expect(rejected.status).toBeGreaterThanOrEqual(400);
    expect((await products()).length).toBe(before); // semua-atau-tidak-sama-sekali

    const good = await buildXlsx([{ name: "Daftar Produk", headers: HEAD, rows: [row("IMP-001", "Impor Satu", 1000, 2000), row("IMP-002", "Impor Dua", 3000, 4500)] }]);
    const res = await uploadXlsx("/product-imports/commit", good);
    expect(res.success, res.message).toBe(true);
    expect((await products()).length).toBe(before + 2);
    const p = await productByCode("IMP-002");
    expect(p.sellingPrice).toBe(4500);

    // ulang dengan harga berubah: upsert, jumlah produk tetap
    const again = await buildXlsx([{ name: "Daftar Produk", headers: HEAD, rows: [row("IMP-001", "Impor Satu", 1000, 2500), row("IMP-002", "Impor Dua", 3000, 4500)] }]);
    expect((await uploadXlsx("/product-imports/commit", again)).success).toBe(true);
    expect((await products()).length).toBe(before + 2);
    expect((await productByCode("IMP-001")).sellingPrice).toBe(2500);
  });

  test("UI: tambah customer (kode otomatis), ubah tidak menggandakan, arsip berpiutang tetap tertagih", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto("/customers");
    await page.getByRole("button", { name: "Tambah Customer" }).click();
    const dlg = page.getByRole("dialog");
    await dlg.getByLabel("Nama").fill("Pelanggan UI E2E");
    await dlg.getByLabel("Telepon").fill("081299990001");
    await dlg.getByRole("button", { name: "Simpan" }).click();
    await expect(dlg).toBeHidden();

    const list = async () => (await ok<any[]>("admin", "GET", "/customers?limit=100&search=Pelanggan UI E2E")).filter((c) => c.name.startsWith("Pelanggan UI E2E"));
    const created = await list();
    expect(created).toHaveLength(1);
    expect(created[0].code).toMatch(/^CUS-\d+/);

    await page.goto(`/customers/${created[0].id}`);
    await page.getByRole("button", { name: "Ubah" }).first().click();
    const edit = page.getByRole("dialog");
    await edit.getByLabel("Nama").fill("Pelanggan UI E2E Ubah");
    await edit.getByRole("button", { name: "Simpan" }).click();
    await expect(edit).toBeHidden();
    const after = await list();
    expect(after).toHaveLength(1);
    expect(after[0].code).toBe(created[0].code);
    expect(after[0].name).toBe("Pelanggan UI E2E Ubah");

    // piutang: order terkirim belum lunas (tempo)
    const p = await productByCode("P017");
    const so = await createSO("admin", created[0].id, [{ p, qty: 1 }], { paymentTerms: "net", dueDate: new Date(Date.now() + 7 * 86400000).toISOString().slice(0, 10) });
    await confirmSO("admin", so.id);
    const d = await createDelivery("admin", so.id, [{ p, qty: 1 }]);
    await confirmDelivery("admin", d.id);

    await page.reload();
    await page.getByRole("button", { name: "Arsipkan" }).first().click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Arsipkan" }).click();
    // Arsip = soft tanpa syarat (BR-19): boleh berpiutang; piutang tetap tertagih, order BARU ditolak.
    await expect.poll(async () => (await ok<any>("admin", "GET", `/customers/${created[0].id}`)).status).toBe("archived");
    const balances = await ok<any>("admin", "GET", `/payments/party-balances?partyId=${created[0].id}`);
    expect(JSON.stringify(balances)).toContain(String(so.grandTotal));
    const again = await call("admin", "POST", "/sales-orders", {
      partyId: created[0].id, channel: "regular", paymentTerms: "cod", taxType: "none",
      items: [{ productId: p.id, variantId: p.variantId, uom: "pcs", qty: 1 }],
    });
    expect(again.status).toBeGreaterThanOrEqual(400);
  });

  // K2: legacy BR-05/E-16 membolehkan bayar piutang pihak terarsip. Revamp sempat menolaknya
  // ("pihak sudah diarsipkan"); kini arsip hanya menghentikan order BARU, piutang tetap tertagih
  // (payment/application/service.go, komentar "legacy BR-05").
  test("pembayaran piutang pihak terarsip diterima (K2)", async () => {
    const c = await ok<any>("admin", "POST", "/customers", { name: "Pelanggan Arsip K2", phone: "081299990002" });
    const p = await productByCode("P017");
    const so = await createSO("admin", c.id, [{ p, qty: 1 }], { paymentTerms: "net", dueDate: new Date(Date.now() + 7 * 86400000).toISOString().slice(0, 10) });
    await confirmSO("admin", so.id);
    await confirmDelivery("admin", (await createDelivery("admin", so.id, [{ p, qty: 1 }])).id);
    await ok("admin", "POST", `/customers/${c.id}/archive`);
    const paid = await call("akuntan", "POST", "/payments", {
      partyId: c.id, amount: so.grandTotal, method: "transfer",
      allocations: [{ orderType: "sales", orderId: so.id, amount: so.grandTotal }],
    });
    expect(paid.status).toBeLessThan(300);
  });
});
