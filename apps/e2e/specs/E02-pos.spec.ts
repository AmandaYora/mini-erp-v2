import { test, expect } from "@playwright/test";
import { branches, call, ok, productByCode, partyByCode, rawLogin, stockQty, today } from "../helpers/api";
import { PASSWORD } from "../helpers/env";
import { loginAs } from "../helpers/ui";

async function posOrderCount(): Promise<number> {
  const r = await call("admin", "GET", "/sales-orders?channel=pos&limit=1");
  return r.meta?.total ?? 0;
}

test.describe.serial("E02 POS", () => {
  test("tunai: scan barcode -> keranjang -> bayar (klik ganda) -> satu order, stok berkurang", async ({ page }) => {
    const p = await productByCode("P008"); // Mie Instan Goreng 3.500
    const before = await stockQty(p.id);
    const ordersBefore = await posOrderCount();
    await loginAs(page, "kasir");
    await page.goto("/pos");

    const scan = page.getByLabel("Pindai QR / kode");
    await scan.fill(p.barcode);
    await scan.press("Enter");
    await expect(page.getByText(p.name).first()).toBeVisible();
    await scan.fill(p.barcode);
    await scan.press("Enter");
    await page.getByRole("button", { name: /^Bayar ·/ }).click();

    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Tunai Diterima").fill("10000");
    await dialog.getByRole("button", { name: "Proses Bayar" }).dblclick();

    await expect(dialog.getByText("Order ORD-", { exact: false }).first()).toBeVisible();
    await expect(dialog.getByText("Total (server)")).toBeVisible();
    await expect(dialog.getByText("Kembalian").first()).toBeVisible();
    await expect(dialog.getByRole("link", { name: "Cetak Struk" })).toBeVisible();

    expect(await posOrderCount()).toBe(ordersBefore + 1);
    expect(await stockQty(p.id)).toBe(before - 2);
  });

  test("bayar nanti: wajib jatuh tempo; tercatat sebagai piutang pelanggan", async ({ page }) => {
    const p = await productByCode("P009");
    const c = await partyByCode("customers", "C003");
    await loginAs(page, "kasir");
    await page.goto("/pos");
    await page.getByLabel("Cari Produk").fill(p.code);
    await page.getByRole("button", { name: `Tambah ${p.name}` }).first().click();
    // Mode Kasir: pelanggan dipilih lewat Sheet "Ganti" (pencarian + daftar), bukan dropdown inline.
    await page.getByRole("button", { name: "Ganti pelanggan" }).click();
    await page.getByPlaceholder("Ketik untuk mencari customer…").fill(c.name);
    await page.getByRole("button", { name: new RegExp(c.name) }).first().click();
    await page.getByRole("button", { name: /^Bayar ·/ }).click();

    const dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: "Bayar Nanti" }).click();
    await dialog.getByLabel("Jatuh Tempo").fill(today(14));
    await dialog.getByRole("button", { name: "Buat Piutang" }).click();
    await expect(dialog.getByText("Piutang tercatat")).toBeVisible();

    const ar = await ok<any>("akuntan", "GET", "/payments/party-balances?partyId=" + c.id);
    expect(JSON.stringify(ar)).toContain(String(p.sellingPrice));
  });

  test("PPN opsional (K-03): kena pajak exclude 11% -> total server = estimasi checkout, order membawa PPN", async ({ page }) => {
    const p = await productByCode("P008"); // 3.500, satu buah
    await loginAs(page, "kasir");
    await page.goto("/pos");
    const scan = page.getByLabel("Pindai QR / kode");
    await scan.fill(p.barcode);
    await scan.press("Enter");
    await expect(page.getByText(p.name).first()).toBeVisible();
    await page.getByRole("button", { name: /^Bayar ·/ }).click();

    const dialog = page.getByRole("dialog");
    // Bawaan tanpa pajak: kotak tidak dicentang, tak ada ringkasan DPP/PPN.
    await expect(dialog.getByLabel(/Kena pajak \(PPN\)/)).not.toBeChecked();
    await expect(dialog.getByTestId("pos-tax-summary")).toHaveCount(0);
    await dialog.getByLabel(/Kena pajak \(PPN\)/).click();
    // 3.500 + 11% (385) = 3.885
    await expect(dialog.getByTestId("pos-tax-summary")).toContainText("3.885");
    await dialog.getByLabel("Tunai Diterima").fill("5000");
    await dialog.getByRole("button", { name: "Proses Bayar" }).click();
    await expect(dialog.getByText("Total (server)")).toBeVisible();
    await expect(dialog.getByText(/3\.885/).first()).toBeVisible();

    const list = await call("admin", "GET", "/sales-orders?channel=pos&limit=1");
    const order = list.data[0];
    expect(order.taxType).toBe("exclude");
    expect(order.taxRate).toBe(11);
    expect(order.taxTotal).toBe(385);
    expect(order.grandTotal).toBe(3885);
  });

  // K1 (legacy BR-18): konfirmasi SJ order tunai mewajibkan pembayaran, jadi kasir tanpa
  // payment.create dulu meninggalkan order yatim. Kini (1) role kasir BAWAAN membawa
  // payment.create/view — seed e2e tidak lagi menambahkannya, jadi tes tunai di atas memakai
  // izin bawaan; (2) role tanpa izin itu ditolak SEBELUM dokumen apa pun dibuat.
  test("kasir tanpa izin pembayaran: bayar sekarang ditolak di awal, tanpa order yatim (K1)", async ({ page }) => {
    const roles = await ok<any[]>("admin", "GET", "/roles");
    const kasir = await ok<any>("admin", "GET", `/roles/${roles.find((r) => r.code === "kasir").id}`);
    expect(kasir.permissions).toEqual(expect.arrayContaining(["payment.create", "payment.view"]));

    const n = Date.now() % 1_000_000;
    const role = await ok<any>("admin", "POST", "/roles", {
      code: `kasir_k1_${n}`,
      name: `Kasir tanpa bayar ${n}`,
      description: "uji e2e K1",
    });
    await ok("admin", "PUT", `/roles/${role.id}/permissions`, {
      permissions: kasir.permissions.filter((c: string) => !c.startsWith("payment.")),
    });
    const hq = (await branches()).find((b) => b.code === "HQ");
    const username = `kasirk1${n}`;
    await ok("admin", "POST", "/users", {
      username,
      fullName: "Kasir K1",
      password: PASSWORD,
      roleIds: [role.id],
      branches: [{ branchId: hq.id, isDefault: true }],
    });
    const login = await rawLogin(username, PASSWORD);
    await page.addInitScript(([a, r]) => {
      localStorage.setItem("mini-erp.access-token", a);
      localStorage.setItem("mini-erp.refresh-token", r);
    }, [login.accessToken, login.refreshToken]);

    const p = await productByCode("P009");
    const ordersBefore = await posOrderCount();
    await page.goto("/pos");
    await page.getByRole("button", { name: `Tambah ${p.name}` }).first().click();
    await page.getByRole("button", { name: /^Bayar ·/ }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("Bayar sekarang tidak tersedia untuk akun ini")).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Proses Bayar" })).toBeDisabled();
    expect(await posOrderCount()).toBe(ordersBefore);
  });
});
