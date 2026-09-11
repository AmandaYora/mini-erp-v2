import { z } from "zod";

// Create: {code, name, parentId} — Update: {name, parentId} (kode dikunci).
// Diverifikasi dari CreateCategory/UpdateCategory di
// apps/api/internal/modules/product/presentation/handler.go.

export const categoryCreateSchema = z.object({
  code: z.string().min(1, "Kode wajib diisi"),
  name: z.string().min(1, "Nama wajib diisi"),
  parentId: z.number().int().positive().nullable().default(null),
});

export type CategoryCreateValues = z.infer<typeof categoryCreateSchema>;

export const categoryUpdateSchema = z.object({
  name: z.string().min(1, "Nama wajib diisi"),
  parentId: z.number().int().positive().nullable().default(null),
});

export type CategoryUpdateValues = z.infer<typeof categoryUpdateSchema>;
