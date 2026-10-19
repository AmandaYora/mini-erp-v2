import { describe, expect, it } from "vitest";
import {
  normalizeOrderReport,
  normalizeOrderRow,
  orderDetailPath,
  validRange,
} from "@/modules/reporting/lib/order-report";

describe("order-report", () => {
  it("normalizes PascalCase rows", () => {
    const row = normalizeOrderRow({ ID: 3, Number: "SO-1", GrandTotal: 500, Status: "completed", PartyID: 2, PartyName: "A", OrderDate: "2026-01-01", DueDate: "" });
    expect(row).toMatchObject({ id: 3, number: "SO-1", grandTotal: 500, partyName: "A" });
  });
  it("normalizes camelCase rows", () => {
    const row = normalizeOrderRow({ id: 4, number: "PO-1", grandTotal: 10, status: "draft", partyId: 1, partyName: "B", orderDate: "x", dueDate: "y" });
    expect(row.id).toBe(4);
    expect(row.dueDate).toBe("y");
  });
  it("normalizes report with null rows", () => {
    const rep = normalizeOrderReport({ type: "purchasing", rows: null, total: 0, grandTotal: 0 });
    expect(rep.rows).toEqual([]);
    expect(rep.type).toBe("purchasing");
  });
  it("validates range", () => {
    expect(validRange("2026-01-02", "2026-01-01")).toBe(false);
    expect(validRange("", "2026-01-01")).toBe(true);
  });
  it("builds detail path", () => {
    expect(orderDetailPath("sales", 5)).toBe("/sales-orders/5");
    expect(orderDetailPath("purchasing", 5)).toBe("/purchase-orders/5");
  });
});
