// Kesiapan POS menurut izin sesi: yang kurang dijelaskan per fitur (bukan
// katalog kosong diam-diam) dan checkout hanya diblokir bila izin INTI kurang.
// Izin inti = alur order tunai penuh: buat/konfirmasi order (sales.*) dan
// serah-terima barang (delivery.create). Pembayaran bisa dicatat belakangan.

export interface PosReadiness {
  /** Izin inti yang kurang; tidak kosong = checkout diblokir. */
  missingCore: { perm: string; label: string }[];
  /** Fitur opsional yang tidak tersedia dan dampaknya. */
  notices: { perm: string; text: string }[];
  canCheckout: boolean;
}

const CORE: { perm: string; label: string }[] = [
  { perm: "sales.create", label: "Membuat order jual" },
  { perm: "sales.update", label: "Mengonfirmasi order jual" },
  { perm: "delivery.create", label: "Surat jalan / serah-terima barang" },
];

const OPTIONAL: { perms: string[]; text: string }[] = [
  { perms: ["customers.view"], text: "Tanpa izin Pelanggan: pencarian pelanggan dinonaktifkan, transaksi hanya walk-in." },
  { perms: ["products.view"], text: "Tanpa izin Produk: katalog dan pencarian produk bisa kosong; gunakan pindai barcode." },
  { perms: ["stock.view"], text: "Tanpa izin Stok: ketersediaan stok tidak ditampilkan." },
  { perms: ["pricing.quote"], text: "Tanpa izin harga member: harga member tidak dihitung otomatis, memakai harga katalog." },
  { perms: ["member_types.view", "member_types.manage"], text: "Tanpa izin Tipe Member: nama tipe member tidak ditampilkan." },
];

export function posReadiness(can: (perm: string) => boolean): PosReadiness {
  const missingCore = CORE.filter((c) => !can(c.perm));
  const notices = OPTIONAL.filter((o) => !o.perms.some((p) => can(p))).map((o) => ({
    perm: o.perms[0],
    text: o.text,
  }));
  return { missingCore, notices, canCheckout: missingCore.length === 0 };
}
