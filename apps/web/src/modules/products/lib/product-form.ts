// Logika murni form produk (tanpa React): aturan tipe, bentuk payload varian,
// dan pemetaan pesan penjaga server ke tampilan. Diuji di product-form.test.ts.
//
// Aturan backend yang dicerminkan (product/application/service.go, variants.go):
// - tipe selain barang: tracked=false, faktor 1, ketiga satuan sama (normalizeInput);
// - payload TANPA `variants` = set varian tersimpan dipertahankan;
//   `variants: []` = kembali ke varian bawaan (tersembunyi);
// - varian yang punya riwayat stok tak boleh hilang diam-diam: kirim ulang
//   dengan status "inactive" (karena itu baris tersimpan tidak pernah dibuang
//   dari form, hanya dinonaktifkan).

import { rowsToAttrs, attrsToRows } from "@/shared/lib/key-value";
import type {
  ProductFormValues,
  ProductVariantFormValues,
} from "@/modules/products/schemas/product.schema";
import type {
  Product,
  ProductPayload,
  ProductType,
  ProductVariant,
  VariantPayload,
} from "@/modules/products/types";

export const PRODUCT_TYPE_OPTIONS: readonly { value: ProductType; label: string }[] = [
  { value: "barang", label: "Barang" },
  { value: "jasa", label: "Jasa" },
  { value: "paket", label: "Paket" },
  { value: "non_stok", label: "Non stok" },
];

export function productTypeLabel(type: string): string {
  return PRODUCT_TYPE_OPTIONS.find((o) => o.value === type)?.label ?? type;
}

/** Hanya barang yang boleh dipantau stoknya. */
export function canTrack(type: string): boolean {
  return type === "barang";
}

export function asProductType(type: string): ProductType {
  return PRODUCT_TYPE_OPTIONS.some((o) => o.value === type) ? (type as ProductType) : "barang";
}

/** Terapkan aturan tipe ke nilai form (server melakukan hal yang sama). */
export function applyTypeRules(v: ProductFormValues): ProductFormValues {
  if (canTrack(v.type)) return v;
  return {
    ...v,
    tracked: false,
    purchaseFactor: 1,
    salesFactor: 1,
    purchaseUom: v.baseUom,
    salesUom: v.baseUom,
  };
}

export function emptyVariantRow(index: number): ProductVariantFormValues {
  return {
    code: "",
    name: "",
    barcode: "",
    isDefault: index === 0,
    status: "active",
    attrs: [],
  };
}

/** Varian yang tampil di form: bukan bawaan-tersembunyi dan bukan terarsip. */
export function visibleVariants(variants: readonly ProductVariant[] | undefined): ProductVariant[] {
  return (variants ?? [])
    .filter((v) => !v.isHidden && v.status !== "archived")
    .slice()
    .sort((a, b) => a.sortOrder - b.sortOrder || a.id - b.id);
}

export function variantRowsFromProduct(p: Product): ProductVariantFormValues[] {
  return visibleVariants(p.variants).map((v) => ({
    id: v.id,
    code: v.code,
    name: v.name,
    barcode: v.barcode ?? "",
    isDefault: v.isDefault,
    status: v.status === "inactive" ? "inactive" : "active",
    attrs: attrsToRows(v.attributes),
  }));
}

/** Pastikan paling banyak satu varian aktif bertanda utama. */
export function normalizeDefault(rows: ProductVariantFormValues[]): ProductVariantFormValues[] {
  let seen = false;
  return rows.map((r) => {
    const keep = r.isDefault && r.status === "active" && !seen;
    if (keep) seen = true;
    return keep === r.isDefault ? r : { ...r, isDefault: keep };
  });
}

/**
 * Bentuk `variants` untuk payload:
 * - punya varian -> daftar eksplisit (baris tersimpan membawa id + status);
 * - tidak punya varian, dulu punya -> [] (kembali ke bawaan);
 * - tidak punya varian dan dulu tidak -> undefined (kunci tidak dikirim).
 */
export function buildVariantsPayload(
  hasVariants: boolean,
  hadVisibleVariants: boolean,
  rows: readonly ProductVariantFormValues[],
): VariantPayload[] | undefined {
  if (!hasVariants) return hadVisibleVariants ? [] : undefined;
  return normalizeDefault([...rows]).map((r, i) => ({
    ...(r.id ? { id: r.id } : {}),
    code: r.code.trim(),
    name: r.name.trim(),
    barcode: r.barcode.trim(),
    isDefault: r.isDefault,
    status: r.status,
    sortOrder: i,
    attributes: rowsToAttrs(r.attrs),
    isHidden: false,
  }));
}

export function buildProductPayload(
  values: ProductFormValues,
  hadVisibleVariants: boolean,
): ProductPayload {
  const v = applyTypeRules(values);
  const variants = buildVariantsPayload(v.hasVariants, hadVisibleVariants, v.variants);
  const payload: ProductPayload = {
    code: v.code.trim(),
    name: v.name.trim(),
    categoryId: v.categoryId,
    type: v.type,
    tracked: v.tracked,
    baseUom: v.baseUom.trim(),
    purchaseUom: v.purchaseUom.trim(),
    salesUom: v.salesUom.trim(),
    purchaseFactor: v.purchaseFactor,
    salesFactor: v.salesFactor,
    purchasePrice: v.purchasePrice,
    sellingPrice: v.sellingPrice,
    minSellingPrice: v.minSellingPrice,
    minStock: v.minStock,
    status: v.status,
    summary: v.summary.trim(),
    // {} mengosongkan di server; selalu dikirim agar tak ada selisih basi.
    attributes: rowsToAttrs(v.attrs),
  };
  if (variants !== undefined) payload.variants = variants;
  return payload;
}

// --- pesan penjaga server ---

export type GuardKind =
  | "archive_blocked"
  | "history_locked"
  | "tracking_locked"
  | "variant_stock"
  | "variant_history"
  | "holds"
  | "code_conflict"
  | "variant_conflict"
  | "other";

/** Kenali kelas penjaga dari pesan server (pesan backend stabil, bahasa Indonesia). */
export function classifyServerMessage(message: string): GuardKind {
  const m = message.toLowerCase();
  if (m.includes("produk masih memiliki stok atau reservasi")) return "archive_blocked";
  if (m.includes("terkunci karena sudah ada riwayat stok") && m.startsWith("kode varian")) {
    return "variant_history";
  }
  if (m.includes("terkunci karena sudah ada riwayat")) return "history_locked";
  if (m.includes("pantau stok tidak dapat dimatikan")) return "tracking_locked";
  if (m.includes("reservasi berjalan")) return "holds";
  if (m.includes("varian") && (m.includes("masih punya stok") || m.includes("pada varian bawaan"))) {
    return "variant_stock";
  }
  if (m.includes("varian") && m.includes("riwayat stok")) return "variant_history";
  if (m.startsWith("kode produk")) return "code_conflict";
  if (m.includes("barcode") || m.includes("kode varian") || m.includes("kode produk lain")) {
    return "variant_conflict";
  }
  return "other";
}

const GUARD_TITLES: Record<GuardKind, string> = {
  archive_blocked: "Produk tidak dapat diarsipkan",
  history_locked: "Satuan, faktor, atau tipe terkunci",
  tracking_locked: "Pantau stok tidak dapat dimatikan",
  variant_stock: "Varian masih punya stok",
  variant_history: "Varian punya riwayat stok",
  holds: "Ada reservasi berjalan",
  code_conflict: "Kode produk sudah dipakai",
  variant_conflict: "Kode atau barcode varian bertabrakan",
  other: "Gagal menyimpan",
};

export function guardTitle(kind: GuardKind): string {
  return GUARD_TITLES[kind];
}

/** Penjaga yang bukan galat isian: tampil sebagai pemberitahuan di atas form. */
export function isGuardKind(kind: GuardKind): boolean {
  return kind !== "other" && kind !== "code_conflict" && kind !== "variant_conflict";
}

export interface ServerFieldError {
  field: string;
  message: string;
}

export interface MappedProductErrors {
  /** Kunci seperti galat zod: "code", "variants", "variants.2.barcode", ... */
  fields: Record<string, string>;
  /** Pesan penjaga untuk pemberitahuan di atas form, bila ada. */
  banner: { kind: GuardKind; message: string } | null;
}

function quoted(message: string): string[] {
  const out: string[] = [];
  for (const m of message.matchAll(/'([^']+)'/g)) out.push(m[1].toUpperCase());
  return out;
}

/**
 * Petakan galat server ke isian. Tabrakan kode/barcode varian disebut dengan
 * nilainya ('X'): pesan ditempel ke BARIS varian yang memuat nilai itu; kode
 * produk bentrok ke kolom kode produk. Penjaga stok/riwayat jadi banner.
 */
export function mapProductServerErrors(
  message: string,
  errors: readonly ServerFieldError[] | undefined,
  variants: readonly Pick<ProductVariantFormValues, "code" | "barcode">[],
): MappedProductErrors {
  const fields: Record<string, string> = {};
  let banner: MappedProductErrors["banner"] = null;

  const put = (key: string, msg: string) => {
    fields[key] = fields[key] ? `${fields[key]}; ${msg}` : msg;
  };
  const attachVariant = (msg: string): void => {
    const vals = quoted(msg);
    for (const val of vals) {
      const idx = variants.findIndex(
        (v) => v.code.trim().toUpperCase() === val || v.barcode.trim().toUpperCase() === val,
      );
      if (idx >= 0) {
        const byBarcode = variants[idx].barcode.trim().toUpperCase() === val;
        put(`variants.${idx}.${byBarcode ? "barcode" : "code"}`, msg);
        return;
      }
    }
    put("variants", msg);
  };

  for (const fe of errors ?? []) {
    const k = classifyServerMessage(fe.message);
    if (isGuardKind(k)) {
      // Penjaga stok/riwayat varian: pemberitahuan di atas form, bukan isian.
      banner ??= { kind: k, message: fe.message };
    } else if (fe.field === "variants") {
      attachVariant(fe.message);
    } else {
      put(fe.field, fe.message);
    }
  }

  const kind = classifyServerMessage(message);
  if (kind === "code_conflict") {
    // "Kode produk 'X' sudah digunakan ..." -> kolom kode produk.
    put("code", message);
  } else if (kind === "variant_conflict" && (errors ?? []).length === 0) {
    attachVariant(message);
  } else if (isGuardKind(kind)) {
    banner ??= { kind, message };
  }
  return { fields, banner };
}
