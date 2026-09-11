import { z } from "zod";

// Batasan disalin dari sales/application/service.go:
// validateHeader (channel regular|pos, paymentTerms cod|net, net wajib dueDate,
// tanggal YYYY-MM-DD) + priceLines (qty > 0, diskon 0–100, nominal >= 0,
// minimal 1 baris). Harga TIDAK ada di form — server menghitung ulang.

export const salesLineSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  uom: z.string().min(1, "Satuan wajib diisi"),
  qty: z.coerce.number().positive("Qty harus lebih dari 0"),
  discountPct: z.coerce
    .number()
    .min(0, "Diskon minimal 0")
    .max(100, "Diskon maksimal 100")
    .default(0),
  discountNominal: z.coerce
    .number()
    .int("Diskon nominal harus bilangan bulat")
    .min(0, "Diskon nominal tidak boleh negatif")
    .default(0),
});

export type SalesLineFormValues = z.infer<typeof salesLineSchema>;

export const salesOrderSchema = z
  .object({
    // null = tunai walk-in (backend: partyId 0 = tanpa tautan party).
    partyId: z.number().int().positive().nullable().default(null),
    orderDate: z.string().min(1, "Tanggal order wajib diisi"),
    paymentTerms: z.enum(["cod", "net"], {
      errorMap: () => ({ message: "Termin harus cod atau net" }),
    }),
    dueDate: z.string().default(""),
    notes: z.string().default(""),
    items: z.array(salesLineSchema).min(1, "Minimal 1 baris item"),
  })
  .refine((v) => v.paymentTerms === "cod" || v.dueDate.trim() !== "", {
    path: ["dueDate"],
    message: "Jatuh tempo wajib diisi untuk termin net",
  });

export type SalesOrderFormValues = z.infer<typeof salesOrderSchema>;
