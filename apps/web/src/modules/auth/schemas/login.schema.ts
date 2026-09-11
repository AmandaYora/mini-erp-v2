import { z } from "zod";

export const loginSchema = z.object({
  username: z.string().min(1, "Username wajib diisi"),
  password: z.string().min(1, "Password wajib diisi"),
});

export type LoginFormValues = z.infer<typeof loginSchema>;

export const selectBranchSchema = z.object({
  branchId: z.number().int().positive("Pilih cabang"),
});

export type SelectBranchValues = z.infer<typeof selectBranchSchema>;
