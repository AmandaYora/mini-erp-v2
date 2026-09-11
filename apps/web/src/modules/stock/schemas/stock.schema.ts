import { z } from "zod";

// Skema request diverifikasi dari stock/presentation/handler.go:
// CreateLocation {code,name,parentId?}, UpdateLocation {name,parentId},
// Adjust {productId,variantId,locationId,mode,qtyAfter,qtyDelta,reason,approver,approverPassword},
// CreateTransfer {toBranchId,fromLocationId,toLocationId,notes?,items[+]-wajib-isi},
// damagedInput {productId,variantId,locationId,qty,reason?},
// write-off {productId,variantId,qty,reason-wajib}.

export const locationCreateSchema = z.object({
  code: z.string().min(1, "Kode wajib diisi"),
  name: z.string().min(1, "Nama wajib diisi"),
  parentId: z.number().int().positive().nullable().default(null),
});

export type LocationCreateValues = z.infer<typeof locationCreateSchema>;

export const locationUpdateSchema = z.object({
  name: z.string().min(1, "Nama wajib diisi"),
  parentId: z.number().int().positive().nullable().default(null),
});

export type LocationUpdateValues = z.infer<typeof locationUpdateSchema>;

export const adjustmentModeSchema = z.enum(["in", "out", "set"]);

export type AdjustmentMode = z.infer<typeof adjustmentModeSchema>;

export const adjustmentSchema = z
  .object({
    productId: z.number().int().positive("Produk wajib dipilih"),
    variantId: z.number().int().positive("Varian wajib dipilih"),
    locationId: z.number().int().positive("Lokasi wajib dipilih"),
    mode: adjustmentModeSchema,
    qty: z.coerce.number().min(0, "Jumlah minimal 0"),
    reason: z.string().min(1, "Alasan wajib diisi"),
    approver: z.string().default(""),
    approverPassword: z.string().default(""),
  })
  .refine((v) => (v.mode === "set" ? true : v.qty > 0), {
    path: ["qty"],
    message: "Jumlah harus lebih dari 0 untuk penambahan/pengurangan",
  });

export type AdjustmentFormValues = z.infer<typeof adjustmentSchema>;

export const transferItemSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  qty: z.coerce.number().positive("Jumlah harus lebih dari 0"),
});

export type TransferItemValues = z.infer<typeof transferItemSchema>;

export const transferSchema = z.object({
  toBranchId: z.number().int().positive("Cabang tujuan wajib dipilih"),
  fromLocationId: z.number().int().positive("Lokasi asal wajib dipilih"),
  toLocationId: z.number().int().positive("Lokasi tujuan wajib dipilih"),
  notes: z.string().default(""),
  items: z.array(transferItemSchema).min(1, "Minimal satu baris barang"),
});

export type TransferFormValues = z.infer<typeof transferSchema>;

// Pindah lokasi satu langkah (J4) — selalu dalam cabang aktif, tanpa dokumen
// bertahap. Lokasi asal dan tujuan wajib berbeda (server menolak juga).
export const moveLocationSchema = z
  .object({
    fromLocationId: z.number().int().positive("Lokasi asal wajib dipilih"),
    toLocationId: z.number().int().positive("Lokasi tujuan wajib dipilih"),
    notes: z.string().default(""),
    items: z.array(transferItemSchema).min(1, "Minimal satu baris barang"),
  })
  .refine((v) => v.fromLocationId !== v.toLocationId, {
    path: ["toLocationId"],
    message: "Tujuan harus berbeda dari asal",
  });

export type MoveLocationFormValues = z.infer<typeof moveLocationSchema>;

export const damagedMoveSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  locationId: z.number().int().positive("Lokasi wajib dipilih"),
  qty: z.coerce.number().positive("Jumlah harus lebih dari 0"),
  reason: z.string().default(""),
});

export type DamagedMoveValues = z.infer<typeof damagedMoveSchema>;

export const damagedWriteOffSchema = z.object({
  qty: z.coerce.number().positive("Jumlah harus lebih dari 0"),
  reason: z.string().min(1, "Alasan wajib diisi"),
});

export type DamagedWriteOffValues = z.infer<typeof damagedWriteOffSchema>;
