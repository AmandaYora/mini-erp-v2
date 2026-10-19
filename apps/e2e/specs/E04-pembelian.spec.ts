import { test, expect } from "@playwright/test";
import {
  call, ok, productByCode, partyByCode, createPO, confirmPO, receiveBody, locationByCode, stockQty, pay, today,
} from "../helpers/api";
import { loginAs } from "../helpers/ui";

test.describe.serial("E04 PO -> GR -> bayar", () => {
  let p: Awaited<ReturnType<typeof productByCode>>;
  let supplier: any;
  let loc: any;
  let poId = 0;
  let poNumber = "";
  let base = 0;
  const total = 10 * 13500;

  test.beforeAll(async () => {
    p = await productByCode("P003");
    supplier = await partyByCode("suppliers", "S001");
    loc = await locationByCode("HQ-GA");
    base = await stockQty(p.id);
  });

  test("PO tempo dikonfirmasi; GR parsial menambah stok", async () => {
    const po = await createPO("admin", supplier.id, [{ p, qty: 10, unitPrice: 13500 }]);
    expect(po.grandTotal).toBe(total);
    poId = po.id;
    poNumber = po.number;
    await confirmPO("admin", poId);
    const gr = await ok<any>("gudang", "POST", "/goods-receipts", receiveBody(poId, loc.id, [{ p, qty: 4 }]));
    expect(gr.status).not.toBe("failed");
    expect(await stockQty(p.id)).toBe(base + 4);
  });

  test("over-receipt ditolak; sisa pas diterima", async () => {
    const over = await call("gudang", "POST", "/goods-receipts", receiveBody(poId, loc.id, [{ p, qty: 7 }]));
    expect(over.status).toBeGreaterThanOrEqual(400);
    expect(await stockQty(p.id)).toBe(base + 4);
    await ok("gudang", "POST", "/goods-receipts", receiveBody(poId, loc.id, [{ p, qty: 6 }]));
    expect(await stockQty(p.id)).toBe(base + 10);
  });

  test("bayar supplier sebagian lalu lunas; kelebihan ditolak", async () => {
    await pay("akuntan", supplier.id, 50_000, [{ orderType: "purchase", orderId: poId, amount: 50_000 }], "transfer");
    await pay("akuntan", supplier.id, total - 50_000, [{ orderType: "purchase", orderId: poId, amount: total - 50_000 }], "transfer");
    const over = await call("akuntan", "POST", "/payments", {
      partyId: supplier.id, amount: 1, method: "transfer",
      allocations: [{ orderType: "purchase", orderId: poId, amount: 1 }],
    });
    expect(over.status).toBeGreaterThanOrEqual(400);
  });

  test("posting antrean menjurnal GR dan neraca saldo tetap seimbang", async () => {
    await ok("akuntan", "POST", "/finance/posting/close-day", { date: today() });
    const tb = await ok<any>("akuntan", "GET", `/finance/reports/trial-balance?asOf=${today()}`);
    expect(tb.balanced).toBe(true);
  });

  test("PO prabayar wajib lunas sebelum penerimaan", async () => {
    const po = await createPO("admin", supplier.id, [{ p, qty: 2, unitPrice: 13500 }], { paymentTerms: "prepaid", dueDate: undefined });
    await confirmPO("admin", po.id);
    const early = await call("gudang", "POST", "/goods-receipts", receiveBody(po.id, loc.id, [{ p, qty: 2 }]));
    expect(early.status).toBeGreaterThanOrEqual(400);
    await pay("akuntan", supplier.id, 27_000, [{ orderType: "purchase", orderId: po.id, amount: 27_000 }], "transfer");
    await ok("gudang", "POST", "/goods-receipts", receiveBody(po.id, loc.id, [{ p, qty: 2 }]));
  });

  test("UI: PO tampil di daftar Order Beli", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto("/purchase-orders");
    await expect(page.getByText(poNumber).first()).toBeVisible();
    await page.goto("/goods-receipts");
    await expect(page.getByRole("heading", { level: 1, name: "Penerimaan" })).toBeVisible();
  });
});
