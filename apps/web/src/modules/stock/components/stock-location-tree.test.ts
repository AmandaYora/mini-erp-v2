import { describe, expect, it } from "vitest";
import {
  buildLocationOptions,
  getStockLocationLabel,
} from "@/modules/stock/components/stock-location-tree";
import type { StockLocation } from "@/modules/stock/types";

function loc(partial: Partial<StockLocation> & { id: number }): StockLocation {
  return {
    branchId: 1,
    code: `L${partial.id}`,
    name: `Lokasi ${partial.id}`,
    parentId: null,
    isSystem: false,
    status: "active",
    ...partial,
  };
}

const LOCATIONS: StockLocation[] = [
  loc({ id: 1, code: "GDG", name: "Gudang", parentId: null }),
  loc({ id: 2, code: "RA", name: "Rak A", parentId: 1 }),
  loc({ id: 3, code: "RB", name: "Rak B", parentId: 1 }),
  loc({ id: 4, code: "BIN", name: "Bin 1", parentId: 2 }),
  loc({ id: 5, code: "OFF", name: "Nonaktif", parentId: null, status: "archived" }),
];

describe("getStockLocationLabel", () => {
  it("menggabung path dari akar ke lokasi", () => {
    expect(getStockLocationLabel(LOCATIONS, 4)).toBe("Gudang > Rak A > Bin 1");
    expect(getStockLocationLabel(LOCATIONS, 1)).toBe("Gudang");
  });
});

describe("buildLocationOptions", () => {
  it("hanya daun yang bisa dipilih, induk jadi grup disabled", () => {
    const options = buildLocationOptions(LOCATIONS, []);
    const byValue = new Map(options.map((o) => [o.value, o]));
    expect(byValue.get(1)?.disabled).toBe(true);
    expect(byValue.get(1)?.isGroup).toBe(true);
    expect(byValue.get(2)?.disabled).toBe(true);
    expect(byValue.get(3)?.disabled).toBe(false);
    expect(byValue.get(4)?.disabled).toBe(false);
    expect(byValue.get(4)?.depth).toBe(2);
    expect(byValue.get(4)?.path).toEqual(["Gudang", "Rak A", "Bin 1"]);
  });

  it("mengeluarkan lokasi nonaktif dan menghormati excludeIds", () => {
    const options = buildLocationOptions(LOCATIONS, [3]);
    const values = options.map((o) => o.value);
    expect(values).not.toContain(5);
    expect(options.find((o) => o.value === 3)?.disabled).toBe(true);
    expect(options.find((o) => o.value === 3)?.disabledReason).toContain("Tidak tersedia");
  });
});
