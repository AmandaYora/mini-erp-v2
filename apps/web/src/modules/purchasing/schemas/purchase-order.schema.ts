// Validasi form order beli. Cermin aturan backend
// (purchasing/application/service.go):
// - paymentTerms hanya "net" | "cod"; termin net WAJIB dueDate.
// - taxType hanya "none" | "include" | "exclude"; taxType none → taxRate 0,
//   selain itu taxRate > 0.
// - qty > 0, uom wajib (backend me-resolve UOM ke master produk).
// - minimal 1 baris.

import { z } from "zod";

export const purchaseOrderLineSchema = z.object({
  productId: z.number().int().positive("Pilih produk"),
  variantId: z.number().int().positive("Pilih varian"),
  uom: z.string().trim().min(1, "Satuan wajib diisi"),
  qty: z.number().positive("Qty harus lebih dari 0"),
  unitPrice: z.number().nonnegative("Harga tidak boleh negatif"),
  discountPct: z.number().min(0, "Diskon % minimal 0").max(100, "Diskon % maksimal 100"),
  discountNominal: z.number().nonnegative("Diskon nominal tidak boleh negatif"),
});

export const purchaseOrderSchema = z
  .object({
    partyId: z.number().int().positive("Pilih supplier"),
    orderDate: z.string().optional().default(""),
    paymentTerms: z.enum(["net", "cod"], {
      errorMap: () => ({ message: "Termin harus net atau cod" }),
    }),
    dueDate: z.string().optional().default(""),
    taxType: z.enum(["none", "include", "exclude"], {
      errorMap: () => ({ message: "Tipe pajak harus none, include, atau exclude" }),
    }),
    taxRate: z.number().min(0, "Tarif pajak minimal 0").max(100, "Tarif pajak maksimal 100"),
    notes: z.string().max(1000, "Catatan maksimal 1000 karakter").optional().default(""),
    lines: purchaseOrderLineSchema.array().min(1, "Minimal 1 baris barang"),
  })
  .superRefine((v, ctx) => {
    if (v.paymentTerms === "net" && v.dueDate.trim() === "") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["dueDate"],
        message: "Jatuh tempo wajib diisi untuk termin net",
      });
    }
    if (v.taxType === "none" && v.taxRate !== 0) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["taxRate"],
        message: "Tarif harus 0 bila pajak none",
      });
    }
    if (v.taxType !== "none" && v.taxRate <= 0) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["taxRate"],
        message: "Tarif harus lebih dari 0 bila ada pajak",
      });
    }
  });

export type PurchaseOrderFormValues = z.infer<typeof purchaseOrderSchema>;
export type PurchaseOrderLineValues = z.infer<typeof purchaseOrderLineSchema>;
