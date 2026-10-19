import { z } from "zod";

// Skema request diverifikasi dari stock/presentation/handler.go:
// CreateLocation {code,name,parentId?,isPickingArea?,isPrimary?,sortOrder?},
// UpdateLocation {code?,name,parentId,isPickingArea?,isPrimary?,sortOrder?},
// Adjust {productId,variantId,locationId,mode,qtyAfter,qtyDelta,reason,approver,approverPassword},
// CreateTransfer {toBranchId,fromLocationId,toLocationId,notes?,items[+]-wajib-isi},
// damagedInput {productId,variantId,locationId,qty,reason?},
// write-off {productId,variantId,qty,reason-wajib}.

const locationAttributes = {
  isPickingArea: z.boolean().optional(),
  isPrimary: z.boolean().optional(),
  sortOrder: z.number().int("Urutan harus bilangan bulat").min(0, "Urutan tidak boleh negatif").max(100000, "Urutan terlalu besar").optional(),
};

export const locationCreateSchema = z.object({
  code: z.string().min(1, "Kode wajib diisi").max(30, "Kode maksimal 30 karakter"),
  name: z.string().min(1, "Nama wajib diisi"),
  parentId: z.number().int().positive().nullable().default(null),
  forceTransfer: z.boolean().optional(),
  ...locationAttributes,
});

export type LocationCreateValues = z.infer<typeof locationCreateSchema>;

export const locationUpdateSchema = z.object({
  /** Kode boleh diubah (ST-01); kosong = tidak berubah. Server menolak bila bentrok. */
  code: z.string().max(30, "Kode maksimal 30 karakter").optional(),
  name: z.string().min(1, "Nama wajib diisi"),
  parentId: z.number().int().positive().nullable().default(null),
  forceTransfer: z.boolean().optional(),
  ...locationAttributes,
});

export type LocationUpdateValues = z.infer<typeof locationUpdateSchema>;

export const adjustmentModeSchema = z.enum(["in", "out", "set"]);

export type AdjustmentMode = z.infer<typeof adjustmentModeSchema>;

/**
 * Kebijakan koreksi (backend strictAdjust): penyetuju WAJIB kecuali pengaturan
 * stock_adjust_always_approve secara eksplisit "false".
 */
export function isStrictAdjust(setting: string | null | undefined): boolean {
  return String(setting ?? "").trim().toLowerCase() !== "false";
}

export function adjustmentSchemaFor(strictApprover: boolean) {
  return z
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
  })
  .refine((v) => !strictApprover || v.approver.trim() !== "", {
    path: ["approver"],
    message: "Penyetuju wajib diisi (kebijakan: semua koreksi perlu persetujuan)",
  })
  .refine((v) => !strictApprover || v.approverPassword !== "", {
    path: ["approverPassword"],
    message: "Kata sandi penyetuju wajib diisi",
  });
}

export const adjustmentSchema = adjustmentSchemaFor(false);

export type AdjustmentFormValues = z.infer<typeof adjustmentSchema>;

export const transferItemSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  qty: z.coerce.number().positive("Jumlah harus lebih dari 0"),
  /** Tujuan per baris (opsional); 0/undefined = tujuan dokumen. */
  toLocationId: z.number().int().nonnegative().optional(),
});

export type TransferItemValues = z.infer<typeof transferItemSchema>;

/**
 * Transfer antar cabang: sopir wajib bila cabang tujuan beda dari cabang asal
 * (BR-20; server menegakkan yang sama). `sameBranch` = cabang tujuan = cabang sesi.
 */
export function transferSchemaFor(sameBranch: boolean) {
  return z
    .object({
      toBranchId: z.number().int().positive("Cabang tujuan wajib dipilih"),
      fromLocationId: z.number().int().positive("Lokasi asal wajib dipilih"),
      toLocationId: z.number().int().positive("Lokasi tujuan wajib dipilih"),
      notes: z.string().default(""),
      driverName: z.string().trim().max(150, "Maksimal 150 karakter").default(""),
      vehiclePlate: z.string().trim().max(30, "Maksimal 30 karakter").default(""),
      transferDate: z.string().regex(/^(\d{4}-\d{2}-\d{2})?$/, "Format tanggal harus YYYY-MM-DD").default(""),
      items: z.array(transferItemSchema).min(1, "Minimal satu baris barang"),
    })
    .refine((v) => sameBranch || v.driverName !== "", {
      path: ["driverName"],
      message: "Sopir wajib diisi untuk transfer antar cabang",
    });
}

export const transferSchema = transferSchemaFor(true);

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
  // Alasan wajib untuk catat rusak maupun pulihkan (backend moveDamaged).
  reason: z.string().trim().min(1, "Alasan wajib diisi"),
});

/** Catat rusak: lokasi asal opsional (0 = barang datang sudah rusak, ST-06). */
export const damagedMoveInSchema = damagedMoveSchema.extend({
  locationId: z.number().int().nonnegative(),
});

export type DamagedMoveValues = z.infer<typeof damagedMoveSchema>;

export const damagedWriteOffSchema = z.object({
  qty: z.coerce.number().positive("Jumlah harus lebih dari 0"),
  reason: z.string().min(1, "Alasan wajib diisi"),
});

export type DamagedWriteOffValues = z.infer<typeof damagedWriteOffSchema>;
