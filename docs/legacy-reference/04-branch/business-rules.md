# Business Rules — Modul 04 Branch (Multi-Cabang)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Seluruh validasi, normalisasi,
formula, kondisi khusus, dan aturan izin yang berlaku di modul ini. Setiap aturan diturunkan
langsung dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [feature-inventory.md](feature-inventory.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md) ·
[algorithms-legacy.md](algorithms-legacy.md)

---

## 1. Aturan Identitas & Kepemilikan

### BR-01 — Kode cabang unik dalam satu perusahaan

Dua cabang dalam perusahaan yang sama tidak boleh punya kode yang sama. Dijaga di **dua lapis**:
pemeriksaan aplikasi sebelum menyimpan, dan kunci unik di basis data (`id_company` + `code`).

| Kejadian | Pesan server |
|---|---|
| Kode ganda saat **tambah** | `Kode cabang '<kode apa adanya yang diketik>' sudah digunakan` |
| Kode ganda saat **ubah** | `Kode cabang '<KODE HURUF BESAR>' sudah digunakan` |

Perbedaan huruf besar-kecil pada pesan itu nyata di kode: alur tambah memakai teks mentah yang
diketik, alur ubah memakai versi huruf besar.

Perbandingan di basis data **tidak membedakan huruf besar-kecil** (kolase `utf8mb4_unicode_ci`),
sehingga `blr` dan `BLR` dianggap sama.

### BR-02 — Normalisasi kode cabang

Kode selalu **dipangkas spasi depan-belakang lalu dijadikan HURUF BESAR** sebelum disimpan, baik
saat tambah maupun ubah. `" sby "` tersimpan sebagai `SBY`.

**Lubang yang ada sekarang:** pemeriksaan duplikat pada alur **tambah** memakai kode **mentah**
(belum dipangkas/dibesarkan), sementara yang disimpan adalah versi ternormalisasi. Kode dengan
spasi di depan bisa lolos pemeriksaan aplikasi lalu ditolak basis data sebagai galat teknis —
lihat KI-48.

### BR-03 — Normalisasi field lain

| Field | Saat **tambah** | Saat **ubah** |
|---|---|---|
| Nama | Dipangkas spasi | **Tidak** dipangkas — disimpan apa adanya |
| Kota | Dipangkas spasi; bila tidak dikirim → `""` (string kosong) | **Tidak** dipangkas |
| Alamat | Bila tidak dikirim → kosong (`null`) | Disimpan apa adanya bila dikirim |
| Telepon | Bila tidak dikirim → kosong (`null`) | Disimpan apa adanya bila dikirim |
| Label lokasi stok | Dipangkas; bila hasilnya kosong → **`Default`** | Dipangkas; bila hasilnya kosong → **`Default`** |
| Status | Disimpan apa adanya | Disimpan apa adanya bila dikirim |

Ketidaksimetrisan pemangkasan antara tambah dan ubah ini nyata di kode. **[PERLU KONFIRMASI]**
apakah perlu diseragamkan (pangkas semuanya) di sistem baru — konsekuensinya hanya kosmetik.

### BR-04 — Field yang boleh dihilangkan saat ubah

Pada alur **ubah**, setiap field diperiksa "apakah dikirim". Field yang **tidak dikirim** tidak
diubah. Ini yang membuat form web aman meski tidak pernah mengirim nomor telepon: nilai lama
bertahan.

Konsekuensinya juga sebaliknya: mengirim nilai kosong **bukan** hal yang sama dengan tidak
mengirim. Mengirim alamat berisi string kosong akan mengosongkan alamat.

### BR-05 — Status cabang tidak divalidasi

Server menerima **nilai status apa pun** sebagai teks; tidak ada daftar nilai yang sah. Nilai yang
benar-benar dipahami sistem lain hanyalah `active`; segala nilai lain diperlakukan sebagai
"bukan aktif" oleh pemeriksaan berbasis `status = 'active'`.

Form web **selalu** mengirim `active` — baik saat tambah maupun ubah.

### BR-06 — Kode cabang tidak dibatasi panjang/format di server

Batas **10 karakter** hanya berlaku di browser. Server menerima sampai batas kolom (50 karakter),
termasuk spasi di tengah dan simbol.

Ini punya konsekuensi teknis nyata: kode lokasi stok default diturunkan sebagai `GDG-<KODE>` ke
kolom yang juga berkapasitas 50 karakter, sehingga kode cabang lebih dari 46 karakter membuat
penyimpanan gagal. Lihat KI-41.

### BR-07 — Cakupan perusahaan selalu dari sesi

`id_company` **tidak pernah** diambil dari isi permintaan. Alur ubah mencari cabang dengan
`id_branch` **dan** `id_company` sesi — cabang milik perusahaan lain dijawab
`Cabang tidak ditemukan`, bukan "tidak berhak".

---

## 2. Aturan Penyediaan Otomatis (Provisioning)

### BR-08 — Cabang, gudang, dan penomoran lahir bersama atau tidak sama sekali

Pembuatan cabang berjalan dalam satu transaksi yang mencakup: baris cabang, lima penghitung nomor
dokumen, dan satu lokasi stok default. Bila salah satu gagal, **tidak ada** yang tersimpan.

Efek yang dijamin bagi pengguna: **tidak ada cabang setengah jadi**. Cabang yang muncul di daftar
pasti sudah bisa dipakai transaksi.

### BR-09 — Lima penghitung nomor dokumen yang wajib ada

| Kunci penghitung | Prefix | Dipakai untuk |
|---|---|---|
| `order` | `ORD-<KODE>` | Nomor order penjualan & pembelian |
| `payment` | `PAY-<KODE>` | Nomor pembayaran |
| `sj` | `SJ-<KODE>` | Nomor surat jalan |
| `sales_return` | `RTR-<KODE>` | Nomor retur penjualan |
| `purchase_return` | `RTB-<KODE>` | Nomor retur pembelian |

Semua dibuat dengan nilai berjalan **0**, kebijakan reset `yearly`, dan pola
`{prefix}/{year}/{seq:05}`.

Penghitung **mutasi stok** (`stock_transfer`) **tidak** termasuk daftar ini — ia dibuat sendiri
oleh modul Stok saat mutasi pertama, dengan prefix berbasis nomor internal cabang, bukan kode.
Lihat KI-46 dan [numbering-sequence.md](numbering-sequence.md) §5.

### BR-10 — Penyediaan penghitung bersifat "pastikan ada", bukan "buat ulang"

Aturan ini berjalan pada **tambah maupun ubah**:

1. Untuk setiap dari lima kunci wajib, cari penghitung cabang itu.
2. Bila **sudah ada**: perbarui prefix mengikuti kode cabang saat ini; isi kebijakan reset dan
   pola format bila kosong. **Nilai berjalan tidak disentuh.**
3. Bila **belum ada**: buat baru dengan nilai berjalan 0.

Dua konsekuensi yang dijamin:

- **Nomor tidak pernah mundur atau bertabrakan** akibat mengubah kode cabang.
- **Cabang lama otomatis dilengkapi** penghitung jenis baru begitu disimpan ulang — inilah cara
  cabang yang dibuat sebelum fitur retur ada bisa mendapatkan penghitung retur.

### BR-11 — Satu lokasi stok default per cabang

| Alur | Perilaku |
|---|---|
| Tambah | Buat lokasi stok baru: kode `GDG-<KODE>`, nama = label lokasi stok, ditandai **default**, status mengikuti status cabang |
| Ubah | Cari lokasi bertanda default milik cabang; bila tidak ada, buat baru. Lalu **selalu** tulis ulang kode, nama, dan status |

Aturan "selalu tulis ulang" ini berlaku bahkan ketika pengguna hanya mengubah kota — sehingga
perubahan nama lokasi default yang dilakukan dari menu Stok akan hilang. Lihat KI-42.

Pencarian lokasi default **tidak menyaring lokasi yang sudah diarsipkan**. Bila lokasi default
sempat diarsipkan dari menu Stok, alur ubah cabang akan memperbarui baris arsip itu — bukan
membuat lokasi default baru. Cabang jadi tanpa lokasi default yang hidup.

### BR-12 — Lokasi stok default adalah titik awal, bukan "gudang utama" permanen

Lokasi yang dibuat otomatis hanya menjamin cabang punya satu tempat penyimpanan yang bisa dipakai.
Struktur gudang/lantai/rak sepenuhnya milik modul Stok, dan aturan bahwa **mutasi stok hanya boleh
ke lokasi daun** (lokasi tanpa anak aktif) tetap berlaku — termasuk untuk lokasi default ini bila
kemudian diberi anak.

---

## 3. Aturan Audit

### BR-13 — Setiap penulisan wajib tercatat

| Aksi | `actionKey` | Yang dicatat |
|---|---|---|
| Tambah cabang | `branch.create` | Sesudah: kode & nama |
| Ubah cabang | `branch.update` | Sebelum & sesudah: kode, nama, kota, status, label lokasi stok |

Pencatatan dilakukan **di luar transaksi**, setelah data tersimpan. Artinya: bila pencatatan audit
gagal, perubahan cabang **tetap tersimpan** — konsisten dengan modul lain.

Kolom cabang pada catatan audit diisi dengan **id cabang yang sedang dibuat/diubah**, bukan cabang
aktif si pelaku.

**Yang tidak ikut tercatat pada `branch.update`:** perubahan alamat dan nomor telepon. Keduanya
bisa berubah tanpa jejak isi di riwayat aktivitas.

---

## 4. Aturan Izin & Akses

### BR-14 — Dua permission, tiga tingkat pengalaman

| Permission | Yang bisa dilakukan |
|---|---|
| Tidak punya keduanya | Menu tidak muncul; rute `/branches` dialihkan ke 403; `branches/list` ditolak |
| `branch.view` | Melihat daftar lengkap; tanpa tombol tambah/edit |
| `branch.view` + `branch.manage` | Tambah dan ubah cabang |

`branch.manage` **tidak menyiratkan** `branch.view` secara teknis, tetapi peran bawaan selalu
memberikan keduanya bersamaan.

### BR-15 — Dua endpoint tanpa permission

`branches/my-access` dan `branches/active` hanya butuh sesi login yang sah. Ini disengaja: setiap
pengguna — termasuk kasir — harus bisa mengetahui cabang mana yang boleh ia masuki.

### BR-16 — Controller cabang tidak memerlukan cabang aktif

Berbeda dengan controller order/stok/pengiriman, controller cabang **tidak memakai penjaga cabang
aktif**. Secara teknis endpoint cabang bisa dipanggil oleh sesi yang belum memilih cabang.
Praktiknya tidak pernah terjadi karena penjaga rute di sisi web mengalihkan pengguna tanpa cabang
aktif ke halaman pemilihan cabang lebih dulu.

---

## 5. Aturan Status Cabang (dampak lintas modul)

Status cabang tidak punya UI, tapi punya arti nyata di lima tempat:

| Tempat | Aturan |
|---|---|
| Daftar cabang saya (`branches/my-access`) | Hanya cabang berstatus `active` yang dikembalikan |
| Data cabang aktif (`branches/active`) | Hanya dikembalikan bila statusnya `active` |
| Ganti cabang aktif (`auth/switch-branch`) | Wajib punya akses **dan** cabang berstatus `active` — bila tidak, gagal sebagai galat teknis (KI-02) |
| Tujuan mutasi stok antar-cabang | Cabang tujuan wajib satu perusahaan **dan** berstatus `active`; bila tidak: `Cabang tujuan tidak ditemukan` |
| Metrik operasional harian (job 01:00) | Hanya cabang berstatus `active` yang dihitung |

**Yang TIDAK menyaring status:** `branches/list` — daftar di halaman `/branches` menampilkan semua
cabang apa pun statusnya, tanpa penanda apa pun (KI-39).

### BR-17 — Status lokasi stok tidak berpengaruh pada daftar lokasi

Alur ubah cabang menyalin status cabang ke lokasi stok defaultnya, tetapi daftar lokasi di modul
Stok menyaring berdasarkan **arsip**, bukan status. Jadi menonaktifkan cabang tidak menyembunyikan
lokasi defaultnya dari layar Stok. Kolom status lokasi ini praktis dekoratif.

---

## 6. Aturan Turunan di Sisi Web

### BR-18 — Tambah vs ubah ditentukan ada tidaknya id

Sisi web memutuskan memanggil "tambah" atau "ubah" semata dari ada tidaknya id cabang pada data
form. Id itu berasal dari pencarian cabang dengan id dari alamat halaman di daftar cabang yang
sudah dimuat.

**Konsekuensi:** membuka alamat edit dengan id yang tidak dikenal membuat form berubah diam-diam
menjadi form tambah, dan penyimpanan akan **membuat cabang baru**. Lihat KI-44.

### BR-19 — Form selalu mengirim status aktif

Setiap penyimpanan dari form web menyertakan status `active`, baik tambah maupun ubah. Tidak ada
jalan di antarmuka untuk mengirim nilai lain.

### BR-20 — Data yang dikirim form web

| Dikirim | Tidak dikirim |
|---|---|
| kode, nama, kota, alamat, label lokasi stok, status (`active`), dan id cabang (khusus ubah) | nomor telepon, penanda cabang pusat |

### BR-21 — Pemuatan ulang setelah menyimpan

Setelah tambah/ubah berhasil, sisi web **memuat ulang seluruh daftar cabang** dari server sebelum
berpindah halaman. Daftar yang dilihat pengguna karena itu selalu hasil dari server, bukan tebakan
lokal.

### BR-22 — Penanganan galat menelan pesan server

Kegagalan apa pun dari server ditampilkan sebagai satu toast merah **"Gagal menyimpan cabang"**
tanpa deskripsi, lalu galatnya dilempar ulang sehingga halaman **tidak** berpindah. Pesan asli
server tidak pernah sampai ke pengguna (KI-43).

### BR-23 — Tidak ada pemberitahuan sukses

Penyimpanan cabang yang berhasil tidak memunculkan toast apa pun — berbeda dari penyimpanan profil
perusahaan yang menampilkan "Profil perusahaan disimpan". Satu-satunya umpan balik adalah
berpindahnya halaman ke daftar cabang.

---

## 7. Aturan Akses Cabang Pengguna (bersinggungan dengan modul 02)

### BR-24 — Akses cabang selalu ditulis ulang seluruhnya

Menyimpan daftar cabang seorang pengguna **menghapus seluruh akses lamanya** lalu menulis ulang
dari daftar baru. Tidak ada penambahan/pengurangan bertahap.

### BR-25 — Cabang default = cabang pertama dalam daftar yang dikirim

Cabang pertama pada daftar yang dikirim ditandai sebagai cabang default pengguna; sisanya tidak.
Karena daftar yang dikirim mengikuti urutan tampil di layar (bukan urutan pengguna mencentang),
cabang default bisa berpindah tanpa penanda. Sudah tercatat sebagai KI-20.

Hasil akhirnya tetap konsisten: **tepat satu** cabang selalu bertanda default, sehingga kondisi
"beberapa cabang default" tidak dapat terjadi.

### BR-26 — Daftar cabang di sesi tidak ikut menyegarkan diri

Daftar cabang yang bisa diakses seorang pengguna disimpan di sesi lokal saat login. Menambah akses
cabang untuk pengguna yang sedang login **tidak** langsung terasa — ia perlu menyegarkan halaman
atau login ulang.

---

## 8. Aturan yang TIDAK Ada (penting untuk rebuild)

| Aturan yang mungkin diharapkan | Kenyataan |
|---|---|
| Cabang tidak boleh dihapus bila punya transaksi | Tidak relevan — **tidak ada** fitur hapus sama sekali |
| Cabang tidak boleh dinonaktifkan bila punya order berjalan | Tidak ada; menonaktifkan cabang tidak diperiksa apa pun |
| Kode cabang tidak boleh diubah setelah ada dokumen terbit | **Tidak ada penjagaan.** Kode bebas diubah kapan pun (KI-40) |
| Minimal satu cabang harus ada | Tidak ada; perusahaan boleh tanpa cabang (daftar menampilkan keadaan kosong) |
| Minimal satu cabang harus bertanda pusat | Tidak ada; penanda pusat hanya berasal dari seed |
| Kode cabang harus alfanumerik / tanpa spasi | Tidak ada validasi format di server |
| Nomor telepon harus format E.164 | Nama kolomnya menyiratkan itu, tapi **tidak ada validasi** |
| Cabang harus punya minimal satu pengguna | Tidak ada |

---

## 9. Formula & Turunan Nilai

Seluruh nilai turunan di modul ini hanya berupa penggabungan teks:

| Nilai | Formula | Contoh (kode `SBY`) |
|---|---|---|
| Kode lokasi stok default | `GDG-` + kode huruf besar | `GDG-SBY` |
| Prefix nomor order | `ORD-` + kode huruf besar | `ORD-SBY` |
| Prefix nomor pembayaran | `PAY-` + kode huruf besar | `PAY-SBY` |
| Prefix nomor surat jalan | `SJ-` + kode huruf besar | `SJ-SBY` |
| Prefix nomor retur penjualan | `RTR-` + kode huruf besar | `RTR-SBY` |
| Prefix nomor retur pembelian | `RTB-` + kode huruf besar | `RTB-SBY` |
| Label cabang di dropdown mutasi stok | `<Nama>` atau `<Nama> - <Kota>` bila kota terisi | `Cabang Surabaya - Surabaya` |
| Label cabang tak dikenal di riwayat aktivitas | `Branch #<id>` | `Branch #7` |

Semua penggabungan memangkas spasi dan membesarkan huruf kode terlebih dahulu.

---

## 10. Tidak Ada Aturan Persetujuan

Modul ini **tidak punya alur persetujuan** apa pun: tidak ada status draf, tidak ada peninjauan,
tidak ada tanda tangan kedua. Perubahan cabang berlaku seketika bagi pemegang `branch.manage`.

Bandingkan dengan modul Stok yang punya mode persetujuan koreksi stok, atau modul Finance yang
punya alur posting-reversal.
