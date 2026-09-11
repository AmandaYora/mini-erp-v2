# Data Model (Legacy) — Modul 04 Branch (Multi-Cabang)

**Kelompok B — cukup dipahami maksudnya, BUKAN untuk ditiru strukturnya.** Dokumen ini menjelaskan
*apa yang disimpan dan mengapa*, bukan menyarankan bentuk tabel untuk sistem baru.

Skema lintas modul ada di [../shared/shared-data-model.md](../shared/shared-data-model.md).

---

## 1. Dua Tabel yang Dimiliki Modul Ini

### 1.1 `branches` — identitas cabang

Maksudnya: satu baris = satu tempat operasional nyata (toko/gudang/kantor cabang) milik perusahaan.

| Kolom | Maksud |
|---|---|
| `id_branch` | Identitas internal; dipakai sebagai kunci pemisah data di **hampir semua tabel transaksi** |
| `id_company` | Pemilik; sistem ini satu perusahaan, jadi praktis selalu bernilai sama |
| `code` | Kode singkat yang **terlihat pengguna**, jadi sumber prefix nomor dokumen dan kode gudang |
| `name` | Nama tampilan di seluruh layar dan kop dokumen |
| `city` | Kota; muncul di daftar, di kartu pemilihan cabang, dan sebagai cadangan baris kota kop dokumen |
| `address_text` | Alamat lengkap; cadangan baris alamat kop dokumen |
| `phone_e164` | Nomor telepon cabang; **tidak pernah bisa diisi lewat aplikasi** dan tidak dipakai layar mana pun |
| `default_stock_location_label` | Nama yang ingin dipakai untuk lokasi stok pertama cabang. Bukan data operasional — ini "template" yang disalin ke tabel lokasi stok |
| `status` | Aktif atau tidak. Tidak ada UI-nya, tapi dibaca lima tempat berbeda (lihat business-rules §5) |
| `is_default` | Penanda "cabang pusat". Hanya berasal dari seed; tidak pernah ditulis aplikasi |
| `created_at`, `updated_at` | Jejak waktu |

Aturan keunikan: kombinasi perusahaan + kode.

**Yang perlu dipahami untuk rebuild:** `default_stock_location_label` adalah **duplikasi disengaja**
— nilainya juga hidup sebagai nama baris di tabel lokasi stok. Duplikasi inilah sumber KI-42
(nilai di cabang selalu menang saat cabang disimpan). Sistem baru sebaiknya memilih satu pemilik:
entah cabang menyimpan label ini dan lokasi stok hanya turunannya, atau lokasi stok jadi pemilik
tunggal dan cabang tidak menyimpan apa-apa soal nama gudang.

**Yang tidak ada:** kolom arsip. Cabang tidak bisa dihapus atau diarsipkan — hanya bisa ditandai
non-aktif, dan itu pun tidak lewat aplikasi.

### 1.2 `branch_document_sequences` — penghitung nomor dokumen

Maksudnya: satu baris = satu deret nomor berjalan, untuk satu jenis dokumen di satu cabang.

| Kolom | Maksud |
|---|---|
| `id_branch` + `sequence_key` | Identitas deret. `sequence_key` adalah jenis dokumen (`order`, `payment`, `sj`, `sales_return`, `purchase_return`, `stock_transfer`, dan kunci dinamis bulanan) |
| `prefix` | Awalan nomor, diturunkan dari kode cabang. Disimpan agar pembuat nomor tidak perlu membaca tabel cabang setiap kali |
| `current_value` | Nomor terakhir yang sudah diterbitkan |
| `reset_policy` | **Dokumentasi niat, bukan perilaku.** Tidak pernah dibaca satu pun kode |
| `format_template` | Sama — tidak pernah dibaca. Pola sebenarnya ditulis tetap di kode tiap pembuat nomor |
| `updated_at` | Jejak waktu |

Aturan keunikan: kombinasi cabang + jenis dokumen. Ini yang menjamin dua dokumen bersamaan tidak
bisa mendapat nomor yang sama.

**Yang perlu dipahami untuk rebuild:**

1. **`sequence_key` bukan enum tertutup** — ia string bebas, dan modul Order memanfaatkan itu untuk
   membuat kunci dinamis per bulan (`order_sales_2026-08`). Akibatnya tabel ini tumbuh satu baris
   per cabang per jenis order per bulan, selamanya. Untuk cabang yang sudah lama berjalan ini
   ratusan baris yang tidak pernah dibaca lagi.
2. **Kebijakan reset diletakkan di data tapi diberlakukan di kode** — dan kedua tempat itu tidak
   sinkron. Sistem baru harus memilih satu tempat.
3. **Prefix disalin, bukan dirujuk.** Ini keputusan sadar demi kecepatan (menghindari join), tapi
   berarti setiap perubahan kode cabang harus menyapu ulang semua baris — dan alur penyapuan itu
   melewatkan `stock_transfer` (KI-46).

---

## 2. Tabel yang Ikut Ditulis Modul Ini

### 2.1 `stock_locations` (milik modul Stok)

Modul Cabang menulis **satu baris** di tabel ini: lokasi bertanda default milik cabang.

Yang dipahami: cabang tanpa satu pun lokasi stok tidak bisa menerima barang, sehingga sistem
menjamin setiap cabang lahir dengan minimal satu tempat penyimpanan. Selebihnya (lantai, area,
rak, hierarki, aturan lokasi daun) sepenuhnya milik modul Stok.

Kolom yang disentuh: `id_branch`, `code` (turunan kode cabang), `name` (dari label cabang),
`is_default`, `status`.

Kolom yang **tidak** disentuh dan karena itu bernilai bawaan: induk lokasi (kosong — lokasi default
selalu jadi akar), penanda area picking, penanda utama, urutan tampil, dan arsip.

---

## 3. Tabel yang Merujuk ke Cabang (dibaca, tidak ditulis modul ini)

Cabang adalah tulang punggung pemisahan data. **26 tabel** menaut ke cabang lewat kunci asing
(salah satunya tabel penghitung milik modul ini sendiri), ditambah **3 tabel** yang menyimpan
nomor cabang tanpa kunci asing.

| Kelompok | Tabel |
|---|---|
| Akses | akses cabang pengguna |
| Penjualan & pembelian | order, penerimaan barang, retur penjualan, retur pembelian |
| Uang | pembayaran, alokasi pembayaran, biaya usaha |
| Stok | saldo persediaan, mutasi persediaan, lokasi stok, reservasi stok, alokasi pengeluaran, dokumen mutasi antar-cabang (menaut **dua** cabang: asal dan tujuan) |
| Keuangan | jurnal dan barisnya, sumber posting, saldo awal persediaan, keadaan & mutasi biaya persediaan |
| Kecerdasan | percakapan, pesan, jalannya asisten, eksekusi alat |
| Pantauan | metrik operasional harian |
| **Tanpa kunci asing** | sesi pengguna (cabang aktif), riwayat aktivitas, surat jalan |

Surat jalan menyimpan nomor cabang dan mengindeksnya, tapi tidak menautkannya secara formal —
tautan resminya lewat order induk.

**Yang perlu dipahami untuk rebuild:** karena `id_branch` tersebar seluas ini, mengubah cara cabang
diidentifikasi adalah perubahan yang menyentuh hampir seluruh sistem. Sebaliknya, mengubah
*bagaimana cabang disimpan* (kolom, nama tabel, bentuk penomoran) hanya menyentuh modul ini.

Dokumen mutasi antar-cabang adalah satu-satunya tempat yang merujuk **dua** cabang sekaligus
(asal dan tujuan).

---

## 4. Bentuk Data yang Sampai ke Layar

Bagian ini menjelaskan **penerjemahan**, bukan struktur — karena antarmuka dan basis data memakai
bentuk yang berbeda dan perbedaannya sempat menimbulkan kebingungan.

| Yang dilihat layar | Asalnya |
|---|---|
| Daftar cabang di halaman `/branches` | Entitas cabang mentah dari server, diterjemahkan ke bentuk web |
| Nomor cabang | Selalu **diubah jadi teks** di sisi web, sementara di server ia angka. Setiap pengiriman balik ke server harus mengubahnya kembali jadi angka |
| Alamat | Penerjemah menerima **dua nama field** (`address` dan `address_text`) karena dua jalur data berbeda pernah dipakai |
| Nama | Penerjemah menerima **dua nama field** (`name` dan `branch_name`) — jalur daftar cabang memakai yang pertama, jalur sesi memakai yang kedua |
| Label lokasi stok | Bila kosong dari mana pun, dijadikan `"Default"` di sisi web juga |

**Yang perlu dipahami untuk rebuild:** keberadaan penerjemah yang menerima banyak nama field ini
adalah **fosil** dari dua jalur data yang tidak pernah diseragamkan (jalur "daftar cabang" dan jalur
"cabang di sesi"). Sistem baru sebaiknya punya satu bentuk cabang saja.

---

## 5. Data Cabang di Penyimpanan Lokal Browser

Selain basis data, ringkasan cabang juga hidup di penyimpanan lokal browser sebagai bagian dari
sesi:

| Yang disimpan | Untuk apa |
|---|---|
| Nomor, nama, dan kode cabang aktif | Chip topbar, badge menu avatar, kop dokumen |
| Daftar cabang yang bisa diakses (nomor, nama, kode, kota, label lokasi stok, penanda default) | Kartu-kartu di halaman pemilihan cabang |

**Yang perlu dipahami untuk rebuild:** salinan lokal ini **tidak menyegarkan diri**. Menambah akses
cabang untuk pengguna yang sedang login tidak terasa sampai ia menyegarkan halaman. Dan karena
data cabang di daftar (dari server) digabung dengan data cabang di sesi (dari penyimpanan lokal),
ada dua sumber untuk hal yang sama — sumber utama masalah KI-47 (peran tanpa izin lihat cabang
kehilangan alamat cabang karena hanya punya salinan sesi yang tidak memuatnya).

---

## 6. Ringkasan Niat Model Ini

Kalau seluruh detail di atas dilupakan, tiga niat inilah yang harus bertahan:

1. **Cabang adalah unit pemisah data operasional**, dan identitasnya harus stabil karena dirujuk
   di mana-mana.
2. **Kode cabang adalah identitas yang terlihat pengguna**, dan sistem menurunkan penomoran dokumen
   serta penamaan gudang darinya — sehingga kode punya konsekuensi yang jauh melampaui tampilan.
3. **Cabang harus lahir siap pakai**: begitu ada, ia sudah punya tempat menyimpan barang dan sudah
   bisa menerbitkan nomor dokumen.
