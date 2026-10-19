import { describe, expect, it } from "vitest";
import { minStockText, stockCell } from "@/modules/products/lib/product-list-view";
import { uploadInitialPhoto } from "@/modules/products/lib/initial-photo";
import {
  PRODUCT_LIST_DEFAULTS,
  productListFilterSchema,
  productStatusParam,
} from "@/modules/products/schemas/product-list-filter.schema";
import {
  PARTY_LIST_DEFAULTS,
  partyListFilterSchema,
  partyStatusParam,
} from "@/modules/party/schemas/party-list-filter.schema";
import { parseFilters, serializeFilters } from "@/shared/hooks/use-url-filters";
import type { Product } from "@/modules/products/types";

const base = {
  id: 1, code: "A", name: "A", categoryId: null, categoryName: "", type: "barang", tracked: true,
  baseUom: "pcs", purchaseUom: "pcs", salesUom: "pak", purchaseFactor: 1, salesFactor: 4,
  purchasePrice: 0, sellingPrice: 0, minSellingPrice: 0, minStock: 0, status: "active", summary: "",
  attributes: {}, variants: [],
} satisfies Product;

describe("stockCell / minStockText", () => {
  it("tidak dipantau → tanpa angka; belum dimuat → kosong tetapi terpantau", () => {
    expect(stockCell({ ...base, tracked: false })).toEqual({ text: "", below: false, tracked: false });
    expect(stockCell(base)).toEqual({ text: "", below: false, tracked: true });
  });
  it("angka dalam satuan jual + penanda di bawah minimum", () => {
    expect(stockCell({ ...base, stockAvailable: 1.5, stockUom: "pak", belowMinStock: true })).toEqual({
      text: "1,5 pak",
      below: true,
      tracked: true,
    });
  });
  it("Min dikonversi ke satuan jual; kosong bila tidak ada minimum atau tak dipantau", () => {
    expect(minStockText({ ...base, minStock: 10 })).toBe("Min 2,5");
    expect(minStockText({ ...base, minStock: 0 })).toBe("");
    expect(minStockText({ ...base, minStock: 5, tracked: false })).toBe("");
  });
});

describe("filter daftar produk & pihak: bawaan Aktif, 'all' tahan di URL", () => {
  it("default aktif; ?status=all menjadi 'all'; kembali ke default tidak menulis URL", () => {
    expect(PRODUCT_LIST_DEFAULTS.status).toBe("active");
    expect(PARTY_LIST_DEFAULTS.status).toBe("active");
    expect(parseFilters(productListFilterSchema, PRODUCT_LIST_DEFAULTS, new URLSearchParams("")).status).toBe("active");
    expect(parseFilters(productListFilterSchema, PRODUCT_LIST_DEFAULTS, new URLSearchParams("status=all")).status).toBe("all");
    expect(parseFilters(partyListFilterSchema, PARTY_LIST_DEFAULTS, new URLSearchParams("status=archived")).status).toBe("archived");
    // Nilai tak sah jatuh ke default, bukan membuka arsip.
    expect(parseFilters(partyListFilterSchema, PARTY_LIST_DEFAULTS, new URLSearchParams("status=zzz")).status).toBe("active");
    expect(serializeFilters(PRODUCT_LIST_DEFAULTS, { ...PRODUCT_LIST_DEFAULTS, status: "all" }).get("status")).toBe("all");
    expect(serializeFilters(PRODUCT_LIST_DEFAULTS, PRODUCT_LIST_DEFAULTS).has("status")).toBe(false);
  });
  it("param API: 'all' tanpa filter", () => {
    expect(productStatusParam("all")).toBe("");
    expect(productStatusParam("archived")).toBe("archived");
    expect(partyStatusParam("all")).toBe("");
    expect(partyStatusParam("active")).toBe("active");
  });
});

describe("uploadInitialPhoto", () => {
  const file = new File(["x"], "f.jpg", { type: "image/jpeg" });
  it("mengunggah sebagai foto utama urutan 0", async () => {
    const calls: unknown[] = [];
    const res = await uploadInitialPhoto(async (f, opts) => void calls.push([f.name, opts]), file);
    expect(res).toEqual({ ok: true });
    expect(calls).toEqual([["f.jpg", { isPrimary: true, sortOrder: 0 }]]);
  });
  it("gagal unggah dilaporkan sebagai hasil (tidak melempar)", async () => {
    const res = await uploadInitialPhoto(async () => {
      throw new Error("jaringan putus");
    }, file);
    expect(res.ok).toBe(false);
  });
});
