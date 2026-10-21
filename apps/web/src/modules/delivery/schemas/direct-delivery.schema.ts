import { z } from "zod";
import { todayWIB } from "@/shared/lib/format";
import type { DirectDeliveryPayload } from "@/modules/delivery/types";

// Cermin createDirect + resolveShipTo + directLines backend
// (delivery/application/direct.go), ADR-0022:
// - pelanggan wajib (SJ Langsung bukan untuk penjualan tunai tanpa pelanggan);
// - tanggal kirim YYYY-MM-DD, tidak boleh di masa depan (kosong = hari ini);
// - alamat kirim: id alamat tersimpan ATAU isian ad-hoc (alamat wajib bila ada
//   detail lain; label ≤120, penerima ≤150, telepon ≤30, alamat ≤1000);
// - sopir/petugas ≤100, plat ≤20; baris ≥1, qty > 0, lokasi 0 = alokasi
//   server. Tanpa harga. Jasa/non-stok ditolak server dengan pesan.

const dateRe = /^\d{4}-\d{2}-\d{2}$/;

export const directDeliveryLineSchema = z.object({
  productId: z.number().int().positive("Produk wajib dipilih"),
  variantId: z.number().int().positive("Varian wajib dipilih"),
  locationId: z.number().int().min(0).default(0),
  uom: z.string().trim().min(1, "Satuan wajib diisi"),
  qty: z.coerce.number().positive("Qty harus lebih dari 0"),
});

export const directDeliverySchema = z
  .object({
    partyId: z.number().int().positive("Pelanggan wajib dipilih"),
    deliveryDate: z
      .string()
      .trim()
      .refine((v) => v === "" || dateRe.test(v), "Format tanggal YYYY-MM-DD")
      .default(""),
    shipToAddressId: z.number().int().min(0).default(0),
    shipToLabel: z.string().trim().max(120, "Label maksimal 120 karakter").default(""),
    shipToRecipient: z.string().trim().max(150, "Penerima maksimal 150 karakter").default(""),
    shipToPhone: z.string().trim().max(30, "Telepon maksimal 30 karakter").default(""),
    shipToAddress: z.string().trim().max(1000, "Alamat maksimal 1000 karakter").default(""),
    driverName: z.string().trim().max(100, "Nama supir maksimal 100 karakter").default(""),
    vehiclePlate: z.string().trim().max(20, "Plat kendaraan maksimal 20 karakter").default(""),
    warehouseStaffName: z.string().trim().max(100, "Nama petugas gudang maksimal 100 karakter").default(""),
    dropLocationNote: z.string().trim().default(""),
    notes: z.string().default(""),
    items: z.array(directDeliveryLineSchema).min(1, "Minimal 1 baris barang"),
  })
  .superRefine((v, ctx) => {
    if (v.deliveryDate !== "" && v.deliveryDate > todayWIB()) {
      ctx.addIssue({ code: "custom", path: ["deliveryDate"], message: "Tanggal kirim tidak boleh di masa depan" });
    }
    const adHoc = v.shipToAddressId === 0 && (v.shipToLabel || v.shipToRecipient || v.shipToPhone) !== "";
    if (adHoc && v.shipToAddress === "") {
      ctx.addIssue({
        code: "custom",
        path: ["shipToAddress"],
        message: "Alamat kirim wajib diisi bila memakai alamat sekali pakai",
      });
    }
    const seen = new Set<string>();
    v.items.forEach((it, i) => {
      const key = `${it.productId}/${it.variantId}/${it.uom}/${it.locationId}`;
      if (seen.has(key)) {
        ctx.addIssue({
          code: "custom",
          path: ["items", i],
          message: `Baris ${i + 1} ganda (produk/varian/satuan/lokasi sama)`,
        });
      }
      seen.add(key);
    });
  });

export type DirectDeliveryFormInput = z.input<typeof directDeliverySchema>;
export type DirectDeliveryFormValues = z.infer<typeof directDeliverySchema>;

/** Nilai form tervalidasi → body POST /deliveries/direct (kunci kosong dihilangkan). */
export function toDirectDeliveryPayload(v: DirectDeliveryFormValues, clientRef: string): DirectDeliveryPayload {
  const opt = (s: string) => (s !== "" ? s : undefined);
  const body: DirectDeliveryPayload = {
    partyId: v.partyId,
    deliveryDate: opt(v.deliveryDate),
    driverName: opt(v.driverName),
    vehiclePlate: opt(v.vehiclePlate),
    warehouseStaffName: opt(v.warehouseStaffName),
    dropLocationNote: opt(v.dropLocationNote),
    notes: v.notes.trim() !== "" ? v.notes.trim() : undefined,
    clientRef,
    // locationId 0 = dihilangkan: server mengalokasikan lintas lokasi.
    items: v.items.map((it) => ({
      productId: it.productId,
      variantId: it.variantId,
      locationId: it.locationId > 0 ? it.locationId : undefined,
      uom: it.uom,
      qty: it.qty,
    })),
  };
  if (v.shipToAddressId > 0) {
    body.shipToAddressId = v.shipToAddressId;
  } else if (v.shipToAddress !== "") {
    body.shipToLabel = opt(v.shipToLabel);
    body.shipToRecipient = opt(v.shipToRecipient);
    body.shipToPhone = opt(v.shipToPhone);
    body.shipToAddress = v.shipToAddress;
  }
  return body;
}
