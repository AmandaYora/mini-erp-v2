# User Flows — Modul 08 Order Purchasing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua endpoint `@Post` +
`@HttpCode(200)` + `{ data }` + `BranchGuard` (cabang dari sesi). Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Buka `/orders` (semua tab), detail, riwayat, daftar penerimaan | `order.view` | Rute diblokir |
| Buat PO + `/orders/create` | `order.create` | Tombol hilang; rute diblokir |
| Edit + pindah status + Terima Barang + `goods-receipts/create` | `order.update` | Tombol hilang |
| Arsip PO | `order.archive` | — |
| Export | `order.export` | Tombol hilang |
| Kelola status (`/settings/order-status`, modul 03) | `company_config.view/manage` | — |
| COD `pay_now` di dialog terima | `order.update` + **`payment.create` di sesi** (403 khusus) | Opsi Bayar disembunyikan + teks; hanya Ubah-ke-Tempo |
| Retur ke Supplier | `purchase_return.create` (modul 13) | Tombol hilang |

## UF-01 — Buat PO tempo (alur utama)

1. `/orders` tab Pembelian → **Buat Order Baru** → form (jenis default Penjualan → ganti
   **Pembelian**: termin otomatis Tempo, pihak di-reset).
2. Pilih Supplier* → tanggal → baris (produk → harga saran beli → qty; varian bila diminta) →
   finansial: invoice supplier (no + tanggal) + termin hari (30) → jatuh tempo terisi otomatis →
   pajak bila perlu → catatan.
3. **Simpan** → `orders/create` → nomor `.../PB/...` + status awal (kind lalu `all`) + history +
   audit → toast → detail. Total = subtotal − 0 + pajak (rupiah utuh).

## UF-02 — Edit PO

1. Detail → Edit → ubah (supplier/termin/tanggal/invoice/pajak/baris) → Simpan.
2. Baris diganti total (hapus + tulis ulang, `lineNo` ulang dari 1) — hanya bila tanpa
   movement/bayar/finance; header saja selalu boleh (selama non-terminal).
3. Ganti jenis → pihak+termin divalidasi ulang (customer di PO ditolak; tanpa pihak ditolak).

## UF-03 — Terima parsial bertahap (E2E 05)

1. PO tempo 100 btg + 20 pak → proses ke tahap terima (transisi manual) → detail → **Terima
   Barang**.
2. Dialog terisi sisa penuh + lokasi default → ubah qty (50 + 10) + pilah lokasi (sumber vs
   tujuan) + SJ supplier + catatan → **Simpan Penerimaan Parsial**.
3. Hasil: stok +50/+10 di lokasi masing-masing; `goodsReceivedAt` tetap null; status tetap;
   penerimaan tercatat (riwayat 1 baris). Toast `Barang berhasil diterima` + deskripsi parsial.
4. Coba terima 60 dari sisa 50 → `400 ...melebihi sisa...`. Terima sisa → tanggal terisi + status
   selesai otomatis + history + toast selesai-otomatis.

## UF-04 — Prabayar (blokir → bayar → terima)

1. Buat PO prabayar → tombol Terima **disembunyikan** + notice pelunasan → catat bayar penuh
   (modul 14) → tombol muncul → terima sekaligus → selesai + stok masuk (E2E 05 mengunci pesan
   `prabayar|lunas` dan `fully_received`).

## UF-05 — COD dua keputusan

1. Terima sebagian (tanpa keputusan) → terima terakhir membuka `Keputusan pembayaran saat terima`.
2. **Bayar Sekarang** (perlu `payment.create`): total readonly + tanggal + metode + referensi +
   catatan + bukti opsional → **Terima, Bayar & Selesaikan** → stok + payment `payable` +
   selesai; bukti diunggah sesudahnya (gagal → penerimaan tetap). Saldo supplier → 0 (E2E 05).
3. **Ubah ke Tempo**: jatuh tempo baru → **Ubah ke Tempo & Selesaikan** → termin net + jatuh tempo
   + `creditApproved` + audit `order.approve_credit` (sumber `receive_goods_cod`); sisa jadi utang.

## UF-06 — Pindah status & arsip manual

1. Tombol transisi (label: Konfirmasi/Mulai Proses/Selesaikan/Batalkan/custom) + alasan opsional
   → `orders/update-status` → history + audit. Selesai pre-terima disembunyikan & ditolak;
   pasca-terima hanya selesai; batal pasca-gerak ditolak dengan 3 pesan.
2. Arsip: hanya bila tanpa movement/bayar/finance → hilang dari daftar (tanpa Sampah/restore).

## UF-07 — Export pembelian

1. Tab Pembelian → **Export** (jenis terisi Pembelian; bulan berjalan) → ubah tanggal/format →
   **Export** → file `report-order_pembelian_...` + toast statistik. Tanggal kosong/terbalik →
   warning tanpa request.

## UF-08 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| PO tanpa supplier / pihak salah tipe / arsip | `400` spesifik (dibuat & diubah) |
| Termin hari di luar 0–3650 / net tanpa tempo | `400` spesifik |
| Edit terminal / baris pasca-gerak | `400` spesifik; dialog tetap |
| Terima over-sisa/duplikat/qty-0/luar-order | `400` spesifik per baris |
| Terima tanpa sisa / tanpa tahap / bukan purchase | `400` spesifik |
| COD final tanpa keputusan / tanpa izin bayar | `400` pilihan / `403` izin |
| Export invalid / >366 hari / >5000 | `400` spesifik; warning FE untuk tanggal |
