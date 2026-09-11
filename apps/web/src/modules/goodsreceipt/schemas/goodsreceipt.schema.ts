import { z } from "zod";

// Batasan disalin dari goodsreceipt/application/service.go + handler.go:
// - PO harus confirmed (server menolak selain itu: "Order harus dikonfirmasi
//   dulu"), qty > 0, satuan harus sama dengan PO (maka UOM read-only dari PO),
//   over-receipt ditolak server ("Melebihi sisa PO") — klien menjepit ke qty
//   PO sebagai perlindungan awal. Lokasi wajib (CheckLocation server).

export const receiveLineSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  locationId: z.number().int().positive("Lokasi wajib dipilih"),
  uom: z.string().min(1, "Satuan wajib diisi"),
  qty: z.coerce.number().positive("Qty harus lebih dari 0"),
});

export type ReceiveLineFormValues = z.infer<typeof receiveLineSchema>;

export const receiveSchema = z.object({
  purchaseOrderId: z.number().int().positive("Purchase order wajib dipilih"),
  // "" = kosongkan → server memakai tanggal hari ini (WIB).
  receivedAt: z.string().default(""),
  notes: z.string().default(""),
  items: z.array(receiveLineSchema).min(1, "Minimal 1 baris item"),
});

export type ReceiveFormValues = z.infer<typeof receiveSchema>;
