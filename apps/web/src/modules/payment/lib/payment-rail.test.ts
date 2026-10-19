import { describe, expect, it } from "vitest";
import { allocationHref, paymentRail } from "@/modules/payment/lib/payment-rail";

const alloc = { orderType: "sales", orderId: 1, orderNumber: "SO-1", amount: 100 };

describe("paymentRail", () => {
  it("tercatat tanpa alokasi", () => {
    const r = paymentRail({ status: "active", allocations: [] });
    expect(r.current).toBe("recorded");
    expect(r.stopped).toBe(false);
    expect(r.steps.map((s) => s.key)).toEqual(["recorded", "allocated"]);
  });

  it("dialokasikan bila ada alokasi", () => {
    expect(paymentRail({ status: "active", allocations: [alloc] }).current).toBe("allocated");
  });

  it("dibatalkan menambah langkah berhenti", () => {
    const r = paymentRail({ status: "cancelled", allocations: [alloc] });
    expect(r.current).toBe("cancelled");
    expect(r.stopped).toBe(true);
    expect(r.steps.map((s) => s.key)).toEqual(["recorded", "allocated", "cancelled"]);
  });
});

describe("allocationHref", () => {
  it("memetakan jenis dokumen ke rute detail", () => {
    expect(allocationHref("sales", 5)).toBe("/sales-orders/5");
    expect(allocationHref("purchase", 5)).toBe("/purchase-orders/5");
    expect(allocationHref("sales_return", 5)).toBe("/sales-returns/5");
    expect(allocationHref("purchase_return", 5)).toBe("/purchase-returns/5");
    expect(allocationHref("lain", 5)).toBeNull();
  });
});
