import { describe, expect, it } from "vitest";
import { goodsReceiptRail } from "@/modules/goodsreceipt/lib/goods-receipt-rail";

describe("goodsReceiptRail", () => {
  it("receipt pending berada di langkah awal dan menaut ke PO", () => {
    const rail = goodsReceiptRail("pending", { id: 7, number: "PO-7", status: "confirmed" }, "/purchase-orders");
    expect(rail.current).toBe("pending");
    expect(rail.stopped).toBe(false);
    expect(rail.related[0]).toMatchObject({ label: "PO-7", to: "/purchase-orders/7" });
    expect(rail.related[0].status?.label).toBe("Terkonfirmasi");
  });

  it("receipt posted = Diterima", () => {
    const rail = goodsReceiptRail("posted", null, "/purchase-orders");
    expect(rail.current).toBe("posted");
    expect(rail.related).toHaveLength(0);
  });

  it("receipt void berhenti di Dibatalkan", () => {
    const rail = goodsReceiptRail("void", { id: 1, number: "PO-1" }, "/purchase-orders");
    expect(rail.stopped).toBe(true);
    expect(rail.steps.map((s) => s.key)).toEqual(["pending", "void"]);
    expect(rail.related[0].status).toBeUndefined();
  });
});
