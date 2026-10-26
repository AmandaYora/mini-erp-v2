import { z } from "zod";

// Batasan tervalidasi dari backend (payment/application/service.go):
// - method create hanya "cash" | "transfer" | "check" ("offset" hanya kaki settle-credit).
// - amountTendered hanya untuk cash dan harus >= amount; reference maks 100.
// - allocations[] opsional; bila diisi, jumlahnya HARUS sama dengan amount.
// - settle-credit hanya pasangan retur jual↔order jual (customer) atau
//   retur beli↔order beli (supplier).

const amountField = z.coerce
  .number({ invalid_type_error: "Nominal harus angka" })
  .int("Nominal harus bilangan bulat")
  .positive("Nominal harus lebih dari 0");

export const paymentAllocationSchema = z.object({
  orderType: z.string().min(1, "Pilih tipe dokumen"),
  orderId: z.coerce
    .number({ invalid_type_error: "ID dokumen harus angka" })
    .int("ID dokumen harus bilangan bulat")
    .positive("ID dokumen harus lebih dari 0"),
  amount: amountField,
});

export const paymentFormSchema = z
  .object({
    partyId: z
      .number({ invalid_type_error: "Pilih pihak dulu" })
      .int()
      .positive("Pilih pihak dulu"),
    method: z.enum(["cash", "transfer", "check"], {
      required_error: "Pilih metode",
      invalid_type_error: "Pilih metode",
    }),
    amount: amountField,
    paidAt: z
      .string()
      .regex(/^\d{4}-\d{2}-\d{2}$/, "Format tanggal harus YYYY-MM-DD")
      .optional(),
    notes: z.string().max(500, "Catatan maksimal 500 karakter").optional(),
    reference: z.string().max(100, "Referensi maksimal 100 karakter").optional(),
    amountTendered: z.coerce
      .number({ invalid_type_error: "Uang diterima harus angka" })
      .int("Uang diterima harus bilangan bulat")
      .positive("Uang diterima harus lebih dari 0")
      .optional(),
    clientRef: z.string().max(100).optional(),
    /** Rekening perusahaan transfer/cek (ADR-0026): id akun buku besar. */
    cashAccountId: z.number().int().positive().optional(),
    allocations: z.array(paymentAllocationSchema).default([]),
  })
  .superRefine((v, ctx) => {
    if (v.cashAccountId !== undefined && v.method === "cash") {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["cashAccountId"], message: "Rekening hanya untuk transfer atau cek" });
    }
    if (v.amountTendered === undefined) return;
    if (v.method !== "cash") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["amountTendered"],
        message: "Uang diterima hanya untuk pembayaran tunai",
      });
    } else if (v.amountTendered < v.amount) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["amountTendered"],
        message: "Uang diterima kurang dari nominal pembayaran",
      });
    }
  });

export type PaymentFormValues = z.infer<typeof paymentFormSchema>;

export const settleCreditSchema = z
  .object({
    partyId: z
      .number({ invalid_type_error: "Pilih pihak dulu" })
      .int()
      .positive("Pilih pihak dulu"),
    returnType: z.enum(["sales_return", "purchase_return"], {
      required_error: "Pilih tipe retur",
      invalid_type_error: "Pilih tipe retur",
    }),
    returnId: z.coerce
      .number({ invalid_type_error: "ID retur harus angka" })
      .int("ID retur harus bilangan bulat")
      .positive("ID retur harus lebih dari 0"),
    orderType: z.enum(["sales", "purchase"], {
      required_error: "Pilih tipe tagihan",
      invalid_type_error: "Pilih tipe tagihan",
    }),
    orderId: z.coerce
      .number({ invalid_type_error: "ID tagihan harus angka" })
      .int("ID tagihan harus bilangan bulat")
      .positive("ID tagihan harus lebih dari 0"),
    amount: amountField,
    notes: z.string().max(500, "Catatan maksimal 500 karakter").optional(),
  })
  .superRefine((v, ctx) => {
    const ok =
      (v.returnType === "sales_return" && v.orderType === "sales") ||
      (v.returnType === "purchase_return" && v.orderType === "purchase");
    if (!ok) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["orderType"],
        message:
          "Pasangan tidak sesuai (retur jual↔order jual, retur beli↔order beli)",
      });
    }
  });

export type SettleCreditFormValues = z.infer<typeof settleCreditSchema>;

// Batal pembayaran dari web: alasan wajib (backend menyimpan maks 255 di cancel_reason).
export const cancelPaymentSchema = z.object({
  reason: z
    .string()
    .trim()
    .min(1, "Alasan pembatalan wajib diisi")
    .max(255, "Alasan maksimal 255 karakter"),
});

export type CancelPaymentValues = z.infer<typeof cancelPaymentSchema>;

// Koreksi pembayaran (POST /payments/:id/correct, service: Correct): metode/nominal/tanggal/
// referensi/uang diterima/catatan baru + alasan wajib 5–255 karakter. Alokasi tidak diisi pengguna:
// backend membagi nominal baru ke dokumen pembayaran asli sesuai urutannya.
export const paymentCorrectSchema = z
  .object({
    method: z.enum(["cash", "transfer", "check"], {
      required_error: "Pilih metode",
      invalid_type_error: "Pilih metode",
    }),
    amount: amountField,
    paidAt: z
      .string()
      .regex(/^\d{4}-\d{2}-\d{2}$/, "Format tanggal harus YYYY-MM-DD")
      .optional(),
    reference: z.string().max(100, "Referensi maksimal 100 karakter").optional(),
    amountTendered: z.coerce
      .number({ invalid_type_error: "Uang diterima harus angka" })
      .int("Uang diterima harus bilangan bulat")
      .positive("Uang diterima harus lebih dari 0")
      .optional(),
    notes: z.string().max(500, "Catatan maksimal 500 karakter").optional(),
    reason: z
      .string()
      .trim()
      .min(5, "Alasan koreksi minimal 5 karakter")
      .max(255, "Alasan maksimal 255 karakter"),
    /** Rekening transfer/cek (ADR-0026). */
    cashAccountId: z.number().int().positive().optional(),
  })
  .superRefine((v, ctx) => {
    if (v.amountTendered === undefined) return;
    if (v.method !== "cash") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["amountTendered"],
        message: "Uang diterima hanya untuk pembayaran tunai",
      });
    } else if (v.amountTendered < v.amount) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["amountTendered"],
        message: "Uang diterima kurang dari nominal pembayaran",
      });
    }
  });

export type PaymentCorrectValues = z.infer<typeof paymentCorrectSchema>;
