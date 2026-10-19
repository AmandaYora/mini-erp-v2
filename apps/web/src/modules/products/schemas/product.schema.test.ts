import { describe, expect, it } from "vitest";
import { productSchema } from "@/modules/products/schemas/product.schema";

const base = {
  code: "BRG-001",
  name: "Kopi",
  type: "barang",
  baseUom: "pcs",
  purchaseUom: "dus",
  salesUom: "pcs",
  purchaseFactor: 12,
  salesFactor: 1,
  purchasePrice: 10000,
  sellingPrice: 15000,
  minSellingPrice: 12000,
};

function issues(input: unknown): Record<string, string> {
  const r = productSchema.safeParse(input);
  if (r.success) return {};
  const out: Record<string, string> = {};
  for (const i of r.error.issues) out[i.path.join(".")] ??= i.message;
  return out;
}

describe("productSchema", () => {
  it("produk barang valid dengan default", () => {
    const r = productSchema.safeParse(base);
    expect(r.success).toBe(true);
    if (r.success) {
      expect(r.data.status).toBe("active");
      expect(r.data.hasVariants).toBe(false);
      expect(r.data.variants).toEqual([]);
      expect(r.data.attrs).toEqual([]);
    }
  });

  it("menerima keempat tipe; menolak tipe lain", () => {
    for (const type of ["barang", "jasa", "paket", "non_stok"]) {
      const tracked = type === "barang";
      expect(issues({ ...base, type, tracked })).toEqual({});
    }
    expect(issues({ ...base, type: "service" }).type).toMatch(/Tipe/);
  });

  it("batas panjang: kode 50, nama 255, satuan 30", () => {
    expect(issues({ ...base, code: "A".repeat(51) }).code).toMatch(/50/);
    expect(issues({ ...base, code: "A".repeat(50) }).code).toBeUndefined();
    expect(issues({ ...base, name: "N".repeat(256) }).name).toMatch(/255/);
    expect(issues({ ...base, baseUom: "u".repeat(31) }).baseUom).toMatch(/30/);
  });

  it("hanya barang yang boleh terpantau", () => {
    expect(issues({ ...base, type: "jasa", tracked: true }).tracked).toMatch(/barang/);
    expect(issues({ ...base, type: "jasa", tracked: false }).tracked).toBeUndefined();
  });

  it("produk terpantau wajib berharga beli; tak terpantau tidak", () => {
    expect(issues({ ...base, tracked: true, purchasePrice: 0 }).purchasePrice).toMatch(/Harga beli wajib/);
    expect(issues({ ...base, tracked: false, purchasePrice: 0 }).purchasePrice).toBeUndefined();
  });

  it("harga jual minimal tidak boleh melebihi harga jual", () => {
    expect(issues({ ...base, minSellingPrice: 20000 }).minSellingPrice).toMatch(/melebihi/);
  });

  it("status hanya active|inactive (arsip lewat endpoint sendiri)", () => {
    expect(issues({ ...base, status: "inactive" }).status).toBeUndefined();
    expect(issues({ ...base, status: "archived" }).status).toBeDefined();
  });

  it("info tambahan produk: maksimal 30 baris dan aturan kunci/isi", () => {
    const many = Array.from({ length: 31 }, (_, i) => ({ key: `k${i}`, value: "v" }));
    expect(issues({ ...base, attrs: many }).attrs).toMatch(/30/);
    expect(issues({ ...base, attrs: [{ key: "", value: "x" }] })["attrs.0"]).toMatch(/Nama/);
  });

  describe("varian", () => {
    const v = (over: object) => ({ code: "V1", name: "Merah", barcode: "", isDefault: true, status: "active", ...over });

    it("hasVariants tanpa varian aktif ditolak", () => {
      expect(issues({ ...base, hasVariants: true, variants: [] }).variants).toMatch(/aktif/);
      expect(
        issues({ ...base, hasVariants: true, variants: [v({ status: "inactive" })] }).variants,
      ).toMatch(/aktif/);
    });

    it("tanpa hasVariants, daftar varian kosong valid (varian bawaan dipakai server)", () => {
      expect(issues({ ...base, hasVariants: false, variants: [] })).toEqual({});
    });

    it("kode varian maks 80, ganda ditolak (tanpa beda huruf besar/kecil)", () => {
      expect(
        issues({ ...base, hasVariants: true, variants: [v({ code: "C".repeat(81) })] })["variants.0.code"],
      ).toMatch(/80/);
      const dup = issues({
        ...base,
        hasVariants: true,
        variants: [v({ code: "a1" }), v({ code: "A1", isDefault: false })],
      });
      expect(dup["variants.1.code"]).toMatch(/ganda/);
    });

    it("barcode dan kode varian satu namespace", () => {
      const clash = issues({
        ...base,
        hasVariants: true,
        variants: [v({ code: "V1" }), v({ code: "V2", barcode: "v1", isDefault: false })],
      });
      expect(clash["variants.1.barcode"]).toMatch(/sama dengan kode varian lain/);
      const twice = issues({
        ...base,
        hasVariants: true,
        variants: [v({ code: "V1", barcode: "B" }), v({ code: "V2", barcode: "b", isDefault: false })],
      });
      expect(twice["variants.1.barcode"]).toMatch(/ganda/);
    });

    it("varian boleh memakai kodenya sendiri sebagai barcode", () => {
      expect(
        issues({ ...base, hasVariants: true, variants: [v({ code: "V1", barcode: "V1" })] }),
      ).toEqual({});
    });
  });
});
