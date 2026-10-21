# ADR-0024: Koreksi saldo awal dibukukan ke ekuitas, bukan ke laba

## Status
Accepted (2026-10-22). Diputuskan pemilik ("betul harusnya laba itu harus menggambarkan usaha
sebenarnya"). **Mohon tinjauan akuntan**, sama seperti ADR-0018 D11. Merevisi ADR-0018 D11: akun lawan
jurnal koreksi migrasi berubah, tanggal dan isinya tetap.

## Context
Muat percobaan data produksi (2026-10-22) memperlihatkan laba yang tidak menggambarkan usaha:

- Stok awal 30 Juni 2026 diimpor massal dari Excel: 4.013 produk, hampir semuanya qty tepat 100,
  bernilai Rp304,5 miliar lawan **3000 Modal Awal / Saldo Awal** (ADR-0018 D9–D10). Pemilik lalu
  membetulkannya lewat stock opname, tambah stok, dan koreksi "salah stock" di sistem lama.
- Sistem lama membukukan setiap penyesuaian itu ke **5100 Selisih Stok** (beban). Juli–Oktober:
  3.278 jurnal, netto beban Rp24,17 miliar (Juli saja Rp22,47 miliar). Penjualan dikurangi HPP dan
  beban operasional Juli–Oktober justru untung Rp0,43–0,57 miliar per bulan.
- Jurnal koreksi migrasi (ADR-0018 D11) bertanggal hari freeze dan memakai akun laba rugi, sehingga
  laba **hari migrasi** −Rp244,8 juta, padahal tidak ada transaksi usaha hari itu.

Yang dibetulkan semua itu adalah saldo awal yang salah dan kesalahan pembukuan sistem lama sebelum
v2 dipakai, bukan untung-rugi usaha.

## Decision
1. **Akun ekuitas baru `3100 Koreksi Saldo Awal`** (tipe modal) dengan kunci pemetaan
   `opening_correction`. Dibuat oleh `seed init` (`DefaultChart`), oleh migrasi
   `000044_finance_opening_correction` untuk basis data yang sudah berjalan, dan oleh ETL.
   Akun 3000 tetap menyimpan saldo awal apa adanya, 3100 menyimpan seluruh koreksinya. Keduanya
   terlihat terpisah di neraca.
2. **Penyesuaian stok "koreksi saldo awal" di v2.** Form Koreksi Stok punya pilihan *Tujuan koreksi*:
   - **Selisih stok** (bawaan): opname, hilang, rusak. Dijurnal ke 5100 seperti biasa.
   - **Koreksi saldo awal**: stok awal yang salah input/impor. Mutasi `ref_type = opening_correction`
     dijurnal **Persediaan lawan 3100**, tidak memengaruhi laba.

   Penjaganya:
   - izin baru `stock.adjust_opening` (bawaan hanya owner/superadmin);
   - produk harus punya stok awal (`ref_type = 'opening'`) di cabang itu;
   - alasan wajib, ambang/penyetuju sama seperti koreksi biasa;
   - audit `stock_adjustments.opening_correction`.

   Nilai dihitung ledger biaya seperti koreksi biasa (ADR-0020), jadi Persediaan buku = ledger tetap
   terjaga.
3. **ETL, jurnal penyesuaian stok legacy.** Baris akun *stock_difference* pada jurnal legacy ber-event
   `stock_adjustment` (tahun berjalan) dipindah ke 3100. Jurnal lain tidak disentuh. Alasannya, selama
   periode legacy stok awal sedang dihitung dan dibetulkan, sehingga data legacy tidak bisa
   membedakan opname "stok awal salah" dari kehilangan sungguhan. Pemindahan seluruhnya lebih dekat
   ke kenyataan daripada membebankan Rp24 miliar ke laba.
   - Setiap baris tercatat di `report/adjustments.csv` (aturan `opening_stock_correction`), dan
     ringkasannya di temuan `OPENING_STOCK_RECLASS`.
   - V-11 menghitungnya kembali di akun asal, sehingga tetap membuktikan saldo legacy utuh.
4. **ETL, jurnal koreksi migrasi (ADR-0018 D11).** Kaki laba rugi (HPP, Selisih Stok, Penjualan)
   dibukukan ke 3100, kecuali reklasifikasi HPP order batal (`cancelled_order_cogs`). Kedua kaki
   reklasifikasi itu akun laba rugi, sehingga nettonya nol. Persediaan, piutang, hutang, kas, dan
   PPN tetap seperti sebelumnya.

## Consequences
- Laba bulanan dan laba hari migrasi hanya memuat usaha. Gladi atas snapshot produksi 22 Okt:
  - laba Juli +Rp0,57 M, Agustus +Rp0,43 M, September +Rp0,57 M;
  - koreksi migrasi Rp0 terhadap laba;
  - 3100 bersaldo debit Rp24,41 M;
  - verify 92 pemeriksaan lulus.
- Kehilangan stok sungguhan di masa legacy (bila ada) ikut masuk 3100. Daftar per baris tersedia
  untuk ditinjau akuntan, dan baris yang memang beban dapat dipindah dengan jurnal manual.
- Ke depan, opname biasa tetap ke 5100. Pilihan *koreksi saldo awal* hanya untuk membetulkan stok
  awal. Pemakaiannya dibatasi izin dan teraudit, sehingga laba tidak bisa "dibersihkan" diam-diam.
- Pendapatan 14 order yang tidak pernah dijurnal legacy (`revenue_unbooked`) menjadi piutang lawan
  3100, bukan penjualan bulan freeze. Alternatifnya, menjurnalnya di bulan SJ-nya, menyentuh bulan
  yang mungkin sudah dilaporkan pajaknya. Keputusan itu diserahkan ke akuntan.
