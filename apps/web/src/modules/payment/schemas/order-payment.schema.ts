import { z } from "zod";
import type { PaymentCreatePayload } from "@/modules/payment/types";

// Pembayaran dari dalam detail order: alokasi dikunci ke order itu, nominal
// tidak boleh melebihi sisa tagihan (server tetap penentu). Aturan metode
// sama dengan payment/application: uang diterima hanya cash dan >= nominal.

export function buildOrderPaymentSchema(outstanding: number) {
  return z
    .object({
      amount: z.coerce
        .number({ invalid_type_error: "Nominal harus angka" })
        .int("Nominal harus bilangan bulat")
        .positive("Nominal harus lebih dari 0")
        .max(outstanding > 0 ? outstanding : Number.MAX_SAFE_INTEGER, "Nominal melebihi sisa tagihan"),
      method: z.enum(["cash", "transfer", "check"]),
      paidAt: z
        .string()
        .regex(/^\d{4}-\d{2}-\d{2}$/, "Format tanggal harus YYYY-MM-DD")
        .or(z.literal("")),
      reference: z.string().max(100, "Referensi maksimal 100 karakter").default(""),
      amountTendered: z
        .string()
        .default("")
        .refine((v) => v.trim() === "" || (Number.isInteger(Number(v)) && Number(v) > 0), "Uang diterima harus rupiah bulat"),
      notes: z.string().max(500, "Catatan maksimal 500 karakter").default(""),
    })
    .superRefine((v, ctx) => {
      if (v.amountTendered.trim() === "") return;
      if (v.method !== "cash") {
        ctx.addIssue({ code: "custom", path: ["amountTendered"], message: "Uang diterima hanya untuk pembayaran tunai" });
      } else if (Number(v.amountTendered) < v.amount) {
        ctx.addIssue({ code: "custom", path: ["amountTendered"], message: "Uang diterima kurang dari nominal" });
      }
    });
}

export type OrderPaymentValues = z.infer<ReturnType<typeof buildOrderPaymentSchema>>;

/** Nilai tervalidasi → body POST /payments dengan satu alokasi ke order. */
export function toOrderPaymentPayload(
  v: OrderPaymentValues,
  ctx: { partyId: number; orderType: "sales" | "purchase"; orderId: number; clientRef: string; cashAccountId?: number | null },
): PaymentCreatePayload {
  const out: PaymentCreatePayload = {
    partyId: ctx.partyId,
    amount: v.amount,
    method: v.method,
    clientRef: ctx.clientRef,
    allocations: [{ orderType: ctx.orderType, orderId: ctx.orderId, amount: v.amount }],
  };
  if (v.paidAt !== "") out.paidAt = v.paidAt;
  if (v.notes.trim() !== "") out.notes = v.notes.trim();
  if (v.method === "cash") {
    if (v.amountTendered.trim() !== "") out.amountTendered = Number(v.amountTendered);
  } else {
    if (v.reference.trim() !== "") out.reference = v.reference.trim();
    if (ctx.cashAccountId) out.cashAccountId = ctx.cashAccountId;
  }
  return out;
}
