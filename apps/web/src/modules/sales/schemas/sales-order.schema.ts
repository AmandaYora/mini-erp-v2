import { z } from "zod";

// Batasan disalin dari sales/application/service.go:
// validateHeader (channel regular|pos, paymentTerms cod|net|prepaid, net wajib
// dueDate + customer ber-telepon, tanggal YYYY-MM-DD) + priceLines (qty > 0,
// diskon 0–100, nominal >= 0, harga manual >= 0, alasan diskon/catatan
// maks 255 karakter, minimal 1 baris, baris produk/varian/satuan kembar
// ditolak). Harga: aturan member > harga manual > katalog; unitPrice null =
// server memberi harga. Klem harga minimum dicek server (pesan ditampilkan).

export const MAX_LINE_TEXT = 255;

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
  // null = pakai harga server (member/katalog).
  unitPrice: z
    .number()
    .int("Harga harus bilangan bulat rupiah")
    .min(0, "Harga tidak boleh negatif")
    .nullable()
    .default(null),
  discountReason: z
    .string()
    .max(MAX_LINE_TEXT, "Alasan diskon maksimal 255 karakter")
    .default(""),
  note: z.string().max(MAX_LINE_TEXT, "Catatan baris maksimal 255 karakter").default(""),
  taxable: z.boolean().default(true),
});

export type SalesLineFormValues = z.infer<typeof salesLineSchema>;

export const salesOrderSchema = z
  .object({
    // null = tunai walk-in (backend: partyId 0 = tanpa tautan party).
    partyId: z.number().int().positive().nullable().default(null),
    orderDate: z.string().min(1, "Tanggal order wajib diisi"),
    paymentTerms: z.enum(["cod", "net", "prepaid"], {
      errorMap: () => ({ message: "Termin harus cod, net, atau prabayar" }),
    }),
    dueDate: z.string().default(""),
    notes: z.string().default(""),
    items: z.array(salesLineSchema).min(1, "Minimal 1 baris item"),
  })
  .superRefine((v, ctx) => {
    if (v.paymentTerms === "net" && v.dueDate.trim() === "") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["dueDate"],
        message: "Jatuh tempo wajib diisi untuk termin net",
      });
    }
    if (v.paymentTerms === "net" && v.partyId === null) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["paymentTerms"],
        message: "Termin net tidak untuk penjualan tunai tanpa customer",
      });
    }
    const seen = new Set<string>();
    v.items.forEach((it, i) => {
      const key = `${it.productId}/${it.variantId}/${it.uom.trim()}`;
      if (seen.has(key)) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["items", i],
          message: `Baris ${i + 1}: baris ganda (produk/varian/satuan sama)`,
        });
      }
      seen.add(key);
    });
  });

export type SalesOrderFormValues = z.infer<typeof salesOrderSchema>;
