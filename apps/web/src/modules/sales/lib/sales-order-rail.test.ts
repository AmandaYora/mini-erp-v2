import { describe, expect, it } from "vitest";
import { billingView } from "@/modules/payment/lib/order-billing";
import type { HistoryEvent } from "@/modules/payment/lib/order-history";
import { salesOrderRail, salesOrderRailCurrent } from "@/modules/sales/lib/sales-order-rail";

const delivery = (status: string): HistoryEvent => ({
  key: "delivery:1",
  kind: "delivery",
  id: 1,
  number: "SJ-0001",
  date: "2026-10-02",
  status,
  href: "/deliveries/1",
});
const payment: HistoryEvent = {
  key: "payment:2",
  kind: "payment",
  id: 2,
  number: "PAY-0002",
  date: "2026-10-03",
  status: "active",
  amount: 100,
  href: "/payments/2",
};

function input(over: Partial<Parameters<typeof salesOrderRailCurrent>[0]> = {}) {
  return {
    status: "confirmed",
    channel: "regular",
    billing: billingView({ status: "confirmed", grandTotal: 100, paidAmount: 0, outstandingAmount: 100, paymentStatus: "unpaid" }),
    events: [] as HistoryEvent[],
    ...over,
  };
}

describe("rel dokumen order jual", () => {
  it("draf tetap di langkah draf", () => {
    expect(salesOrderRailCurrent(input({ status: "draft" }))).toBe("draft");
  });

  it("terkonfirmasi tanpa surat jalan terkonfirmasi tetap di Terkonfirmasi", () => {
    expect(salesOrderRailCurrent(input({ events: [delivery("draft")] }))).toBe("confirmed");
  });

  it("surat jalan terkonfirmasi memajukan ke Dikirim", () => {
    expect(salesOrderRailCurrent(input({ events: [delivery("confirmed")] }))).toBe("shipped");
  });

  it("dikirim dan lunas memajukan ke Lunas", () => {
    const billing = billingView({ status: "confirmed", grandTotal: 100, paidAmount: 100, outstandingAmount: 0, paymentStatus: "paid" });
    expect(salesOrderRailCurrent(input({ billing, events: [delivery("confirmed"), payment] }))).toBe("paid");
  });

  it("lunas tetapi belum dikirim (prabayar) tidak melompati Dikirim", () => {
    const billing = billingView({ status: "confirmed", grandTotal: 100, paidAmount: 100, outstandingAmount: 0, paymentStatus: "paid" });
    expect(salesOrderRailCurrent(input({ billing, events: [payment] }))).toBe("confirmed");
  });

  it("order POS dianggap sudah dikirim", () => {
    expect(salesOrderRailCurrent(input({ channel: "pos" }))).toBe("shipped");
  });

  it("selesai", () => {
    expect(salesOrderRailCurrent(input({ status: "completed" }))).toBe("done");
  });

  it("batal menghentikan rel dan memuat dokumen terkait sebagai chip", () => {
    const rail = salesOrderRail(input({ status: "cancelled", events: [delivery("cancelled")] }));
    expect(rail.stopped).toBe(true);
    expect(rail.current).toBe("confirmed");
    expect(rail.related).toEqual([
      { label: "SJ-0001", kind: "Surat jalan", to: "/deliveries/1", status: { label: "Dibatalkan", tone: "muted" } },
    ]);
  });

  it("order dari SJ Langsung: rel mulai dari Dikirim tanpa langkah Draf", () => {
    const rail = salesOrderRail(input({ origin: "delivery", status: "confirmed" }));
    expect(rail.steps.map((s) => s.key)).toEqual(["shipped", "confirmed", "paid", "done"]);
    // Barang sudah keluar sebelum order: minimal Dikirim walau belum ada SJ di riwayat.
    expect(rail.current).toBe("shipped");
    expect(salesOrderRailCurrent(input({ origin: "delivery", status: "completed" }))).toBe("done");
    expect(salesOrderRailCurrent(input({ origin: "delivery", status: "cancelled" }))).toBe("shipped");
  });
});
