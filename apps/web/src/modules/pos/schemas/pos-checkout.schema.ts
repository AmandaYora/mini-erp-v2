import { z } from "zod";
import { PAY_LATER_NEEDS_CUSTOMER_MESSAGE } from "@/modules/pos/lib/checkout";

// Form checkout POS. Kecukupan tunai (tendered ≥ total estimasi) dicek di
// halaman karena total estimasi hanya diketahui saat runtime — skema ini
// menjaga bentuk + angka, halaman menambahkan aturan bisnisnya.
// mode "pay_later" = Bayar Nanti: termin net + jatuh tempo wajib, dan hanya untuk
// pelanggan terdaftar (backend sales/application/service.go: validateHeader menolak
// net tanpa dueDate dan net tanpa customer).
export const posCheckoutSchema = z
  .object({
    method: z.enum(["cash", "transfer"]),
    tendered: z.coerce.number().int().nonnegative(),
    mode: z.enum(["pay_now", "pay_later"]).default("pay_now"),
    dueDate: z.string().default(""),
    /** 0 = umum/walk-in (tanpa pelanggan). */
    partyId: z.number().int().nonnegative().default(0),
  })
  .refine((v) => v.mode === "pay_now" || v.partyId > 0, {
    path: ["partyId"],
    message: PAY_LATER_NEEDS_CUSTOMER_MESSAGE,
  })
  .refine((v) => v.mode === "pay_now" || v.dueDate.trim() !== "", {
    path: ["dueDate"],
    message: "Jatuh tempo wajib diisi untuk Bayar Nanti",
  });

export type PosCheckoutValues = z.infer<typeof posCheckoutSchema>;
