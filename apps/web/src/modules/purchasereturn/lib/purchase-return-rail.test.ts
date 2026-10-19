import { describe, expect, it } from "vitest";
import { purchaseReturnRail } from "@/modules/purchasereturn/lib/purchase-return-rail";

const base = { status: "confirmed", settledTotal: 0, unsettledTotal: 50, total: 50 };

describe("purchaseReturnRail", () => {
  it("draf, barang keluar, selesai", () => {
    expect(purchaseReturnRail({ ...base, status: "draft" }).current).toBe("draft");
    expect(purchaseReturnRail(base).current).toBe("returned");
    expect(purchaseReturnRail({ ...base, settledTotal: 50, unsettledTotal: 0 }).current).toBe("settled");
  });
  it("selesai bila tak ada kelebihan lagi menurut server (offset menyerap seluruh retur)", () => {
    expect(purchaseReturnRail({ ...base, externalRemaining: 0 }).current).toBe("settled");
    expect(purchaseReturnRail({ ...base, externalRemaining: 20 }).current).toBe("returned");
  });
  it("dibatalkan berhenti", () => {
    const r = purchaseReturnRail({ ...base, status: "cancelled" });
    expect(r.stopped).toBe(true);
    expect(r.steps.at(-1)?.key).toBe("cancelled");
  });
});
