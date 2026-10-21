import { describe, expect, it } from "vitest";
import {
  ageDays,
  clearClientRef,
  clientRefFor,
  clientRefKey,
  fromDeliveriesPath,
  parseFromDeliveriesParam,
  sameCustomer,
} from "@/modules/sales/lib/from-deliveries";

function memStorage() {
  const m = new Map<string, string>();
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => void m.set(k, v),
    removeItem: (k: string) => void m.delete(k),
    size: () => m.size,
  };
}

describe("order dari surat jalan — util", () => {
  it("memparse ?fromDeliveries: unik, positif, urut, maksimal 20", () => {
    expect(parseFromDeliveriesParam("3,1,1,x,0,-2")).toEqual([1, 3]);
    expect(parseFromDeliveriesParam(null)).toEqual([]);
    expect(parseFromDeliveriesParam(Array.from({ length: 25 }, (_, i) => i + 1).join(","))).toHaveLength(20);
  });

  it("tautan form membawa id SJ", () => {
    expect(fromDeliveriesPath([4, 9])).toBe("/sales-orders/new?fromDeliveries=4,9");
  });

  it("umur SJ dalam hari, tidak negatif", () => {
    expect(ageDays("2026-10-18", "2026-10-21")).toBe(3);
    expect(ageDays("2026-10-22", "2026-10-21")).toBe(0);
    expect(ageDays("", "2026-10-21")).toBe(0);
  });

  it("gabungan hanya untuk satu pelanggan", () => {
    expect(sameCustomer([3, 3])).toBe(true);
    expect(sameCustomer([3, 4])).toBe(false);
    expect(sameCustomer([])).toBe(true);
  });

  it("clientRef dipakai ulang untuk gabungan id yang sama sampai dibersihkan", () => {
    const st = memStorage();
    const a = clientRefFor([2, 1], st);
    expect(a.length).toBeGreaterThan(0);
    expect(clientRefFor([1, 2], st)).toBe(a);
    expect(clientRefKey([2, 1])).toBe(clientRefKey([1, 2]));
    expect(clientRefFor([1, 3], st)).not.toBe(a);
    clearClientRef([1, 2], st);
    expect(clientRefFor([1, 2], st)).not.toBe(a);
  });

  it("tanpa sessionStorage tetap memberi clientRef", () => {
    expect(clientRefFor([1], null)).toMatch(/.+/);
    expect(() => clearClientRef([1], null)).not.toThrow();
  });
});
