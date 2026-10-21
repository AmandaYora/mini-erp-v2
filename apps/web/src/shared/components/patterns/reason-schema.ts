import { z } from "zod";

// Alasan koreksi (batal/buang dokumen): wajib, maks 255 — sama dengan
// validator.Reason di backend (kolom cancel_reason varchar(255)).
export const reasonSchema = z.object({
  reason: z.string().trim().min(1, "Alasan wajib diisi").max(255, "Alasan maksimal 255 karakter"),
});

export type ReasonValues = z.infer<typeof reasonSchema>;
