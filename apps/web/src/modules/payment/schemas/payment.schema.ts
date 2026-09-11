import { z } from "zod";

// Batasan tervalidasi dari backend (payment/application/service.go):
// - method create hanya "cash" | "transfer" ("offset" hanya kaki settle-credit).
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

export const paymentFormSchema = z.object({
  partyId: z
    .number({ invalid_type_error: "Pilih pihak dulu" })
    .int()
    .positive("Pilih pihak dulu"),
  method: z.enum(["cash", "transfer"], {
    required_error: "Pilih metode",
    invalid_type_error: "Pilih metode",
  }),
  amount: amountField,
  paidAt: z
    .string()
    .regex(/^\d{4}-\d{2}-\d{2}$/, "Format tanggal harus YYYY-MM-DD")
    .optional(),
  notes: z.string().max(500, "Catatan maksimal 500 karakter").optional(),
  allocations: z.array(paymentAllocationSchema).default([]),
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
