# Open Questions — Analisis Legacy Mini ERP

Kumpulan seluruh tanda **[PERLU KONFIRMASI]** dari 21 modul + lapisan shared, dikumpulkan jadi
satu daftar yang bisa dijawab sekali duduk.

**Ditemukan 130 tanda** tersebar di 92 berkas. Setelah disaring, tidak semuanya pertanyaan untuk
Anda — sebagian adalah **pekerjaan saya yang belum tuntas**. Karena itu daftar ini dibagi tiga:

| Kelompok | Isi | Siapa yang menjawab | Jumlah |
|---|---|---|---|
| **A** | Keputusan produk & fakta production yang hanya Anda tahu | **Anda** | **48** |
| **B** | Gap analisis — bisa saya tutup sendiri dengan membaca kode/DB lagi | Saya | 32 *(19 sudah tertutup)* |
| **C** | Konsistensi kosmetik (wording, format, toast) | Anda, tapi bisa diborong satu keputusan | 38 |

**Yang perlu Anda kerjakan sekarang hanya Kelompok A** — dan di dalamnya hanya **15 item P0**
yang benar-benar memblokir fase desain. Sisanya bisa menyusul.

> **Pembaruan pendalaman modul 17 Finance (selesai):** 19 gap Kelompok B tertutup, dokumen modul
> ditulis ulang, **6 known-issue baru** (KI-144…KI-149) dan **7 pertanyaan baru** (OQ-A42…OQ-A48)
> muncul. Yang paling perlu perhatian Anda: **OQ-A42** — dua versi paket berkas pajak.

Legenda prioritas:

| Tanda | Arti |
|---|---|
| 🔴 **P0** | Memblokir desain — jawaban mengubah bentuk sistem baru |
| 🟠 **P1** | Memengaruhi desain satu modul; bisa diputuskan saat modul itu dirancang |
| 🟡 **P2** | Tidak memblokir; keputusan bisa ditunda sampai implementasi |

---

## Kelompok A — Keputusan Pemilik Sistem (48)

### A.1 Keamanan & Sesi (6)

| ID | Prio | Pertanyaan | Kenapa penting | Sumber |
|---|---|---|---|---|
| **OQ-A01** | 🔴 P0 | **Apakah API sudah dibatasi origin-nya di reverse proxy VPS?** Aplikasi memanggil `enableCors()` tanpa opsi = seluruh origin diizinkan. | Bila tidak dibatasi di mana pun, situs mana pun bisa memanggil API dari browser pengguna Anda. Dampaknya tertahan karena token disimpan di `localStorage` (bukan cookie), jadi penyerang tetap butuh token — tapi ini harus dipastikan, bukan diasumsikan. | `shared/shared-services.md:744` |
| **OQ-A02** | 🔴 P0 | **Apakah sesi perlu punya batas umur mutlak?** Sekarang sesi memperpanjang diri terus-menerus (sliding); sesi aktif bisa hidup bertahun-tahun. | Menentukan apakah sistem baru butuh "wajib login ulang tiap N hari". Menyentuh seluruh alur auth. | `01/algorithms-legacy.md:110`, KI-12 |
| **OQ-A03** | 🟠 P1 | **Apakah percobaan login gagal perlu dicatat?** Sekarang tidak ada jejak, tidak ada rate limit, tidak ada lockout. | Tanpa ini tidak ada cara mendeteksi percobaan tebak-password. | `01/business-rules.md:503`, KI-11 |
| **OQ-A04** | 🟠 P1 | **Apakah perlu jeda/rate limit pada endpoint login?** | Melindungi dari tebak-password beruntun. | `01/algorithms-legacy.md:51` |
| **OQ-A05** | 🟡 P2 | **Apakah pesan login boleh membedakan "password salah" dari "akun tanpa role"?** Sekarang bisa dibedakan. | Kebocoran informasi kecil vs kejelasan bagi pengguna. | `01/business-rules.md:87` |
| **OQ-A06** | 🟡 P2 | **Apakah `trust proxy` di-set di deployment?** Tanpa itu, IP yang tercatat di riwayat sesi adalah IP proxy, bukan IP pengguna. | Menentukan apakah kolom IP di audit ada gunanya. | `01/reports-list.md:66` |

### A.2 Pengguna, Role & Izin (5)

| ID | Prio | Pertanyaan | Kenapa penting | Sumber |
|---|---|---|---|---|
| **OQ-A07** | 🟠 P1 | **Apakah ada akun production yang memegang lebih dari satu role?** Sistem mengizinkannya, tapi role aktif jadi tidak dapat diprediksi. | Bila tidak pernah dipakai, sistem baru bisa disederhanakan jadi satu-role-per-user. | `01/user-flows.md:168`, `02/feature-inventory.md:280`, KI-05 |
| **OQ-A08** | 🟠 P1 | **Apakah status `locked` pada pengguna dimaksudkan bisa diset manual?** Kolomnya ada, tapi tidak ada satu pun jalur yang mengisinya. | Menentukan apakah fitur "kunci akun" dibuat atau kolomnya dibuang. | `01/data-model-legacy.md:123`, `02/business-rules.md:69` |
| **OQ-A09** | 🟠 P1 | **Apakah pernah muncul kebutuhan "satu pengguna dengan pengecualian izin"** (izin di luar role-nya)? | Menentukan apakah model izin baru cukup role-based, atau butuh override per-user. | `02/data-model-legacy.md:82` |
| **OQ-A10** | 🟠 P1 | **Apakah pemegang hak kelola role boleh menaikkan hak dirinya sendiri?** Sekarang bisa, tanpa penjagaan. | Keputusan kebijakan, bukan bug. | `02/business-rules.md` (KI-15) |
| **OQ-A11** | 🟡 P2 | **Apakah audit perubahan role/izin perlu dilengkapi?** Sekarang tiga hal tidak tercatat isinya. | Menentukan kelengkapan jejak audit sistem baru. | `02/business-rules.md:373`, `02/reports-list.md:85` |

### A.3 Perusahaan & Pengaturan (5)

| ID | Prio | Pertanyaan | Kenapa penting | Sumber |
|---|---|---|---|---|
| **OQ-A12** | 🔴 P0 | **Nasib sistem feature flag** — seluruh tabelnya mati di tiga lapisan (tanpa UI, tanpa pembaca, jalur datanya diblokir adapter). Dibuang, atau dirancang ulang? | Bila dibuang, satu tabel + tiga endpoint hilang dari sistem baru. | `03/feature-inventory.md:297`, `03/data-model-legacy.md:228`, KI-33 |
| **OQ-A13** | 🟠 P1 | **Apakah kode perusahaan dipakai di luar sistem** (mis. tercetak di dokumen, dipakai partner)? Wajib diisi tapi tidak dipakai apa pun di dalam sistem. | Bila tidak, field-nya bisa dibuang. | `03/numbering-sequence.md:85`, `03/data-model-legacy.md:229` |
| **OQ-A14** | 🟠 P1 | **Apakah nama legal perusahaan dipakai di dokumen resmi?** Tersimpan, tidak ditemukan pembacanya. | Sama seperti di atas. | `03/data-model-legacy.md:61` |
| **OQ-A15** | 🟠 P1 | **Kartu "Cabang" di halaman Pengaturan menghitung cabang milik pengguna, bukan total cabang perusahaan.** Apakah itu yang dimaksud? | Angka yang salah tafsir di layar ringkasan. | `03/reports-list.md:36` |
| **OQ-A16** | 🟡 P2 | **Apakah perubahan kebijakan persetujuan koreksi stok perlu penjagaan lebih ketat?** Sekarang bisa dilonggarkan dalam satu klik — teraudit, tapi tidak dicegah. | Kebijakan kontrol internal. | `03/business-rules.md:536` |

### A.4 Cabang & Penomoran Dokumen (5)

| ID | Prio | Pertanyaan | Kenapa penting | Sumber |
|---|---|---|---|---|
| **OQ-A17** | 🔴 P0 | **Apakah nomor pembayaran / surat jalan / retur seharusnya kembali ke `00001` setiap 1 Januari?** Kebijakan "reset tahunan" tersimpan di data tapi **tidak pernah dijalankan** — nomornya terus naik lintas tahun. | Menyentuh rekap tahunan dan dokumen resmi/SPT. Apa pun jawabannya sah, tapi harus dipilih sadar. | `04/numbering-sequence.md:135`, KI-45 |
| **OQ-A18** | 🔴 P0 | **Apakah kode cabang pernah diubah di production?** Mengubahnya menulis ulang prefix seluruh nomor dokumen, sehingga dokumen lama dan baru dari cabang yang sama punya prefix berbeda dengan urutan yang menyambung. | Menentukan apakah kode cabang dikunci setelah ada dokumen terbit, atau tetap bebas diubah dengan peringatan. | `04/user-flows.md:173`, KI-40 |
| **OQ-A19** | 🟠 P1 | **Apakah pernah ada cabang yang perlu ditutup?** Sekarang tidak ada caranya sama sekali — status non-aktif punya arti nyata di 5 tempat, tapi tombolnya tidak pernah dibuat. | Menentukan apakah fitur tutup-cabang dibuat. | `04/user-flows.md:256`, KI-36 |
| **OQ-A20** | 🟠 P1 | **Apakah Anda pernah butuh angka gabungan seluruh cabang** (mis. total penjualan perusahaan)? Sekarang tidak ada layarnya — semua angka terbatas pada cabang aktif. | Kesenjangan produk, bukan bug. Menentukan apakah dashboard konsolidasi dibuat. | `04/reports-list.md:30` |
| **OQ-A21** | 🟡 P2 | **Apakah nomor berbentuk `PAY-<angka>-<angka panjang>` pernah terlihat di data?** Itu "nomor darurat" — penanda cabang yang lahir di luar alur normal. | Bila pernah, ada data yang perlu dirapikan sebelum migrasi. | `04/numbering-sequence.md:160` |

### A.5 Produk & Katalog (6)

| ID | Prio | Pertanyaan | Kenapa penting | Sumber |
|---|---|---|---|---|
| **OQ-A22** | 🔴 P0 | **Harga beli wajib > 0 hanya saat produk dibuat, tidak saat diubah.** Produk berstok bisa diedit harga belinya jadi kosong → HPP-nya macet di "Menunggu data" di Keuangan. Ditutup? | Ini lubang yang langsung merusak angka keuangan. | `05/business-rules.md:49` (BR-21), KI-52 |
| **OQ-A23** | 🔴 P0 | **Upload bulk melewati seluruh penjagaan yang ada di form**: satuan & faktor konversi produk yang sudah punya riwayat stok bisa ditimpa, tipe produk berstok bisa diubah jadi jasa, dan info tambahan terhapus bila sheet-nya tidak disertakan. Dipertahankan atau ditutup? | Jalur ini bisa merusak angka historis dan valuasi persediaan tanpa peringatan. | `05/business-rules.md`, KI-51/KI-52 |
| **OQ-A24** | 🟠 P1 | **Apakah "saldo" yang menghalangi arsip produk seharusnya `on_hand` atau `available`?** Sekarang memakai `on_hand`, jadi stok yang seluruhnya ter-reservasi tetap menghalangi arsip. | Menentukan arti "produk masih punya stok". | `05/business-rules.md:69`, `05/feature-inventory.md:191` |
| **OQ-A25** | 🟠 P1 | **Perlukah aturan `harga jual minimum ≤ harga jual normal`?** Sekarang tidak dicek sama sekali. | Salah ketik di sini mahal. | `05/business-rules.md:49` |
| **OQ-A26** | 🟠 P1 | **Import: nilai "Pantau Stok?" yang tidak dikenali jatuh ke `true`.** Salah ketik jadi produk berstok, bukan ditolak. Disengaja? | Menentukan arah kegagalan yang aman. | `05/business-rules.md:97` |
| **OQ-A27** | 🟡 P2 | **API menerima angka negatif** (harga, kuantitas) tanpa penolakan. Disengaja atau celah? | Perlu batas eksplisit di sistem baru. | `05/business-rules.md:114` |

### A.6 Mitra Bisnis & Member (4)

| ID | Prio | Pertanyaan | Kenapa penting | Sumber |
|---|---|---|---|---|
| **OQ-A28** | 🟠 P1 | **Tombol ubah/arsip buku alamat memakai izin `order.update` (bukan `order.archive`) dan tidak disembunyikan di UI** — tombolnya tampil untuk semua, panggilannya gagal di server. Perlu digate di UI? | Pengguna melihat tombol yang selalu gagal. | `06/user-flows.md:19` |
| **OQ-A29** | 🟠 P1 | **Tombol simpan mitra tidak dinonaktifkan saat menyimpan** — klik ganda bisa mengirim dua kali. Perlu penjagaan anti-kirim-ganda? | Berpotensi membuat data ganda. | `06/ui-ux-spec.md:87` |
| **OQ-A30** | 🟠 P1 | **Bila perhitungan harga member gagal, baris order mempertahankan harga sebelumnya** (bukan kembali ke harga standar). Mana yang benar? | Menentukan harga yang benar-benar ditagihkan saat sistem tersendat. | `07/user-flows.md:59`, `07/business-rules.md:56` |
| **OQ-A31** | 🟡 P2 | **Pengguna tanpa izin `order.create` di POS tidak mendapat harga member, tanpa pesan apa pun.** Perlu pemberitahuan? | Kasir bisa menagih harga salah tanpa sadar. | `07/user-flows.md:81` |

### A.7 Order, Penerimaan, Pengiriman & Retur (5)

| ID | Prio | Pertanyaan | Kenapa penting | Sumber |
|---|---|---|---|---|
| **OQ-A32** | 🔴 P0 | **Filter tanggal `date_to` di API tidak mencakup akhir hari** — order yang dibuat setelah pukul 00:00 di hari terakhir hilang dari hasil. Frontend menambal dengan `T23:59:59`, tapi pemanggil API langsung tidak. | Laporan/ekspor bisa kehilangan data hari terakhir. Perlu jadi KI baru + diperbaiki. | `08/feature-inventory.md:140` |
| **OQ-A33** | 🟠 P1 | **Nomor invoice supplier boleh ganda** — tidak ada penolakan duplikat. Disengaja? | Berisiko bayar dua kali untuk invoice yang sama. | `08/business-rules.md:95` |
| **OQ-A34** | 🟠 P1 | **Dua penerimaan barang bersamaan bisa melebihi sisa PO** (tidak ada penguncian baris, berbeda dari retur pembelian yang menguncinya). Pernah terjadi? | Menentukan apakah penguncian perlu ditambahkan. | `10/feature-inventory.md:113`, KI-92 |
| **OQ-A35** | 🟡 P2 | **Ada anomali penautan order/cabang di surat jalan** (tercatat sebagai anomali di analisis). Disengaja? | Perlu dipastikan sebelum model data baru dibekukan. | `11/data-model-legacy.md:43` |
| **OQ-A36** | 🟡 P2 | **Daftar retur menerima nilai status apa pun tanpa validasi, dan tanggal tak sah diproses mentah.** Perilaku yang diharapkan? | Menentukan ketatnya validasi filter. | `12/business-rules.md:57` |

### A.8 Pembayaran, POS & Stok (5)

| ID | Prio | Pertanyaan | Kenapa penting | Sumber |
|---|---|---|---|---|
| **OQ-A37** | 🟠 P1 | **Ambang "lunas" tidak seragam: `0,009` untuk baris terbuka, `0` untuk bayar-order.** Disengaja? | Selisih receh bisa membuat dokumen dianggap lunas di satu layar dan belum di layar lain. | `14/feature-inventory.md:107` |
| **OQ-A38** | 🔴 P0 | **Keranjang POS hilang saat halaman disegarkan** — tidak ada penyimpanan draf. Diperbaiki di sistem baru? | Kasir kehilangan transaksi berjalan bila perangkat/tab bermasalah. Ini pengalaman harian. | `15/data-model-legacy.md:32` |
| **OQ-A39** | 🟠 P1 | **Indeks basis data audit hanya mencakup perusahaan+cabang+waktu**; filter aksi/entitas tanpa indeks. Sudah terasa lambat di volume Anda? | Menentukan indeks yang perlu disiapkan. | `20/business-rules.md:26`, KI-129 |
| **OQ-A40** | 🟠 P1 | **Batas mana yang benar untuk ekspor/laporan keuangan: 366 hari? 5000 baris?** Angka ini muncul di modul Order tapi belum dipastikan berlaku di Finance. | Menentukan batas yang konsisten. | `17/business-rules.md:83`, `17/reports-list.md:97` |
| **OQ-A41** | 🟠 P1 | **Selisih pada penutupan kas/periode: dibukukan sebagai biaya atau pendapatan?** Tidak ada aksinya di sistem sekarang. | Kebijakan akuntansi yang harus diputuskan pemilik, bukan ditebak. | `17/user-flows.md:93` |

### A.9 Keuangan — hasil pendalaman modul 17 (7)

| ID | Prio | Pertanyaan | Kenapa penting | Sumber |
|---|---|---|---|---|
| **OQ-A42** | 🔴 **P0** | **Paket berkas pajak punya dua versi: "Data riil (apa adanya)" dan "Versi dibatasi Rp4,8 M".** Versi kedua membuang order penjualan sampai omzet kumulatif berada di bawah ambang PP-23. Apakah versi kedua dibawa ke sistem baru? | Ini **keputusan bisnis dengan konsekuensi kepatuhan pajak**, bukan pilihan teknis — dan hanya Anda yang bisa memutuskannya, sebaiknya bersama konsultan pajak Anda. Selama belum dijawab, saya tidak merancang ulang bagian ini. Data aslinya tidak pernah diubah; yang berbeda hanya isi berkas yang diekspor. | `17/algorithms-legacy.md` §7, `17/reports-list.md` §6.1 |
| **OQ-A43** | 🔴 P0 | **Mana DPP yang benar saat tarif pajak pada order kosong: nol, atau subtotal sebelum pajak?** Dua laporan PPN memakai rumus berbeda dan tidak rekonsiliasi. | Konsultan pajak akan melihat dua angka DPP berbeda untuk periode yang sama, dalam satu berkas. | KI-145 |
| **OQ-A44** | 🔴 P0 | **Berapa rentang tanggal terpanjang yang benar-benar Anda butuhkan sekali unduh berkas pajak?** Sekarang tidak ada batas apa pun, dan seluruh berkas dibangun di memori pada VPS 2 GB. | Menentukan batas yang aman tanpa mengganggu kebutuhan nyata. | KI-144 |
| **OQ-A45** | 🟠 P1 | **Apakah tipe akun / arah saldo pernah diubah setelah akunnya dipakai?** Sekarang bebas diubah tanpa penjagaan — satu klik bisa membalik seluruh laporan historis. | Bila belum pernah, penjagaannya bisa langsung ditambahkan tanpa mengganggu kebiasaan siapa pun. | KI-147 |
| **OQ-A46** | 🟠 P1 | **Adakah produk bervarian yang harga belinya berbeda antar varian?** Harga modal dihitung per produk, bukan per varian — padahal stoknya dipisah per varian. | Bila semua varian selalu seharga sama, masalahnya tidak pernah muncul di praktik. | KI-148 |
| **OQ-A47** | 🟠 P1 | **Aksi keuangan mana yang wajib punya jejak audit?** Posting, pembalikan jurnal, batal posting, dan **unduh berkas pajak** semuanya tidak teraudit. | Saran saya minimal tiga: pembalikan jurnal, batal posting, dan unduh berkas pajak. | KI-149 |
| **OQ-A48** | 🟡 P2 | **Apakah laba komersil di worksheet penyesuaian fiskal sebaiknya diisi otomatis dari laporan Untung Rugi?** Sekarang diketik manual tanpa pemeriksaan kecocokan. | Mengurangi salah salin pada dokumen yang dipakai lapor tahunan. | `17/user-flows.md` UF-13 |

---

## Kelompok B — Gap Analisis (32 tersisa, 19 tertutup) — **tugas saya, bukan Anda**

Ini tanda [PERLU KONFIRMASI] yang sebenarnya berarti *"saya belum selesai membaca kode/DB"*.
Saya cantumkan supaya terlacak, **Anda tidak perlu menjawabnya**.

### B.1 Modul 17 Finance — ✅ **19 gap TERTUTUP** (pendalaman selesai)

Modul 17 sudah diperdalam menyeluruh. Seluruh 19 tanda yang dulu berarti *"belum dibaca"* /
*"cek cepat"* / *"grep pemanggil"* kini terjawab dari kode, dan dokumennya ditulis ulang
(973 → 2.100+ baris).

| Yang dulu belum tuntas | Jawabannya |
|---|---|
| Peta foreign key antar 16 entity | **Lengkap** dari migrasi 022–032 — `17/data-model-legacy.md` §2. Baris jurnal = simpul terpadat (8 FK, menyentuh 4 modul). Empat tautan sengaja **tanpa** FK karena polimorfik |
| Isi `finance_tax_report_snapshots` | 3 angka utama (PPN keluaran/masukan/kurang bayar) **+ seluruh hasil ringkasan PPN sebagai JSON** |
| Relasi tax-period → snapshot / adjustment | Periode pajak **1:1** snapshot (dibuat saat ditutup), **1:N** worksheet penyesuaian (FK opsional — worksheet boleh berdiri sendiri) |
| Pemakai `formatDocumentSequenceNumber` | **Tiga**: posting (jurnal), saldo awal + suplemen, biaya usaha (nomor biaya & jurnalnya) |
| Format nomor aktual | `JRN/2026/00001` · `EXP/2026/00001`. Sequence `OB` ter-seed tapi **tidak pernah dibaca** |
| Batas validasi tanggal ekspor (366/5000?) | **Tidak ada batas apa pun** — angka 366/5.000 milik ekspor modul Order, bukan Finance → **KI-144** |
| **Di mana tarif PPN & DPP-fallback berada** | **Di modul Order, per dokumen.** Tarif diketik per order, di-snapshot ke `orders.tax_rate_snapshot`, bawaan **0**. Tidak ada 11% di mana pun. DPP dihitung `PPN ÷ tarif × 100` — dan **dua laporan memakai cadangan berbeda** saat tarif 0 → **KI-145** |
| Istilah & paginasi laporan | Seluruh 12 laporan + logika perhitungannya di `17/reports-list.md` §4 |
| Baris Piutang/Utang yang masih `?` | Keduanya laporan yang sama (`partyBalanceReport`) dengan mapping key berbeda, dikelompokkan per pihak dari kolom dimensi baris jurnal |
| Perilaku ubah tipe/tanggal dokumen terposting | Jurnal **tidak pernah** bisa diubah — hanya dibalik. Tetapi **tipe akun bebas diubah** meski sudah berjurnal → **KI-147** |
| Breakpoint responsif & CSS cetak | Tidak ada tata letak cetak khusus di halaman finance — laporan diekspor ke Excel, bukan dicetak dari layar. Tersisa satu [PERLU KONFIRMASI] kecil di `17/ui-ux-spec.md` §10 |
| Kunci audit pembatalan biaya usaha | **`business_expense.cancel`**. Sekaligus ditemukan bahwa 7 aksi lain **tidak teraudit sama sekali** → **KI-149** |
| Perilaku tutup-periode | 6 pemeriksaan kesiapan, `failed` memblokir & `warning` tidak, kunci paksa butuh permission terpisah — `17/reports-list.md` §7 |

**Enam temuan baru** muncul dari pendalaman ini: KI-144 … KI-149. Dua di antaranya menghasilkan
pertanyaan P0 baru untuk Anda (OQ-A42 … OQ-A44).

### B.2 Presisi kolom DB — 7 gap (satu akar yang sama)

Entity TypeORM meminta `DATETIME(6)` / `DECIMAL(24,12)`, sementara migrasi baseline menulis
`DATETIME(3)` / `DECIMAL(18,4)`. Nilai yang benar-benar hidup di production **harus diperiksa
dengan `SHOW COLUMNS`**, bukan ditebak dari kode.

Terdampak: `05/business-rules.md:37`, `05/data-model-legacy.md:94, 97`,
`06/data-model-legacy.md:65`, `19/data-model-legacy.md:22`, `20/data-model-legacy.md:18`.

Satu perintah SQL menutup ketujuhnya sekaligus.

### B.3 Indeks unik yang hanya ada di kode — 2 gap

Keunikan yang ditegakkan aplikasi tetapi belum dipastikan punya indeks unik di DB:
lokasi stok (`16/data-model-legacy.md:28`) dan nomor WhatsApp (`21/data-model-legacy.md:31`).
Tanpa indeks DB, keunikan bisa bocor lewat penulisan bersamaan.

### B.4 Pertanyaan yang sudah terjawab oleh modul lain — 5 gap

Ditandai saat modulnya dianalisis, dan sekarang **sudah bisa ditutup** karena modul rujukannya
selesai:

| Pertanyaan | Ditandai di | Dijawab oleh |
|---|---|---|
| Apakah modul Users menampilkan data sesi? | `01/reports-list.md:49` | modul 02 |
| Apakah modul WhatsApp menormalkan ulang kuota AI? | `03/business-rules.md:148` | modul 21 |
| Apakah verifikasi kredensial approver dipakai di tempat lain? | `02/business-rules.md:527` | modul 16 |
| Apakah ada pemanggil internal endpoint pengaturan? | `03/business-rules.md:288` | modul 21 |
| Teks persis envelope error 401/403/500 | `19/business-rules.md:56`, `19/feature-inventory.md:96` | `shared/shared-business-rules.md` §envelope |

### B.5 Sisa gap kecil — 18

Termasuk: apakah pemanggilan audit dibungkus try/catch (`20/feature-inventory.md:43`), siapa
pemakai `summary-card.tsx` (`18/ui-ux-spec.md:52`), apakah frontend selalu mengirim ISO
ber-timezone (`shared/shared-business-rules.md:240`), batas modul Order vs Finance untuk margin di
ekspor (`08/reports-list.md:37`), perilaku cache pada satu alur produk (`05/user-flows.md:80`),
prioritas nama vs default eksplisit (`08/ui-ux-spec.md:104`), dan pemisahan komponen toast
(`shared/shared-data-model.md:527`).

---

## Kelompok C — Konsistensi Kosmetik (38) — bisa diborong satu keputusan

Semuanya bertipe sama: **ketidakseragaman kecil yang terlihat pengguna**. Alih-alih menjawab 38
item, cukup jawab tiga pertanyaan payung ini:

| ID | Pertanyaan payung | Menutup |
|---|---|---|
| **OQ-C01** | **Bahasa antarmuka: Indonesia sepenuhnya, atau campuran seperti sekarang?** Halaman login/pilih-cabang masih berbahasa Inggris ("Select Branch", "Access Profile"), badge POS memakai `{n} items`, sementara sisanya Indonesia. | `01/ui-ux-spec.md:13, 352`, `15/ui-ux-spec.md:52`, + 6 lainnya |
| **OQ-C02** | **Umpan balik seragam: setiap aksi tulis yang berhasil memunculkan toast sukses, dan setiap kegagalan menampilkan pesan asli server?** Sekarang campur — sebagian aksi diam saat berhasil (simpan cabang, simpan kategori), sebagian membuang pesan server, dan ada satu tempat yang masih memakai `alert()` browser. | `02/ui-ux-spec.md:401`, `05/ui-ux-spec.md:47`, KI-04/KI-17/KI-43, + 12 lainnya |
| **OQ-C03** | **Label & format diseragamkan?** "Bundle" vs "Bundle ringan", teks bantu kode cabang yang menyebut 3 dari 5 jenis dokumen, format tanggal (`toLocaleString` bebas vs `dd-mm-yyyy hh:mm` baku), titik di akhir pesan. | `05/ui-ux-spec.md:195`, `04/ui-ux-spec.md:126`, `06/ui-ux-spec.md:129`, `05/feature-inventory.md:250`, `07/ui-ux-spec.md:82`, + 12 lainnya |

Jawaban "ya, seragamkan" pada ketiganya menutup seluruh Kelompok C sekaligus.

---

## Ringkasan per Modul

| Modul | Total tanda | A (Anda) | B (saya) | C (kosmetik) |
|---|---|---|---|---|
| 01 Auth & Session | 17 | 6 | 3 | 8 |
| 02 Users, Roles & Permissions | 9 | 5 | 2 | 2 |
| 03 Company & Settings | 10 | 5 | 2 | 3 |
| 04 Branch | 9 | 5 | 0 | 4 |
| 05 Product / Catalog | 13 | 6 | 4 | 3 |
| 06 Business Party | 4 | 2 | 1 | 1 |
| 07 Member Pricing | 5 | 2 | 0 | 3 |
| 08 Order — Purchasing | 5 | 2 | 2 | 1 |
| 09 Order — Sales | 1 | 0 | 0 | 1 |
| 10 Goods Receipt | 2 | 1 | 0 | 1 |
| 11 Delivery | 2 | 1 | 0 | 1 |
| 12 Sales Return | 3 | 1 | 1 | 1 |
| 13 Purchase Return | 2 | 0 | 0 | 2 |
| 14 Payment | 3 | 1 | 0 | 2 |
| 15 POS | 2 | 2 | 0 | 0 |
| 16 Stock & Inventory | 2 | 0 | 1 | 1 |
| **17 Finance** | **23** | **9** ✅ | **0** *(19 tertutup)* | 2 |
| 18 Dashboard | 1 | 0 | 1 | 0 |
| 19 Reporting | 3 | 0 | 3 | 0 |
| 20 Audit Log | 4 | 1 | 2 | 1 |
| 21 Assistant/WhatsApp/Knowledge | 1 | 0 | 1 | 0 |
| shared/ | 7 | 2 | 4 | 1 |
| **Total** | **130** | **48** | **32** | **38** |

---

## Yang Saya Sarankan Anda Jawab Lebih Dulu

Lima belas item **P0** ini yang benar-benar memblokir fase desain:

| ID | Ringkas |
|---|---|
| OQ-A01 | Apakah CORS dibatasi di reverse proxy? |
| OQ-A02 | Sesi perlu batas umur mutlak? |
| OQ-A12 | Sistem feature flag dibuang atau dirancang ulang? |
| OQ-A17 | Nomor pembayaran/SJ/retur reset tiap tahun? |
| OQ-A18 | Kode cabang pernah diubah di production? |
| OQ-A22 | Tutup lubang harga beli saat ubah produk? |
| OQ-A23 | Upload bulk boleh terus melewati penjagaan? |
| OQ-A32 | Perbaiki filter tanggal akhir-hari? |
| OQ-A38 | Keranjang POS perlu bertahan saat refresh? |
| OQ-C01 | Bahasa antarmuka seragam Indonesia? |
| OQ-C02 | Umpan balik (toast & pesan galat) seragam? |
| OQ-C03 | Label & format tanggal seragam? |
| **OQ-A42** | **Paket berkas pajak versi "dibatasi Rp4,8 M" dibawa ke sistem baru?** |
| OQ-A43 | DPP mana yang benar saat tarif pajak order kosong? |
| OQ-A44 | Berapa rentang terpanjang ekspor berkas pajak yang Anda butuhkan? |

Sisanya (33 item P1/P2 di Kelompok A) bisa dijawab sambil jalan, saat modulnya dirancang.

---

## Keputusan Analis atas 15 P0 (delegasi pemilik)

Pemilik menyerahkan keputusan kepada analis. Di bawah ini keputusan final per item — pemilik
boleh meng-override kapan pun, tetapi implementasi berjalan dengan nilai ini kecuali diubah
eksplisit. Item bertanda ⚖️ menyentuh kepatuhan/keuangan dan tetap disarankan dikonsultasikan
ke konsultan pajak saat modul `finance` didesain.

| ID | Keputusan | Alasan satu kalimat |
|---|---|---|
| OQ-A01 | Model same-origin (tanpa middleware CORS); bila deploy pisah, allowlist eksplisit, tidak pernah `*` | Dev via proxy Vite + produksi satu container membuat CORS tak diperlukan; `*` warisan adalah lubang |
| OQ-A02 | Refresh sliding 7 hari dalam batas umur mutlak 30 hari | Sliding-selamanya berisiko di perangkat kasir bersama; batas mutlak tanpa batas sama sekali mengganggu shift — 30 hari titik tengahnya, N bisa disetel |
| OQ-A12 | **Dibuang** — tanpa tabel, tanpa endpoint | Mati di 3 lapisan, nol pembaca; sudah tercermin di `DB_SCHEMA` |
| OQ-A17 | **Tanpa reset** — penomoran berlanjut lintas tahun; kebijakan "yearly" palsu tidak disimpan | Nomor berhenti-di-tengah-tahun yang berubah perilaku merusak kontinuitas dokumen resmi/SPT; urutan tak terputus justru baik untuk audit |
| OQ-A18 | Kode cabang **dikunci** setelah ada dokumen terbit; bebas diubah hanya bila nol dokumen | Mengubah kode menulis ulang prefix dokumen lama+baru (KI-40) — integritas arsip di atas kenyamanan admin |
| OQ-A22 | Harga beli wajib > 0 saat **ubah** untuk produk berstok; boleh 0 untuk jasa/non-tracked | Lubang ini langsung memacetkan HPP (KI-52) |
| OQ-A23 | Bulk import **wajib** menegakkan penjagaan yang sama dengan form; pola resumable + galat per baris dipertahankan | Impor yang diam-diam merusak histori lebih buruk daripada impor yang ditolak dengan pesan jelas |
| OQ-A32 | `date_to` inklusif akhir-hari di server (WIB) | Workaround `T23:59:59` di frontend membuktikan niatnya; pemanggil API langsung kehilangan data hari terakhir |
| OQ-A38 | Draf keranjang POS **bertahan** (localStorage per perangkat); saat pulih, reservasi divalidasi/ di-hold ulang | Kehilangan transaksi berjalan adalah derita harian kasir; tanpa efek samping lintas-perangkat |
| OQ-C01 | Indonesia penuh di seluruh UI | Konsisten dengan locale terkunci (`id-ID`) |
| OQ-C02 | Tiap tulis sukses → toast sukses; tiap gagal → pesan asli server | Sudah kontrak `API_CONTRACT`; memperbaiki KI-04/KI-17/KI-43 |
| OQ-C03 | Seragam: tanggal `dd-mm-yyyy hh:mm` WIB, teks bantu kode cabang menyebut semua jenis dokumen, tanpa titik-akhir ganda | Detail diberlakukan per modul saat didesain |
| OQ-A42 ⚖️ | **Bangun HANYA versi data riil**; versi "dibatasi Rp4,8 M" **tidak dibangun** kecuali pemilik + konsultan pajak menyetujui eksplisit | Mengekspor omzet yang dibuang ke berkas pajak adalah keputusan kepatuhan, bukan teknis — defaultnya jangan ada fiturnya |
| OQ-A43 ⚖️ | DPP = subtotal bila tarif pajak order kosong; satukan rumus kedua laporan PPN | DPP nol menyembunyikan omzet; subtotal mencerminkan penjualan sebenarnya — bawa ke konsultan pajak saat desain `finance` |
| OQ-A44 | Batas ekspor 366 hari per unduh, selebihnya bertahap | SPT tahunan butuh ≤366 hari; tanpa batas, VPS 2 GB membangun berkas di memori (KI-144) |

---

## Catatan Metodologis

Seluruh tanda di atas berasal dari **pembacaan kode**, bukan pengujian aplikasi berjalan. Untuk
pertanyaan bertipe *"pernah terjadi di production?"* (OQ-A07, A18, A19, A21, A34, A39), jawaban
tercepat biasanya datang dari pengalaman Anda sendiri — apakah gejalanya pernah dilaporkan
pengguna.

Daftar ini **berbeda** dari [known-issues.md](known-issues.md): di sana 143 perilaku aneh yang
menunggu keputusan *perbaiki / replikasi*; di sini 130 hal yang **tidak bisa dipastikan dari kode
saja**. Beberapa item saling merujuk — ditandai dengan nomor KI di kolom sumber.
