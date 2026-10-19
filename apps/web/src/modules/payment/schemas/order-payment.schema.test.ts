import { describe, expect, it } from "vitest";
import { buildOrderPaymentSchema, toOrderPaymentPayload } from "@/modules/payment/schemas/order-payment.schema";

const ctx = { partyId: 0, orderType: "sales" as const, orderId: 9, clientRef: "ref-1" };
const ok = { amount: "50000", method: "cash", paidAt: "2026-10-09", reference: "", amountTendered: "", notes: "" };

describe("orderPaymentSchema", () => {
  it("nominal tidak boleh melebihi sisa", () => {
    const s = buildOrderPaymentSchema(40000);
    const r = s.safeParse(ok);
    expect(r.success).toBe(false);
    expect(buildOrderPaymentSchema(50000).safeParse(ok).success).toBe(true);
  });

  it("uang diterima hanya cash dan >= nominal", () => {
    const s = buildOrderPaymentSchema(100000);
    expect(s.safeParse({ ...ok, amountTendered: "40000" }).success).toBe(false);
    expect(s.safeParse({ ...ok, method: "transfer", amountTendered: "60000" }).success).toBe(false);
    expect(s.safeParse({ ...ok, amountTendered: "60000" }).success).toBe(true);
  });

  it("payload: satu alokasi ke order, walk-in partyId 0, tunai membawa tendered", () => {
    const s = buildOrderPaymentSchema(100000);
    const r = s.parse({ ...ok, amountTendered: "60000" });
    expect(toOrderPaymentPayload(r, ctx)).toEqual({
      partyId: 0,
      amount: 50000,
      method: "cash",
      clientRef: "ref-1",
      paidAt: "2026-10-09",
      amountTendered: 60000,
      allocations: [{ orderType: "sales", orderId: 9, amount: 50000 }],
    });
  });

  it("transfer membawa referensi, bukan tendered", () => {
    const s = buildOrderPaymentSchema(100000);
    const r = s.parse({ ...ok, method: "transfer", reference: "TRX-1" });
    const p = toOrderPaymentPayload(r, { ...ctx, partyId: 3, orderType: "purchase" });
    expect(p.reference).toBe("TRX-1");
    expect(p.amountTendered).toBeUndefined();
    expect(p.allocations?.[0].orderType).toBe("purchase");
  });
});
