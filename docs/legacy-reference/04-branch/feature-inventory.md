# Feature Inventory — Modul 04 Branch (Multi-Cabang)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [numbering-sequence.md](numbering-sequence.md) ·
[test-cases.md](test-cases.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 5 — `branches/{my-access,active,list,create,update}` |
| Halaman | 3 rute: `/branches`, `/branches/create`, `/branches/:branchId/edit` (2 komponen halaman) |
| Menu sidebar | 1, grup **"Pengaturan"**, label **"Cabang & Lokasi"**, ikon peta (`MapPin`) |
| Permission | `branch.view` (lihat daftar), `branch.manage` (tambah/ubah) |
| Tabel yang dimiliki | `branches`, `branch_document_sequences` |
| Tabel yang ikut ditulis | `stock_locations` (lokasi stok default cabang) |
| Laporan | Tidak ada laporan milik sendiri — lihat [reports-list.md](reports-list.md) |
| Penomoran dokumen | **Ya — modul ini pemilik seluruh penomoran dokumen cabang.** Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | 2 `actionKey`: `branch.create`, `branch.update` |
| Aksi yang TIDAK ada | Hapus cabang, arsipkan cabang, nonaktifkan cabang lewat UI |

**Catatan lingkup.** Cabang adalah **sumbu pemisah data operasional** di seluruh sistem: order,
stok, pengiriman, pembayaran, retur, jurnal keuangan, dan metrik harian semuanya bergantung pada
`id_branch` dari sesi aktif. Karena itu modul ini punya dua wajah:

1. **Wajah CRUD kecil** — satu halaman daftar + satu form, hanya dua permission.
2. **Wajah fondasi** — pembuatan cabang otomatis menyiapkan lokasi stok dan lima penghitung nomor
   dokumen, sehingga cabang baru bisa langsung dipakai transaksi tanpa setup lanjutan.

Pemilihan/penggantian cabang aktif (`/select-branch`, `auth/switch-branch`) adalah milik
**modul 01 Auth & Session**; di sini hanya didokumentasikan sisi yang menyentuh data cabang.
Pengelolaan gudang/rak/lokasi detail adalah milik **modul Stock**.

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Daftar Cabang (`/branches`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Tampilkan semua cabang perusahaan | Diurutkan **berdasarkan nama, A→Z** (diurutkan di server) |
| F-01.2 | Tampilkan nama + kode cabang | Kode jadi baris kedua di bawah nama, warna redup |
| F-01.3 | Tandai cabang yang sedang aktif | Badge hijau **"Aktif Saat Ini"** pada baris cabang aktif sesi ini |
| F-01.4 | Tandai cabang pusat | Badge biru **"Pusat"** bila penanda `is_default` bernilai benar |
| F-01.5 | Tandai akses pengguna | Badge hijau **"Anda Memiliki Akses"** / badge abu **"Tidak Ada Akses"** |
| F-01.6 | Kolom kota, alamat, label lokasi stok awal | Teks apa adanya, tanpa format |
| F-01.7 | Tombol Edit per baris | Hanya muncul bila punya `branch.manage` |
| F-01.8 | Tombol "+ Tambah Cabang" | Hanya muncul bila punya `branch.manage` |
| F-01.9 | Keadaan kosong | Kartu "Belum ada cabang" + tombol "Tambah Cabang" (bila berhak) |
| F-01.10 | Mode baca-saja | Tanpa `branch.manage`: seluruh tombol tambah/edit hilang, tabel tetap tampil |

**Yang perlu diketahui:** daftar ini **tidak menyaring status cabang**. Cabang berstatus non-aktif
tetap muncul dan tampak identik dengan cabang aktif — kolom berjudul "Status / Akses" sebenarnya
hanya menampilkan *akses* dan badge *Pusat*, tidak pernah status cabang. Lihat
[known-issues.md](../known-issues.md) KI-39.

### F-02 — Tambah Cabang (`/branches/create`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Isi nama cabang | Wajib (dijaga browser) |
| F-02.2 | Isi kode cabang | Wajib; kotak input menampilkan **HURUF BESAR**; dibatasi **10 karakter** di browser |
| F-02.3 | Isi kota | Wajib |
| F-02.4 | Isi alamat lengkap | Opsional (kotak teks banyak baris, bisa diperbesar vertikal) |
| F-02.5 | Isi label lokasi stok awal | Wajib di browser; bila kosong di server → jatuh ke **"Default"** |
| F-02.6 | Simpan | Membuat cabang **beserta** lokasi stok default dan 5 penghitung nomor dokumen |
| F-02.7 | Batal / Kembali | Dua jalan keluar: "← Kembali" di kepala halaman dan "Batal" di bawah form |
| F-02.8 | Penolakan kode ganda | Server menolak dengan pesan `Kode cabang '<KODE>' sudah digunakan` |

**Tersedia di API tapi TIDAK ada di form:** `phone` (nomor telepon cabang) dan `status`. Form web
selalu mengirim `status: "active"` dan tidak pernah mengirim `phone`. Lihat KI-36 & KI-37.

### F-03 — Ubah Cabang (`/branches/:branchId/edit`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Semua field F-02 dapat diubah | Nama, kode, kota, alamat, label lokasi stok |
| F-03.2 | Judul & tombol menyesuaikan | Judul "Edit Cabang", tombol "Simpan Perubahan", remah roti memuat nama cabang |
| F-03.3 | Ubah kode → prefix nomor dokumen ikut berubah | Kelima prefix (`ORD-`, `PAY-`, `SJ-`, `RTR-`, `RTB-`) ditulis ulang mengikuti kode baru |
| F-03.4 | Ubah kode → kode gudang default ikut berubah | Kode lokasi stok default menjadi `GDG-<KODE BARU>` |
| F-03.5 | Ubah label lokasi stok → nama gudang default ikut berubah | Nama lokasi stok default disamakan dengan label |
| F-03.6 | Penghitung nomor **tidak** direset | Nilai berjalan tiap penghitung dipertahankan apa adanya saat prefix berubah |
| F-03.7 | Penolakan kode ganda | Sama seperti F-02.8, memakai kode versi huruf besar |
| F-03.8 | Cabang tidak ditemukan | Server menjawab `Cabang tidak ditemukan` |

**Yang perlu diketahui:** menyimpan form edit **selalu** mengirim `status: "active"`. Menyimpan
cabang yang (lewat basis data) berstatus non-aktif akan diam-diam mengaktifkannya kembali —
beserta lokasi stok defaultnya.

### F-04 — Penyediaan Otomatis Saat Cabang Lahir

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Lokasi stok default | Kode `GDG-<KODE>`, nama = label lokasi stok awal, ditandai default, status mengikuti status cabang |
| F-04.2 | Penghitung nomor order | Kunci `order`, prefix `ORD-<KODE>` |
| F-04.3 | Penghitung nomor pembayaran | Kunci `payment`, prefix `PAY-<KODE>` |
| F-04.4 | Penghitung nomor surat jalan | Kunci `sj`, prefix `SJ-<KODE>` |
| F-04.5 | Penghitung nomor retur penjualan | Kunci `sales_return`, prefix `RTR-<KODE>` |
| F-04.6 | Penghitung nomor retur pembelian | Kunci `purchase_return`, prefix `RTB-<KODE>` |
| F-04.7 | Semua dalam satu transaksi | Cabang + gudang + 5 penghitung berhasil bersama, atau gagal bersama |

Efek yang dirasakan pengguna: **cabang baru langsung bisa dipakai berjualan** — tanpa langkah
setup gudang atau setup penomoran manual. Ini yang dibuktikan skenario E2E
`04-master-data-branch-gudang`.

### F-05 — Daftar Cabang Milik Saya (`branches/my-access`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Kembalikan hanya cabang yang diakses pengguna | Dari tabel akses cabang pengguna |
| F-05.2 | Saring hanya cabang berstatus aktif | Cabang non-aktif dibuang dari daftar |
| F-05.3 | Sertakan penanda cabang default pengguna | Dipakai badge "Default" di halaman pemilihan cabang |
| F-05.4 | Tanpa permission khusus | Cukup login — tidak butuh `branch.view` |

Dipakai saat **login** untuk mengisi kartu-kartu di halaman "Select Branch" dan chip cabang di
topbar. Halaman pemilihan cabang itu sendiri milik modul 01.

### F-06 — Cabang Aktif (`branches/active`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | Kembalikan data cabang aktif sesi | Hanya bila cabang berstatus aktif; kosong bila sesi belum punya cabang aktif |

**Tidak ada satu pun pemanggil** endpoint ini di web maupun E2E. Lihat KI-49.

---

## 3. Titik Sentuh Cabang di Modul Lain (milik modul lain, didaftar agar tidak hilang)

| Tempat | Yang ditampilkan / dilakukan |
|---|---|
| Topbar (semua halaman) | Chip **"Cabang"** berisi nama cabang aktif; berubah jadi tombol **"Ganti Cabang"** bila pengguna punya akses ke **lebih dari satu** cabang |
| Menu avatar | Badge berisi **kode** cabang aktif |
| Penjaga rute | Tanpa cabang aktif, semua halaman terlindungi dialihkan ke `/select-branch` |
| Kop nota & surat jalan | Baris identitas memakai **nama + alamat cabang aktif**; alamat/kota cabang jadi cadangan bila profil dokumen kosong |
| Halaman Pengaturan → kartu ringkasan | Nama cabang aktif + jumlah cabang yang bisa diakses |
| Form pengguna | Daftar centang cabang untuk memberi akses; cabang pertama yang tercentang jadi cabang default pengguna (KI-20) |
| Mutasi stok antar-cabang | Dropdown cabang asal/tujuan berlabel `Nama - Kota`; cabang tujuan wajib berstatus **aktif** |
| Saldo awal persediaan (finance) | Dropdown cabang, memanggil `branches/list` langsung |
| Riwayat aktivitas | Kolom cabang; bila cabang tak dikenal ditampilkan `Branch #<id>` |
| Metrik harian (job 01:00) | Hanya menghitung cabang berstatus **aktif** |

---

## 4. Yang TIDAK Ada di Modul Ini

| Hal | Keterangan |
|---|---|
| Hapus / arsip cabang | Tidak ada endpoint maupun kolom arsip di tabel cabang |
| Nonaktifkan cabang dari UI | Kolom status ada dan dipakai logika lain, tapi form selalu mengirim `active` |
| Isi nomor telepon cabang | Kolom & API ada, field form tidak ada |
| Tetapkan cabang pusat | Hanya bisa lewat seed/basis data |
| Pencarian / filter / paginasi daftar cabang | Tabel menampilkan seluruh cabang sekaligus |
| Halaman detail cabang | Tidak ada; hanya daftar dan form |
| Salin pengaturan antar-cabang | Tidak ada |
| Validasi format kode di server | Tidak ada; batas 10 karakter hanya di browser |
| Cabang lintas perusahaan | Tidak berlaku — sistem ini satu perusahaan, banyak cabang |

---

## 5. Ringkasan Endpoint

| Endpoint | Permission | Masukan | Keluaran |
|---|---|---|---|
| `branches/my-access` | — (cukup login) | — | Daftar cabang aktif yang diakses pengguna: `id_branch`, `code`, `name`, `city`, `default_stock_location_label`, `is_default` |
| `branches/active` | — (cukup login) | — | Data cabang aktif sesi, atau kosong |
| `branches/list` | `branch.view` | — | Seluruh cabang perusahaan (semua status), urut nama A→Z |
| `branches/create` | `branch.manage` | `code`, `name`, `city?`, `address?`, `phone?`, `default_stock_location_label?`, `status` | Cabang yang tersimpan |
| `branches/update` | `branch.manage` | `id_branch` + field yang sama, semuanya opsional | Cabang setelah diperbarui |

Cakupan perusahaan dan identitas pengguna selalu diambil dari sesi aktif, tidak pernah dari isi
permintaan.

---

## 6. Izin Bawaan Per Peran

| Peran | `branch.view` | `branch.manage` |
|---|---|---|
| `superadmin` | ✅ | ✅ |
| `owner` | ✅ | ✅ |
| `admin` | ✅ | ❌ |
| `staff` | ❌ | ❌ |
| `kasir` (peran kustom bawaan) | ❌ | ❌ |

Konsekuensi yang terasa: **staff dan kasir tidak bisa memuat daftar cabang sama sekali** — bukan
hanya kehilangan menunya. Efek sampingnya terasa di kop nota. Lihat KI-47.
