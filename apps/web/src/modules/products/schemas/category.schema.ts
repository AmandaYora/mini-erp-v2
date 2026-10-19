import { z } from "zod";

// Create: {code, name, parentId, sortOrder} — Update: {name, parentId, sortOrder} (kode dikunci).
// Diverifikasi dari CreateCategory/UpdateCategory di
// apps/api/internal/modules/product/presentation/handler.go.

/** Urutan tampil 0-9999 (sama dengan batas server); kecil tampil lebih dulu. */
const sortOrderField = z
  .number({ message: "Urutan harus berupa angka" })
  .int("Urutan harus bilangan bulat")
  .min(0, "Urutan minimal 0")
  .max(9999, "Urutan maksimal 9999")
  .default(0);

export const categoryCreateSchema = z.object({
  code: z.string().min(1, "Kode wajib diisi"),
  name: z.string().min(1, "Nama wajib diisi"),
  parentId: z.number().int().positive().nullable().default(null),
  sortOrder: sortOrderField,
});

export type CategoryCreateValues = z.infer<typeof categoryCreateSchema>;

export const categoryUpdateSchema = z.object({
  name: z.string().min(1, "Nama wajib diisi"),
  parentId: z.number().int().positive().nullable().default(null),
  sortOrder: sortOrderField,
});

export type CategoryUpdateValues = z.infer<typeof categoryUpdateSchema>;
