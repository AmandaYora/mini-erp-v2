import { describe, expect, it } from "vitest";
import { salesReturnRail } from "@/modules/salesreturn/lib/sales-return-rail";

const base = { status: "confirmed", returnMode: "return_only", replacementDeliveryStatus: "not_required", settledTotal: 0, unsettledTotal: 100, total: 100 };

describe("salesReturnRail", () => {
  it("draf", () => {
    expect(salesReturnRail({ ...base, status: "draft" }).current).toBe("draft");
  });
  it("terkonfirmasi belum diselesaikan = barang kembali", () => {
    expect(salesReturnRail(base).current).toBe("returned");
  });
  it("selesai bila seluruh nilai sudah diselesaikan", () => {
    expect(salesReturnRail({ ...base, settledTotal: 100, unsettledTotal: 0 }).current).toBe("settled");
  });
  it("tukar barang menambah langkah pengganti dan menahan sampai diterima", () => {
    const ex = { ...base, returnMode: "exchange", replacementDeliveryStatus: "dispatched", settledTotal: 100, unsettledTotal: 0 };
    const r = salesReturnRail(ex);
    expect(r.steps.map((s) => s.key)).toEqual(["draft", "returned", "replacement", "settled"]);
    expect(r.current).toBe("replacement");
    expect(salesReturnRail({ ...ex, replacementDeliveryStatus: "confirmed" }).current).toBe("settled");
  });
  it("selesai menurut posisi server: tak ada sisa refund/saldo maupun selisih yang belum ditagih", () => {
    expect(salesReturnRail({ ...base, externalRemaining: 0, chargeRemaining: 0 }).current).toBe("settled");
    expect(salesReturnRail({ ...base, externalRemaining: 40, chargeRemaining: 0 }).current).toBe("returned");
    expect(salesReturnRail({ ...base, externalRemaining: 0, chargeRemaining: 25 }).current).toBe("returned");
  });
  it("dibatalkan berhenti", () => {
    const r = salesReturnRail({ ...base, status: "cancelled" });
    expect(r.stopped).toBe(true);
    expect(r.current).toBe("cancelled");
  });
});
