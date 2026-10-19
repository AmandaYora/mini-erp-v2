import { describe, expect, it } from "vitest";
import {
  addDays,
  buildActionItems,
  compactIDR,
  greetingForHour,
  greetingTitle,
  overdueDays,
  statusSegments,
} from "@/modules/dashboard/lib/dashboard";
import type { CriticalStockItem, PriorityOrder } from "@/modules/dashboard/types";

describe("lib dasbor", () => {
  it("addDays menggeser kalender tanpa terpengaruh zona waktu", () => {
    expect(addDays("2026-10-16", -6)).toBe("2026-10-10");
    expect(addDays("2026-03-01", -1)).toBe("2026-02-28");
  });

  it("compactIDR meringkas rb/jt/M", () => {
    expect(compactIDR(250_000)).toBe("250 rb");
    expect(compactIDR(1_500_000)).toBe("2 jt");
    expect(compactIDR(2_500_000_000)).toBe("2.5 M");
    expect(compactIDR(0)).toBe("0");
  });

  it("overdueDays positif bila lewat tempo", () => {
    expect(overdueDays("2026-10-14T00:00:00Z", "2026-10-16")).toBe(2);
    expect(overdueDays("2026-10-18", "2026-10-16")).toBe(-2);
  });

  it("sapaan mengikuti jam WIB dan nama depan", () => {
    expect(greetingForHour(8)).toBe("Selamat pagi");
    expect(greetingForHour(12)).toBe("Selamat siang");
    expect(greetingForHour(16)).toBe("Selamat sore");
    expect(greetingForHour(22)).toBe("Selamat malam");
    // 2026-10-16T03:00Z = 10:00 WIB
    const t = Date.UTC(2026, 9, 16, 3, 0, 0);
    expect(greetingTitle("Rina Wulandari", t)).toBe("Selamat pagi, Rina");
    expect(greetingTitle("", t)).toBe("Selamat pagi");
  });

  it("statusSegments menghitung persen dan aman untuk total 0", () => {
    const { total, segments } = statusSegments({ draft: 1, confirmed: 1, completed: 2 });
    expect(total).toBe(4);
    expect(segments.map((s) => s.percent)).toEqual([25, 25, 50, 0]);
    expect(statusSegments(null).total).toBe(0);
    expect(statusSegments(null).segments.every((s) => s.percent === 0)).toBe(true);
  });

  it("buildActionItems menggabungkan SO & stok, yang mendesak lebih dulu, dengan tautan terfilter", () => {
    const orders: PriorityOrder[] = [
      { id: 1, number: "SO-1", grandTotal: 10, status: "confirmed", partyId: 1, partyName: "", orderDate: "2026-10-01", dueDate: "2026-10-18", score: 0 },
      { id: 2, number: "SO-2", grandTotal: 10, status: "confirmed", partyId: 1, partyName: "Budi", orderDate: "2026-10-01", dueDate: "2026-10-14", score: 0 },
    ];
    const stock: CriticalStockItem[] = [{ productId: 7, productCode: "P7", productName: "Gula", available: 0, minStock: 5 }];
    const items = buildActionItems(orders, stock, "2026-10-16");
    expect(items.map((i) => i.key)).toEqual(["so-2", "stk-7", "so-1"]);
    expect(items[0].detail).toBe("Terlambat 2 hari");
    expect(items[0].to).toBe("/sales-orders?search=SO-2");
    expect(items[1].to).toBe("/stock/items/7");
    expect(items[2].title).toBe("SO-1 · Tunai");
    expect(buildActionItems(null, null, "2026-10-16")).toEqual([]);
  });

  it("buildActionItems memuat PO dengan alasan dan umur (DB-01)", () => {
    const orders: PriorityOrder[] = [
      { id: 5, number: "PO-5", grandTotal: 10, status: "confirmed", partyId: 1, partyName: "Sup A", orderDate: "2026-09-01", dueDate: "2026-10-10", score: 130, kind: "purchase", reason: "Lewat jatuh tempo", ageDays: 45 },
      { id: 6, number: "PO-6", grandTotal: 10, status: "draft", partyId: 1, partyName: "", orderDate: "2026-10-12", dueDate: "", score: 64, kind: "purchase", reason: "Menunggu keputusan", ageDays: 4 },
    ];
    const items = buildActionItems(orders, [], "2026-10-16");
    expect(items[0].key).toBe("po-5");
    expect(items[0].detail).toBe("Pembelian: Lewat jatuh tempo · terlambat 6 hari · umur 45 hari");
    expect(items[0].to).toBe("/purchase-orders?search=PO-5");
    expect(items[1].detail).toBe("Pembelian: Menunggu keputusan · umur 4 hari");
    expect(items[1].title).toBe("PO-6 · Supplier");
    expect(items[1].tone).toBe("warning");
  });
});
