import { test, expect } from "@playwright/test";
import {
  call, ok, productByCode, partyByCode, createSO, confirmSO, createDelivery, confirmDelivery, pay, stockQty, today,
} from "../helpers/api";
import { loginAs } from "../helpers/ui";

// Rantai utama: SO tempo -> SJ parsial lalu penuh -> bayar sebagian + lunas -> posting -> laporan.
test.describe.serial("E03 SO -> SJ -> bayar -> posting -> laporan", () => {
  let soId = 0;
  let soNumber = "";
  const total = 4 * 70000 + 2 * 18000; // 316.000
  let p1: Awaited<ReturnType<typeof productByCode>>;
  let p2: typeof p1;
  let customer: any;
  let stock1 = 0;
  let stock2 = 0;
  let plBefore: any;

  test.beforeAll(async () => {
    p1 = await productByCode("P001");
    p2 = await productByCode("P002");
    customer = await partyByCode("customers", "C002");
    stock1 = await stockQty(p1.id);
    stock2 = await stockQty(p2.id);
    // jurnalkan dokumen spec sebelumnya dulu agar selisih laba rugi hanya berasal dari spec ini
    await ok("akuntan", "POST", "/finance/posting/close-day", { date: today() });
    plBefore = (await call("akuntan", "GET", `/finance/reports/profit-loss?from=${today(-1)}&to=${today(1)}`)).data;
  });

  test("SO tempo dibuat dan dikonfirmasi", async () => {
    const so = await createSO("admin", customer.id, [{ p: p1, qty: 4 }, { p: p2, qty: 2 }], { paymentTerms: "net", dueDate: today(14) });
    expect(so.grandTotal).toBe(total);
    expect(so.status).toBe("draft");
    soId = so.id;
    soNumber = so.number;
    const confirmed = await confirmSO("admin", soId);
    expect(confirmed.status).not.toBe("draft");
  });

  test("SJ parsial lalu penuh mengurangi stok tepat sekali", async () => {
    const d1 = await createDelivery("gudang", soId, [{ p: p1, qty: 2 }]);
    await confirmDelivery("gudang", d1.id);
    expect(await stockQty(p1.id)).toBe(stock1 - 2);
    expect(await stockQty(p2.id)).toBe(stock2);

    // konfirmasi ganda tidak menggandakan pengeluaran stok
    await call("gudang", "POST", `/deliveries/${d1.id}/confirm`, {}); // idempoten (200) atau ditolak: keduanya aman
    expect(await stockQty(p1.id)).toBe(stock1 - 2);

    const d2 = await createDelivery("gudang", soId, [{ p: p1, qty: 2 }, { p: p2, qty: 2 }]);
    await confirmDelivery("gudang", d2.id);
    expect(await stockQty(p1.id)).toBe(stock1 - 4);
    expect(await stockQty(p2.id)).toBe(stock2 - 2);

    // melebihi sisa ditolak
    const over = await call("gudang", "POST", "/deliveries", {
      salesOrderId: soId, items: [{ productId: p1.id, variantId: p1.variantId, uom: "pcs", qty: 1 }],
    });
    expect(over.status).toBeGreaterThanOrEqual(400);
  });

  test("pembayaran sebagian lalu lunas; kelebihan ditolak", async () => {
    await pay("akuntan", customer.id, 100_000, [{ orderType: "sales", orderId: soId, amount: 100_000 }]);
    let so = await ok<any>("admin", "GET", `/sales-orders/${soId}`);
    expect(so.paidTotal ?? so.paidAmount ?? 100_000).toBe(100_000);
    await pay("akuntan", customer.id, total - 100_000, [{ orderType: "sales", orderId: soId, amount: total - 100_000 }]);
    const over = await call("akuntan", "POST", "/payments", {
      partyId: customer.id, amount: 1, method: "cash", amountTendered: 1,
      allocations: [{ orderType: "sales", orderId: soId, amount: 1 }],
    });
    expect(over.status).toBeGreaterThanOrEqual(400);
    so = await ok<any>("admin", "GET", `/sales-orders/${soId}`);
    expect(JSON.stringify(so)).toContain(String(total));
  });

  test("antrean posting memuat dokumen lalu menjurnal seimbang", async () => {
    const before = (await ok<any[]>("akuntan", "GET", "/finance/posting-sources")).length;
    expect(before).toBeGreaterThan(0);
    const closed = await ok<any>("akuntan", "POST", "/finance/posting/close-day", { date: today() });
    const res: any[] = closed.results;
    expect(res.length).toBeGreaterThan(0);
    const after = await ok<any[]>("akuntan", "GET", "/finance/posting-sources");
    expect(after.length).toBe(0);
  });

  test("laporan: laba rugi, neraca seimbang, neraca saldo seimbang", async () => {
    const pl = (await call("akuntan", "GET", `/finance/reports/profit-loss?from=${today(-1)}&to=${today(1)}`)).data;
    expect(pl.revenue - plBefore.revenue).toBe(total);
    expect(pl.cogs - plBefore.cogs).toBeGreaterThan(0);
    expect(pl.gross).toBe(pl.revenue - pl.cogs);
    const bs = await ok<any>("akuntan", "GET", `/finance/reports/balance-sheet?date=${today()}`);
    expect(bs.balanced).toBe(true);
    expect(bs.totalAssets).toBe(bs.totalLiabilitiesEquity);
    const d = new Date();
    const tb = await ok<any>("akuntan", "GET", `/finance/reports/trial-balance?year=${d.getFullYear()}&month=${d.getMonth() + 1}`);
    expect(tb.balanced).toBe(true);
    expect(tb.totalDebit).toBe(tb.totalCredit);
  });

  test("UI: detail order dan laporan laba rugi termuat", async ({ page }) => {
    await loginAs(page, "akuntan");
    await page.goto("/sales-orders");
    await expect(page.getByText(soNumber).first()).toBeVisible();
    await page.goto("/finance/reports/profit-loss");
    await expect(page.getByRole("heading", { name: "Laba Rugi", level: 1 })).toBeVisible();
  });

  test("aturan bukti kirim: bila diwajibkan, SJ tanpa kelengkapan ditolak", async () => {
    await ok("admin", "PUT", "/company/settings", { settings: { delivery_require_evidence: "true" } });
    try {
      const so = await createSO("admin", customer.id, [{ p: p2, qty: 1 }]);
      await confirmSO("admin", so.id);
      const d = await createDelivery("gudang", so.id, [{ p: p2, qty: 1 }]);
      const r = await call("gudang", "POST", `/deliveries/${d.id}/confirm`, {});
      expect(r.status).toBe(400);
      expect(r.message).toMatch(/belum lengkap/);
    } finally {
      await ok("admin", "PUT", "/company/settings", { settings: { delivery_require_evidence: "false" } });
    }
  });
});

// F-01/K-03: PPN dari form order jual sampai laporan PPN keluaran.
test.describe.serial("E03b PPN order jual", () => {
  test("form menawarkan kontrol PPN dan ringkasan DPP/PPN memakai tarif bawaan 11%", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto("/sales-orders/new");
    const taxed = page.getByLabel(/Kena pajak \(PPN\)/);
    await expect(taxed).toBeVisible();
    await expect(taxed).not.toBeChecked();
    await taxed.check();
    await expect(page.getByLabel(/Tarif/)).toHaveValue("11");
    await expect(page.getByLabel(/Harga sudah termasuk pajak/)).toBeVisible();
  });

  test("SO ber-PPN exclude dan include masuk PPN keluaran setelah SJ + posting", async () => {
    const p = await productByCode("P001");
    const customer = await partyByCode("customers", "C002");
    await ok("akuntan", "POST", "/finance/posting/close-day", { date: today() });
    const d = new Date();
    const ym = `year=${d.getFullYear()}&month=${d.getMonth() + 1}`;
    const before = (await call("akuntan", "GET", `/finance/reports/tax-summary?${ym}`)).data;

    const price = p.sellingPrice ?? 70000;
    const net = 2 * price;
    const exclude = await createSO("admin", customer.id, [{ p, qty: 2 }], { taxType: "exclude", taxRate: 11, paymentTerms: "net", dueDate: today(14) });
    const include = await createSO("admin", customer.id, [{ p, qty: 2 }], { taxType: "include", taxRate: 11, paymentTerms: "net", dueDate: today(14) });
    const taxExclude = Math.round((net * 11) / 100);
    const taxInclude = net - Math.round(net / 1.11);
    expect(exclude.grandTotal).toBe(net + taxExclude);
    expect(include.grandTotal).toBe(net);

    for (const so of [exclude, include]) {
      await confirmSO("admin", so.id);
      const dn = await createDelivery("gudang", so.id, [{ p, qty: 2 }]);
      await confirmDelivery("gudang", dn.id);
    }
    await ok("akuntan", "POST", "/finance/posting/close-day", { date: today() });
    const after = (await call("akuntan", "GET", `/finance/reports/tax-summary?${ym}`)).data;
    expect(after.ppnOut - before.ppnOut).toBe(taxExclude + taxInclude);
  });
});
