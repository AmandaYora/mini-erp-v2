import { z } from "zod";
import { PRODUCT_ATTR_LIMITS, validateRows } from "@/shared/lib/key-value";

// Body diverifikasi dari productPayload di
// apps/api/internal/modules/product/presentation/handler.go dan batas di
// application/service.go (validateProductCore): kode produk maks 50, kode
// varian maks 80, nama maks 255, barcode maks 100, satuan maks 30, tipe
// barang|jasa|paket|non_stok, status active|inactive, harga beli wajib untuk
// produk terpantau, varian: minimal satu aktif, kode+barcode satu namespace.

export const PRODUCT_CODE_MAX = 50;
export const VARIANT_CODE_MAX = 80;
export const NAME_MAX = 255;
export const BARCODE_MAX = 100;
export const UOM_MAX = 30;
export const SUMMARY_MAX = 5000;

export const productTypeValues = ["barang", "jasa", "paket", "non_stok"] as const;
export const productStatusValues = ["active", "inactive"] as const;

export const attrRowSchema = z.object({ key: z.string(), value: z.string() });

export const productVariantSchema = z.object({
  /** Terisi untuk varian yang sudah tersimpan (identitas stabil). */
  id: z.number().int().positive().optional(),
  code: z
    .string()
    .trim()
    .min(1, "Kode varian wajib diisi")
    .max(VARIANT_CODE_MAX, `Kode varian maksimal ${VARIANT_CODE_MAX} karakter`),
  name: z
    .string()
    .trim()
    .min(1, "Nama varian wajib diisi")
    .max(NAME_MAX, `Nama varian maksimal ${NAME_MAX} karakter`),
  barcode: z.string().trim().max(BARCODE_MAX, `Barcode maksimal ${BARCODE_MAX} karakter`).default(""),
  isDefault: z.boolean().default(false),
  status: z.enum(["active", "inactive"]).default("active"),
  attrs: z.array(attrRowSchema).default([]),
});

export type ProductVariantFormValues = z.infer<typeof productVariantSchema>;

export const productSchema = z
  .object({
    code: z
      .string()
      .trim()
      .min(1, "Kode wajib diisi")
      .max(PRODUCT_CODE_MAX, `Kode maksimal ${PRODUCT_CODE_MAX} karakter`),
    name: z
      .string()
      .trim()
      .min(1, "Nama wajib diisi")
      .max(NAME_MAX, `Nama maksimal ${NAME_MAX} karakter`),
    type: z.enum(productTypeValues, {
      errorMap: () => ({ message: "Tipe harus barang, jasa, paket, atau non stok" }),
    }),
    status: z.enum(productStatusValues).default("active"),
    categoryId: z.number().int().positive().nullable().default(null),
    tracked: z.boolean().default(true),
    baseUom: z.string().trim().min(1, "Satuan dasar wajib diisi").max(UOM_MAX, `Maksimal ${UOM_MAX} karakter`),
    purchaseUom: z.string().trim().min(1, "Satuan beli wajib diisi").max(UOM_MAX, `Maksimal ${UOM_MAX} karakter`),
    salesUom: z.string().trim().min(1, "Satuan jual wajib diisi").max(UOM_MAX, `Maksimal ${UOM_MAX} karakter`),
    purchaseFactor: z.coerce.number().positive("Faktor beli harus lebih dari 0").default(1),
    salesFactor: z.coerce.number().positive("Faktor jual harus lebih dari 0").default(1),
    purchasePrice: z.coerce.number().int("Harga beli harus bilangan bulat").min(0, "Harga beli minimal 0").default(0),
    sellingPrice: z.coerce.number().int("Harga jual harus bilangan bulat").min(0, "Harga jual minimal 0").default(0),
    minSellingPrice: z.coerce
      .number()
      .int("Harga jual minimal harus bilangan bulat")
      .min(0, "Harga jual minimal minimal 0")
      .default(0),
    minStock: z.coerce.number().min(0, "Stok minimum minimal 0").default(0),
    summary: z.string().trim().max(SUMMARY_MAX, `Ringkasan maksimal ${SUMMARY_MAX} karakter`).default(""),
    attrs: z.array(attrRowSchema).default([]),
    /** Toggle "Produk punya varian" (varian bawaan disembunyikan bila mati). */
    hasVariants: z.boolean().default(false),
    variants: z.array(productVariantSchema).default([]),
  })
  .superRefine((v, ctx) => {
    if (v.minSellingPrice > v.sellingPrice) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["minSellingPrice"],
        message: "Harga jual minimal tidak boleh melebihi harga jual",
      });
    }
    if (v.tracked && v.type !== "barang") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["tracked"],
        message: "Hanya produk bertipe barang yang dapat dipantau stoknya",
      });
    }
    if (v.tracked && v.purchasePrice <= 0) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["purchasePrice"],
        message: "Harga beli wajib diisi untuk produk terpantau",
      });
    }
    const attr = validateRows(v.attrs, PRODUCT_ATTR_LIMITS);
    if (attr.general) ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["attrs"], message: attr.general });
    for (const [i, msg] of Object.entries(attr.byRow)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["attrs", Number(i)], message: msg });
    }

    if (!v.hasVariants) return;
    if (v.variants.length === 0 || !v.variants.some((x) => x.status === "active")) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["variants"],
        message: "Minimal satu varian harus aktif",
      });
    }
    // Kode varian dan barcode memakai satu namespace pindai (legacy seenScanKeys).
    const codes = new Map<string, number>();
    v.variants.forEach((x, i) => {
      const c = x.code.trim().toUpperCase();
      if (c === "") return;
      if (codes.has(c)) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["variants", i, "code"],
          message: `Kode varian '${c}' ganda`,
        });
      }
      codes.set(c, i);
    });
    const barcodes = new Set<string>();
    v.variants.forEach((x, i) => {
      const b = x.barcode.trim().toUpperCase();
      if (b === "") return;
      if (barcodes.has(b)) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["variants", i, "barcode"],
          message: `Barcode '${x.barcode.trim()}' ganda`,
        });
      }
      barcodes.add(b);
      const owner = codes.get(b);
      if (owner !== undefined && owner !== i) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["variants", i, "barcode"],
          message: `Barcode '${x.barcode.trim()}' sama dengan kode varian lain`,
        });
      }
    });
    v.variants.forEach((x, i) => {
      const a = validateRows(x.attrs, PRODUCT_ATTR_LIMITS);
      const first = Object.values(a.byRow)[0] ?? a.general;
      if (first) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["variants", i, "attrs"], message: first });
      }
    });
  });

export type ProductFormValues = z.infer<typeof productSchema>;
/** Masukan form sebelum parse (nilai default belum diterapkan). */
export type ProductFormInput = z.input<typeof productSchema>;
