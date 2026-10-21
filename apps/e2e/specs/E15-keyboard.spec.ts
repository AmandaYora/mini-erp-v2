import { test, expect, type Locator, type Page } from "@playwright/test";
import { createSO, ok, partyByCode, productByCode, stockQty, today } from "../helpers/api";
import { loginAs } from "../helpers/ui";

// U7-2 (docs/plan/redesain-ui/PLAN.md §9 Fase 7): tiga alur back-office dikerjakan TANPA tetikus.
// Aturan spec ini: tidak ada .click()/.hover(); fokus hanya berpindah lewat Tab/Shift+Tab, panah, Enter,
// Spasi, dan Esc. `tabTo` sekaligus membuktikan tiap kontrol terjangkau lewat Tab (urutan fokus wajar).
// `fill`/`selectOption` hanya dipakai pada kontrol yang SUDAH difokus lewat Tab (setara mengetik).

const MAX_TABS = 200;

/** Tekan Tab sampai `target` memegang fokus; gagal bila tak terjangkau. */
async function tabTo(page: Page, target: Locator, label = String(target)): Promise<void> {
  await expect(target).toBeVisible();
  for (let i = 0; i < MAX_TABS; i++) {
    if (await target.evaluate((el) => el === document.activeElement || el.contains(document.activeElement))) return;
    await page.keyboard.press("Tab");
  }
  const focused = await page.evaluate(() => {
    const el = document.activeElement as HTMLElement | null;
    return el ? `${el.tagName.toLowerCase()} "${(el.getAttribute("aria-label") ?? el.textContent ?? "").trim().slice(0, 60)}"` : "tidak ada";
  });
  throw new Error(`tidak terjangkau dengan Tab: ${label} (fokus terakhir: ${focused})`);
}

/** Buka pemilih (Enter), ketik, lalu Enter memilih hasil pertama yang cocok. */
async function pick(page: Page, trigger: Locator, text: string, label: string): Promise<void> {
  await tabTo(page, trigger, label);
  await page.keyboard.press("Enter");
  await page.keyboard.type(text);
  await expect(page.getByRole("option", { name: new RegExp(text) }).first()).toBeVisible();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("listbox")).toHaveCount(0);
}

/** Fokus di awal dokumen: setelah navigasi, Tab pertama mulai dari atas halaman. */
async function fromTop(page: Page) {
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
}

test.describe.serial("E15 keyboard saja (U7-2)", () => {
  test("buat order jual -> terbitkan surat jalan -> catat pembayaran", async ({ page }) => {
    test.setTimeout(180_000);
    const p = await productByCode("P012");
    const c = await partyByCode("customers", "C004");
    const before = await stockQty(p.id);
    await loginAs(page, "admin");

    // 1. Order jual tempo dengan satu produk qty 3.
    await page.goto("/sales-orders/new");
    await fromTop(page);
    await pick(page, page.getByRole("button", { name: /Cari pelanggan/ }), c.name, "pemilih pelanggan");
    const terms = page.getByLabel("Termin");
    await tabTo(page, terms, "Termin");
    await terms.selectOption("net");
    const due = page.getByLabel("Jatuh tempo");
    await tabTo(page, due, "Jatuh tempo");
    await due.fill(today(14));
    await pick(page, page.getByLabel("Tambah produk"), p.code, "Tambah produk");
    const qty = page.getByLabel(new RegExp(`^Qty .*${p.name}`)).first();
    await tabTo(page, qty, "Qty");
    await qty.fill("3");
    await tabTo(page, page.getByRole("button", { name: "Simpan order" }), "Simpan order");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/sales-orders\/\d+$/);
    const soId = Number(page.url().split("/").pop());

    // 2. Konfirmasi lalu terbitkan surat jalan dari detail order.
    await fromTop(page);
    await tabTo(page, page.getByRole("button", { name: "Konfirmasi order" }), "Konfirmasi order");
    await page.keyboard.press("Enter");
    const issue = page.getByRole("link", { name: "Terbitkan surat jalan" });
    await expect(issue).toBeVisible();
    await fromTop(page);
    await tabTo(page, issue, "Terbitkan surat jalan");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/deliveries\/new/);
    await fromTop(page);
    await tabTo(page, page.getByRole("button", { name: /Simpan|Buat surat jalan|Terbitkan/ }).last(), "simpan surat jalan");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/deliveries\/\d+$/);

    // 3. Konfirmasi surat jalan (stok keluar).
    await fromTop(page);
    await tabTo(page, page.getByRole("button", { name: /Konfirmasi/ }).first(), "Konfirmasi surat jalan");
    await page.keyboard.press("Enter");
    const confirm = page.getByRole("dialog", { name: "Konfirmasi Surat Jalan" });
    await expect(confirm).toBeVisible();
    const recipient = confirm.getByLabel(/Nama Penerima/);
    await tabTo(page, recipient, "Nama Penerima");
    await recipient.fill("Penerima Keyboard");
    const sig = confirm.getByLabel(/Status Tanda Tangan/);
    await tabTo(page, sig, "Status Tanda Tangan");
    await sig.selectOption("signed");
    await tabTo(page, confirm.getByRole("button", { name: "Konfirmasi" }), "Konfirmasi (dialog)");
    await page.keyboard.press("Enter");
    await expect(confirm).toBeHidden();
    await expect.poll(() => stockQty(p.id)).toBe(before - 3);

    // 4. Kembali ke order dan catat pembayaran penuh lewat sheet.
    await page.goto(`/sales-orders/${soId}`);
    await fromTop(page);
    await tabTo(page, page.getByRole("button", { name: "Catat pembayaran" }).last(), "Catat pembayaran");
    await page.keyboard.press("Enter");
    const sheet = page.getByRole("dialog");
    await expect(sheet).toBeVisible();
    await tabTo(page, sheet.getByRole("button", { name: /Simpan|Catat/ }).last(), "simpan pembayaran");
    await page.keyboard.press("Enter");
    await expect(sheet).toBeHidden();
    const so = await ok<any>("admin", "GET", `/sales-orders/${soId}`);
    expect(so.paymentStatus ?? (so.outstandingAmount === 0 ? "paid" : "?")).toBe("paid");
  });

  test("buat order beli -> konfirmasi -> terima barang", async ({ page }) => {
    test.setTimeout(180_000);
    const p = await productByCode("P014");
    const supplier = await partyByCode("suppliers", "S002");
    const before = await stockQty(p.id);
    await loginAs(page, "admin");

    await page.goto("/purchase-orders/new");
    await fromTop(page);
    await pick(page, page.getByRole("button", { name: /^Pemasok/ }), supplier.name, "pemilih pemasok");
    const due = page.getByLabel(/^Jatuh tempo/);
    if (await due.isVisible()) {
      await tabTo(page, due, "Jatuh tempo");
      await due.fill(today(30));
    }
    await pick(page, page.getByRole("button", { name: "Produk baris 1" }), p.code, "Produk baris 1");
    const qty = page.getByLabel("Qty baris 1");
    await tabTo(page, qty, "Qty baris 1");
    await qty.fill("5");
    const price = page.getByLabel("Harga satuan baris 1");
    await tabTo(page, price, "Harga satuan baris 1");
    await price.fill("12000");
    // Enter pada pemilih produk tidak boleh ikut menambah baris kosong (event portal merambat).
    await expect(page.getByLabel("Qty baris 2")).toHaveCount(0);
    await tabTo(page, page.getByRole("button", { name: "Buat order" }), "Buat order");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/purchase-orders\/\d+$/);

    await fromTop(page);
    await tabTo(page, page.getByRole("button", { name: "Konfirmasi order" }), "Konfirmasi order");
    await page.keyboard.press("Enter");
    const confirm = page.getByRole("alertdialog");
    await expect(confirm).toBeVisible();
    await tabTo(page, confirm.getByRole("button", { name: /Konfirmasi/ }), "Konfirmasi (dialog)");
    await page.keyboard.press("Enter");
    await expect(confirm).toBeHidden();

    const receive = page.getByRole("button", { name: "Terima barang" });
    await expect(receive).toBeVisible();
    await fromTop(page);
    await tabTo(page, receive, "Terima barang");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/goods-receipts\/new/);
    const received = page.getByLabel(new RegExp(`^Qty terima .*${p.name}`)).first();
    await expect(received).toBeVisible();
    await tabTo(page, received, "Qty terima");
    await received.fill("5");
    await pick(page, page.getByRole("button", { name: `Lokasi ${p.name}` }), "HQ-GA", "Lokasi");
    await tabTo(page, page.getByRole("button", { name: "Simpan penerimaan" }), "Simpan penerimaan");
    await page.keyboard.press("Enter");
    await expect.poll(() => stockQty(p.id)).toBe(before + 5);
  });

  test("posting: tutup hari dari antrean", async ({ page }) => {
    await loginAs(page, "akuntan");
    await page.goto("/finance/posting");
    await expect(page.getByRole("heading", { level: 1, name: "Posting" })).toBeVisible();
    await fromTop(page);
    // Pola tab WAI-ARIA: hanya tab aktif yang ada di urutan Tab; pindah tab dengan panah.
    await tabTo(page, page.getByRole("tab", { name: "Antrian" }), "tab Antrian");
    await page.keyboard.press("ArrowRight");
    await expect(page.getByRole("tab", { name: "Tutup hari" })).toHaveAttribute("aria-selected", "true");
    const closeDay = page.getByRole("button", { name: /^Tutup hari \(\d+\)$/ });
    await expect(closeDay).toBeEnabled();
    await tabTo(page, closeDay, "Tutup hari");
    await page.keyboard.press("Enter");
    const confirm = page.getByRole("alertdialog");
    await expect(confirm).toBeVisible();
    await tabTo(page, confirm.getByRole("button", { name: /^Posting \d+ dokumen$/ }), "Posting (dialog)");
    await page.keyboard.press("Enter");
    await expect(confirm).toBeHidden();
    await expect.poll(async () => (await ok<any[]>("akuntan", "GET", "/finance/posting-sources")).length).toBe(0);
  });

  test("command palette: Ctrl+K lalu ketik nomor dokumen membuka detailnya (U6-2)", async ({ page }) => {
    const p = await productByCode("P016");
    const c = await partyByCode("customers", "C005");
    const so = await createSO("admin", c.id, [{ p, qty: 1 }]);
    await loginAs(page, "admin");
    await page.goto("/dashboard");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.keyboard.press("Control+k");
    const input = page.getByRole("combobox");
    await expect(input).toBeFocused();
    await page.keyboard.type(so.number);
    const hit = page.getByRole("option", { name: new RegExp(so.number.replace(/[/]/g, "\\/")) });
    await expect(hit).toBeVisible();
    await expect(hit).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/sales-orders/${so.id}$`));
  });
});
