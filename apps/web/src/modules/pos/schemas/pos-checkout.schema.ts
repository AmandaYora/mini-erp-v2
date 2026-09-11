import { z } from "zod";

// Form checkout POS. Kecukupan tunai (tendered ≥ total estimasi) dicek di
// halaman karena total estimasi hanya diketahui saat runtime — skema ini
// menjaga bentuk + angka, halaman menambahkan aturan bisnisnya.
// mode "pay_later" = Bayar Nanti: termin net + jatuh tempo wajib (backend
// sales/application/service.go: validateHeader menolak net tanpa dueDate).
export const posCheckoutSchema = z
  .object({
    method: z.enum(["cash", "transfer"]),
    tendered: z.coerce.number().int().nonnegative(),
    mode: z.enum(["pay_now", "pay_later"]).default("pay_now"),
    dueDate: z.string().default(""),
  })
  .refine((v) => v.mode === "pay_now" || v.dueDate.trim() !== "", {
    path: ["dueDate"],
    message: "Jatuh tempo wajib diisi untuk Bayar Nanti",
  });

export type PosCheckoutValues = z.infer<typeof posCheckoutSchema>;
