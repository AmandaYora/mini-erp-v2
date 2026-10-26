import { test, expect } from "@playwright/test";
import { call, confirmSO, createSO, drainPosting, ok, partyByCode, productByCode, today } from "../helpers/api";
import { collectErrors, loginAs } from "../helpers/ui";

// Rekening perusahaan (ADR-0026): admin membuat dua rekening (BCA default, Mandiri), order baru membawa
// rekening default, admin mengganti rekening tujuan di detail order, pembayaran transfer terisi rekening
// itu dan dijurnal ke akunnya. Di akhir rekening diarsipkan agar spec berikutnya kembali ke perilaku
// tanpa rekening (transfer ke akun pemetaan "bank").
test.describe.serial("E26 rekening tujuan → pembayaran transfer → jurnal per rekening", () => {
  let bca: any;
  let mandiri: any;
  let orderId = 0;
  let total = 0;

  test.beforeAll(async () => {
    bca = await ok<any>("admin", "POST", "/finance/bank-accounts", {
      accountCode: "1011", bankName: "BCA", accountNumber: "1110001", accountHolder: "PT E2E", showOnDocuments: true, isDefault: true,
    });
    mandiri = await ok<any>("admin", "POST", "/finance/bank-accounts", {
      accountCode: "1012", bankName: "Mandiri", accountNumber: "2220002", accountHolder: "PT E2E", showOnDocuments: true, isDefault: false,
    });
  });

  test.afterAll(async () => {
    for (const b of [bca, mandiri]) if (b) await call("admin", "POST", `/finance/bank-accounts/${b.id}/archive`);
  });

  test("kasir membaca daftar rekening tanpa izin keuangan, tetapi tidak bisa menambah", async () => {
    const opts = await ok<any[]>("kasir", "GET", "/finance/bank-accounts/options");
    expect(opts.map((b) => b.bankName)).toEqual(expect.arrayContaining(["BCA", "Mandiri"]));
    expect(opts.find((b) => b.bankName === "BCA").isDefault).toBe(true);
    const denied = await call("kasir", "POST", "/finance/bank-accounts", {
      accountCode: "1013", bankName: "BRI", accountNumber: "3", accountHolder: "PT", showOnDocuments: true, isDefault: false,
    });
    expect(denied.status).toBe(403);
  });

  test("order baru membawa rekening default; admin menggantinya di detail order", async ({ page }) => {
    const customer = await partyByCode("customers", "C005");
    const p = await productByCode("P003");
    const so = await createSO("admin", customer.id, [{ p, qty: 2 }]);
    expect(so.receiptAccountId).toBe(bca.accountId);
    await confirmSO("admin", so.id);
    orderId = so.id;
    total = so.grandTotal;

    const errors = collectErrors(page);
    await loginAs(page, "admin");
    await page.goto(`/sales-orders/${orderId}`);
    await expect(page.getByText("BCA 1110001 a.n. PT E2E")).toBeVisible();
    await page.getByRole("button", { name: "Ubah rekening tujuan" }).click();
    const dialog = page.getByRole("dialog", { name: "Ubah rekening tujuan" });
    await dialog.getByRole("combobox").selectOption(String(mandiri.accountId));
    await dialog.getByRole("button", { name: "Simpan", exact: true }).click();
    await expect(page.getByText("Mandiri 2220002 a.n. PT E2E")).toBeVisible();
    const after = await ok<any>("admin", "GET", `/sales-orders/${orderId}`);
    expect(after.receiptAccountId).toBe(mandiri.accountId);

    // Pembayaran transfer: "Ke rekening" terisi rekening tujuan order (Mandiri).
    await page.getByRole("button", { name: "Catat pembayaran" }).first().click();
    await page.getByLabel("Metode").selectOption("transfer");
    await expect(page.getByLabel("Ke rekening")).toHaveValue(String(mandiri.accountId));
    await page.getByLabel("Referensi Transfer").fill("TRX-E26");
    await page.getByRole("button", { name: "Simpan pembayaran" }).click();
    // Tombol berganti "Menyimpan…" selama permintaan berjalan: tunggu pembayaran benar-benar tersimpan.
    const findTransfer = async () => {
      const pays = await ok<any[]>("admin", "GET", `/payments?orderType=sales&orderId=${orderId}`);
      return pays.find((x) => x.method === "transfer" && x.status === "active");
    };
    await expect.poll(findTransfer, { timeout: 10_000 }).toBeTruthy();
    expect(errors).toEqual([]);

    const transfer = await findTransfer();
    expect(transfer?.cashAccountId).toBe(mandiri.accountId);
    expect(transfer?.amount).toBe(total);
    const byAccount = await ok<any[]>("admin", "GET", `/payments?cashAccountId=${mandiri.accountId}`);
    expect(byAccount.map((x) => x.id)).toContain(transfer.id);
  });

  test("tutup harian menjurnal pembayaran itu ke akun rekening Mandiri, bukan 1010", async () => {
    await drainPosting(today());
    const [year, month] = today().split("-").map(Number);
    const sum = await ok<any>("akuntan", "GET", `/finance/reports/cash-summary?year=${year}&month=${month}`);
    const row = (code: string) => sum.rows.find((r: any) => r.code === code);
    expect(row("1012")?.balance).toBe(total);
  });

  test("transfer tanpa rekening ditolak bila tidak ada default", async () => {
    await ok("admin", "PUT", `/finance/bank-accounts/${bca.id}`, {
      bankName: "BCA", accountNumber: "1110001", accountHolder: "PT E2E", showOnDocuments: true, isDefault: false,
    });
    const customer = await partyByCode("customers", "C005");
    const p = await productByCode("P003");
    const so = await createSO("admin", customer.id, [{ p, qty: 1 }]);
    await confirmSO("admin", so.id);
    // Tagihan baru tanpa default: rekening tujuan kosong; transfer tanpa rekening ditolak (validasi 400).
    expect(so.receiptAccountId ?? 0).toBe(0);
    const denied = await call("admin", "POST", "/payments", {
      partyId: customer.id, amount: so.grandTotal, method: "transfer",
      allocations: [{ orderType: "sales", orderId: so.id, amount: so.grandTotal }],
    });
    expect(denied.status).toBe(400);
    expect(JSON.stringify(denied.errors)).toContain("cashAccountId");
    const okPay = await ok<any>("admin", "POST", "/payments", {
      partyId: customer.id, amount: so.grandTotal, method: "transfer", cashAccountId: bca.accountId,
      allocations: [{ orderType: "sales", orderId: so.id, amount: so.grandTotal }],
    });
    expect(okPay.cashAccountId).toBe(bca.accountId);
  });
});
