import { describe, expect, it } from "vitest";
import { deliveryRail, deliveryRailCurrent } from "@/modules/delivery/lib/delivery-rail";

describe("rel dokumen surat jalan", () => {
  it("draf tanpa waktu berangkat tetap di Draf", () => {
    expect(deliveryRailCurrent({ status: "draft", dispatchedAt: null })).toBe("draft");
  });

  it("draf yang sudah diberangkatkan ada di Dikirim", () => {
    expect(deliveryRailCurrent({ status: "draft", dispatchedAt: "2026-10-02T08:00:00Z" })).toBe("dispatched");
  });

  it("terkonfirmasi = Diterima", () => {
    expect(deliveryRailCurrent({ status: "confirmed", dispatchedAt: null })).toBe("received");
  });

  it("batal menghentikan rel; dokumen terkait memuat order dan retur", () => {
    const rail = deliveryRail({
      status: "cancelled",
      dispatchedAt: null,
      salesOrderId: 7,
      orderNumber: "SO-0007",
      orderStatus: "confirmed",
      salesReturnId: 3,
    });
    expect(rail.stopped).toBe(true);
    expect(rail.related).toEqual([
      { label: "SO-0007", kind: "Order jual", to: "/sales-orders/7", status: { label: "Terkonfirmasi", tone: "info" } },
      { label: "Retur #3", kind: "Retur jual", to: "/sales-returns/3" },
    ]);
  });
});
