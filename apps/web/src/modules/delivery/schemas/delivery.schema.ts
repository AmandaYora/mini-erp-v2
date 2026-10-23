import { z } from "zod";

// Batasan disalin dari delivery/application/service.go + handler.go:
// - SO harus confirmed (server: "Order harus dikonfirmasi dulu"), qty > 0,
//   satuan harus sama dengan SO (maka UOM read-only dari SO), over-SO ditolak
//   server ("Melebihi sisa SO") — klien menjepit ke qty SO sebagai
//   perlindungan awal. Lokasi opsional (server mengalokasikan). Draf tidak
//   menggerakkan stok; stok keluar saat konfirmasi.

export const deliverLineSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  // 0/kosong = server mengalokasikan lintas lokasi (barang stok); lokasi 0
  // juga dipakai server untuk barang non-stok.
  locationId: z.number().int().min(0).default(0),
  uom: z.string().min(1, "Satuan wajib diisi"),
  qty: z.coerce.number().positive("Qty harus lebih dari 0"),
});

export type DeliverLineFormValues = z.infer<typeof deliverLineSchema>;

export const deliverSchema = z.object({
  salesOrderId: z.number().int().positive("Sales order wajib dipilih"),
  // "" = kosongkan → server memakai tanggal hari ini (WIB).
  deliveryDate: z.string().default(""),
  notes: z.string().default(""),
  // Hanya baris yang dicentang "Kirim" yang divalidasi dan dikirim; sisanya tetap
  // tersedia untuk surat jalan berikutnya (server menerima SJ sebagian baris).
  items: z.array(deliverLineSchema).min(1, "Pilih minimal 1 barang yang dikirim"),
});

export type DeliverFormValues = z.infer<typeof deliverSchema>;
