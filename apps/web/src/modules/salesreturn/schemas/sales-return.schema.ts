import { z } from "zod";

// Batasan disalin dari salesreturn/application/service.go createLocked:
// salesOrderId wajib, returnDate kosong = hari ini (server), minimal 1
// baris, tiap baris qty > 0 + satuan sama dengan SO + lokasi wajib
// (CheckLocation). Harga TIDAK ada di form — server memakai snapshot SO.
//
// Tahap D: returnMode ("return_only"|"exchange", kosong = return_only) +
// replacementItems (wajib ≥1 baris iff exchange, dilarang iff return_only —
// service.go: normalizeMode; server final, klien cermin agar pesan cepat).

export const salesReturnLineSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  locationId: z.number().int().positive("Lokasi wajib dipilih"),
  uom: z.string().min(1, "Satuan wajib diisi"),
  qty: z.coerce.number().positive("Qty harus lebih dari 0"),
});

export type SalesReturnLineFormValues = z.infer<typeof salesReturnLineSchema>;

export const salesReturnSchema = z
  .object({
    salesOrderId: z.number().int().positive("Order jual wajib dipilih"),
    returnDate: z.string().default(""),
    notes: z.string().default(""),
    returnMode: z.string().default("return_only"),
    items: z.array(salesReturnLineSchema).min(1, "Minimal 1 baris item dengan qty > 0"),
    replacementItems: z.array(salesReturnLineSchema).default([]),
  })
  .refine((v) => v.returnMode === "return_only" || v.returnMode === "exchange", {
    message: "Mode retur tidak dikenal",
    path: ["returnMode"],
  })
  .refine(
    (v) => v.returnMode !== "exchange" || v.replacementItems.length > 0,
    {
      message: "Retur tukar wajib membawa minimal 1 barang pengganti",
      path: ["replacementItems"],
    },
  )
  .refine(
    (v) => v.returnMode !== "return_only" || v.replacementItems.length === 0,
    {
      message: "Barang pengganti hanya untuk retur tukar",
      path: ["replacementItems"],
    },
  );

export type SalesReturnFormValues = z.infer<typeof salesReturnSchema>;

// Batasan disalin dari service.go AddSettlement: retur harus confirmed
// (dicek server), type dikenal, amount > 0 dan total settlement tidak boleh
// melebihi total retur (server 409 bila lebih).
export const salesReturnSettlementSchema = z.object({
  type: z
    .string()
    .refine(
      (v) =>
        v === "collect_payment" ||
        v === "reduce_receivable" ||
        v === "refund" ||
        v === "customer_credit",
      { message: "Jenis penyelesaian wajib dipilih" },
    ),
  date: z.string().default(""),
  amount: z.coerce.number().positive("Nominal harus lebih dari 0"),
  paymentMethod: z.string().default(""),
  referenceNumber: z.string().default(""),
  notes: z.string().default(""),
});

export type SalesReturnSettlementFormValues = z.infer<
  typeof salesReturnSettlementSchema
>;
