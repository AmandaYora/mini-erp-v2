import { test, expect } from "@playwright/test";
import { call, ok, productByCode, partyByCode, confirmDelivery, stockQty, today, uuid } from "../helpers/api";

// SJ Langsung (ADR-0022, PLAN sj-langsung T-01): gudang mengirim lebih dulu tanpa order, admin
// membuat order dari dua surat jalan, akuntan menjurnal, dan satu surat jalan lain dibatalkan sebelum
// diorder. Seluruhnya lewat API (seed: admin = owner memegang delivery.create_direct; gudang hanya
// delivery.create, jadi ia mengonfirmasi tetapi tidak boleh membuat SJ Langsung).
test.describe.serial("E17 SJ Langsung -> order dari surat jalan -> jurnal", () => {
  let p: Awaited<ReturnType<typeof productByCode>>;
  let customer: any;
  let stock = 0;
  let sjA = 0;
  let sjB = 0;
  let sjCancel = 0;
  let orderId = 0;
  const clientRef = uuid();

  const directBody = (qty: number) => ({
    partyId: customer.id,
    deliveryDate: today(),
    driverName: "Sopir E17",
    warehouseStaffName: "Gudang E17",
    items: [{ productId: p.id, variantId: p.variantId, uom: "pcs", qty }],
  });

  test.beforeAll(async () => {
    p = await productByCode("P019");
    customer = await partyByCode("customers", "C004");
    stock = await stockQty(p.id);
  });

  test("gudang tidak boleh membuat SJ Langsung; admin boleh (dua SJ draf)", async () => {
    const denied = await call("gudang", "POST", "/deliveries/direct", directBody(1));
    expect(denied.status).toBe(403);

    const a = await ok<any>("admin", "POST", "/deliveries/direct", directBody(3));
    expect(a.origin).toBe("direct");
    expect(a.salesOrderId).toBe(0);
    expect(a.partyName).toBe(customer.name);
    expect(a.status).toBe("draft");
    sjA = a.id;
    const b = await ok<any>("admin", "POST", "/deliveries/direct", directBody(2));
    sjB = b.id;
    const c = await ok<any>("admin", "POST", "/deliveries/direct", directBody(1));
    sjCancel = c.id;
    expect(await stockQty(p.id)).toBe(stock); // draf tidak mengurangi stok
  });

  test("konfirmasi mengurangi stok; pelunasan ditolak; SJ masuk antrean sebagai menunggu order", async () => {
    await confirmDelivery("gudang", sjA, { recipientName: "Penerima A", recipientSignatureStatus: "signed" });
    await confirmDelivery("gudang", sjB, { recipientName: "Penerima B", recipientSignatureStatus: "signed" });
    await confirmDelivery("gudang", sjCancel, { recipientName: "Penerima C", recipientSignatureStatus: "signed" });
    expect(await stockQty(p.id)).toBe(stock - 6);

    const withSettlement = await call("gudang", "POST", `/deliveries/${sjA}/confirm`, {
      settlement: { mode: "switch_to_net", dueDate: today(14) },
    });
    expect(withSettlement.status).toBeGreaterThanOrEqual(400);

    const open = await ok<any>("admin", "GET", `/deliveries/open-direct?partyId=${customer.id}&limit=50`);
    expect(open.count).toBe(3);
    expect(open.items.map((n: any) => n.id)).toEqual(expect.arrayContaining([sjA, sjB, sjCancel]));

    const queue = await ok<any[]>("akuntan", "GET", "/finance/posting-sources");
    const row = queue.find((q) => q.docType === "delivery" && q.docId === sjA);
    expect(row?.awaitingOrder).toBe(true);
    const ignored = await call("akuntan", "POST", "/finance/posting/ignore", {
      docType: "delivery", docId: sjA, reason: "coba abaikan",
    });
    expect(ignored.status).toBeGreaterThanOrEqual(400);
  });

  test("pratinjau lalu order dari dua SJ: selesai, SJ tertaut, qty terkunci", async () => {
    const preview = await ok<any>("admin", "POST", "/sales-orders/from-deliveries/preview", {
      deliveryIds: [sjA, sjB], paymentTerms: "net", dueDate: today(14),
    });
    expect(preview.partyId).toBe(customer.id);
    expect(preview.items).toHaveLength(1);
    expect(preview.items[0].qty).toBe(5);
    expect(preview.grandTotal).toBe(5 * p.sellingPrice);

    const prepaid = await call("admin", "POST", "/sales-orders/from-deliveries/preview", {
      deliveryIds: [sjA], paymentTerms: "prepaid",
    });
    expect(prepaid.status).toBeGreaterThanOrEqual(400);

    const order = await ok<any>("admin", "POST", "/sales-orders/from-deliveries", {
      deliveryIds: [sjA, sjB], paymentTerms: "cod", clientRef,
      settlement: { mode: "switch_to_net", dueDate: today(14) },
    });
    expect(order.status).toBe("completed");
    expect(order.origin).toBe("delivery");
    expect(order.paymentTerms).toBe("net");
    expect(order.grandTotal).toBe(5 * p.sellingPrice);
    orderId = order.id;

    // ulang dengan clientRef sama = replay, bukan order kedua
    const again = await call("admin", "POST", "/sales-orders/from-deliveries", {
      deliveryIds: [sjA, sjB], paymentTerms: "cod", clientRef,
    });
    expect(again.status).toBe(200);
    expect(again.data.id).toBe(orderId);

    for (const id of [sjA, sjB]) {
      const n = await ok<any>("admin", "GET", `/deliveries/${id}`);
      expect(n.salesOrderId).toBe(orderId);
      expect(n.orderNumber).toBe(order.number);
      expect(n.linkedAt).not.toBe("");
    }
    // SJ yang sudah diorder tidak bisa dipakai lagi dan tidak bisa dibatalkan dari sini
    const reuse = await call("admin", "POST", "/sales-orders/from-deliveries", {
      deliveryIds: [sjA], paymentTerms: "net", dueDate: today(14), clientRef: uuid(),
    });
    expect(reuse.status).toBe(409);
    const cancelLinked = await call("admin", "POST", `/deliveries/${sjA}/cancel`, { reason: "coba" });
    expect(cancelLinked.status).toBe(409);
  });

  test("tutup harian menjurnal kedua SJ; kesiapan tidak lagi menyebut SJ belum diorder setelah SJ terakhir dibatalkan", async () => {
    const closed = await ok<any>("akuntan", "POST", "/finance/posting/close-day", { date: today() });
    const mine = (closed.results as any[]).filter((r) => r.docType === "delivery" && [sjA, sjB].includes(r.docId));
    expect(mine).toHaveLength(2);
    for (const r of mine) expect(r.error ?? "").toBe("");

    // SJ ketiga tidak jadi: dibatalkan beralasan setelah terkonfirmasi, stok kembali
    const cancelled = await ok<any>("admin", "POST", `/deliveries/${sjCancel}/cancel`, { reason: "pelanggan membatalkan" });
    expect(cancelled.status).toBe("cancelled");
    expect(cancelled.cancelReason).toBe("pelanggan membatalkan");
    expect(await stockQty(p.id)).toBe(stock - 5);

    const open = await ok<any>("admin", "GET", `/deliveries/open-direct?partyId=${customer.id}&limit=0`);
    expect(open.count).toBe(0);
    const [y, m] = today().split("-").map(Number);
    const ready = await ok<any>("akuntan", "GET", `/finance/close/readiness?year=${y}&month=${m}`);
    const check = (ready.checks as any[]).find((c) => c.key === "direct_unordered");
    expect(check?.status).toBe("ok");
  });
});
