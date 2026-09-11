# Algorithms Legacy — Modul 05 Product / Catalog

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Setiap logika dijelaskan
sebagai **HASIL yang harus dicapai** sistem baru, bukan cara implementasi detail.

---

## A-01 — Pencarian tokenized produk (hasil: tahan typo-spasi & urutan kata)

**Hasil yang diharapkan:** query `SPECIAL  1 KG` menemukan produk bernama `SPECIAL  1 KG` (spasi
ganda di data), `budi santoso` menemukan `Santoso Budi`, `santoso budi` juga menemukannya. Tiap kata
dicari terpisah (AND) di kode ATAU nama, dalam grup kurung agar tidak bocor ke filter perusahaan/arsip.
Test mengunci perilaku ini. **Bebas diubah:** implementasi SQL (LIKE per token saat ini), selama
hasilnya sama. Catatan: modul Users tidak memakai pola ini (KI-25) — produk adalah acuan yang benar.

## A-02 — Peringkat relevansi search-options (hasil: yang paling pas di urutan pertama)

**Hasil yang diharapkan:** untuk ketikan `CAT`: kode persis `CAT` > kode berawalan `CAT` > nama
berawalan `CAT` > lainnya; di dalam tiap tingkat, produk tersedia-dulu (bila diminta + ada cabang),
lalu nama A→Z, lalu kode. Mengambil `limit+1` untuk penanda `has_more`. Ringan: tanpa foto/URL.
**Bebas diubah:** rumus CASE saat ini, selama urutannya sama.

## A-03 — Peringkat stok-tersedia (hasil: yang bisa dijual muncul dulu)

**Hasil yang diharapkan:** bila pemanggil meminta + sesi punya cabang: produk berstok-tersedia di
cabang itu naik, non-stok/jasa di tengah, produk berstok-habis turun paling bawah — **tanpa
mengalahkan relevansi teks** dan tanpa menyembunyikan yang habis (habis tetap terlihat, hanya di
bawah). Stok = jumlah `available` di lokasi aktif non-arsip cabang sesi. Berlaku untuk tabel daftar,
picker async, dan grid POS (dikunci E2E 17). **Bebas diubah:** join agregat saat ini.

## A-04 — Pasangan konversi operasional ⇄ teknis (hasil: operator tak berhitung pecahan)

**Hasil yang diharapkan:** operator mengisi bahasa sehari-hari (`1 roll = 10 meter → 10`) dan sistem
menyimpan pecahan teknisnya (`0.1`); mengedit pecahan memperbarui bahasa operasionalnya; nilai tak
valid mengosongkan pasangannya (bukan menebak); preview selalu menunjukkan dampak nyata
(`Jual 5 meter mengurangi 0.5 roll`). Aturan tampilan: faktor 1 + satuan sama = label `1 X = 1 X`.
**Wajib dipertahankan** (aturan knowledge: jangan minta operator bernalar dalam faktor internal).

## A-05 — Slug kode varian (hasil: kode unik terbaca tanpa diketik)

**Hasil yang diharapkan:** varian tanpa kode mendapat `{KODEPRODUK}-{SLUG(NAMA)}` huruf besar yang
stabil untuk nama yang sama (slug: non-alnum → `-`, pangkas tepi, fallback index). Tabrakan
diselesaikan dengan penolakan berpesan (bukan suffix otomatis). **Bebas diubah:** aturan slug,
selama stabil + uppercase + pesan tabrakan setara.

## A-06 — Import chunked resumable (hasil: file besar masuk sebagian, bukan gagal total)

**Hasil yang diharapkan:** kategori atomik-penuh (induk rekursif harus utuh); produk masuk per 200
baris, chunk yang macet (lock/timeout) diulang per baris sehingga baris baik tetap masuk dan tiap
baris gagal tercatat (sheet+baris+alasan, dengan pesan ramah untuk duplikat/lock/deadlock); varian
atomik-penuh; upload ulang file yang diperbaiki tidak menggandakan (upsert per kode ternormalkan);
satu entri audit ringkas. **Bebas diubah:** ukuran chunk, strategi transaksi, selama sifat di atas
bertahan. Inkonsistensi yang dicatat (bukan untuk ditiru): jalur import melewati guard UOM/tipe/stok
yang berlaku di form (→ KI-53).

## A-07 — Target QR operasional (hasil: yang dipindai = yang punya stok)

**Hasil yang diharapkan:** produk bervarian → QR per varian aktif-terlihat (nama `{produk} -
{varian}`), tanpa QR induk; selain itu → QR kode produk. Gambar QR selalu: kode terbaca mesin +
nama terbaca manusia + kode tercetak. ZIP massal terkelompok folder kategori, nama file aman untuk
Windows (`<>:"/\|?*` → `-`), duplikat bersuffix. **Bebas diubah:** library QR/ZIP, ukuran kanvas,
selama payload = kode mentah dan aturan target sama.

## A-08 — Pohon kategori (hasil: hierarki tampil terurut + anti-swa-pilih)

**Hasil yang diharapkan:** root dulu lalu anak, tiap level urut `sortOrder` lalu nama; jalur penuh
(`A > B > C`) dipakai untuk pencarian dan tampilan; saat edit, diri + seluruh keturunannya hilang
dari pilihan induk (mencegah siklus di UI). Mode penugasan produk menolak induk (leaf-only).
**Bebas diubah:** struktur rekursif saat ini. Celah yang dicatat: API tidak menegakkan keduanya
(→ KI-54) — sistem baru harus memutuskan: tegakkan di server atau longgarkan di UI.

## A-09 — Foto utama + fallback default (hasil: produk tak pernah tanpa gambar)

**Hasil yang diharapkan:** tepat satu foto utama per produk (transaksi tunggal saat ganti); foto
pertama otomatis utama; arsip foto utama mempromosikan pengganti paling awal; produk tanpa foto
menampilkan gambar default (bukan kotak rusak); URL selalu bertanda-tangan dan kedaluwarsa.
**Bebas diubah:** driver storage, profil optimasi, masa URL, selama sifat di atas bertahan.

## Tabel wajib-dipertahankan vs bebas-diubah

| Wajib dipertahankan (hasil) | Bebas diubah (cara) |
|---|---|
| Tokenized search + peringkat relevansi + peringkat stok (A-01…A-03) | SQL LIKE/CASE/join agregat |
| Bahasa operasional konversi, bukan faktor mentah (A-04) | State draft FE, rumus sinkron |
| Slug varian stabil + penolakan tabrakan berpesan (A-05) | Aturan slug |
| Import idempoten + chunked + per-baris tertelusur (A-06) | Chunk size, transaksi |
| QR per varian, payload kode mentah (A-07) | Library, kanvas, ZIP |
| Pohon terurut + anti-swa-pilih (A-08) | Rekursi/tree-table |
| Satu utama + promosi otomatis + default (A-09) | Storage, optimasi, URL |
| Guard stok-nol sebelum arsip/ubah-varian/ubah-UOM (lihat business-rules) | Query cek, ambang 0.0001 |
| Harga beli wajib untuk berstok di create/import (BR-20) | Lapisan validasi |
