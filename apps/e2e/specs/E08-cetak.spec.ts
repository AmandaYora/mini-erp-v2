import { test, expect } from "@playwright/test";
import { today, productByCode, partyByCode, createSO, confirmSO, createDelivery, confirmDelivery, pay, ok, locationByCode } from "../helpers/api";
import { collectErrors, loginAs } from "../helpers/ui";

// Render + angka benar. Verifikasi cetak fisik tetap manual (F8-6).
test.describe.serial("E08 cetak", () => {
  let so: any;
  let delivery: any;
  let payment: any;
  let p1: Awaited<ReturnType<typeof productByCode>>;
  let p2: typeof p1;
  const total = 3 * 5500 + 2 * 1500; // 19.500

  test.beforeAll(async () => {
    p1 = await productByCode("P009");
    p2 = await productByCode("P010");
    const c = await partyByCode("customers", "C005");
    so = await createSO("admin", c.id, [{ p: p1, qty: 3 }, { p: p2, qty: 2 }], { paymentTerms: "net", dueDate: today(7) });
    await confirmSO("admin", so.id);
    delivery = await createDelivery("admin", so.id, [{ p: p1, qty: 3 }, { p: p2, qty: 2 }]);
    await confirmDelivery("admin", delivery.id);
    payment = await pay("admin", c.id, total, [{ orderType: "sales", orderId: so.id, amount: total }]);
    expect(so.grandTotal).toBe(total);
  });

  test("faktur/nota penjualan memuat nomor, barang, dan total", async ({ page }) => {
    await loginAs(page, "admin");
    const errors = collectErrors(page);
    await page.goto(`/print/sales/${so.id}`);
    await expect(page.getByText(so.number).first()).toBeVisible();
    await expect(page.getByText(p1.name).first()).toBeVisible();
    await expect(page.getByText(/19\.500/).first()).toBeVisible();
    expect(errors).toEqual([]);
  });

  test("surat jalan memuat nomor SJ dan barang", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto(`/print/deliveries/${delivery.id}`);
    await expect(page.getByText(delivery.number).first()).toBeVisible();
    await expect(page.getByText(p2.name).first()).toBeVisible();
  });

  test("surat transfer barang memuat nomor, sopir, kendaraan, barang, dan tiga kolom tanda tangan (ST-02)", async ({ page }) => {
    const ga = await locationByCode("HQ-GA");
    const gb = await locationByCode("HQ-GB");
    const t = await ok<any>("admin", "POST", "/stock/transfers", {
      toBranchId: ga.branchId, fromLocationId: ga.id, toLocationId: gb.id, notes: "uji cetak",
      driverName: "Pak Sopir Cetak", vehiclePlate: "B 777 CT", transferDate: today(),
      items: [{ productId: p1.id, variantId: p1.variantId, qty: 1, notes: "" }],
    });
    await loginAs(page, "admin");
    await page.goto(`/print/transfers/${t.id}`);
    await expect(page.getByText("Surat Transfer Barang").first()).toBeVisible();
    await expect(page.getByText(t.number).first()).toBeVisible();
    await expect(page.getByText("Pak Sopir Cetak").first()).toBeVisible();
    await expect(page.getByText("B 777 CT").first()).toBeVisible();
    await expect(page.getByText(p1.name).first()).toBeVisible();
    for (const role of ["Gudang Asal", "Sopir", "Gudang Tujuan"]) {
      await expect(page.locator(".sj-sign-role", { hasText: role })).toBeVisible();
    }
  });

  test("kwitansi pembayaran memuat nomor dan nominal", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto(`/print/payments/${payment.id}`);
    await expect(page.getByText(payment.number).first()).toBeVisible();
    await expect(page.getByText(/19\.500/).first()).toBeVisible();
  });

  test("struk POS memuat nomor order dan total", async ({ page }) => {
    const pos = await createSO("kasir", 0, [{ p: p1, qty: 1 }, { p: p2, qty: 2 }], { channel: "pos" });
    await confirmSO("kasir", pos.id);
    const d = await createDelivery("kasir", pos.id, [{ p: p1, qty: 1 }, { p: p2, qty: 2 }]);
    await confirmDelivery("kasir", d.id, { settlement: { mode: "pay_now", method: "cash", amountTendered: 10000 } });
    expect(pos.grandTotal).toBe(5500 + 3000);
    await loginAs(page, "admin");
    await page.goto(`/print/pos/${pos.id}`);
    await expect(page.getByText(pos.number).first()).toBeVisible();
    await expect(page.getByText(/8\.500/).first()).toBeVisible();
  });

  test("label QR produk termuat dengan kode produk", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto(`/print/labels?ids=${p1.id},${p2.id}`);
    await expect(page.getByText(p1.code).first()).toBeVisible();
    await expect(page.getByText(p2.code).first()).toBeVisible();
    await expect(page.locator("img, svg, canvas").first()).toBeVisible();
  });

  test("kalibrasi kertas termuat", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto("/print/calibration");
    await expect(page.getByRole("heading").first()).toBeVisible();
  });
});
