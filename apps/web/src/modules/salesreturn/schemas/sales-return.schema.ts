import { z } from "zod";

// Batasan disalin dari salesreturn/application/service.go createLocked:
// salesOrderId wajib, returnDate kosong = hari ini (server), minimal 1
// baris, tiap baris qty > 0 + satuan sama dengan SO + lokasi wajib
// (CheckLocation). Harga TIDAK ada di form — server memakai snapshot SO.
//
// Tahap D: returnMode ("return_only"|"exchange", kosong = return_only) +
// replacementItems (wajib ≥1 baris iff exchange, dilarang iff return_only —
// service.go: normalizeMode; server final, klien cermin agar pesan cepat).

// F2a: tiap baris retur punya kondisi (normal | damaged | not_returned).
// normal: lokasi hanya untuk produk berstok (server final, klien tak tahu
// tracked); damaged: server memilih lokasi rusak — klien TIDAK mewajibkan;
// not_returned: tanpa lokasi & tanpa gerak stok.
export const salesReturnLineSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  locationId: z.number().int().positive().nullable().optional(),
  uom: z.string().min(1, "Satuan wajib diisi"),
  qty: z.coerce.number().positive("Qty harus lebih dari 0"),
  condition: z.enum(["normal", "damaged", "not_returned"]),
});

export type SalesReturnLineFormValues = z.infer<typeof salesReturnLineSchema>;

// Baris pengganti: harga opsional (kosong = harga katalog), integer rupiah >= 0.
export const salesReturnReplacementLineSchema = z.object({
  productId: z.number().int().positive("Produk pengganti wajib dipilih"),
  variantId: z.number().int().positive("Varian pengganti wajib dipilih"),
  locationId: z.number().int().positive().nullable().optional(),
  uom: z.string().min(1, "Satuan pengganti wajib diisi"),
  qty: z.coerce.number().positive("Qty pengganti harus lebih dari 0"),
  unitPrice: z.number().int("Harga harus bilangan bulat").min(0, "Harga tidak boleh negatif").optional(),
});

export const salesReturnSchema = z
  .object({
    salesOrderId: z.number().int().positive("Order jual wajib dipilih"),
    returnDate: z.string().default(""),
    notes: z.string().default(""),
    reason: z
      .string()
      .trim()
      .min(1, "Alasan retur wajib diisi")
      .max(255, "Alasan maksimal 255 karakter"),
    returnMode: z.string().default("return_only"),
    items: z.array(salesReturnLineSchema).min(1, "Minimal 1 baris item dengan qty > 0"),
    replacementItems: z.array(salesReturnReplacementLineSchema).default([]),
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

// Batasan disalin dari service.go AddSettlement + settlement_rules.go (K-04c):
// retur harus confirmed (dicek server), jenis manual refund | customer_credit |
// collect_payment (reduce_receivable = offset otomatis), amount > 0 dan tidak
// melebihi sisa yang sah menurut arah selisih (server 409 bila lebih).
export const salesReturnSettlementSchema = z.object({
  type: z
    .string()
    .refine(
      (v) => v === "collect_payment" || v === "refund" || v === "customer_credit",
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
