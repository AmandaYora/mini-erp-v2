// Validasi koreksi order beli. Cermin aturan backend
// (purchasing/application/corrections.go):
// - alasan batal / tutup sisa order / koreksi harga WAJIB, maks 500 karakter;
//   alasan ubah info dokumen opsional.
// - info dokumen: masa tempo 0..3650; tanggal YYYY-MM-DD.
// - koreksi harga: harga >= 0, diskon % 0..100, diskon nominal >= 0, minimal
//   satu baris berubah.

import { z } from "zod";

const DATE = /^\d{4}-\d{2}-\d{2}$/;
const optionalDate = z
  .string()
  .trim()
  .refine((v) => v === "" || DATE.test(v), "Format tanggal YYYY-MM-DD");

export const correctionReasonSchema = z
  .string()
  .trim()
  .min(1, "Alasan wajib diisi")
  .max(500, "Alasan maksimal 500 karakter");

export const purchaseHeaderSchema = z.object({
  dueDate: optionalDate,
  paymentTermDays: z
    .number()
    .int("Masa tempo harus bilangan bulat")
    .min(0, "Masa tempo minimal 0")
    .max(3650, "Masa tempo maksimal 3650 hari"),
  notes: z.string().max(1000, "Catatan maksimal 1000 karakter"),
  supplierInvoiceNumber: z.string().trim().max(100, "Nomor faktur maksimal 100 karakter"),
  supplierInvoiceDate: optionalDate,
  taxInvoiceNumber: z.string().trim().max(100, "Nomor faktur pajak maksimal 100 karakter"),
  taxInvoiceDate: optionalDate,
  assignedUserId: z.number().int().min(0),
  reason: z.string().trim().max(500, "Alasan maksimal 500 karakter"),
});

export type PurchaseHeaderValues = z.infer<typeof purchaseHeaderSchema>;

export const priceCorrectionLineSchema = z.object({
  lineId: z.number().int().positive(),
  unitPrice: z.number().int("Harga harus rupiah bulat").nonnegative("Harga tidak boleh negatif"),
  discountPct: z.number().min(0, "Diskon % minimal 0").max(100, "Diskon % maksimal 100"),
  discountNominal: z.number().int("Diskon harus rupiah bulat").nonnegative("Diskon nominal tidak boleh negatif"),
});

export const priceCorrectionSchema = z.object({
  lines: priceCorrectionLineSchema.array().min(1, "Ubah minimal satu harga"),
  reason: correctionReasonSchema,
});

export type PriceCorrectionLineValues = z.infer<typeof priceCorrectionLineSchema>;
export type PriceCorrectionValues = z.infer<typeof priceCorrectionSchema>;

/** Pesan galat pertama per path ("lines.0.unitPrice" → pesan). */
export function issueMap(error: z.ZodError): Record<string, string> {
  const out: Record<string, string> = {};
  for (const issue of error.issues) {
    const key = issue.path.join(".");
    if (!(key in out)) out[key] = issue.message;
  }
  return out;
}
