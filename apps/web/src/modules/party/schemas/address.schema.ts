import { z } from "zod";
import { phoneField } from "@/modules/party/schemas/party.schema";

// Kunci payload = addressPayload backend: label, recipient, phone, text,
// isPrimary, sortOrder. Backend mewajibkan text (field error "address").
export const addressFormSchema = z.object({
  label: z.string().trim().max(100, "Label maksimal 100 karakter").default(""),
  recipient: z.string().trim().max(200, "Penerima maksimal 200 karakter").default(""),
  phone: phoneField,
  text: z.string().trim().min(1, "Alamat wajib diisi").max(1000, "Alamat maksimal 1000 karakter"),
  isPrimary: z.boolean().default(false),
  sortOrder: z.coerce.number().int().min(0, "Urutan tidak boleh negatif").default(0),
});

export type AddressFormValues = z.infer<typeof addressFormSchema>;
