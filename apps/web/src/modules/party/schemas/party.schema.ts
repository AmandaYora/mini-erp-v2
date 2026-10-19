import { z } from "zod";
import { PARTY_ATTR_LIMITS, validateRows } from "@/shared/lib/key-value";

// Validasi telepon menyalin backend validPhone (service.go):
// digit 7–20, karakter selain itu hanya + spasi - ( ).
function phoneOk(v: string): boolean {
  if (v === "") return true;
  let digits = 0;
  for (const ch of v) {
    if (ch >= "0" && ch <= "9") {
      digits++;
    } else if (ch === "+" || ch === " " || ch === "-" || ch === "(" || ch === ")") {
      continue;
    } else {
      return false;
    }
  }
  return digits >= 7 && digits <= 20;
}

export const phoneField = z
  .string()
  .trim()
  .max(30, "Telepon maksimal 30 karakter")
  .refine(phoneOk, "Format telepon tidak valid")
  .default("");

const emailField = z
  .string()
  .trim()
  .max(100, "Email maksimal 100 karakter")
  .refine(
    (v) => v === "" || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v),
    "Format email tidak valid",
  )
  .default("");

// Dipakai bersama customer & supplier. Backend menolak memberTypeId untuk
// supplier — form supplier tidak merender/melempar field member.
export const partyFormSchema = z.object({
  // Opsional saat buat (kosong = kode otomatis CUS-001 / SUP-001). Terkunci saat ubah.
  code: z.string().trim().max(50, "Kode maksimal 50 karakter").default(""),
  name: z.string().trim().min(1, "Nama wajib diisi").max(200, "Nama maksimal 200 karakter"),
  phone: phoneField,
  email: emailField,
  address: z.string().trim().max(500, "Alamat maksimal 500 karakter").default(""),
  notes: z.string().trim().max(500, "Catatan maksimal 500 karakter").default(""),
  memberTypeId: z.number().int().positive().nullable().optional(),
  // Informasi Tambahan: baris editor; dikirim sebagai peta (maks 20, nama 1-60, isi 1-255).
  attrs: z
    .array(z.object({ key: z.string(), value: z.string() }))
    .default([])
    .superRefine((rows, ctx) => {
      const r = validateRows(rows, PARTY_ATTR_LIMITS);
      if (r.general) ctx.addIssue({ code: z.ZodIssueCode.custom, message: r.general });
      for (const msg of Object.values(r.byRow)) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, message: msg });
      }
    }),
});

export type PartyFormValues = z.infer<typeof partyFormSchema>;
