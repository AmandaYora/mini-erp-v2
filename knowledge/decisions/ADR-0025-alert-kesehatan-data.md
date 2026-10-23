# ADR-0025: Alert kesehatan data dihitung langsung; hanya penundaan yang disimpan

## Status
Accepted (2026-10-23). Disetujui pemilik bersama keputusan: izin lihat bawaan untuk admin, alert sandi
seed khusus superadmin, aksi harga modal diarahkan ke revaluasi terfilter dengan peringatan, tren tanpa
tabel snapshot. Alert antrean posting (A3) **ditunda ke tahap 2**.

## Context
Sesudah cutover (2026-10-23) audit data menemukan masalah yang tidak akan terlihat di layar harian:
64 produk berharga modal di atas 1,2× harga jual (nilai persediaan Rp14,2 M), 2.154 produk yang stoknya
masih persis stok awal, periode Juli–September belum ditutup, 38 produk dengan harga minimum di atas
harga jual, SJ terkirim tanpa status tanda tangan, order yang sudah dibayar tetapi barangnya belum
keluar, dan akun superadmin yang masih bersandi bawaan. Pemilik ingin masalah semacam ini muncul
sendiri di dasbor, berikut besarnya, dampaknya, dan halaman untuk membereskannya.

Endpoint finance yang sudah ada (`close/readiness`, `finance/integrity`, antrean posting) berat
(pindai seumur buku) dan sebagian **menulis** (`Sync`, `drainCosting`), jadi tidak boleh dipanggil
berkala dari dasbor.

## Decision
1. **Alert tidak disimpan.** Setiap alert dihitung dari data saat dibaca; ia hilang sendiri begitu
   ceknya mengembalikan 0. Yang disimpan hanya **penundaan**: tabel `dashboard_alert_snoozes`
   (migrasi `000045`, satu baris per (cabang, alert), tanpa FK lintas modul). Modul `dashboard` pindah
   dari `noSql` ke `migrated` sqlc.
2. **Modul pemilik data menghitung, dashboard menyusun.** Tiap modul (`user`, `finance`, `stock`,
   `product`, `party`, `delivery`, `sales`) mendapat antarmuka kontrak `HealthClient` read-only yang
   mengembalikan agregat kecil (`shared/health.Result`: jumlah, rupiah, contoh ≤ 5). Tidak ada join SQL
   lintas modul. Tingkat, judul, dampak, rute aksi, dan urutan diputuskan satu katalog di
   `dashboard/application/health_catalog.go`. Id alert permanen (kunci penundaan): definisi baru = id baru.
   Cek konsistensi dokumen (A13) masuk lewat port `IntegritySource` dari `bootstrap`.
3. **Cache in-process dua lapis per cabang.** Lapis cepat TTL 60 dtk (ditunggu maks 5 dtk saat dingin),
   lapis berat TTL 10 mnt (jurnal tak seimbang, konsistensi dokumen) yang **tidak pernah ditunggu**:
   dihitung di latar, sebelumnya cek berstatus `pending`. Satu komputasi per (cabang, lapis); entri basi
   (≤ 3× TTL) disajikan sambil disegarkan. Cek dijalankan berurutan; galat satu cek tidak menggagalkan
   yang lain (status `error`, pesan aman). "Periksa ulang" membuang cache cabang, dibatasi 1×/30 dtk.
   Penundaan tidak di-cache.
4. **Aturan tunda.** Berlaku untuk seluruh pengguna cabang; alasan 5–255 karakter; 1/3/7/14 hari; alert
   kritis maks 7 hari; area keamanan tidak bisa ditunda. Tunda **gugur otomatis** bila jumlah naik atau
   rupiah naik > 10 % (`snooze.reactivated`). Tunda dan batal tunda diaudit di dalam transaksinya
   (`data_health_snoozes.create/delete`, ADR-0011); gugur otomatis tidak diaudit (tanpa aktor).
5. **Izin.** `dashboard.alerts.view_all` (owner, superadmin otomatis; admin lewat `DefaultGrants` sekali,
   dicatat `authz_seed_applied`, tidak ditambahkan lagi bila pemilik mencabutnya) dan
   `dashboard.alerts.security` (hanya owner/superadmin). Area keamanan disaring di server. Kasir/staf
   tetap melihat kartu "Perlu tindakan" versi lama.
6. **Sandi seed (A1)** hanya untuk pemegang peran superadmin. Daftar sandi seed legacy pindah ke
   `user/contracts.MatchesLegacySeedPassword` (tak diekspor; ETL memakai fungsi yang sama). bcrypt
   dijalankan satu goroutine latar dan di-cache per hash; hash yang berubah membatalkan vonisnya.
   Sandi dan hash tidak pernah keluar dari modul `user`.

## Consequences
- Tidak ada "sejak kapan terdeteksi" atau tren: yang ada `oldestAt` dari data (SJ, order, periode) dan
  `countAtSnooze`. Jejak "x dari y tersisa sejak Anda lihat" dibuat frontend per pengguna (localStorage).
- Alert baru hilang ≤ 60 dtk (lapis cepat) atau ≤ 10 mnt (lapis berat) setelah datanya beres, kecuali
  pengguna menekan "Periksa ulang".
- Beban DB: ± 10 kueri berkelompok per menit per cabang yang dibuka; lapis berat menjalankan kolektor
  integritas yang masih N+1 tiap 10 menit (perbaikan batch = tugas lanjutan).
- Revaluasi harga modal di web menjurnal lawan 5100; alert A2 hanya mengarahkan ke daftar dan revaluasi
  terfilter dengan peringatan, keputusan akuntansi tetap di pemilik/akuntan.
- A3 (antrean posting) butuh varian read-only dari antrean posting dan ditunda; menambahkannya kelak =
  id baru di katalog tanpa perubahan skema.
- Rute, bentuk respons, dan aturan tunda: [API_CONTRACT §6](../../docs/API_CONTRACT.md); tabel:
  [DB_SCHEMA §3/§8](../../docs/DB_SCHEMA.md).
