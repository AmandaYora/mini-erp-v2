# User Flows — Modul 17 Finance / Accounting & Pajak

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Alur end-to-end per skenario.
Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## Daftar Skenario

| # | Skenario | Pelaku | Irama |
|---|---|---|---|
| UF-01 | Menyiapkan keuangan pertama kali | Owner | Sekali seumur sistem |
| UF-02 | Mengunci saldo awal | Owner | Sekali |
| UF-03 | Melengkapi stok awal yang terlewat | Owner | Sesekali |
| UF-04 | Tutup buku harian | Owner / admin | **Setiap hari** |
| UF-05 | Menyelesaikan transaksi tertahan "Menunggu data" | Owner | Setiap hari–mingguan |
| UF-06 | Mencatat biaya usaha | Admin | Harian |
| UF-07 | Membatalkan biaya yang salah | Admin | Sesekali |
| UF-08 | Membalik jurnal yang salah | Owner | Jarang |
| UF-09 | Mengecek untung rugi | Owner | Mingguan–bulanan |
| UF-10 | Menutup bulan | Owner | **Bulanan** |
| UF-11 | Menyiapkan berkas untuk konsultan pajak | Owner | Bulanan |
| UF-12 | Membuka kembali bulan yang sudah dikunci | Owner | Jarang |
| UF-13 | Mengisi worksheet penyesuaian fiskal | Owner / konsultan | Tahunan |

---

## UF-01 — Menyiapkan keuangan pertama kali

**Pelaku:** Owner. Sistem baru dipasang; bagan akun bawaan sudah ada dari seed, tetapi belum ada
apa pun yang bisa dilaporkan.

1. **Pengaturan Keuangan** → periksa **daftar akun** bawaan. Tambah akun bila perlu.
2. Periksa **Arah Posting Akun** — 15 mapping harus terisi. Seed sudah mengisinya dari akun
   ber-`system_key`.
3. Tambah **Kas & Rekening** yang benar-benar dipakai (kas laci, rekening bank).
4. Buat **Periode Keuangan** pertama. Rentangnya tidak boleh tumpang tindih dengan periode lain.
5. Lanjut ke **Saldo Awal** (UF-02).

**Yang menghentikan alur bila dilewati:** tanpa mapping lengkap, posting apa pun gagal dengan
`Mapping akun '{kunci}' belum dikonfigurasi`. Tanpa periode, penguncian tidak berlaku sama sekali —
semua tanggal dianggap terbuka.

---

## UF-02 — Mengunci saldo awal

**Pelaku:** Owner. Ini alur wizard empat langkah.

1. **Langkah 1 — "Kapan pembukuan dimulai?"** Isi tanggal cutover.
2. **Langkah 2 — "Saldo uang & utang-piutang di tanggal itu"** Isi kas, bank, total piutang, total
   hutang.
3. **Langkah 3 — "Stok barang & harga modal awal"** Tambahkan produk per cabang beserta jumlahnya.
   **Kolom harga modal tidak ada** — sistem mengambilnya dari harga beli master produk.
4. **Langkah 4 — "Tinjau & kunci saldo awal"** Sistem menampilkan hasil validasi:

| Keadaan | Yang tampil |
|---|---|
| Cutover kosong | **Tanggal mulai belum diisi** |
| Ada masalah | **Masih perlu dilengkapi** + daftar masalah |
| Bersih | **Saldo awal siap dikunci** |

5. Tekan kunci. Peringatan **"Sekali dikunci, tidak bisa diubah"** muncul.
6. Setelah terkunci: satu jurnal pembuka terbit (bernomor mengikuti **tanggal cutover**, bukan hari
   ini), seluruh buku besar harga modal terisi, dan halaman menampilkan **"Saldo awal sudah
   dikunci"**.

**Yang mungkin menghentikan:**

| Pesan | Artinya |
|---|---|
| `Produk "{nama}" belum punya Harga Beli di Data Produk...` | Isi harga beli di modul Produk dulu |
| `Mapping akun '{kunci}' belum dikonfigurasi` | Kembali ke Pengaturan Keuangan |
| `Periode keuangan {kode} sudah dikunci` | Tanggal cutover jatuh di periode terkunci |

**Yang dijamin:** ekuitas dihitung sebagai **penyeimbang otomatis** — pemilik tidak perlu tahu
berapa modal awalnya, sistem yang menghitung dari selisih harta dan hutang.

---

## UF-03 — Melengkapi stok awal yang terlewat

**Pelaku:** Owner. Muncul beberapa hari/minggu setelah mulai beroperasi.

**Gejalanya:** di halaman Tutup Buku ada transaksi tertahan bertuliskan "Menunggu data", dan di
halaman Saldo Awal muncul kartu **"Lengkapi stok awal yang terlewat"**.

Penyebabnya: produk terjual padahal tidak pernah dimasukkan ke saldo awal dan belum pernah dibeli
lewat sistem — jadi tidak ada basis harga modal untuk menghitung HPP-nya.

1. Buka **Saldo Awal**. Kartu itu menampilkan daftar produk beserta cabang, jumlah transaksi
   tertahan, dan **harga modal yang disarankan** (dari harga beli master).
2. Isi jumlah stok awal tiap produk.
3. Simpan.

**Yang terjadi:** produk ditambahkan **secara aditif** — jumlah produk lain tidak direset, jurnal
pembuka lama tidak disentuh. Satu jurnal suplemen terbit (debit Persediaan, kredit Modal Awal),
bertanggal **cutover**, dan transaksi yang tadinya tertahan otomatis jadi siap diposting.

**Kenapa bukan "buka kembali lalu posting ulang":** membuka dan memposting ulang akan mereset
jumlah stok dan menghapus pengurangan dari transaksi yang sudah terjadi. Jalur aditif ini adalah
pengganti yang sengaja dirancang.

**Yang mungkin menghentikan:**

| Pesan | Artinya |
|---|---|
| `Lengkapi stok awal hanya untuk saldo awal yang sudah dikunci...` | Saldo awal masih draft — isi langsung saja |
| `Produk "{nama}" sudah ada di saldo awal` | Produk itu sudah masuk; masalahnya bukan di sini |
| `Jumlah stok awal harus lebih dari 0` | — |

---

## UF-04 — Tutup buku harian

**Pelaku:** Owner atau admin. **Ini alur harian utama modul ini.**

1. Sore/malam, buka **Keuangan → Tutup Buku**.
2. Halaman menampilkan status hari ini:

| Status | Yang harus dilakukan |
|---|---|
| **kosong** | Belum ada transaksi hari ini — tidak ada yang perlu ditutup |
| **belum** | Ada yang harus diposting — lanjut langkah 3 |
| **selesai_catatan** | Posting beres, ada catatan butuh data — lanjut UF-05 |
| **lengkap** | Sudah selesai |

3. Tekan **Tarik Transaksi** bila ada transaksi baru yang belum muncul.
4. (Opsional) Tekan **Pratinjau** pada satu baris untuk melihat jurnal yang akan terbentuk —
   **tanpa menyimpan apa pun**.
5. Tekan **Tutup Hari**. Sistem memposting seluruh antrian siap **dalam urutan dependensi**:
   penerimaan → penjualan → retur → HPP → pembayaran → koreksi stok.
6. Status berubah jadi **lengkap** (atau **selesai_catatan** bila masih ada Tipe B).

**Yang tidak perlu dipikirkan pemilik:** transaksi yang sekadar menunggu urutan (Tipe A)
**tidak ditampilkan sebagai tugas** — ia selesai sendiri di langkah 5.

**Yang dijamin:** transaksi bertanggal **setelah** tanggal tutup tidak ikut diposting, tetapi
backlog lama tetap dibereskan.

---

## UF-05 — Menyelesaikan transaksi "Menunggu data"

**Pelaku:** Owner. Ini satu-satunya jenis tugas finance yang benar-benar butuh keputusannya.

Tiga akar masalah, dikelompokkan sistem:

| Alasan | Artinya | Cara menyelesaikan |
|---|---|---|
| **Butuh harga modal** | Produk terjual tanpa basis biaya | Isi harga beli di modul Produk, atau lengkapi stok awal (UF-03) |
| **Butuh alokasi pembayaran** | Pembayaran belum ditautkan ke order | Alokasikan di modul Pembayaran |
| **Butuh order asal** | Dokumen menunjuk order yang tidak ditemukan | Perlu ditelusuri manual |

Setelah datanya dilengkapi, tekan **Tarik Transaksi** — sistem **mengevaluasi ulang** dan
transaksi itu berpindah ke siap-posting. Tidak perlu tindakan khusus untuk "membangunkannya".

**Sifat yang dijamin:** sumber yang pernah **gagal** juga dievaluasi ulang setiap sinkronisasi —
sehingga **sembuh sendiri** begitu penyebabnya diperbaiki, tanpa perlu tombol "coba lagi".

---

## UF-06 — Mencatat biaya usaha

**Pelaku:** Admin. Satu-satunya jalur jurnal manual di seluruh sistem.

1. **Keuangan → Biaya Usaha** → **Catat Biaya Baru**.
   Deskripsi: *"Isi tanggal, jenis biaya, sumber pembayaran, dan nominal."*
2. Isi tanggal, pilih **jenis biaya** (akun bertipe biaya), pilih **sumber pembayaran** (kas atau
   rekening terdaftar), isi nominal dan keterangan.
3. Simpan.

**Yang terjadi seketika:** nomor biaya (`EXP/...`) terbit, jurnal (`JRN/...`) langsung terposting —
debit akun biaya, kredit kas/bank. **Tidak lewat antrian.**

**Yang mungkin menghentikan:**

| Pesan | Artinya |
|---|---|
| `Nominal biaya harus lebih dari 0` | — |
| `Akun yang dipilih bukan akun biaya` | Salah pilih akun |
| `Kas/rekening pembayaran tidak valid` | Akun itu belum terdaftar sebagai kas/rekening |
| `Akun pembayaran harus akun aset (kas atau bank)` | — |
| `Cabang biaya tidak valid` | Cabang tidak aktif |
| `Periode keuangan {kode} sudah dikunci` | Tanggal biaya jatuh di bulan terkunci |

---

## UF-07 — Membatalkan biaya yang salah

1. Buka daftar biaya, tekan batalkan.
2. Dialog **"Batalkan Biaya"** muncul:
   *"Catatan keuangannya akan otomatis dibalik. Aksi tidak menghapus data; jejak audit tersimpan."*
3. **Isi alasan** — wajib.
4. Konfirmasi.

**Yang terjadi:** biaya berstatus dibatalkan, jurnal asli jadi terbalik, dan **jurnal pembalik
baru** terbit.

**Dua hal yang perlu diketahui:**

1. Penjagaan periode memakai periode **jurnal asli** — membatalkan biaya dari bulan terkunci
   **ditolak**, meski pembaliknya akan jatuh di bulan berjalan.
2. Jurnal pembalik bertanggal **hari ini**, bukan tanggal biaya asli. Ini pilihan akuntansi yang
   disengaja — tetapi "hari ini" dihitung UTC, sehingga pembatalan dini hari WIB menghasilkan
   pembalik bertanggal kemarin (KI-146).

---

## UF-08 — Membalik jurnal yang salah

**Pelaku:** Owner dengan `finance.journal.reverse`.

1. **Keuangan → Catatan Keuangan**. Deskripsi halaman sudah memperingatkan:
   *"Termasuk catatan pembalik bila ada koreksi."*
2. Buka detail jurnal, tekan balik.
3. Jurnal pembalik terbit dengan seluruh baris asli ditukar debit↔kreditnya; asli jadi terbalik.

**Alternatif yang lebih tepat untuk transaksi otomatis:** gunakan **Batal Posting** dari halaman
Tutup Buku. Bedanya, batal posting **juga mengembalikan antriannya** ke keadaan siap-posting,
sehingga bisa diposting ulang setelah datanya diperbaiki. Membalik jurnal saja meninggalkan
antrian dalam keadaan sudah-diposting.

**Yang mungkin menghentikan:** periode jurnal asli maupun tanggal pembaliknya harus terbuka.

**Kesenjangan:** **tidak ada jejak audit** untuk pembalikan jurnal maupun batal posting —
satu-satunya jejak adalah keberadaan jurnal pembaliknya sendiri.

---

## UF-09 — Mengecek untung rugi

**Pelaku:** Owner.

Dua tingkat kedalaman yang sengaja dipisah:

| Kebutuhan | Halaman |
|---|---|
| "Bulan ini untung berapa?" — cepat | **Untung Rugi (Cepat)** — pendapatan − HPP, tanpa biaya operasional |
| Laporan laba rugi lengkap | **Untung Rugi** — seluruh akun pendapatan & biaya |
| Posisi harta | **Posisi Harta & Hutang** |
| Siapa berhutang | **Hutang & Tagihan** |
| Kas masuk/keluar | **Kas & Rekening** |

**Yang wajib dipahami pemilik:** seluruh angka berasal dari **transaksi yang sudah diposting**.
Transaksi yang masih tertahan di Tutup Buku **tidak muncul**. Inilah alasan tutup buku harian
(UF-04) penting — bukan formalitas.

---

## UF-10 — Menutup bulan

**Pelaku:** Owner.

1. **Keuangan → Kunci Bulan & Berkas Pajak**, pilih periode.
2. Tekan **Refresh Cek**. Enam pemeriksaan muncul dengan legenda:
   *"Hijau = OK; Kuning = peringatan (boleh lanjut); Merah = harus diperbaiki dulu."*
3. Bereskan yang merah:

| Pemeriksaan gagal | Cara membereskan |
|---|---|
| Ada transaksi belum masuk laporan | Kembali ke Tutup Buku |
| Ada source gagal posting | Perbaiki penyebabnya, tarik ulang |
| Cek Saldo Akun tidak seimbang | Perlu investigasi — seharusnya mustahil |
| Harta ≠ Hutang + Modal | Idem |
| Ada nilai persediaan negatif | **Koreksi manual dengan supervisi** — basis biaya rusak |

4. Setelah semua hijau, tekan kunci → dialog **"Kunci Periode?"**.
5. Setelah terkunci, seluruh penulisan pada rentang tanggal itu ditolak.

**Bila terpaksa mengunci dengan item merah:** tombol paksa tersedia tetapi butuh permission
terpisah `finance.close.force`. Dialog **"Kunci Paksa?"** berbunyi:
*"Anda akan mengunci periode meski masih ada checklist gagal. Aksi ini tercatat di audit log.
Lanjutkan?"*

**Catatan penting tentang pemeriksaan nilai persediaan negatif:** ia memeriksa **seluruh
perusahaan**, bukan hanya bulan itu — karena kerusakan basis biaya bisa berasal dari periode kapan
pun. Ini penjagaan hasil insiden Agustus 2026.

---

## UF-11 — Menyiapkan berkas untuk konsultan pajak

**Pelaku:** Owner. Alur bulanan yang perlu keputusan sadar sebelum dibawa ke sistem baru.

1. Di halaman **Kunci Bulan & Berkas Pajak**, gulir ke kartu **"Unduh Berkas Pajak"**:
   *"Paket Excel berisi laba rugi, neraca, buku besar, dan rincian PPN — untuk diberikan ke
   konsultan pajak."*
2. Dua tombol berdampingan:

| Tombol | Isi berkas |
|---|---|
| **Data riil (apa adanya)** *(Layer 1)* | Seluruh transaksi periode |
| **Versi dibatasi Rp4,8 M** *(Layer 2)* | Order penjualan dibuang sampai omzet kumulatif ≤ Rp 4,8 M |

3. Tekan salah satu → berkas Excel 8 sheet terunduh, bernama
   `berkas-pajak_{tanggal awal}_{tanggal akhir}.xlsx`.
4. Toast **"Berkas siap"** dengan nama dan ukuran berkas.

**Yang perlu dipahami tentang Layer 2:** ia tidak mengubah atau menghapus data apa pun — buku besar
sebenarnya tetap utuh. Ia hanya menyaring saat laporan dibentuk, membuang **seluruh jurnal milik
order terpilih** (penjualan + HPP + PPN + pembayaran) sehingga kedelapan sheet tetap seimbang dan
konsisten. Pemilihan ordernya deterministik — mengunduh dua kali menghasilkan berkas yang sama.

**Ini keputusan bisnis yang harus Anda konfirmasi sebelum fase desain**, sebaiknya bersama
konsultan pajak Anda — lihat [open-questions.md](../open-questions.md) **OQ-A42**. Selama belum ada
keputusan, saya mendokumentasikan perilakunya apa adanya dan tidak merancang ulang bagian ini.

**Dua catatan operasional:**

1. **Tidak ada jejak audit** — tidak tercatat siapa mengunduh versi mana, kapan.
2. **Tidak ada validasi rentang tanggal** — rentang terbalik atau bertahun-tahun diterima, dan
   seluruh berkas dibangun di memori. Pada VPS 2 GB ini bisa menjatuhkan server (KI-144).

---

## UF-12 — Membuka kembali bulan yang sudah dikunci

**Pelaku:** Owner dengan `finance.manage`.

1. Pilih periode terkunci → tekan buka kunci.
2. Dialog **"Buka Kunci Periode Keuangan?"** muncul.
3. **Isi alasan** — wajib: `Alasan koreksi wajib diisi`.
4. Periode kembali terbuka; penulisan diizinkan lagi.

Alasan tersimpan di jejak audit dan **tetap terbaca pemilik** — sesuai aturan
*"wajib `reason_text` saat membuka kembali periode yang terkunci"*.

Alur yang sama berlaku untuk **periode pajak** (dialog **"Buka Kunci Periode Pajak?"**) — tetapi
perlu diketahui: **menutup periode pajak tidak mengunci apa pun**, ia hanya membekukan snapshot
angka PPN. Membukanya kembali hanya memungkinkan snapshot dihitung ulang.

**[PERLU KONFIRMASI]** Perbedaan ini tidak terlihat di antarmuka — kedua "periode" tampak setara,
padahal hanya satu yang benar-benar mengunci. Perlu dibedakan lebih jelas di sistem baru?

---

## UF-13 — Mengisi worksheet penyesuaian fiskal

**Pelaku:** Owner atau konsultan pajak. Alur tahunan.

1. **Keuangan → Penyesuaian Pajak (Worksheet)**.
2. Isi kode periode, rentang tanggal, dan lima komponen:

```
laba fiskal = laba komersil
            + koreksi positif
            − koreksi negatif
            − penghasilan final
            − penghasilan tidak kena pajak
```

3. Simpan sebagai draft, atau tandai final.

**Setelah final, catatan tidak bisa diubah:** `Catatan sudah final, tidak dapat diubah`.

Worksheet ini ikut jadi sheet ke-8 di paket berkas pajak.

**Yang perlu diketahui:** worksheet **tidak menghitung apa pun secara otomatis** — laba komersil
pun diketik manual, tidak diambil dari laporan laba rugi. Ia murni lembar kerja.

**[PERLU KONFIRMASI]** Apakah laba komersil sebaiknya diisi otomatis dari laporan Untung Rugi
periode yang sama? Sekarang pengguna harus menyalinnya sendiri, dan tidak ada pemeriksaan bahwa
angkanya cocok.
