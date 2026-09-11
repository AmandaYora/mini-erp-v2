import { z } from "zod";

// Body diverifikasi dari productPayload di
// apps/api/internal/modules/product/presentation/handler.go:
// code, name, categoryId, type, tracked, baseUom, purchaseUom, salesUom,
// purchaseFactor, salesFactor, purchasePrice, sellingPrice, minSellingPrice,
// minStock, variants[{code, name, barcode, isDefault}].

export const productVariantSchema = z.object({
  code: z.string().min(1, "Kode varian wajib diisi"),
  name: z.string().min(1, "Nama varian wajib diisi"),
  barcode: z.string().optional().default(""),
  isDefault: z.boolean().default(false),
});

export type ProductVariantFormValues = z.infer<typeof productVariantSchema>;

export const productSchema = z
  .object({
    code: z.string().min(1, "Kode wajib diisi"),
    name: z.string().min(1, "Nama wajib diisi"),
    type: z.enum(["barang", "jasa"], {
      errorMap: () => ({ message: "Tipe harus barang atau jasa" }),
    }),
    categoryId: z.number().int().positive().nullable().default(null),
    tracked: z.boolean().default(true),
    baseUom: z.string().min(1, "Satuan dasar wajib diisi"),
    purchaseUom: z.string().min(1, "Satuan beli wajib diisi"),
    salesUom: z.string().min(1, "Satuan jual wajib diisi"),
    purchaseFactor: z.coerce.number().positive("Faktor beli harus lebih dari 0").default(1),
    salesFactor: z.coerce.number().positive("Faktor jual harus lebih dari 0").default(1),
    purchasePrice: z.coerce.number().int().min(0, "Harga beli minimal 0").default(0),
    sellingPrice: z.coerce.number().int().min(0, "Harga jual minimal 0").default(0),
    minSellingPrice: z.coerce.number().int().min(0, "Harga jual minimal minimal 0").default(0),
    minStock: z.coerce.number().min(0, "Stok minimum minimal 0").default(0),
    variants: z.array(productVariantSchema).default([]),
  })
  .refine((v) => v.minSellingPrice <= v.sellingPrice, {
    path: ["minSellingPrice"],
    message: "Harga jual minimal tidak boleh melebihi harga jual",
  });

export type ProductFormValues = z.infer<typeof productSchema>;
