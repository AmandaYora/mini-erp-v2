import { z } from "zod";

// Aturan sama dengan backend (dashboard/domain/snooze.go): durasi 1/3/7/14 hari,
// alert kritis paling lama 7 hari, alasan 5–255 karakter setelah dipangkas.
export const SNOOZE_DAYS = [1, 3, 7, 14] as const;

export function snoozeSchema(maxDays: number) {
  return z.object({
    days: z
      .number()
      .refine((d) => (SNOOZE_DAYS as readonly number[]).includes(d), "Pilih 1, 3, 7, atau 14 hari")
      .refine((d) => d <= maxDays, "Alert kritis paling lama ditunda 7 hari"),
    reason: z
      .string()
      .trim()
      .min(1, "Alasan wajib diisi")
      .min(5, "Alasan minimal 5 karakter")
      .max(255, "Alasan maksimal 255 karakter"),
  });
}

export type SnoozeValues = z.infer<ReturnType<typeof snoozeSchema>>;
