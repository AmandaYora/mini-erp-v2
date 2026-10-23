import { describe, expect, it } from "vitest";
import { cardDetail, mergeSeen, oldestDays, progressOf, wibClock } from "@/modules/data-health/lib/alerts";
import { makeAlert, minPriceAlert } from "@/modules/data-health/test/fixtures";

describe("alerts lib", () => {
  it("pembanding lokal hanya naik, tidak turun (jejak penyelesaian)", () => {
    const first = mergeSeen({}, [makeAlert({ count: 14 })], "2026-10-20T00:00:00Z");
    const later = mergeSeen(first, [makeAlert({ count: 9 })], "2026-10-23T00:00:00Z");
    expect(later["inventory.cost_above_price"]).toEqual({ count: 14, at: "2026-10-20T00:00:00Z" });
    const p = progressOf(makeAlert({ count: 9 }), later["inventory.cost_above_price"]);
    expect(p).toMatchObject({ remaining: 9, from: 14, basis: "seen" });
    expect(progressOf(makeAlert({ count: 15 }), later["inventory.cost_above_price"])).toBeNull();
  });

  it("baris kartu: Rp bila ada, umur tertua bila tanpa Rp", () => {
    expect(cardDetail(makeAlert(), "2026-10-23")).toMatch(/^64 produk · Rp/);
    expect(cardDetail({ ...minPriceAlert, oldestAt: "2026-10-17" }, "2026-10-23")).toBe("38 produk · tertua 6 hari");
    expect(oldestDays(null, "2026-10-23")).toBeNull();
  });

  it("jam selalu WIB", () => {
    expect(wibClock("2026-10-23T03:42:00Z")).toBe("10.42");
  });
});
