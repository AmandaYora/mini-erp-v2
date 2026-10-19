import { describe, expect, it } from "vitest";
import {
  deliveryItemsFor,
  holdKey,
  neededDelta,
  planHolds,
} from "@/modules/pos/lib/holds";

describe("holdKey", () => {
  it("memakai format pos:<ref>:<produk>:<varian>:<lokasi> dan < 100 karakter", () => {
    const ref = "123e4567-e89b-42d3-a456-426614174000";
    const key = holdKey(ref, 12, 34, 5);
    expect(key).toBe(`pos:${ref}:12:34:5`);
    expect(key.length).toBeLessThan(100);
  });
  it("ref terpanjang (64) tetap < 100", () => {
    expect(holdKey("r".repeat(64), 999999, 999999, 999999).length).toBeLessThan(100);
  });
});

describe("planHolds", () => {
  it("baris baru: tahan sesuai saran dan laporkan shortfall", () => {
    const plan = planHolds({}, 5, { lines: [{ locationId: 1, qty: 3 }, { locationId: 2, qty: 1 }], shortfall: 1 });
    expect(plan.target).toEqual({ 1: 3, 2: 1 });
    expect(plan.shortfall).toBe(1);
    expect(plan.holds).toEqual([
      { locationId: 1, qty: 3 },
      { locationId: 2, qty: 1 },
    ]);
  });

  it("menaikkan qty: suggest hanya untuk selisih, tahanan lama ditambah", () => {
    expect(neededDelta({ 1: 3 }, 5)).toBe(2);
    const plan = planHolds({ 1: 3 }, 5, { lines: [{ locationId: 1, qty: 2 }], shortfall: 0 });
    expect(plan.target).toEqual({ 1: 5 });
    expect(plan.holds).toEqual([{ locationId: 1, qty: 5 }]);
    expect(plan.shortfall).toBe(0);
  });

  it("menurunkan qty: kurangi lokasi terakhir dan lepas yang kosong", () => {
    const plan = planHolds({ 1: 3, 2: 2 }, 3, null);
    expect(plan.target).toEqual({ 1: 3 });
    expect(plan.releases).toEqual([2]);
    expect(plan.holds).toEqual([]);
  });

  it("menurunkan sebagian: upsert qty lebih kecil", () => {
    const plan = planHolds({ 1: 5 }, 2, null);
    expect(plan.target).toEqual({ 1: 2 });
    expect(plan.holds).toEqual([{ locationId: 1, qty: 2 }]);
  });

  it("qty sama: tidak ada operasi", () => {
    const plan = planHolds({ 1: 4 }, 4, null);
    expect(plan.holds).toEqual([]);
    expect(plan.releases).toEqual([]);
    expect(plan.shortfall).toBe(0);
  });

  it("baris dihapus (desired 0): semua dilepas", () => {
    const plan = planHolds({ 1: 2, 7: 1 }, 0, null);
    expect(plan.target).toEqual({});
    expect(plan.releases.sort()).toEqual([1, 7]);
  });

  it("tanpa saran saat perlu tambahan dianggap kekurangan penuh", () => {
    expect(planHolds({}, 2, null).shortfall).toBe(2);
  });
});

describe("deliveryItemsFor", () => {
  const line = { productId: 1, variantId: 2, uom: "pcs", qty: 5 };
  const ref = "abc";

  it("tanpa tahanan: baris polos tanpa lokasi/key", () => {
    expect(deliveryItemsFor(line, {}, ref)).toEqual([
      { productId: 1, variantId: 2, uom: "pcs", qty: 5 },
    ]);
  });
  it("satu tahanan: locationId dikosongkan, key terisi", () => {
    const items = deliveryItemsFor(line, { 9: 5 }, ref);
    expect(items).toHaveLength(1);
    expect(items[0].locationId).toBeUndefined();
    expect(items[0].reservationKey).toBe("pos:abc:1:2:9");
  });
  it("beberapa lokasi: satu baris per lokasi, jumlah qty = qty baris", () => {
    const items = deliveryItemsFor(line, { 9: 3, 10: 2 }, ref);
    expect(items.map((i) => [i.locationId, i.qty, i.reservationKey])).toEqual([
      [9, 3, "pos:abc:1:2:9"],
      [10, 2, "pos:abc:1:2:10"],
    ]);
  });
});
