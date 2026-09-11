import { z } from "zod";

// Batas kode 10 karakter ditegakkan server (KI-41); validasi di sini agar
// gagal cepat di form. Kode dikirim uppercase saat submit.
export const branchSchema = z.object({
  code: z
    .string()
    .trim()
    .min(1, "Kode wajib diisi")
    .max(10, "Kode maksimal 10 karakter"),
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
    .max(255, "Kota maksimal 255 karakter")
    .optional()
    .default(""),
  phone: z
    .string()
    .max(64, "Telepon maksimal 64 karakter")
    .optional()
    .default(""),
  isHead: z.boolean().optional().default(false),
  status: z.enum(["active", "inactive"]).optional().default("active"),
});

export type BranchFormValues = z.infer<typeof branchSchema>;
