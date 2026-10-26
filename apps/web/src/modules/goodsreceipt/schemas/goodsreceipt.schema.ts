import { z } from "zod";
import type { ReceiptSettlementPayload } from "@/modules/goodsreceipt/types";

// Batasan disalin dari goodsreceipt/application/service.go + settlement.go +
// handler.go:
// - PO harus confirmed (server menolak selain itu: "Order harus dikonfirmasi
//   dulu"), qty > 0, satuan harus sama dengan PO (maka UOM read-only dari PO),
//   over-receipt ditolak server ("Melebihi sisa PO") — klien menjepit ke qty
//   PO sebagai perlindungan awal.
// - Lokasi opsional di klien: barang non-stok tidak butuh lokasi; untuk barang
//   stok server menolak lokasi kosong/invalid (CheckLocation).
// - supplierDeliveryNumber maks 60, clientRef maks 64 karakter.
// - settlement hanya untuk PO COD yang dituntaskan penerimaan ini dan masih
//   ada sisa tagihan (wajib di sana); server menolaknya untuk net/prepaid.

const dateRe = /^\d{4}-\d{2}-\d{2}$/;

export const receiveLineSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  locationId: z.number().int().min(0).default(0),
  uom: z.string().min(1, "Satuan wajib diisi"),
  qty: z.coerce.number().positive("Qty harus lebih dari 0"),
});

export type ReceiveLineFormValues = z.infer<typeof receiveLineSchema>;

export const receiptSettlementSchema = z
  .object({
    mode: z.enum(["pay_now", "switch_to_net"]),
    method: z.enum(["cash", "transfer", "check"]).default("cash"),
    paymentDate: z.string().default(""),
    reference: z.string().default(""),
    dueDate: z.string().default(""),
    notes: z.string().trim().max(255, "Catatan maksimal 255 karakter").default(""),
    cashAccountId: z.number().int().positive().nullable().default(null),
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
  });

export type ReceiptSettlementFormValues = z.infer<typeof receiptSettlementSchema>;

/** Nilai form tervalidasi → payload settlement API. */
export function toSettlementPayload(v: ReceiptSettlementFormValues): ReceiptSettlementPayload {
  if (v.mode === "switch_to_net") return { mode: "switch_to_net", dueDate: v.dueDate.trim() };
  const out: ReceiptSettlementPayload = { mode: "pay_now", method: v.method };
  if (v.paymentDate.trim() !== "") out.paymentDate = v.paymentDate.trim();
  if (v.reference.trim() !== "") out.reference = v.reference.trim();
  if (v.notes.trim() !== "") out.notes = v.notes.trim();
  if (v.method !== "cash" && v.cashAccountId !== null) out.cashAccountId = v.cashAccountId;
  return out;
}

export const receiveSchema = z.object({
  purchaseOrderId: z.number().int().positive("Purchase order wajib dipilih"),
  // "" = kosongkan → server memakai tanggal hari ini (WIB).
  receivedAt: z.string().default(""),
  notes: z.string().default(""),
  supplierDeliveryNumber: z.string().trim().max(60, "Maksimal 60 karakter").default(""),
  clientRef: z.string().max(64, "clientRef maksimal 64 karakter").default(""),
  settlement: receiptSettlementSchema.nullable().default(null),
  items: z.array(receiveLineSchema).min(1, "Minimal 1 baris item"),
});

export type ReceiveFormValues = z.infer<typeof receiveSchema>;
