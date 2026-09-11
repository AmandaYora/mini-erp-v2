# Test Cases — Modul 04 Branch (Multi-Cabang)

**Kelompok A — skenario input→output nyata, diturunkan dari kode dan test yang sudah ada.**

Sumber: `branch.service.spec.ts` (**10 kasus**), `04-master-data-branch-gudang.spec.ts` (1 skenario
E2E berantai), `route-access.test.ts` (1), `module-registry.test.ts` (1), plus kasus turunan
pembacaan kode (ditandai **[dari kode]**).

Data bawaan: cabang `BLR` — *Cabang Blora (Pusat)*, kota Blora, alamat *Jl. Pemuda No. 1, Blora*,
lokasi stok default `GDG-BLR` bernama *Gudang Pusat Blora*, bertanda pusat, status aktif.

---

## 1. `branches/list`

### TC-L01 — Mengembalikan seluruh cabang perusahaan ✅ *ada di spec*

```
Sesi    : perusahaan #1
Request : { data: {} }
```
Diharapkan: seluruh cabang perusahaan #1, **diurutkan berdasarkan nama A→Z**. Tidak ada
penyaringan status.

### TC-L02 — Perusahaan tanpa cabang ✅ *ada di spec*

Diharapkan: daftar **kosong** (bukan galat). Halaman menampilkan kartu **"Belum ada cabang"**.

### TC-L03 — Cabang non-aktif tetap ikut **[dari kode]**

```
Data    : BLR (aktif), SBY (status "inactive")
```
Diharapkan: **kedua** cabang muncul di daftar, dan **tidak ada penanda apa pun** yang membedakan
keduanya di layar. Lihat KI-39.

### TC-L04 — Tanpa izin lihat cabang **[dari kode]**

```
Sesi    : peran staff (tanpa branch.view)
```
Diharapkan: server menolak (**403**). Sisi web **menelan kegagalan** dan menyimpan daftar cabang
kosong tanpa pesan apa pun. Efek lanjutan: baris alamat cabang hilang dari kop nota yang dicetak
peran itu. Lihat KI-47.

### TC-L05 — Rute `/branches` butuh izin lihat cabang ✅ *ada di test web*

Diharapkan: izin yang diminta rute `/branches` adalah **`branch.view`**.

### TC-L06 — Judul halaman ✅ *ada di test web*

Diharapkan: judul untuk `/branches` adalah **"Cabang & Lokasi"**.

---

## 2. `branches/create`

### TC-C01 — Cabang baru lahir lengkap dalam satu transaksi ✅ *ada di spec*

```
Sesi    : perusahaan #1, pengguna #1
Request : { data: { code: "JKT", name: "Jakarta", status: "active" } }
```
Diharapkan:

| Yang dibuat | Nilai |
|---|---|
| Cabang | kode `JKT`, nama `Jakarta`, label lokasi stok `Default` |
| Penghitung `order` | prefix **`ORD-JKT`** |
| Penghitung `payment` | prefix **`PAY-JKT`** |
| Penghitung `sj` | prefix **`SJ-JKT`** |
| Penghitung `sales_return` | prefix **`RTR-JKT`** |
| Penghitung `purchase_return` | prefix **`RTB-JKT`** |
| Lokasi stok | kode **`GDG-JKT`**, ditandai default |
| Audit | `branch.create` |

Jumlah penghitung yang tersimpan **tepat 5** — tidak lebih, tidak kurang. Semua terjadi di dalam
**satu transaksi**.

### TC-C02 — Kode dijadikan huruf besar ✅ *ada di spec*

```
Request : { data: { code: "jkt", name: "Jakarta", status: "active" } }
```
Diharapkan: tersimpan sebagai **`JKT`**.

### TC-C03 — Kode ganda ditolak ✅ *ada di spec*

```
Data    : cabang berkode JKT sudah ada
Request : { data: { code: "JKT", name: "Jakarta 2", status: "active" } }
```
Diharapkan: **409** dengan pesan **`Kode cabang 'JKT' sudah digunakan`**.
Di layar: toast merah **"Gagal menyimpan cabang"** tanpa deskripsi; pesan server tidak terlihat.

### TC-C04 — Label lokasi stok kosong jatuh ke "Default" ✅ *ada di spec*

```
Request : { data: { code: "JKT", name: "Jakarta", status: "active" } }   // tanpa label
```
Diharapkan: label lokasi stok cabang **`Default`**, dan lokasi stok yang dibuat bernama `Default`.

### TC-C05 — Label lokasi stok berisi hanya spasi **[dari kode]**

```
Request : { data: { ..., default_stock_location_label: "   " } }
```
Diharapkan: setelah dipangkas menjadi kosong → jatuh ke **`Default`**.

### TC-C06 — Kode ganda beda huruf besar-kecil **[dari kode]**

```
Data    : cabang berkode BLR sudah ada
Request : { data: { code: "blr", ... } }
```
Diharapkan: **ditolak**. Perbandingan di basis data tidak membedakan huruf besar-kecil, sehingga
pencarian dengan `blr` menemukan `BLR`. Pesan yang muncul memakai teks **apa adanya yang diketik**:
`Kode cabang 'blr' sudah digunakan`.

### TC-C07 — Kode ganda dengan spasi di depan **[dari kode]** ⚠️

```
Data    : cabang berkode BLR sudah ada
Request : { data: { code: " BLR", ... } }
```
Diharapkan **saat ini**: pemeriksaan aplikasi **lolos** (mencari kode `" BLR"` yang tidak ada),
lalu penyimpanan ditolak basis data → **500 galat internal tanpa pesan**, bukan 409 berpesan jelas.
Lihat KI-48.

### TC-C08 — Kota tidak dikirim **[dari kode]**

```
Request : { data: { code: "JKT", name: "Jakarta", status: "active" } }
```
Diharapkan: kota tersimpan sebagai **string kosong**, bukan kosong-nilai. Kolom Kota di daftar
tampil kosong.

### TC-C09 — Alamat dan telepon tidak dikirim **[dari kode]**

Diharapkan: keduanya tersimpan **kosong-nilai**. Baris alamat pada kop nota tidak muncul untuk
cabang ini.

### TC-C10 — Status apa pun diterima **[dari kode]** ⚠️

```
Request : { data: { code: "JKT", name: "Jakarta", status: "tutup-sementara" } }
```
Diharapkan **saat ini**: tersimpan apa adanya tanpa validasi, dan lokasi stok defaultnya ikut
berstatus sama. Seluruh pemeriksaan lain memperlakukannya sebagai "bukan aktif". Tidak dapat
dicapai lewat antarmuka (form selalu mengirim `active`).

### TC-C11 — Kode sangat panjang **[dari kode]** ⚠️

```
Request : { data: { code: "<50 karakter>", ... } }
```
Diharapkan **saat ini**: cabang lolos (kolomnya memang 50 karakter), tapi kode lokasi stok
turunannya menjadi 54 karakter dan **melebihi kapasitas kolom** → seluruh transaksi gagal dengan
galat teknis. Tidak dapat dicapai lewat antarmuka (browser membatasi 10 karakter). Lihat KI-41.

### TC-C12 — Tanpa izin kelola cabang **[dari kode]**

```
Sesi    : peran admin (punya branch.view, tanpa branch.manage)
```
Diharapkan: **403**. Di layar, tombol tambah memang tidak pernah muncul; mengetik
`/branches/create` langsung mengalihkan ke halaman **403**.

---

## 3. `branches/update`

### TC-U01 — Mengubah nama ✅ *ada di spec*

```
Request : { data: { id_branch: 2, name: "Jakarta Pusat" } }
```
Diharapkan: nama berubah; audit **`branch.update`** tercatat dengan nilai sebelum dan sesudah
(kode, nama, kota, status, label lokasi stok).

### TC-U02 — Mengubah kode menulis ulang kelima prefix, nilai berjalan dipertahankan ✅ *ada di spec*

```
Data    : cabang #2 berkode JKT
          penghitung order  → prefix ORD-JKT, nilai berjalan 7
          penghitung sj     → prefix SJ-JKT,  nilai berjalan 3
          penghitung payment/sales_return/purchase_return → belum ada
Request : { data: { id_branch: 2, code: "sby" } }
```
Diharapkan — tepat 5 penghitung tersimpan:

| Penghitung | Prefix sesudah | Nilai berjalan sesudah |
|---|---|---|
| `order` | `ORD-SBY` | **7** (tidak direset) |
| `payment` | `PAY-SBY` | 0 (baru dibuat) |
| `sj` | `SJ-SBY` | **3** (tidak direset) |
| `sales_return` | `RTR-SBY` | 0 (baru dibuat) |
| `purchase_return` | `RTB-SBY` | 0 (baru dibuat) |

Ini sekaligus membuktikan mekanisme "lengkapi penghitung yang hilang" pada cabang lama.

### TC-U03 — Cabang tidak ditemukan ✅ *ada di spec*

```
Request : { data: { id_branch: 999 } }
```
Diharapkan: **404** **`Cabang tidak ditemukan`**.

### TC-U04 — Kode baru dipakai cabang lain ✅ *ada di spec*

```
Data    : cabang #2 berkode JKT, cabang #99 berkode SBY
Request : { data: { id_branch: 2, code: "SBY" } }
```
Diharapkan: **409** **`Kode cabang 'SBY' sudah digunakan`** — memakai kode versi **huruf besar**,
berbeda dari alur tambah yang memakai teks apa adanya.

### TC-U05 — Mengirim kode yang sama tidak memicu pemeriksaan duplikat ✅ *ada di spec*

```
Data    : cabang #2 berkode JKT
Request : { data: { id_branch: 2, code: "jkt" } }
```
Diharapkan: setelah dijadikan huruf besar nilainya sama dengan yang sekarang → pemeriksaan duplikat
**dilewati sepenuhnya** (hanya satu pencarian cabang yang terjadi). Penyimpanan berhasil.

### TC-U06 — Cabang milik perusahaan lain **[dari kode]**

```
Sesi    : perusahaan #1
Request : { data: { id_branch: <milik perusahaan #2> } }
```
Diharapkan: **404 `Cabang tidak ditemukan`** — bukan 403. Keberadaan cabang perusahaan lain tidak
bocor.

### TC-U07 — Field yang tidak dikirim tidak berubah **[dari kode]**

```
Request : { data: { id_branch: 2, name: "Nama Baru" } }
```
Diharapkan: kota, alamat, telepon, label lokasi stok, dan status **tidak tersentuh**. Inilah yang
membuat nomor telepon cabang tetap aman meski form web tidak pernah mengirimkannya.

### TC-U08 — Alamat dikirim kosong **[dari kode]**

```
Request : { data: { id_branch: 2, address: "" } }
```
Diharapkan: alamat **dikosongkan**. Mengirim nilai kosong berbeda dengan tidak mengirim sama
sekali.

### TC-U09 — Nama tidak dipangkas saat ubah **[dari kode]**

```
Request : { data: { id_branch: 2, name: "  Jakarta  " } }
```
Diharapkan **saat ini**: tersimpan **apa adanya beserta spasinya** — berbeda dari alur tambah yang
memangkas. Efeknya kosmetik: nama tampil dengan spasi ekstra di daftar dan kop nota.

### TC-U10 — Menyimpan cabang non-aktif dari form mengaktifkannya kembali **[dari kode]** ⚠️

```
Data    : cabang #2 berstatus "inactive" (diubah lewat basis data)
Aksi    : buka form edit, ubah kota saja, klik Simpan Perubahan
```
Diharapkan **saat ini**: form mengirim `status: "active"` → cabang **kembali aktif**, dan lokasi
stok defaultnya juga kembali aktif. Tidak ada peringatan. Lihat KI-36.

### TC-U11 — Nama lokasi stok yang diubah dari menu Stok tertimpa **[dari kode]** ⚠️

```
Data    : lokasi default cabang #2 diganti namanya jadi "Gudang Depan" dari menu Stok
Aksi    : buka form edit cabang, ubah kota saja, simpan
```
Diharapkan **saat ini**: nama lokasi default **kembali** ke label lokasi stok yang tersimpan di
cabang, dan kodenya kembali ke `GDG-<kode>`. Perubahan dari menu Stok hilang tanpa peringatan.
Lihat KI-42.

### TC-U12 — Mengubah kode ke nilai yang bentrok dengan kode gudang **[dari kode]** ⚠️

```
Data    : cabang #2 berkode JKT; pengguna sudah membuat lokasi manual berkode GDG-SBY
Request : { data: { id_branch: 2, code: "SBY" } }
```
Diharapkan **saat ini**: pemeriksaan kode cabang lolos (tidak ada cabang berkode `SBY`), lalu
penulisan kode lokasi default menjadi `GDG-SBY` bentrok dengan lokasi manual → **500 galat
internal**, seluruh perubahan dibatalkan. Lihat KI-41.

### TC-U13 — Lokasi default yang sudah diarsipkan **[dari kode]** ⚠️

```
Data    : lokasi default cabang #2 diarsipkan dari menu Stok
Aksi    : simpan form edit cabang
```
Diharapkan **saat ini**: baris arsip itu yang diperbarui (kode/nama/status), **tanpa dibatalkan
arsipnya** dan **tanpa membuat lokasi default baru**. Cabang tetap tanpa lokasi default yang hidup.

---

## 4. `branches/my-access`

### TC-M01 — Hanya cabang berstatus aktif **[dari kode]**

```
Data    : pengguna punya akses ke BLR (aktif) dan SBY (inactive)
```
Diharapkan: hanya **BLR** yang dikembalikan. Halaman "Select Branch" karena itu tidak pernah
menawarkan cabang yang sudah ditutup — pada jalur login.

### TC-M02 — Bentuk data yang dikembalikan **[dari kode]**

Diharapkan tiap baris memuat: nomor cabang, kode, nama, kota, label lokasi stok, dan penanda
**cabang default pengguna**. Penanda inilah yang menghasilkan badge **"Default"** di kartu
pemilihan cabang.

### TC-M03 — Pengguna tanpa akses cabang **[dari kode]**

Diharapkan: daftar **kosong**. Halaman pemilihan cabang tampil tanpa kartu dan tanpa tombol keluar
— jalan buntu yang sudah tercatat sebagai KI-01.

### TC-M04 — Tidak butuh izin cabang **[dari kode]**

```
Sesi    : peran kasir (tanpa branch.view)
```
Diharapkan: **berhasil**. Inilah sebabnya kasir tetap bisa login dan memilih cabang meski tidak
bisa membuka halaman `/branches`.

### TC-M05 — Perbedaan dengan pemulihan sesi **[dari kode]**

Diharapkan: setelah **menyegarkan halaman**, daftar cabang di halaman pemilihan berasal dari data
sesi (`auth/me`) yang **tidak** menyaring status — sehingga cabang non-aktif bisa muncul, berbeda
dari jalur login. Sudah tercatat sebagai KI-10.

---

## 5. `branches/active`

### TC-A01 — Sesi tanpa cabang aktif **[dari kode]**

Diharapkan: mengembalikan **kosong**, bukan galat.

### TC-A02 — Cabang aktif berstatus non-aktif **[dari kode]**

Diharapkan: mengembalikan **kosong**. Sesi bisa saja menunjuk cabang yang sudah ditutup, dan
endpoint ini tidak menceritakan apa pun tentangnya.

### TC-A03 — Tidak ada pemanggil **[dari kode]**

Diharapkan: pencarian seluruh kode frontend & E2E menemukan **nol** pemanggil. Data cabang aktif
selalu diambil dari sesi lokal. Lihat KI-49.

---

## 6. Skenario E2E — Cabang baru langsung beroperasi

Diambil dari `04-master-data-branch-gudang.spec.ts`. Ini pembuktian end-to-end paling penting untuk
modul ini: **satu cabang yang baru dibuat harus bisa menjalankan siklus jual penuh tanpa setup
tambahan.**

### TC-E01 — Rantai lengkap cabang baru ✅ *ada di E2E*

| # | Langkah | Yang diharapkan |
|---|---|---|
| 1 | Buat cabang baru lewat `branches/create` (kode unik, nama, kota Jakarta, telepon, label lokasi stok `Lokasi Awal`, status aktif) | Cabang tersimpan |
| 2 | Beri akses cabang itu ke pengguna yang sedang login | Akses tercatat |
| 3 | Ganti cabang aktif ke cabang baru | Sesi lokal di browser memuat **nama cabang baru** |
| 4 | Buat lokasi `Lantai 1`, lalu `Rak A` sebagai anaknya, lalu `Lantai 2 Area Tanpa Rak` tanpa anak | Ketiganya tersimpan di cabang baru |
| 5 | Tambah stok 8 unit ke `Rak A` | Berhasil — lokasi berkode anak tetap daun |
| 6 | Tambah stok 5 unit ke `Lantai 2 Area Tanpa Rak` | Berhasil — lokasi tanpa anak tetap dianggap daun meski berada di tingkat atas |
| 7 | Coba tambah stok ke `Lantai 1` (punya anak) | **Ditolak**, pesan galat memuat salah satu kata: *leaf*, *anak*, *turunan*, atau *lokasi* |
| 8 | Buat order penjualan berisi 1 produk fisik (2 unit) dan 1 produk jasa | Order terbit — **membuktikan penghitung nomor order cabang baru aktif** |
| 9 | Buat surat jalan, alokasikan barang fisik dari `Rak A` | Surat jalan terbit — **membuktikan penghitung nomor surat jalan aktif** |
| 10 | Periksa sisa stok tersedia di `Rak A` | Menjadi **6** (8 − 2) — stok berkurang saat surat jalan dibuat |
| 11 | Kembali ke cabang Blora | Sesi kembali ke cabang semula |

**Kontrak yang dikunci skenario ini** dan wajib bertahan di sistem baru:

1. Cabang baru **langsung** punya lokasi stok yang bisa diisi.
2. Cabang baru **langsung** bisa menerbitkan nomor order dan nomor surat jalan.
3. Produk jasa **tidak** menuntut alokasi lokasi stok, produk fisik menuntutnya.
4. Aturan "hanya lokasi daun yang boleh menerima mutasi" berlaku sejak cabang pertama kali dipakai.
5. Berganti cabang aktif memuat ulang konteks kerja sepenuhnya.

---

## 7. Kasus yang Belum Punya Test

Daftar ini untuk rebuild — perilaku yang nyata di kode tapi tidak dijaga satu pun test:

| Perilaku | Kenapa perlu dijaga |
|---|---|
| Alur ubah pada cabang milik perusahaan lain (TC-U06) | Penjagaan lintas perusahaan |
| Cabang non-aktif tetap muncul di daftar (TC-L03) | Perilaku yang terlihat pengguna |
| Menyimpan form mengaktifkan kembali cabang non-aktif (TC-U10) | Efek samping yang merusak niat pengguna |
| Nama lokasi stok tertimpa (TC-U11) | Kehilangan data yang dilakukan pengguna |
| Kode dengan spasi di depan (TC-C07) | Jalur galat teknis yang bisa dijadikan pesan bisnis |
| Bentrok kode gudang saat ubah kode (TC-U12) | Jalur galat teknis |
| Nomor darurat saat penghitung hilang | Lihat [numbering-sequence.md](numbering-sequence.md) §5 |
| Halaman edit dengan id tak dikenal berubah jadi tambah | Lihat KI-44 |
