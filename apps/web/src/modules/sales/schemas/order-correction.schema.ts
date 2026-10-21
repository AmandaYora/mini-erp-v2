import { z } from "zod";

// Koreksi order jual oleh pengguna (sales/application/corrections.go):
// - alasan batal / tutup sisa order: wajib di UI, maks 255 (kolom cancel_reason,
//   short_close_reason);
// - ubah kolom ringan order terkonfirmasi yang sudah punya dokumen turunan
//   (service.go lightOnly + lightDueDateErrors + adHocShipToErrors): jatuh tempo
//   (wajib untuk termin net), catatan, alamat kirim, faktur pajak, penanggung
//   jawab, alasan opsional.

export const MAX_REASON = 255;

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

export const requiredReasonSchema = z
  .string()
  .trim()
  .min(1, "Alasan wajib diisi")
  .max(MAX_REASON, "Alasan maksimal 255 karakter");

export const optionalReasonSchema = z.string().trim().max(MAX_REASON, "Alasan maksimal 255 karakter").default("");

export const headerEditSchema = z
  .object({
    paymentTerms: z.string(),
    dueDate: z.string().trim().default(""),
    notes: z.string().default(""),
    shipToLabel: z.string().trim().max(120, "Label maksimal 120 karakter").default(""),
    shipToRecipient: z.string().trim().max(150, "Penerima maksimal 150 karakter").default(""),
    shipToPhone: z.string().trim().max(30, "Telepon maksimal 30 karakter").default(""),
    shipToAddress: z.string().trim().max(1000, "Alamat maksimal 1000 karakter").default(""),
    taxInvoiceNumber: z.string().trim().max(100, "Nomor faktur maksimal 100 karakter").default(""),
    taxInvoiceDate: z.string().trim().default(""),
    assignedUserId: z.number().int().positive().nullable().default(null),
    reason: optionalReasonSchema,
  })
  .superRefine((v, ctx) => {
    if (v.paymentTerms === "net" && v.dueDate === "") {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["dueDate"], message: "Jatuh tempo wajib diisi untuk termin net" });
    }
    if (v.dueDate !== "" && !DATE_RE.test(v.dueDate)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["dueDate"], message: "Format tanggal YYYY-MM-DD" });
    }
    if (v.taxInvoiceDate !== "" && !DATE_RE.test(v.taxInvoiceDate)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["taxInvoiceDate"], message: "Format tanggal YYYY-MM-DD" });
    }
    const anyShip = [v.shipToLabel, v.shipToRecipient, v.shipToPhone].some((t) => t !== "");
    if (anyShip && v.shipToAddress === "") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["shipToAddress"],
        message: "Alamat wajib diisi bila penerima/telepon/label diisi",
      });
    }
  });

export type HeaderEditValues = z.infer<typeof headerEditSchema>;
