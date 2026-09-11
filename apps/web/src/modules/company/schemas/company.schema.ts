import { z } from "zod";

const optionalText = (max: number) =>
  z.string().max(max).optional().default("");

export const companyProfileSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Nama perusahaan wajib diisi")
    .max(255, "Nama maksimal 255 karakter"),
  legalName: optionalText(255),
  address: optionalText(1000),
  city: optionalText(255),
  phone: optionalText(64),
  email: z
    .string()
    .trim()
    .max(255, "Email maksimal 255 karakter")
    .refine((v) => v === "" || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v), {
      message: "Email tidak valid",
    })
    .optional()
    .default(""),
  taxId: optionalText(64),
});

export type CompanyProfileValues = z.infer<typeof companyProfileSchema>;
