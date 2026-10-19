import { test, expect } from "@playwright/test";
import {
  call, ok, productByCode, partyByCode, createSO, confirmSO, createDelivery, confirmDelivery, createPO, confirmPO,
  receiveBody, locationByCode, stockQty,
} from "../helpers/api";
import { loginAs } from "../helpers/ui";

test.describe.serial("E05 retur jual/beli + tukar barang", () => {
  let p: Awaited<ReturnType<typeof productByCode>>;
  let repl: typeof p;
  let customer: any;
  let supplier: any;
  let loc: any;
  let soId = 0;
  let returnNumber = "";

  test.beforeAll(async () => {
    p = await productByCode("P011"); // Deterjen 21.000
    repl = await productByCode("P012"); // Pasta Gigi 12.500
    customer = await partyByCode("customers", "C004");
    supplier = await partyByCode("suppliers", "S002");
    loc = await locationByCode("HQ-GA");
  });

  async function deliveredOrder(qty: number) {
    const so = await createSO("admin", customer.id, [{ p, qty }]);
    await confirmSO("admin", so.id);
    const d = await createDelivery("admin", so.id, [{ p, qty }]);
    await confirmDelivery("admin", d.id, { settlement: { mode: "pay_now", method: "cash", amountTendered: so.grandTotal } });
    return so;
  }

  const salesReturn = (orderId: number, qty: number, extra: Record<string, unknown> = {}) => ({
    salesOrderId: orderId, reason: "Barang tidak sesuai",
    items: [{ productId: p.id, variantId: p.variantId, locationId: loc.id, uom: "pcs", qty, condition: "normal" }],
    ...extra,
  });

  test("retur jual parsial lalu penuh; melebihi sisa ditolak", async () => {
    const so = await deliveredOrder(5);
    soId = so.id;
    const base = await stockQty(p.id);

    const r1 = await ok<any>("admin", "POST", "/sales-returns", salesReturn(soId, 2));
    returnNumber = r1.number;
    await ok("admin", "POST", `/sales-returns/${r1.id}/confirm`);
    expect(await stockQty(p.id)).toBe(base + 2);

    const over = await call("admin", "POST", "/sales-returns", salesReturn(soId, 4));
    expect(over.status).toBeGreaterThanOrEqual(400);

    const r2 = await ok<any>("admin", "POST", "/sales-returns", salesReturn(soId, 3));
    await ok("admin", "POST", `/sales-returns/${r2.id}/confirm`);
    expect(await stockQty(p.id)).toBe(base + 5);

    const none = await call("admin", "POST", "/sales-returns", salesReturn(soId, 1));
    expect(none.status).toBeGreaterThanOrEqual(400);
  });

  test("tukar barang: pengganti dikirim, stok pengganti keluar saat SJ dikonfirmasi", async () => {
    const so = await deliveredOrder(2);
    const baseP = await stockQty(p.id);
    const baseR = await stockQty(repl.id);
    const r = await ok<any>("admin", "POST", "/sales-returns", salesReturn(so.id, 1, {
      returnMode: "exchange",
      replacementItems: [{ productId: repl.id, variantId: repl.variantId, locationId: loc.id, uom: "pcs", qty: 1 }],
    }));
    await ok("admin", "POST", `/sales-returns/${r.id}/confirm`);
    expect(await stockQty(p.id)).toBe(baseP + 1);
    const view = await ok<any>("admin", "GET", `/sales-returns/${r.id}`);
    expect(view.returnMode).toBe("exchange");
    expect(view.replacementDeliveryStatus).toBe("pending");
    const dn = await ok<any>("admin", "POST", `/sales-returns/${r.id}/replacement-deliveries`, {});
    await ok("admin", "POST", `/sales-returns/${r.id}/replacement-deliveries/${dn.id}/confirm`, { recipientName: "Budi", recipientSignatureStatus: "signed" });
    expect(await stockQty(repl.id)).toBe(baseR - 1);
  });

  test("retur beli parsial lalu penuh; melebihi sisa ditolak", async () => {
    const po = await createPO("admin", supplier.id, [{ p, qty: 6, unitPrice: 16500 }]);
    await confirmPO("admin", po.id);
    await ok("gudang", "POST", "/goods-receipts", receiveBody(po.id, loc.id, [{ p, qty: 6 }]));
    const base = await stockQty(p.id);
    const body = (qty: number) => ({
      purchaseOrderId: po.id, reason: "Cacat produksi",
      items: [{ productId: p.id, variantId: p.variantId, locationId: loc.id, uom: "pcs", qty }],
    });
    const r1 = await ok<any>("admin", "POST", "/purchase-returns", body(2));
    await ok("admin", "POST", `/purchase-returns/${r1.id}/confirm`);
    expect(await stockQty(p.id)).toBe(base - 2);
    expect((await call("admin", "POST", "/purchase-returns", body(5))).status).toBeGreaterThanOrEqual(400);
    const r2 = await ok<any>("admin", "POST", "/purchase-returns", body(4));
    await ok("admin", "POST", `/purchase-returns/${r2.id}/confirm`);
    expect(await stockQty(p.id)).toBe(base - 6);
  });

  test("UI: retur jual tampil di daftar", async ({ page }) => {
    await loginAs(page, "admin");
    await page.goto("/sales-returns");
    await expect(page.getByText(returnNumber).first()).toBeVisible();
    await page.goto("/purchase-returns");
    await expect(page.getByRole("heading", { level: 1, name: "Retur Beli" })).toBeVisible();
  });
});
