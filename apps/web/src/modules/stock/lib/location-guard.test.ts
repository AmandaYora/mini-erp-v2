import { describe, expect, it } from "vitest";
import {
  isParentHasStock,
  locationKindLabel,
  stripLocationCode,
} from "@/modules/stock/lib/location-guard";

const MSG =
  "PARENT_HAS_STOCK: lokasi induk masih memiliki stok; kirim forceTransfer=true untuk memindahkan seluruh stok induk ke lokasi ini";

describe("location-guard", () => {
  it("mengenali konflik induk berstok", () => {
    expect(isParentHasStock(MSG)).toBe(true);
    expect(isParentHasStock("Lokasi tidak ditemukan")).toBe(false);
    expect(isParentHasStock(undefined)).toBe(false);
  });
  it("membuang awalan kode", () => {
    expect(stripLocationCode(MSG).startsWith("lokasi induk")).toBe(true);
  });
  it("label kind", () => {
    expect(locationKindLabel("damaged")).toBe("Rusak");
    expect(locationKindLabel("normal")).toBe("Normal");
  });
});
