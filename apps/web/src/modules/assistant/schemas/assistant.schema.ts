import { z } from "zod";

export const authorizationSchema = z.object({
  phone: z
    .string()
    .trim()
    .regex(/^[0-9]{8,15}$/, "Nomor telepon harus 8-15 digit angka"),
  name: z
    .string()
    .trim()
    .min(1, "Nama wajib diisi")
    .max(255, "Nama maksimal 255 karakter"),
  accessLevel: z.enum(["owner", "authorized_party"], {
    required_error: "Pilih tingkat akses",
    invalid_type_error: "Pilih tingkat akses",
  }),
  isPrimaryOwner: z.boolean(),
});

export type AuthorizationValues = z.infer<typeof authorizationSchema>;

export const assistantConfigSchema = z.object({
  mode: z.literal("rule_based"),
  rateLimitPerMinute: z.coerce
    .number({ invalid_type_error: "Batas harus angka" })
    .int("Batas harus bilangan bulat")
    .min(1, "Batas minimal 1 pesan per menit")
    .max(120, "Batas maksimal 120 pesan per menit"),
});

export type AssistantConfigValues = z.infer<typeof assistantConfigSchema>;

export const chatMessageSchema = z.object({
  message: z
    .string()
    .trim()
    .min(1, "Pesan wajib diisi")
    .max(2000, "Pesan maksimal 2000 karakter"),
});

export type ChatMessageValues = z.infer<typeof chatMessageSchema>;
