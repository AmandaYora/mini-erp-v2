// Peta label aksi & entitas jejak audit (U-02). Kunci = string persis yang ditulis
// backend (apps/api/internal/modules/**: Entry.Action / Entry.Entity).
//
// SUMBER KEBENARAN daftar kunci adalah kode backend. `audit-labels.test.ts` memindai
// seluruh sumber Go dan GAGAL bila ada aksi/entitas yang ditulis backend tanpa label di
// sini (dan bila label menunjuk kunci yang sudah tidak ditulis siapa pun). Menambah aksi
// baru di backend = tambahkan labelnya di sini.
//
// Kunci tak dikenal (data lama/ETL) jatuh ke kunci mentah lewat actionLabel/entityLabelName.

export const ACTION_LABELS: Record<string, string> = {
  // Autentikasi & sesi
  "auth.login": "Masuk",
  "auth.login_failed": "Gagal masuk",
  "auth.logout": "Keluar",
  "auth.refresh_replay": "Pemakaian ulang token (sesi dicabut)",
  "auth.switch_branch": "Ganti cabang aktif",
  "auth.switch_role": "Ganti peran aktif",

  // Pengguna & peran
  "users.create": "Tambah pengguna",
  "users.update": "Ubah pengguna",
  "users.password": "Ubah kata sandi pengguna",
  "users.status": "Ubah status pengguna",
  "roles.create": "Tambah peran",
  "roles.update": "Ubah peran",
  "roles.delete": "Hapus peran",
  "roles.permissions": "Ubah izin peran",

  // Perusahaan & cabang
  "company.profile": "Ubah profil perusahaan",
  "company.settings": "Ubah pengaturan perusahaan",
  "branches.create": "Tambah cabang",
  "branches.update": "Ubah cabang",

  // Asisten WhatsApp
  "assistant.channel_connect": "Hubungkan WhatsApp",
  "assistant.channel_disconnect": "Putuskan WhatsApp",
  "assistant.channel_reset": "Reset sesi WhatsApp",
  "assistant.authorization_create": "Tambah nomor terotorisasi asisten",
  "assistant.authorization_update": "Ubah nomor terotorisasi asisten",
  "assistant.authorization_revoke": "Cabut nomor terotorisasi asisten",
  "assistant.config_update": "Ubah pengaturan asisten",

  // Produk & media
  "products.create": "Tambah produk",
  "products.update": "Ubah produk",
  "products.archive": "Arsipkan produk",
  "products.bulk_import": "Impor produk massal",
  "categories.create": "Tambah kategori produk",
  "categories.update": "Ubah kategori produk",
  "categories.archive": "Arsipkan kategori produk",
  "media.upload": "Unggah foto produk",
  "media.archive": "Arsipkan foto produk",
  "media.primary": "Jadikan foto utama",

  // Pelanggan & pemasok
  "customers.create": "Tambah pelanggan",
  "customers.update": "Ubah pelanggan",
  "customers.archive": "Arsipkan pelanggan",
  "customers.restore": "Pulihkan pelanggan",
  "suppliers.create": "Tambah pemasok",
  "suppliers.update": "Ubah pemasok",
  "suppliers.archive": "Arsipkan pemasok",
  "suppliers.restore": "Pulihkan pemasok",
  "addresses.create": "Tambah alamat",
  "addresses.update": "Ubah alamat",
  "addresses.archive": "Arsipkan alamat",
  "member_types.create": "Tambah jenis member",
  "member_types.update": "Ubah jenis member",
  "member_types.archive": "Arsipkan jenis member",
  "member_types.restore": "Pulihkan jenis member",

  // Penjualan
  "sales_orders.create": "Buat order jual",
  "sales_orders.update": "Ubah order jual",
  "sales_orders.confirm": "Konfirmasi order jual",
  "sales_orders.cancel": "Batalkan order jual",
  "sales_orders.close_short": "Tutup sisa order jual",
  "sales_orders.approve_credit": "Setujui kredit order jual",
  "sales_orders.create_from_delivery": "Buat order dari surat jalan",
  "deliveries.create": "Buat surat jalan",
  "deliveries.create_direct": "Buat surat jalan langsung (tanpa order)",
  "deliveries.link_order": "Tautkan surat jalan langsung ke order",
  "deliveries.confirm": "Konfirmasi surat jalan",
  "deliveries.cancel": "Batalkan surat jalan",
  "sales_returns.create": "Buat retur penjualan",
  "sales_returns.confirm": "Konfirmasi retur penjualan",
  "sales_returns.cancel": "Batalkan retur penjualan",
  "sales_returns.settle": "Selesaikan retur penjualan",
  "sales_returns.settle_memo_failed": "Penyelesaian retur penjualan gagal dicatat",
  "sales_returns.confirm_replacement": "Konfirmasi barang pengganti retur",
  "sales_returns.dispatch_replacement": "Kirim barang pengganti retur",
  "sales_returns.cancel_replacement": "Batalkan pengiriman barang pengganti retur",

  // Pembelian
  "purchase_orders.create": "Buat order beli",
  "purchase_orders.update": "Ubah order beli",
  "purchase_orders.confirm": "Konfirmasi order beli",
  "purchase_orders.cancel": "Batalkan order beli",
  "purchase_orders.short_close": "Tutup sisa order beli",
  "purchase_orders.price_correct": "Koreksi harga beli",
  "goods_receipts.create": "Buat penerimaan barang",
  "goods_receipts.complete": "Selesaikan penerimaan barang",
  "goods_receipts.discard": "Buang draf penerimaan barang",
  "purchase_returns.create": "Buat retur pembelian",
  "purchase_returns.confirm": "Konfirmasi retur pembelian",
  "purchase_returns.cancel": "Batalkan retur pembelian",
  "purchase_returns.settle": "Selesaikan retur pembelian",

  // Pembayaran
  "payments.create": "Catat pembayaran",
  "payments.cancel": "Batalkan pembayaran",
  "payments.correct": "Koreksi pembayaran",
  "payments.offset_return": "Kompensasi retur dengan pembayaran",
  "payments.settle_credit": "Selesaikan saldo kredit",
  "payments.use_credit": "Pakai saldo kredit",

  // Stok
  "stock_adjustments.create": "Koreksi stok",
  "stock_adjustments.opening_correction": "Koreksi saldo awal stok",
  "stock.approver_failed": "Pemberitahuan persetujuan koreksi stok gagal",
  "stock_locations.create": "Tambah lokasi stok",
  "stock_locations.update": "Ubah lokasi stok",
  "stock_locations.archive": "Arsipkan lokasi stok",
  "stock_transfers.create": "Buat transfer stok",
  "stock_transfers.dispatch": "Kirim transfer stok",
  "stock_transfers.receive": "Terima transfer stok",
  "stock_transfers.cancel": "Batalkan transfer stok",
  "stock_transfers.move": "Pindah lokasi stok",
  "stock_damaged.move": "Pindahkan ke stok rusak",
  "stock_damaged.restore": "Pulihkan stok rusak",
  "stock_damaged.write_off": "Hapus buku stok rusak",
  "stock_openings.commit": "Posting saldo awal stok",
  "stock_reservations.hold": "Tahan stok (reservasi)",
  "stock_reservations.release": "Lepas reservasi stok",

  // Keuangan
  "accounts.create": "Tambah akun keuangan",
  "accounts.update": "Ubah akun keuangan",
  "accounts.archive": "Arsipkan akun keuangan",
  "account_mappings.set": "Ubah pemetaan akun",
  "fiscal_periods.close": "Tutup periode",
  "fiscal_periods.reopen": "Buka kembali periode",
  "fiscal_periods.safe_close": "Kunci periode",
  "journals.post": "Posting jurnal otomatis",
  "journals.manual": "Buat jurnal manual",
  "journals.reverse": "Balikkan jurnal",
  "journals.opening": "Buat jurnal saldo awal",
  "journals.opening_complete": "Selesaikan saldo awal persediaan",
  "journals.tax_adjust": "Buat jurnal penyesuaian pajak",
  "expenses.create": "Catat biaya usaha",
  "expenses.cancel": "Batalkan biaya usaha",
  "finance.posting.ignore": "Abaikan transaksi dari antrian posting",
  "finance.posting.restore": "Pulihkan transaksi yang diabaikan",
  "finance.cutover": "Cutover keuangan",
  "finance.autoreverse_failed": "Pembalikan jurnal otomatis gagal",
  "finance.cost_restatement": "Hitung ulang HPP (restatement biaya persediaan)",
  "finance.inventory_revaluation": "Revaluasi harga modal persediaan",

  // Sistem
  "error.unexpected": "Galat server tak terduga",
};

export const ENTITY_LABELS: Record<string, string> = {
  user: "Pengguna",
  role: "Peran",
  branch: "Cabang",
  company_profile: "Profil perusahaan",
  company_settings: "Pengaturan perusahaan",
  assistant_channel: "Saluran WhatsApp",
  assistant_authorization: "Nomor terotorisasi asisten",
  assistant_config: "Pengaturan asisten",
  product: "Produk",
  category: "Kategori produk",
  media: "Foto produk",
  customer: "Pelanggan",
  supplier: "Pemasok",
  party_address: "Alamat",
  member_type: "Jenis member",
  sales_order: "Order jual",
  delivery: "Surat jalan",
  sales_return: "Retur penjualan",
  sales_return_settlement: "Penyelesaian retur penjualan",
  purchase_order: "Order beli",
  goods_receipt: "Penerimaan barang",
  purchase_return: "Retur pembelian",
  purchase_return_settlement: "Penyelesaian retur pembelian",
  payment: "Pembayaran",
  stock_adjustment: "Koreksi stok",
  stock_location: "Lokasi stok",
  stock_transfer: "Transfer stok",
  stock_damaged: "Stok rusak",
  stock_opening: "Saldo awal stok",
  stock_reservation: "Reservasi stok",
  account: "Akun keuangan",
  account_mapping: "Pemetaan akun",
  fiscal_period: "Periode",
  journal: "Jurnal",
  expense: "Biaya usaha",
  finance_cutover: "Cutover keuangan",
  posting_source: "Transaksi antrian posting",
  cost_ledger: "Ledger biaya persediaan",
  inventory_revaluation: "Revaluasi harga modal",
  error: "Galat sistem",
};

/** Label aksi; kunci tak dikenal dikembalikan apa adanya (data lama/ETL). */
export function actionLabel(key: string): string {
  return ACTION_LABELS[key] ?? key;
}

/** Label entitas; kunci tak dikenal dikembalikan apa adanya. */
export function entityName(key: string): string {
  return ENTITY_LABELS[key] ?? key;
}
