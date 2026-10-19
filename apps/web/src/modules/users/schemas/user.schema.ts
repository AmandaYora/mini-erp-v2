import { z } from "zod";

const emailOrEmpty = z
  .string()
  .trim()
  .max(255, "Email maksimal 255 karakter")
  .refine((v) => v === "" || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v), {
    message: "Email tidak valid",
  })
  .optional()
  .default("");

export const USER_PHONE_MAX = 30;
export const USER_TITLE_MAX = 100;

const phoneField = z
  .string()
  .trim()
  .max(USER_PHONE_MAX, "Telepon maksimal 30 karakter")
  .optional()
  .default("");

const titleField = z
  .string()
  .trim()
  .max(USER_TITLE_MAX, "Jabatan maksimal 100 karakter")
  .optional()
  .default("");

export const createUserSchema = z.object({
  username: z
    .string()
    .trim()
    .min(1, "Username wajib diisi")
    .max(64, "Username maksimal 64 karakter"),
  email: emailOrEmpty,
  phone: phoneField,
  displayTitle: titleField,
  fullName: z
    .string()
    .trim()
    .min(1, "Nama lengkap wajib diisi")
    .max(255, "Nama lengkap maksimal 255 karakter"),
  password: z.string().min(8, "Password minimal 8 karakter"),
});

export type CreateUserValues = z.infer<typeof createUserSchema>;

export const editUserSchema = z.object({
  username: z
    .string()
    .trim()
    .min(1, "Username wajib diisi")
    .max(64, "Username maksimal 64 karakter"),
  email: emailOrEmpty,
  phone: phoneField,
  displayTitle: titleField,
  fullName: z
    .string()
    .trim()
    .min(1, "Nama lengkap wajib diisi")
    .max(255, "Nama lengkap maksimal 255 karakter"),
});

export type EditUserValues = z.infer<typeof editUserSchema>;

export const passwordSchema = z
  .object({
    password: z.string().min(8, "Password minimal 8 karakter"),
    confirmPassword: z.string().min(1, "Konfirmasi password wajib diisi"),
  })
  .refine((v) => v.password === v.confirmPassword, {
    message: "Konfirmasi password tidak sama",
    path: ["confirmPassword"],
  });

export type PasswordValues = z.infer<typeof passwordSchema>;
