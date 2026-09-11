import { z } from "zod";

export const createRoleSchema = z.object({
  code: z
    .string()
    .trim()
    .min(1, "Kode wajib diisi")
    .max(64, "Kode maksimal 64 karakter")
    .regex(
      /^[A-Za-z0-9._-]+$/,
      "Kode hanya boleh huruf, angka, titik, strip, underscore",
    ),
  name: z
    .string()
    .trim()
    .min(1, "Nama wajib diisi")
    .max(255, "Nama maksimal 255 karakter"),
  description: z
    .string()
    .max(1000, "Deskripsi maksimal 1000 karakter")
    .optional()
    .default(""),
});

export type CreateRoleValues = z.infer<typeof createRoleSchema>;

export const editRoleSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Nama wajib diisi")
    .max(255, "Nama maksimal 255 karakter"),
  description: z
    .string()
    .max(1000, "Deskripsi maksimal 1000 karakter")
    .optional()
    .default(""),
});

export type EditRoleValues = z.infer<typeof editRoleSchema>;
