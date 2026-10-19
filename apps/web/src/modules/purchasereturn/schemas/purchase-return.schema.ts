import { z } from "zod";

// Batasan disalin dari purchasereturn/application/service.go createLocked:
// purchaseOrderId wajib, returnDate kosong = hari ini (server), minimal 1
// baris, tiap baris qty > 0 + satuan sama dengan PO + lokasi wajib
// (CheckLocation). Harga TIDAK ada di form — server memakai snapshot PO.

export const purchaseReturnLineSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  locationId: z.number().int().positive("Lokasi wajib dipilih"),
  uom: z.string().min(1, "Satuan wajib diisi"),
  qty: z.coerce.number().positive("Qty harus lebih dari 0"),
});

export type PurchaseReturnLineFormValues = z.infer<typeof purchaseReturnLineSchema>;

export const purchaseReturnSchema = z.object({
  purchaseOrderId: z.number().int().positive("Order beli wajib dipilih"),
  // Tanggal tidak boleh di masa depan (cek di modal via validateReturnDate).
  returnDate: z.string().default(""),
  notes: z.string().default(""),
  reason: z
    .string()
    .trim()
    .min(1, "Alasan retur wajib diisi")
    .max(255, "Alasan maksimal 255 karakter"),
  items: z.array(purchaseReturnLineSchema).min(1, "Minimal 1 baris item dengan qty > 0"),
});

export type PurchaseReturnFormValues = z.infer<typeof purchaseReturnSchema>;

// Batasan disalin dari service.go AddSettlement + settlement_rules.go (K-04b/c):
// retur harus confirmed (dicek server), jenis manual hanya refund atau
// supplier_credit (reduce_payable = offset otomatis), amount > 0 dan tidak
// melebihi kelebihan di atas sisa hutang (server 409 bila lebih).
export const purchaseReturnSettlementSchema = z.object({
  type: z
    .string()
    .refine((v) => v === "refund" || v === "supplier_credit", {
      message: "Jenis penyelesaian wajib dipilih",
    }),
  date: z.string().default(""),
  amount: z.coerce.number().positive("Nominal harus lebih dari 0"),
  paymentMethod: z.string().default(""),
  referenceNumber: z.string().default(""),
  notes: z.string().default(""),
});

export type PurchaseReturnSettlementFormValues = z.infer<
  typeof purchaseReturnSettlementSchema
>;
