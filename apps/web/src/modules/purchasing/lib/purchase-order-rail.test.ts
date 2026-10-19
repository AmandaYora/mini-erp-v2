import { describe, expect, it } from "vitest";
import { purchaseOrderRail } from "@/modules/purchasing/lib/purchase-order-rail";

describe("purchaseOrderRail", () => {
  it("memetakan siklus normal dan dokumen terkait", () => {
    const rail = purchaseOrderRail("confirmed", [
      { kind: "receipt", number: "GR-1", status: "posted", href: "/goods-receipts/1" },
      { kind: "payment", number: "PAY-2", status: "active", href: "/payments/2" },
    ]);
    expect(rail.steps.map((s) => s.key)).toEqual(["draft", "confirmed", "completed"]);
    expect(rail.current).toBe("confirmed");
    expect(rail.stopped).toBe(false);
    expect(rail.related).toHaveLength(2);
    expect(rail.related[0]).toMatchObject({ label: "GR-1", kind: "Penerimaan", to: "/goods-receipts/1" });
    expect(rail.related[0].status?.tone).toBe("success");
  });

  it("order batal berhenti di langkah Dibatalkan", () => {
    const rail = purchaseOrderRail("cancelled", []);
    expect(rail.current).toBe("cancelled");
    expect(rail.stopped).toBe(true);
    expect(rail.steps.map((s) => s.key)).toEqual(["draft", "cancelled"]);
  });

  it("melewati dokumen tanpa nomor atau tautan", () => {
    const rail = purchaseOrderRail("completed", [{ kind: "receipt", number: "", status: "posted", href: "/x" }]);
    expect(rail.related).toHaveLength(0);
  });
});
