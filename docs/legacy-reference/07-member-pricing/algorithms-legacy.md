# Algorithms Legacy — Modul 07 Member Type & Member Pricing

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Tiap logika sebagai
**HASIL yang harus dicapai**.

---

## A-01 — Harga member per baris (hasil: kasir tak menghitung, member selalu benar)

**Hasil yang diharapkan:** untuk tiap produk + pelanggan member: ambil basis dalam UOM jual (basis
beli dikonversi `(beli ÷ faktor_beli) × faktor_jual`), tambah/kurang besaran (persen dari basis),
tolak hasil negatif, bulatkan (bila dikonfigurasi), bulatkan 2 desimal. Basis hilang/negatif dan
konversi rusak menghasilkan **error per baris berbahasa Indonesia** (bukan angka diam). Tanpa member:
harga manual ?? jual ?? 0 dengan klasifikasi `standard` vs `manual` yang tepat. **Bebas diubah:**
urutan internal, selama contoh terkunci (30.000→31.000; 20.000+1,5%→20.300; 12.500−500→12.000)
tetap pas.

## A-02 — Snapshot final (hasil: dokumen lama abadi)

**Hasil yang diharapkan:** setiap baris order menyimpan 7 jejak (sumber, id+nama rule, basis, arah,
tipe, nilai, basis harga) sehingga nota/cetak/diskon/margin dapat direproduksi tanpa membaca rule
live. Perubahan rule hanya memengaruhi transaksi baru. **Wajib dipertahankan** (filosofi sama
dengan snapshot item & ship-to).

## A-03 — Bypass minimum yang disengaja (hasil: harga member adalah floor-nya sendiri)

**Hasil yang diharapkan:** baris `member_rule` boleh di bawah `min_selling_price` (2 jalur
create/update, sales saja); baris lain tetap dijaga; diskon manual di atas harga member di-klem ke
harga member (bukan ke minimum). **Bebas diubah:** titik penjagaan, selama pengecualian tetap
tepat di lingkup `member_rule`-sales.

## A-04 — Requote reaktif (hasil: ganti pelanggan = harga ikut, manual dihormati)

**Hasil yang diharapkan:** ganti pelanggan me-quote ulang; baris yang diketik manual tidak ditimpa;
diskon lama yang kelewat besar di-klem ulang (tidak ditolak saat simpan); error per produk jadi
banner agregat; request basi diabaikan. **Bebas diubah:** state/sequence-guard FE.

## A-05 — Guard penetapan (hasil: rule yang dipakai tak bisa dimatikan diam-diam)

**Hasil yang diharapkan:** nonaktifkan/arsip ditolak dengan **hitungan pelanggan aktif** + perintah
lepas/ganti; pelanggan yang telanjur tanpa rule jatuh ke standard tanpa error. **Bebas diubah:**
cara hitung, selama pesan berhitung bertahan (ini yang dibaca user).

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Rumus + contoh terkunci + error per baris (A-01) | Fungsi hitung, tipe desimal |
| 7 snapshot + future-only (A-02) | Kolom, trigger tulis |
| Bypass minimum khusus member_rule-sales (A-03) | Titik guard |
| Requote hormat-manual + klem + banner (A-04) | State FE |
| Tolak-pakai dengan hitungan (A-05) | Query hitung |
