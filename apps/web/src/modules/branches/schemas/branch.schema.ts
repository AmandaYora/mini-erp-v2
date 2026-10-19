import { z } from "zod";

// Batas kolom ditegakkan server (KI-41, F3b-6: kode 50, kota 120, label lokasi 150);
// validasi di sini agar gagal cepat di form. Kode dikirim uppercase saat submit.
export const BRANCH_CODE_MAX = 50;
export const BRANCH_LOCATION_LABEL_MAX = 150;

export const branchSchema = z.object({
  code: z
    .string()
    .trim()
    .min(1, "Kode wajib diisi")
    .max(BRANCH_CODE_MAX, "Kode maksimal 50 karakter"),
  name: z
    .string()
    .trim()
    .min(1, "Nama wajib diisi")
    .max(255, "Nama maksimal 255 karakter"),
  address: z
    .string()
    .max(1000, "Alamat maksimal 1000 karakter")
    .optional()
    .default(""),
  city: z
    .string()
    .max(120, "Kota maksimal 120 karakter")
    .optional()
    .default(""),
  phone: z
    .string()
    .max(64, "Telepon maksimal 64 karakter")
    .optional()
    .default(""),
  defaultLocationLabel: z
    .string()
    .trim()
    .max(BRANCH_LOCATION_LABEL_MAX, "Label lokasi maksimal 150 karakter")
    .optional()
    .default(""),
  isHead: z.boolean().optional().default(false),
  status: z.enum(["active", "inactive"]).optional().default("active"),
});

export type BranchFormValues = z.infer<typeof branchSchema>;
