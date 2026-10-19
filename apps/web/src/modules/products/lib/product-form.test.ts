import { describe, expect, it } from "vitest";
import {
  applyTypeRules,
  buildProductPayload,
  buildVariantsPayload,
  canTrack,
  classifyServerMessage,
  mapProductServerErrors,
  normalizeDefault,
  variantRowsFromProduct,
  visibleVariants,
} from "@/modules/products/lib/product-form";
import type { ProductFormValues, ProductVariantFormValues } from "@/modules/products/schemas/product.schema";
import type { Product, ProductVariant } from "@/modules/products/types";

const row = (over: Partial<ProductVariantFormValues> = {}): ProductVariantFormValues => ({
  code: "V1",
  name: "Merah",
  barcode: "",
  isDefault: false,
  status: "active",
  attrs: [],
  ...over,
});

const pv = (over: Partial<ProductVariant>): ProductVariant => ({
  id: 1,
  code: "P",
  name: "P",
  barcode: "",
  isDefault: false,
  status: "active",
  isHidden: false,
  sortOrder: 0,
  attributes: {},
  ...over,
});

const form = (over: Partial<ProductFormValues> = {}): ProductFormValues => ({
  code: " brg-1 ",
  name: " Kopi ",
  type: "barang",
  status: "active",
  categoryId: null,
  tracked: true,
  baseUom: "pcs",
  purchaseUom: "dus",
  salesUom: "pcs",
  purchaseFactor: 12,
  salesFactor: 1,
  purchasePrice: 100,
  sellingPrice: 150,
  minSellingPrice: 120,
  minStock: 0,
  summary: "  ringkas ",
  attrs: [{ key: "Merek", value: " X " }],
  hasVariants: false,
  variants: [],
  ...over,
});

describe("aturan tipe", () => {
  it("hanya barang yang dapat dipantau", () => {
    expect(canTrack("barang")).toBe(true);
    for (const t of ["jasa", "paket", "non_stok"]) expect(canTrack(t)).toBe(false);
  });

  it("tipe non-barang: tracked off, faktor 1, satuan beli/jual = dasar", () => {
    const v = applyTypeRules(form({ type: "jasa" }));
    expect(v.tracked).toBe(false);
    expect(v.purchaseFactor).toBe(1);
    expect(v.salesFactor).toBe(1);
    expect(v.purchaseUom).toBe("pcs");
    expect(v.salesUom).toBe("pcs");
  });

  it("barang tidak diubah", () => {
    const f = form();
    expect(applyTypeRules(f)).toBe(f);
  });
});

describe("varian terlihat", () => {
  it("membuang bawaan tersembunyi dan terarsip, urut sortOrder", () => {
    const vs = [
      pv({ id: 1, isHidden: true, isDefault: true }),
      pv({ id: 2, code: "B", sortOrder: 2 }),
      pv({ id: 3, code: "A", sortOrder: 1 }),
      pv({ id: 4, code: "Z", status: "archived" }),
      pv({ id: 5, code: "I", status: "inactive", sortOrder: 3 }),
    ];
    expect(visibleVariants(vs).map((v) => v.id)).toEqual([3, 2, 5]);
  });

  it("baris form memuat id dan status inactive", () => {
    const p = { variants: [pv({ id: 5, code: "I", status: "inactive", attributes: { Warna: "Merah" } })] } as Product;
    expect(variantRowsFromProduct(p)).toEqual([
      {
        id: 5,
        code: "I",
        name: "P",
        barcode: "",
        isDefault: false,
        status: "inactive",
        attrs: [{ key: "Warna", value: "Merah" }],
      },
    ]);
  });
});

describe("payload varian", () => {
  it("tanpa varian & tak pernah punya -> kunci variants tidak dikirim", () => {
    expect(buildVariantsPayload(false, false, [])).toBeUndefined();
  });

  it("tanpa varian padahal dulu punya -> [] (kembali ke varian bawaan)", () => {
    expect(buildVariantsPayload(false, true, [row()])).toEqual([]);
  });

  it("punya varian -> daftar eksplisit; baris tersimpan membawa id dan status inactive", () => {
    const out = buildVariantsPayload(true, true, [
      row({ id: 7, code: " v1 ", isDefault: true }),
      row({ id: 8, code: "V2", status: "inactive" }),
      row({ code: "V3" }),
    ]);
    expect(out).toEqual([
      expect.objectContaining({ id: 7, code: "v1", status: "active", sortOrder: 0, isHidden: false, isDefault: true }),
      expect.objectContaining({ id: 8, code: "V2", status: "inactive", sortOrder: 1, isDefault: false }),
      expect.objectContaining({ code: "V3", status: "active", sortOrder: 2 }),
    ]);
    expect(out?.[2]).not.toHaveProperty("id");
  });

  it("varian nonaktif tak bisa jadi utama; hanya satu utama", () => {
    const rows = normalizeDefault([
      row({ code: "A", isDefault: true, status: "inactive" }),
      row({ code: "B", isDefault: true }),
      row({ code: "C", isDefault: true }),
    ]);
    expect(rows.map((r) => r.isDefault)).toEqual([false, true, false]);
  });
});

describe("buildProductPayload", () => {
  it("merapikan teks, mengirim status/summary/attributes, tanpa variants bila tak dipakai", () => {
    const p = buildProductPayload(form(), false);
    expect(p).toMatchObject({
      code: "brg-1",
      name: "Kopi",
      status: "active",
      summary: "ringkas",
      attributes: { Merek: "X" },
    });
    expect("variants" in p).toBe(false);
  });

  it("jasa: tracked false dan satuan disamakan", () => {
    const p = buildProductPayload(form({ type: "jasa" }), false);
    expect(p).toMatchObject({ tracked: false, purchaseFactor: 1, salesFactor: 1, purchaseUom: "pcs", salesUom: "pcs" });
  });

  it("atribut dikosongkan dikirim sebagai {} (mengosongkan di server)", () => {
    expect(buildProductPayload(form({ attrs: [] }), false).attributes).toEqual({});
  });
});

describe("pesan penjaga server", () => {
  it.each([
    ["Produk masih memiliki stok atau reservasi berjalan", "archive_blocked"],
    ["Satuan dan faktor konversi terkunci karena sudah ada riwayat stok", "history_locked"],
    ["Tipe produk terkunci karena sudah ada riwayat stok", "history_locked"],
    ["Pantau stok tidak dapat dimatikan karena produk sudah punya riwayat atau saldo stok", "tracking_locked"],
    ["Varian masih punya stok. Kosongkan stok varian sebelum dihapus atau disembunyikan.", "variant_stock"],
    ["Kode varian 'A' terkunci karena sudah ada riwayat stok (buat varian baru, jangan ubah kodenya)", "variant_history"],
    ["Varian 'A' punya riwayat stok: kirim ulang dengan status 'inactive' untuk menyembunyikannya", "variant_history"],
    ["Produk masih punya reservasi berjalan dan tidak dapat dinonaktifkan", "holds"],
    ["Kode produk 'X' sudah digunakan", "code_conflict"],
    ["Kode produk 'X' sudah digunakan sebagai kode atau barcode varian", "code_conflict"],
    ["barcode 'B1' ganda", "variant_conflict"],
    ["Validasi gagal", "other"],
  ])("%s -> %s", (msg, kind) => {
    expect(classifyServerMessage(msg)).toBe(kind);
  });

  it("kode produk bentrok -> kolom kode, bukan banner", () => {
    const r = mapProductServerErrors("Kode produk 'X' sudah digunakan", undefined, []);
    expect(r.fields.code).toMatch(/sudah digunakan/);
    expect(r.banner).toBeNull();
  });

  it("tabrakan barcode varian ditempel ke baris yang memuat nilainya", () => {
    const variants = [
      { code: "A", barcode: "" },
      { code: "B", barcode: "8991" },
    ];
    const r = mapProductServerErrors(
      "Validasi gagal",
      [{ field: "variants", message: "barcode '8991' sudah dipakai produk lain" }],
      variants,
    );
    expect(r.fields["variants.1.barcode"]).toMatch(/8991/);
    const r2 = mapProductServerErrors(
      "Validasi gagal",
      [{ field: "variants", message: "'a' sudah digunakan sebagai kode produk lain" }],
      variants,
    );
    expect(r2.fields["variants.0.code"]).toMatch(/kode produk lain/);
  });

  it("nilai tak dikenal jatuh ke galat umum variants", () => {
    const r = mapProductServerErrors(
      "Validasi gagal",
      [{ field: "variants", message: "kode varian 'Q' ganda" }],
      [{ code: "A", barcode: "" }],
    );
    expect(r.fields.variants).toMatch(/ganda/);
  });

  it("penjaga stok/riwayat di arsip, ubah, dan varian menjadi banner", () => {
    expect(mapProductServerErrors("Pantau stok tidak dapat dimatikan karena produk sudah punya riwayat atau saldo stok", undefined, []).banner?.kind).toBe("tracking_locked");
    const v = mapProductServerErrors(
      "Validasi gagal",
      [{ field: "variants", message: "Varian masih punya stok. Kosongkan stok varian sebelum dihapus atau disembunyikan." }],
      [],
    );
    expect(v.banner?.kind).toBe("variant_stock");
    expect(v.fields.variants).toBeUndefined();
  });

  it("galat isian lain tetap per kolom", () => {
    const r = mapProductServerErrors("Validasi gagal", [{ field: "code", message: "maksimal 50 karakter" }], []);
    expect(r.fields.code).toBe("maksimal 50 karakter");
  });
});
