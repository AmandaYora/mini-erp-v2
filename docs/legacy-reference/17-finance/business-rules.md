# Business Rules — Modul 17 Finance / Accounting & Pajak

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Seluruh validasi, formula,
kondisi khusus, dan aturan izin. Diturunkan dari 6 controller + 8 service + 8 migrasi. Bagian
ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [algorithms-legacy.md](algorithms-legacy.md)

---

## 1. Aturan Bagan Akun (BR-01…BR-08)

| ID | Aturan |
|---|---|
| **BR-01** | Kode akun **unik per perusahaan**. Pelanggaran → `Kode akun '{kode}' sudah digunakan`. Pemeriksaan aplikasi **tidak** menyaring akun terarsip, jadi kode akun terarsip tidak bisa dipakai ulang |
| **BR-02** | Lima tipe akun sah: `asset` · `liability` · `equity` · `revenue` · `expense`. **Tidak divalidasi** — nilai apa pun diterima |
| **BR-03** | Dua saldo normal: `debit` · `credit`. **Tidak divalidasi** |
| **BR-04** | Kode & nama dipangkas spasi saat simpan (tambah maupun ubah) |
| **BR-05** | **Tipe akun dan saldo normal bebas diubah kapan pun** — termasuk setelah akun punya ratusan jurnal. Tidak ada penjagaan. Mengubah tipe akun mengubah seluruh laporan historis |
| **BR-06** | Arsip akun ditolak bila: dipakai mapping, dipakai kas/rekening, **atau pernah muncul di baris jurnal**. Pesan terakhir: `Akun sudah pernah dipakai di jurnal. Tidak dapat diarsipkan; nonaktifkan saja dari mapping baru bila tidak ingin dipakai lagi.` |
| **BR-07** | Kolom induk akun bisa diisi bebas — **tanpa penjagaan putaran**, dan **tidak dibaca laporan mana pun** |
| **BR-08** | `system_key` hanya diisi seed; alur tambah manual selalu mengisinya kosong |

**BR-05 adalah lubang terbesar di sub-area ini.** Mengubah akun pendapatan jadi akun biaya akan
membalik laporan laba rugi untuk seluruh riwayat, tanpa peringatan apa pun.

---

## 2. Aturan Mapping Akun (BR-09…BR-12)

| ID | Aturan |
|---|---|
| **BR-09** | Hanya **15 kunci** yang sah (lihat [feature-inventory.md](feature-inventory.md) §4). Kunci lain → `Mapping key tidak dikenal: {kunci}` |
| **BR-10** | Daftar mapping kosong ditolak: `Mapping akun tidak boleh kosong` |
| **BR-11** | Akun tujuan wajib ada, milik perusahaan itu, dan belum diarsipkan |
| **BR-12** | **Akun lama yang sudah pernah dipakai jurnal tidak boleh dilepas** dari mapping-nya: `Mapping '{kunci}' tidak bisa diubah: akun lama sudah pernah dipakai jurnal. Buat akun baru untuk transaksi ke depan, jangan ganti mapping yang sudah punya riwayat.` |

BR-12 melindungi laporan yang membaca "akun saat ini" lewat mapping — tanpa itu, riwayat akun lama
hilang dari pandangan begitu mapping diganti.

Seluruh pembaruan mapping berjalan dalam **satu transaksi**: satu kunci gagal → semuanya batal.

---

## 3. Aturan Kas & Rekening (BR-13…BR-16)

| ID | Aturan |
|---|---|
| **BR-13** | Kode kas/rekening unik per perusahaan |
| **BR-14** | Dua jenis: `cash` · `bank`. **Tidak divalidasi**. Jenis menentukan mapping mana yang dipakai saat pembayaran: `cash` → `cash_cash`, selain itu → `cash_bank` |
| **BR-15** | Akun buku besar tujuan wajib ada & belum diarsipkan |
| **BR-16** | **Arsip tanpa penjagaan apa pun** — berbeda dari akun (BR-06). Kas/rekening yang sudah dipakai ratusan transaksi bisa diarsipkan begitu saja |

**Penanda "utama"** tersimpan tapi **tidak ada penjagaan "hanya satu"** dan **tidak dibaca alur
posting**. Pemilihan kas saat pembayaran ditentukan metode pembayaran, bukan penanda ini.

---

## 4. Aturan Periode Akuntansi (BR-17…BR-23)

| ID | Aturan |
|---|---|
| **BR-17** | Kode periode wajib: `Kode periode wajib diisi` |
| **BR-18** | Tanggal wajib `YYYY-MM-DD`: `Tanggal wajib diisi dengan format YYYY-MM-DD` |
| **BR-19** | Tanggal awal ≤ tanggal akhir: `Tanggal awal harus lebih dahulu dari tanggal akhir` |
| **BR-20** | Kode periode unik: `Periode '{kode}' sudah ada` |
| **BR-21** | **Rentang tanggal tidak boleh tumpang tindih** dengan periode mana pun: `Rentang tanggal tumpang tindih dengan periode '{kode}' ({dari} s.d. {sampai})` |
| **BR-22** | Buka kembali periode **wajib alasan**: `Alasan koreksi wajib diisi`. Alasan disimpan di jejak audit |
| **BR-23** | Menutup periode lewat `finance/periods/close` **selalu melewati pemeriksaan kesiapan** — tidak ada jalur tutup yang melewatinya |

**BR-21 punya alasan teknis penting:** penentuan "periode mana yang berlaku untuk tanggal ini"
mengambil satu baris saja. Dengan periode tumpang tindih, periode yang dipilih jadi acak — sehingga
penguncian bisa berlaku atau tidak secara tak terduga. Untuk data lama yang terlanjur tumpang
tindih, ada jaring pengaman: urutkan menurun berdasarkan tanggal awal, ambil yang paling akhir.

---

## 5. Aturan Penguncian Periode (BR-24…BR-27)

| ID | Aturan |
|---|---|
| **BR-24** | Periode terkunci menolak: posting sumber, catat biaya, batal biaya, balik jurnal, batal posting, kunci saldo awal. Pesan: `Periode {kode} sudah ditutup` / `Periode keuangan {kode} sudah dikunci` |
| **BR-25** | Penentuan periode memakai **hari kalender WIB** dari tanggal jurnal, bukan hari UTC |
| **BR-26** | Tanggal transaksi rusak ditolak terkontrol: `Tanggal transaksi tidak valid; tidak bisa menentukan periode keuangan` — bukan galat internal |
| **BR-27** | Tanggal yang **tidak masuk periode mana pun** → **lolos**. Tidak ada periode = tidak terkunci |

**BR-27 penting:** perusahaan yang belum pernah membuat periode sama sekali bisa memposting apa
pun. Penguncian bersifat *opt-in*.

---

## 6. Aturan Kunci Aman & Kunci Paksa (BR-28…BR-31)

| ID | Aturan |
|---|---|
| **BR-28** | Kunci ditolak bila ada pemeriksaan **gagal**: `Periode belum aman dikunci: {n} item kritikal belum beres. Perbaiki dulu atau gunakan force=true.` |
| **BR-29** | **Peringatan tidak memblokir** — hanya `failed` yang memblokir |
| **BR-30** | Kunci paksa butuh permission **terpisah** `finance.close.force`: `Anda tidak memiliki izin untuk menutup periode secara paksa (force)` |
| **BR-31** | Penulisan status terkunci memakai **penguncian baris** — dua permintaan bersamaan tidak bisa saling menimpa atau menghasilkan audit ganda |

Periode yang **sudah** terkunci dan dikunci lagi: dikembalikan apa adanya, **tanpa audit tambahan**.

---

## 7. Aturan Antrian Posting (BR-32…BR-44)

| ID | Aturan |
|---|---|
| **BR-32** | Identitas baris antrian = **(perusahaan, jenis sumber, id sumber, event key)**. Satu dokumen bisa memunculkan beberapa baris dengan event berbeda |
| **BR-33** | Enam status: `pending` · `ready` · `posted` · `failed` · `ignored` · `reversed` |
| **BR-34** | Tujuh kode alasan tertahan, dua kategori: **ordering** (selesai sendiri, disembunyikan) dan **needs_input** (tugas pemilik, dibawa ke hari berikutnya) |
| **BR-35** | Sinkronisasi **idempoten** — dijalankan dua kali tidak mengubah apa pun |
| **BR-36** | Baris **sudah diposting tidak tersentuh** sinkronisasi ulang |
| **BR-37** | Perubahan isi dokumen dideteksi lewat sidik jari; perbandingan memakai serialisasi **berurutan kunci** karena basis data menormalkan urutan kunci JSON |
| **BR-38** | Dokumen sumber yang **diarsipkan** otomatis direkonsiliasi di antrian |
| **BR-39** | Rentang pindai dibatasi watermark otomatis; batas bawahnya = paling awal di antara tanggal cutover, sumber belum-final tertua, dan hari setelah periode tertutup terakhir. Bisa dimatikan lewat variabel lingkungan |
| **BR-40** | Kandidat diproses per **5.000 baris** agar tidak membengkak saat pengisian awal |
| **BR-41** | Tutup hari memposting gelombang `ready` **dalam urutan dependensi** (10→50), bukan urutan id |
| **BR-42** | Tanggal jurnal = **tanggal sumber**, bukan hari memposting |
| **BR-43** | Sumber `ignored` bisa dipulihkan; keduanya mengubah status tanpa menyentuh jurnal |
| **BR-44** | Batal posting membalik jurnal **dan** mengembalikan status sumber, sehingga bisa diposting ulang |

### 7.1 Empat status "tutup hari ini"

Dihitung **hanya dari aktivitas hari itu** — backlog berdiri sendiri:

| Status | Arti | Kondisi |
|---|---|---|
| `kosong` | Belum ada transaksi hari ini | Aktivitas hari ini = 0 |
| `belum` | Masih ada yang harus diposting | Ada pekerjaan `ready`/ordering |
| `selesai_catatan` | Rutin beres, ada Tipe B tertunda | Tidak ada pekerjaan, ada needs_input |
| `lengkap` | Semua sudah masuk buku | Ada aktivitas, tidak ada yang tertahan |

Pembedaan `kosong` dari `lengkap` disengaja: "belum ada apa-apa" tidak boleh terbaca sebagai
"sudah bersih".

---

## 8. Aturan Jurnal (BR-45…BR-52)

| ID | Aturan |
|---|---|
| **BR-45** | Jurnal wajib **seimbang**: `Jurnal tidak balance. Debit {d}, credit {c}`. Toleransi **0,009** |
| **BR-46** | Total debit wajib **> 0** — jurnal bernilai nol ditolak oleh pemeriksaan yang sama |
| **BR-47** | Satu baris **tidak boleh debit dan kredit sekaligus**: `Satu baris jurnal tidak boleh debit dan credit sekaligus` |
| **BR-48** | Seluruh nominal dibulatkan ke **2 desimal** |
| **BR-49** | Mapping key yang belum dikonfigurasi menggagalkan posting: `Mapping akun '{kunci}' belum dikonfigurasi` |
| **BR-50** | Tiga tipe posting: `system` (otomatis) · `manual` (biaya usaha) · `reversal` |
| **BR-51** | Jurnal **tidak pernah dihapus atau diubah** — hanya dibalik |
| **BR-52** | Jurnal pembalik menyalin seluruh baris asli dengan debit↔kredit ditukar; asli jadi `reversed` |

Status `draft` terdefinisi di kode tetapi **tidak pernah dipakai** — semua jurnal lahir `posted`.

---

## 9. Aturan Harga Modal / HPP (BR-53…BR-62)

| ID | Aturan |
|---|---|
| **BR-53** | Metode **rata-rata bergerak**, granularitas **(cabang, produk)** — **bukan** per varian |
| **BR-54** | Barang masuk: `rata-rata baru = (nilai lama + jumlah × harga satuan) ÷ (jumlah lama + jumlah masuk)` |
| **BR-55** | Barang keluar dinilai pada rata-rata saat itu; rata-rata **tidak berubah** |
| **BR-56** | Satu mutasi stok hanya boleh menghasilkan **satu** baris biaya (idempoten) |
| **BR-57** | Sumber harga barang masuk, berurutan: **harga retur → harga beli master → harga modal rata-rata** |
| **BR-58** | Harga beli master dikonversi ke **satuan dasar** dulu (harga tersimpan per satuan beli) |
| **BR-59** | Barang masuk tanpa harga modal ditolak: `Harga modal barang masuk belum tersedia. Lengkapi harga beli produk atau isi saldo awal produk ini dulu.` |
| **BR-60** | Barang keluar tanpa basis biaya → sumber **tertahan** `missing_cost_basis`, bukan diposting nol |
| **BR-61** | **Rasio kewajaran 0,1×–10×** terhadap harga beli master. Pelanggaran → ditolak dengan penjelasan kemungkinan salah digit |
| **BR-62** | **Harga modal rata-rata tidak boleh negatif.** Barang masuk yang akan menghasilkannya ditolak dengan penjelasan bahwa basis biaya sudah rusak sebelumnya |

Presisi: jumlah & harga modal **12 desimal**, nilai rupiah **2 desimal**.

**BR-61 dan BR-62 lahir dari insiden Agustus 2026** (241 produk, salah 10–1000×, ~Rp 3,2 M jurnal
tercemar). Keduanya **wajib dipertahankan**.

---

## 10. Aturan Saldo Awal (BR-63…BR-73)

| ID | Aturan |
|---|---|
| **BR-63** | **Satu saldo awal per perusahaan** |
| **BR-64** | Saldo awal `posted` **tidak bisa diedit**: `Opening balance sudah diposting dan tidak bisa diedit` |
| **BR-65** | **Harga modal tidak pernah dari input pengguna** — server mengabaikan nilai kiriman klien sepenuhnya, selalu mengambil harga beli master saat itu |
| **BR-66** | Produk tanpa harga beli ditolak: `Produk "{nama}" belum punya Harga Beli di Data Produk. Isi dulu harga belinya sebelum ditambahkan ke Saldo Awal.` |
| **BR-67** | Validasi kunci: cutover terisi, jumlah tidak negatif, tiap produk masih punya harga beli, **6 mapping wajib** lengkap (`cash_cash`, `cash_bank`, `accounts_receivable`, `accounts_payable`, `inventory`, `opening_equity`) |
| **BR-68** | Jurnal pembuka: debit kas + bank + piutang + persediaan, kredit hutang, **selisihnya ke Modal Awal** sebagai penyeimbang |
| **BR-69** | Saldo nol semua → saldo awal tetap ditandai terkunci, **tanpa jurnal** |
| **BR-70** | Tanggal jurnal pembuka = **tanggal cutover**, bukan hari mengunci |
| **BR-71** | Posting ulang ditolak: `Catatan pembuka untuk saldo awal sudah ada. Gunakan reversal untuk koreksi.` |
| **BR-72** | Impor CSV wajib header `id_branch,id_product,quantity_on_hand`. **Kolom harga sengaja tidak ada** |
| **BR-73** | Cabang & produk wajib valid dan produk belum diarsipkan |

### 10.1 Lengkapi stok awal (BR-74…BR-78)

| ID | Aturan |
|---|---|
| **BR-74** | Hanya untuk saldo awal **yang sudah dikunci**: `Lengkapi stok awal hanya untuk saldo awal yang sudah dikunci. Selagi draft, isi langsung lalu kunci.` |
| **BR-75** | Jumlah wajib **> 0**: `Jumlah stok awal harus lebih dari 0` |
| **BR-76** | Produk yang sudah ada ditolak: `Produk "{nama}" sudah ada di saldo awal` |
| **BR-77** | Bersifat **aditif** — tidak mereset jumlah, tidak menyentuh yang sudah diposting |
| **BR-78** | Jurnal suplemen: debit Persediaan, kredit Modal Awal — seimbang menurut konstruksi |

---

## 11. Aturan Biaya Usaha (BR-79…BR-88)

| ID | Aturan |
|---|---|
| **BR-79** | Nominal wajib **> 0**: `Nominal biaya harus lebih dari 0` |
| **BR-80** | Tanggal wajib `YYYY-MM-DD` |
| **BR-81** | Cabang: dari input, jatuh ke cabang aktif sesi; wajib **aktif**: `Cabang biaya tidak valid` |
| **BR-82** | Akun biaya wajib bertipe **`expense`**: `Akun yang dipilih bukan akun biaya`, dan wajib **aktif** |
| **BR-83** | Akun pembayaran wajib **terdaftar sebagai kas/rekening aktif**: `Kas/rekening pembayaran tidak valid` |
| **BR-84** | Akun pembayaran wajib bertipe **`asset`**: `Akun pembayaran harus akun aset (kas atau bank)` |
| **BR-85** | Periode tanggal biaya wajib terbuka |
| **BR-86** | Jurnal 2 baris: **debit akun biaya, kredit akun kas/bank**, tipe posting `manual` |
| **BR-87** | Batal wajib alasan: `Alasan pembatalan biaya wajib diisi`. Sudah batal → `Biaya sudah dibatalkan` |
| **BR-88** | Penjagaan periode pembatalan memakai periode **jurnal asli**, tetapi jurnal pembaliknya bertanggal **hari ini** |

**BR-88 punya cacat kecil:** "hari ini" dihitung dalam **UTC**. Pembatalan antara 00:00–07:00 WIB
menghasilkan pembalik bertanggal hari sebelumnya — satu-satunya tempat di finance yang tidak
WIB-aware. Lihat KI-146.

---

## 12. Aturan Pajak (BR-89…BR-97)

| ID | Aturan |
|---|---|
| **BR-89** | **Tarif PPN tidak pernah jadi pengaturan sistem** — diketik per order di modul Order, di-snapshot ke dokumen. Bawaan **0** |
| **BR-90** | Tarif divalidasi di modul Order: 0–100, selain itu `Tarif pajak harus bernilai 0 sampai 100.` |
| **BR-91** | DPP ringkasan PPN = `PPN ÷ tarif × 100`; **bila tarif 0 → DPP 0** |
| **BR-92** | DPP rincian PPN = rumus sama, **tetapi bila tarif 0 → subtotal sebelum pajak**. Dua rumus berbeda → tidak rekonsiliasi (KI-145) |
| **BR-93** | Periode pajak **terpisah** dari periode akuntansi dan **tidak mengunci apa pun** |
| **BR-94** | Tutup periode pajak membekukan **snapshot**: 3 angka utama + seluruh hasil ringkasan sebagai JSON |
| **BR-95** | Periode pajak sudah tertutup → `Periode pajak sudah ditutup`; race → `Periode pajak '{kode}' sedang/sudah ditutup proses lain` |
| **BR-96** | Buka kembali periode pajak **wajib alasan** |
| **BR-97** | Periode pajak **tidak diperiksa tumpang tindih** — berbeda dari periode akuntansi (BR-21) |

### 12.1 Worksheet penyesuaian fiskal (BR-98…BR-101)

| ID | Aturan |
|---|---|
| **BR-98** | Formula: **laba fiskal = laba komersil + koreksi positif − koreksi negatif − penghasilan final − penghasilan tidak kena pajak** |
| **BR-99** | Seluruh nominal wajib angka berhingga |
| **BR-100** | Tanggal awal ≤ tanggal akhir |
| **BR-101** | Status `final` **tidak bisa diubah**: `Catatan sudah final, tidak dapat diubah` |

---

## 13. Aturan Ekspor Paket Berkas Pajak (BR-102…BR-105)

| ID | Aturan |
|---|---|
| **BR-102** | Delapan sheet dibangun dari **satu himpunan jurnal yang sama**, dihitung sekali, agar angka antar-sheet konsisten |
| **BR-103** | **Layer 1** = seluruh data. **Layer 2** = order penjualan dibuang sampai omzet kumulatif ≤ **Rp 4.800.000.000** |
| **BR-104** | Layer 2 membuang **seluruh jurnal milik order terpilih** (penjualan + HPP + PPN + pembayaran), memakai urutan acak-**stabil**, sehingga hasilnya sama setiap kali diunduh dan seluruh laporan tetap seimbang |
| **BR-105** | **Tidak ada validasi rentang tanggal sama sekali** — kosong, terbalik, atau bertahun-tahun semuanya diterima |

**BR-103/BR-104 adalah keputusan bisnis, bukan aturan teknis.** Lihat
[open-questions.md](../open-questions.md) **OQ-A42** dan
[algorithms-legacy.md](algorithms-legacy.md) §7.

**BR-105 adalah risiko operasional nyata** pada VPS 2 GB — lihat KI-144.

---

## 14. Aturan Zona Waktu (BR-106…BR-108)

| ID | Aturan |
|---|---|
| **BR-106** | Seluruh filter tanggal laporan memakai konversi **Asia/Jakarta**, bukan `DATE()` mentah |
| **BR-107** | `date_to` selalu **inklusif akhir hari WIB** |
| **BR-108** | Tahun/bulan nomor dokumen dari kalender **WIB** pada tanggal transaksi |

Satu-satunya pengecualian: perhitungan "hari ini" pada pembatalan biaya usaha (BR-88).

---

## 15. Aturan Izin (BR-109…BR-112)

| ID | Aturan |
|---|---|
| **BR-109** | Sebelas permission — lihat [feature-inventory.md](feature-inventory.md) §6 |
| **BR-110** | Cakupan perusahaan **selalu** dari sesi, tidak pernah dari isi permintaan |
| **BR-111** | Modul ini **tidak memakai penjaga cabang aktif** — cabang hanya dibaca sebagai nilai bawaan biaya usaha |
| **BR-112** | Kunci paksa periode adalah satu-satunya aksi berpermission tersendiri (`finance.close.force`) |

**Keganjilan yang perlu diputuskan:** paket berkas pajak — termasuk Layer 2 — dijaga
`finance.report.view`, permission yang sama dengan melihat laporan biasa.

---

## 16. Yang TIDAK Diperiksa

| Yang mungkin diharapkan | Kenyataan |
|---|---|
| Tipe akun & saldo normal tidak boleh diubah setelah berjurnal | **Tidak ada penjagaan** (BR-05) |
| Hierarki akun tidak boleh melingkar | Tidak ada |
| Hanya satu kas/rekening bertanda utama | Tidak ada |
| Kas/rekening yang sudah dipakai tidak boleh diarsipkan | Tidak ada (BR-16) |
| Periode pajak tidak boleh tumpang tindih | Tidak ada (BR-97) |
| Rentang tanggal ekspor dibatasi | Tidak ada (BR-105) |
| Aksi posting/pembalikan/unduh teraudit | **Tidak ada** — lihat feature-inventory §7 |
| Tarif pajak punya nilai bawaan | Tidak ada (BR-89) |
| Laba berjalan ditutup ke ekuitas tiap tahun | Tidak ada |

---

## 17. Toleransi & Konstanta

| Konstanta | Nilai | Dipakai |
|---|---|---|
| Toleransi keseimbangan jurnal | **0,009** | Posting & jurnal pembuka |
| Rasio kewajaran harga modal | **0,1× – 10×** | Saldo awal & lengkapi stok awal |
| Plafon Layer 2 | **Rp 4.800.000.000** | Paket berkas pajak |
| Ukuran potongan kandidat | **5.000** baris | Sinkronisasi |
| Batas pindai backlog | **5.000** baris | Pengelompokan tugas tertahan |
| Presisi uang | **2 desimal** | Seluruh nominal |
| Presisi jumlah & harga modal | **12 desimal** | Buku besar biaya |
| Zona waktu | **Asia/Jakarta** | Seluruh filter & penomoran |
