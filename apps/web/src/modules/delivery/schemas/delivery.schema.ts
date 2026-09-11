import { z } from "zod";

// Batasan disalin dari delivery/application/service.go + handler.go:
// - SO harus confirmed (server: "Order harus dikonfirmasi dulu"), qty > 0,
//   satuan harus sama dengan SO (maka UOM read-only dari SO), over-SO ditolak
//   server ("Melebihi sisa SO") — klien menjepit ke qty SO sebagai
//   perlindungan awal. Lokasi wajib (CheckLocation server). Draf tidak
//   menggerakkan stok; stok keluar saat konfirmasi.

export const deliverLineSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  locationId: z.number().int().positive("Lokasi wajib dipilih"),
  uom: z.string().min(1, "Satuan wajib diisi"),
  qty: z.coerce.number().positive("Qty harus lebih dari 0"),
});

export type DeliverLineFormValues = z.infer<typeof deliverLineSchema>;

export const deliverSchema = z.object({
  salesOrderId: z.number().int().positive("Sales order wajib dipilih"),
  // "" = kosongkan → server memakai tanggal hari ini (WIB).
  deliveryDate: z.string().default(""),
  notes: z.string().default(""),
  items: z.array(deliverLineSchema).min(1, "Minimal 1 baris item"),
});

export type DeliverFormValues = z.infer<typeof deliverSchema>;
