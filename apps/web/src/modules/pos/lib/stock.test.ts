import { describe, expect, it } from "vitest";
import { availableByLocation, availableByVariant, sellableLocationIds, tileStock } from "@/modules/pos/lib/stock";
import type { PosLocation, ScanBalance } from "@/modules/pos/types";

const bal = (variantId: number, locationId: number, onHand: number, reserved = 0): ScanBalance => ({
  branchId: 1, productId: 7, variantId, locationId, onHand, reserved, available: onHand - reserved,
});
const loc = (id: number, kind = "normal", status = "active"): PosLocation => ({ id, code: `L${id}`, name: `Lokasi ${id}`, kind, status });

describe("stok tersedia POS (tile, varian, info)", () => {
  it("status tile: tidak dilacak, habis, menipis, normal, desimal", () => {
    expect(tileStock({ tracked: false, stockAvailable: 5 })).toEqual({ kind: "none" });
    expect(tileStock({ tracked: true })).toEqual({ kind: "none" }); // tanpa izin stock.view
    expect(tileStock({ tracked: true, stockAvailable: 0, stockUom: "pcs" })).toEqual({ kind: "out", uom: "pcs" });
    expect(tileStock({ tracked: true, stockAvailable: -2, stockUom: "pcs" }).kind).toBe("out");
    expect(tileStock({ tracked: true, stockAvailable: 3, stockUom: "pcs", belowMinStock: true })).toEqual({ kind: "low", qty: 3, uom: "pcs" });
    expect(tileStock({ tracked: true, stockAvailable: 0.5, stockUom: "m" })).toEqual({ kind: "ok", qty: 0.5, uom: "m" });
  });

  it("lokasi rusak/nonaktif tidak dihitung; daftar kosong = tidak diketahui", () => {
    expect([...(sellableLocationIds([loc(1), loc(2, "damaged"), loc(3, "normal", "archived")]) ?? [])]).toEqual([1]);
    expect(sellableLocationIds([])).toBeNull();
    expect(sellableLocationIds(null)).toBeNull();
  });

  it("per varian dan per lokasi: tersedia (on_hand − ditahan), satuan jual", () => {
    const rows = [bal(1, 1, 30, 6), bal(1, 2, 5), bal(2, 1, 12), bal(2, 3, 24)];
    const sellable = new Set([1, 3]);
    expect(availableByVariant(rows, sellable)).toEqual(new Map([[1, 24], [2, 36]]));
    expect(availableByVariant(rows, sellable, 12)).toEqual(new Map([[1, 2], [2, 3]]));
    expect(availableByVariant(rows, null).get(1)).toBe(29);
    expect(availableByLocation(rows, sellable)).toEqual([
      { locationId: 1, qty: 36 },
      { locationId: 3, qty: 24 },
    ]);
  });
});
