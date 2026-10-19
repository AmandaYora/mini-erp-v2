import { z } from "zod";
import type { DeliveryConfirmPayload } from "@/modules/delivery/types";

// Cermin validateRecipient + planSettlement backend (delivery/application/
// service.go): status tanda terima hanya signed|missing; bila status diisi
// nama wajib, bila missing alasan wajib. Settlement hanya berlaku untuk order
// COD (server menolaknya untuk net/prepaid); pay_now = bayar di tempat,
// switch_to_net = jadikan tempo (wajib dueDate YYYY-MM-DD).

const dateRe = /^\d{4}-\d{2}-\d{2}$/;

export const settlementSchema = z
  .object({
    mode: z.enum(["pay_now", "switch_to_net"]),
    method: z.enum(["cash", "transfer", "check"]).default("cash"),
    paymentDate: z.string().default(""),
    reference: z.string().default(""),
    amountTendered: z.string().default(""),
    dueDate: z.string().default(""),
  })
  .superRefine((v, ctx) => {
    if (v.mode === "switch_to_net") {
      if (v.dueDate.trim() === "") {
        ctx.addIssue({ code: "custom", path: ["dueDate"], message: "Jatuh tempo wajib diisi" });
      } else if (!dateRe.test(v.dueDate.trim())) {
        ctx.addIssue({ code: "custom", path: ["dueDate"], message: "Format tanggal YYYY-MM-DD" });
      }
      return;
    }
    if (v.paymentDate.trim() !== "" && !dateRe.test(v.paymentDate.trim())) {
      ctx.addIssue({ code: "custom", path: ["paymentDate"], message: "Format tanggal YYYY-MM-DD" });
    }
    if (v.method !== "cash" && v.reference.trim() === "") {
      ctx.addIssue({
        code: "custom",
        path: ["reference"],
        message: v.method === "check" ? "Nomor cek wajib diisi" : "Referensi transfer wajib diisi",
      });
    }
    if (v.amountTendered.trim() !== "") {
      const n = Number(v.amountTendered);
      if (!Number.isInteger(n) || n < 0) {
        ctx.addIssue({
          code: "custom",
          path: ["amountTendered"],
          message: "Uang diterima harus rupiah bulat",
        });
      }
    }
  });

export const deliveryConfirmSchema = z
  .object({
    recipientName: z.string().max(150, "Maksimal 150 karakter").default(""),
    recipientSignatureStatus: z.enum(["", "signed", "missing"]).default(""),
    recipientSignatureMissingReason: z.string().default(""),
    settlement: settlementSchema.nullable().default(null),
  })
  .superRefine((v, ctx) => {
    if (v.recipientSignatureStatus !== "" && v.recipientName.trim() === "") {
      ctx.addIssue({
        code: "custom",
        path: ["recipientName"],
        message: "Nama penerima wajib diisi bila ada status tanda terima.",
      });
    }
    if (v.recipientSignatureStatus === "missing" && v.recipientSignatureMissingReason.trim() === "") {
      ctx.addIssue({
        code: "custom",
        path: ["recipientSignatureMissingReason"],
        message: "Alasan wajib diisi bila tanda tangan tidak ada.",
      });
    }
  });

export type DeliveryConfirmFormInput = z.input<typeof deliveryConfirmSchema>;
export type DeliveryConfirmFormValues = z.infer<typeof deliveryConfirmSchema>;

/** Nilai form tervalidasi → body confirm; undefined bila konfirmasi polos. */
export function toConfirmPayload(v: DeliveryConfirmFormValues): DeliveryConfirmPayload | undefined {
  const out: DeliveryConfirmPayload = {};
  if (v.recipientName.trim() !== "") out.recipientName = v.recipientName.trim();
  if (v.recipientSignatureStatus !== "") out.recipientSignatureStatus = v.recipientSignatureStatus;
  if (v.recipientSignatureMissingReason.trim() !== "") {
    out.recipientSignatureMissingReason = v.recipientSignatureMissingReason.trim();
  }
  const st = v.settlement;
  if (st) {
    if (st.mode === "switch_to_net") {
      out.settlement = { mode: "switch_to_net", dueDate: st.dueDate.trim() };
    } else {
      const s: NonNullable<DeliveryConfirmPayload["settlement"]> = { mode: "pay_now", method: st.method };
      if (st.paymentDate.trim() !== "") s.paymentDate = st.paymentDate.trim();
      if (st.reference.trim() !== "") s.reference = st.reference.trim();
      if (st.amountTendered.trim() !== "" && Number(st.amountTendered) > 0) {
        s.amountTendered = Number(st.amountTendered);
      }
      out.settlement = s;
    }
  }
  return Object.keys(out).length === 0 ? undefined : out;
}
