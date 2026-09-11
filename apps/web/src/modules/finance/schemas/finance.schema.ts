import { z } from "zod";

// Batasan tervalidasi dari backend (apps/api/internal/modules/finance):
// - posting docType hanya goods_receipt|delivery|payment|sales_return|
//   purchase_return (builders.go: buildLines).
// - manual: memo wajib; lines ≥1; tiap baris TEPAT satu sisi >0 dan total
//   debit == kredit > 0 (posting.go: insertEntry + Manual; akun dicari by
//   CODE, huruf besar otomatis).
// - akun: code unik & tak bisa diubah (UpdateAccount tanpa code); type salah
//   satu dari 5 tipe; ganti type akun berjurnal ditolak backend.
// - expense: expenseAccountId harus akun type beban & aktif; payAccountId
//   harus aktif & berbeda; amount > 0 (expenses.go: CreateExpense).
// - reverse: reason wajib (posting.go: Reverse).
// - mapping: key harus salah satu kunci dikenal; accountId harus akun aktif
//   dengan type yang cocok (service.go: SetMapping).
// - periode: year 2000–2100, month 1–12 (service.go: validYearMonth).

export const postingFormSchema = z.object({
  docType: z.enum(
    ["goods_receipt", "delivery", "payment", "sales_return", "purchase_return"],
    {
      required_error: "Pilih tipe dokumen",
      invalid_type_error: "Pilih tipe dokumen",
    },
  ),
  docId: z.coerce
    .number({ invalid_type_error: "ID dokumen harus angka" })
    .int("ID dokumen harus bilangan bulat")
    .positive("ID dokumen harus lebih dari 0"),
});

export type PostingFormValues = z.infer<typeof postingFormSchema>;

const dateField = z
  .string()
  .regex(/^\d{4}-\d{2}-\d{2}$/, "Format tanggal harus YYYY-MM-DD")
  .optional();

export const manualLineSchema = z.object({
  accountCode: z.string().min(1, "Pilih akun"),
  debit: z.coerce
    .number({ invalid_type_error: "Debit harus angka" })
    .int("Debit harus bilangan bulat")
    .nonnegative("Debit tidak boleh negatif"),
  credit: z.coerce
    .number({ invalid_type_error: "Kredit harus angka" })
    .int("Kredit harus bilangan bulat")
    .nonnegative("Kredit tidak boleh negatif"),
});

export const manualJournalSchema = z
  .object({
    date: dateField,
    memo: z.string().min(1, "Memo wajib diisi").max(500, "Memo maksimal 500 karakter"),
    lines: z.array(manualLineSchema).min(2, "Minimal 2 baris jurnal"),
  })
  .superRefine((v, ctx) => {
    v.lines.forEach((l, i) => {
      const oneSided =
        (l.debit > 0 && l.credit === 0) || (l.credit > 0 && l.debit === 0);
      if (!oneSided) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["lines", i, "debit"],
          message: "Tiap baris tepat satu sisi (debit XOR kredit)",
        });
      }
    });
    const debit = v.lines.reduce((s, l) => s + l.debit, 0);
    const credit = v.lines.reduce((s, l) => s + l.credit, 0);
    if (debit <= 0 || debit !== credit) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["lines"],
        message: "Total debit harus sama dengan kredit dan lebih dari 0",
      });
    }
  });

export type ManualJournalValues = z.infer<typeof manualJournalSchema>;

export const accountCreateSchema = z.object({
  code: z.string().min(1, "Kode wajib diisi").max(20, "Kode maksimal 20 karakter"),
  name: z.string().min(1, "Nama wajib diisi").max(200, "Nama maksimal 200 karakter"),
  type: z.enum(["aset", "kewajiban", "modal", "pendapatan", "beban"], {
    required_error: "Pilih tipe",
    invalid_type_error: "Pilih tipe",
  }),
  isCash: z.boolean(),
});

export type AccountCreateValues = z.infer<typeof accountCreateSchema>;

// Edit tanpa code — code dikunci (backend UpdateAccount tidak menerima code).
export const accountEditSchema = accountCreateSchema.omit({ code: true });

export type AccountEditValues = z.infer<typeof accountEditSchema>;

export const expenseFormSchema = z
  .object({
    expenseAccountId: z
      .number({ invalid_type_error: "Pilih akun beban dulu" })
      .int()
      .positive("Pilih akun beban dulu"),
    payAccountId: z
      .number({ invalid_type_error: "Pilih akun bayar dulu" })
      .int()
      .positive("Pilih akun bayar dulu"),
    amount: z.coerce
      .number({ invalid_type_error: "Nominal harus angka" })
      .int("Nominal harus bilangan bulat")
      .positive("Nominal harus lebih dari 0"),
    date: dateField,
    notes: z.string().max(500, "Catatan maksimal 500 karakter").optional(),
  })
  .superRefine((v, ctx) => {
    if (v.expenseAccountId === v.payAccountId) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["payAccountId"],
        message: "Akun bayar harus berbeda dari akun beban",
      });
    }
  });

export type ExpenseFormValues = z.infer<typeof expenseFormSchema>;

export const reverseSchema = z.object({
  reason: z
    .string()
    .min(1, "Alasan wajib diisi")
    .max(500, "Alasan maksimal 500 karakter"),
});

export type ReverseValues = z.infer<typeof reverseSchema>;

export const mappingSchema = z.object({
  key: z.string().min(1, "Pilih kunci pemetaan"),
  accountId: z
    .number({ invalid_type_error: "Pilih akun dulu" })
    .int()
    .positive("Pilih akun dulu"),
});

export type MappingValues = z.infer<typeof mappingSchema>;

export const periodSchema = z.object({
  year: z.coerce
    .number({ invalid_type_error: "Tahun harus angka" })
    .int("Tahun harus bilangan bulat")
    .min(2000, "Tahun minimal 2000")
    .max(2100, "Tahun maksimal 2100"),
  month: z.coerce
    .number({ invalid_type_error: "Bulan harus angka" })
    .int("Bulan harus bilangan bulat")
    .min(1, "Bulan 1–12")
    .max(12, "Bulan 1–12"),
});

export type PeriodValues = z.infer<typeof periodSchema>;
