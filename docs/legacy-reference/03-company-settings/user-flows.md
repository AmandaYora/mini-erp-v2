# User Flows — Modul 03 Company & Settings

**Kelompok A — alur end-to-end yang harus identik di sistem baru.** Nama endpoint, teks UI, dan
urutan langkah ditulis apa adanya dari kode.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [ui-ux-spec.md](ui-ux-spec.md) ·
[business-rules.md](business-rules.md) · [test-cases.md](test-cases.md)

Notasi: `→` langkah berikutnya · **[server]** panggilan API · **[lokal]** hanya di browser

---

## UF-01 — Membuka halaman Profil Perusahaan

1. Pengguna membuka menu **Pengaturan → "Profil Perusahaan"** (atau URL `/settings`)
2. Penjaga rute memeriksa `company_config.view`; bila tidak punya → `/403` "Akses Ditolak"
3. **[server]** Hook modul memuat **tiga** hal paralel saat modul pertama kali dikunjungi:
   `users/list`, `roles/list`, dan status order (`order-status-definitions/list` +
   `order-status-transitions/list`)
   - Ketiganya dimuat **meski halaman Profil Perusahaan tidak memakai satu pun** — hook yang sama
     melayani tiga halaman pengaturan sekaligus
4. Halaman merender **kosong** sampai data perusahaan dan pengaturan tersedia (tidak ada indikator
   memuat)
5. Setelah tersedia, form terisi dari:
   - **Profil** — dari ringkasan sesi di browser, bukan dari `company/profile/get`
   - **Pengaturan** — dari data pendamping perusahaan yang dimuat saat bootstrap
6. Bila pengguna **tidak** punya `company_config.manage`, seluruh field dinonaktifkan dan tombol
   simpan tidak dirender; teks penjelas berubah menjadi
   **"Anda sedang melihat pengaturan dalam mode baca saja."**

**Yang perlu diketahui:** halaman ini **tidak pernah memanggil `company/profile/get` maupun
`company/settings/get`.** Nilai awal form seluruhnya berasal dari data yang sudah ada di browser
(ringkasan sesi + bootstrap). Kedua endpoint itu hanya dipakai dari luar UI — lihat
[reports-list.md](reports-list.md) §3.

---

## UF-02 — Mengubah identitas perusahaan

1. Pengguna berada di `/settings` dengan izin `company_config.manage`
2. Ia mengubah satu atau beberapa field: Kode perusahaan → Nama perusahaan → Nama legal → Zona
   waktu → Mata uang → Bahasa sistem
   - Mengetik di **Mata uang** langsung mengubah huruf menjadi kapital
3. Ia menekan **"Simpan Pengaturan"**
4. **[lokal]** Validasi browser untuk field bertanda wajib (kode, nama, zona waktu, mata uang,
   bahasa sistem)
5. **[server]** `company/profile/update` dengan `code`, `name`, `legal_name`, `timezone`,
   `currency_code`, `locale`
   - Mata uang dikirim setelah diubah ke huruf besar **lagi** di sisi pengirim
6. Server: cari perusahaan → potong spasi setiap nilai → tolak yang wajib bila kosong → periksa
   keunikan kode bila berubah → simpan
7. Server mencatat audit `company.profile.update` dengan **seluruh 6 field** di `before` dan
   `after`
8. **[lokal]** Ringkasan sesi di browser diperbarui (id, nama, kode, nama legal, zona waktu, mata
   uang, locale) dan disimpan ulang
9. Toast success **"Profil perusahaan disimpan"** / "Identitas perusahaan berhasil diperbarui."
10. **[lokal]** Chip **"Perusahaan"** di topbar langsung menampilkan nama baru — tanpa reload
11. **[server]** Langkah 12 UF-03 berjalan **setelah** ini (dalam satu submit yang sama)

Bila langkah 5–6 gagal: toast danger **"Gagal menyimpan profil perusahaan"** tanpa alasan, dan
**proses berhenti** — pengaturan (UF-03) **tidak ikut disimpan**.

---

## UF-03 — Mengubah pengaturan operasional & dokumen

Alur ini **selalu berjalan bersama UF-02** dalam satu penekanan tombol — tidak ada tombol simpan
terpisah untuk pengaturan.

1. Pengguna mengubah satu atau beberapa dari:
   - Mode persetujuan koreksi stok
   - Kuota AI harian
   - Identitas dokumen cetak (nama kepala, alamat, tagline, kota, telepon, rekening, daftar jual,
     catatan, ucapan penutup)
   - Kalibrasi kertas kontinu (6 angka + 2 checkbox)
2. Ia menekan **"Simpan Pengaturan"**
3. **[lokal]** Sistem menyusun objek pengaturan baru dengan cara **menggabung** ke pengaturan yang
   ada:
   - `operationalRules` — seluruh isi lama **+** `stockAdjustmentApprovalMode` yang baru
   - `uiPreferences` — seluruh isi lama **+** `documentProfile` hasil pembersihan
   - `aiPreferences` — seluruh isi lama **+** `ai_daily_limit` yang sudah dinormalisasi
   - `businessLabels` dan `reportingPreferences` diteruskan **apa adanya**
4. **[lokal]** Profil dokumen dibersihkan: setiap teks di-trim, nilai kosong dibuang, baris telepon
   tanpa nomor dibuang, baris rekening tanpa nomor dibuang, daftar jual dipecah per baris dengan
   baris kosong dibuang
5. **[server]** UF-02 dijalankan lebih dulu dan **ditunggu**
6. **[server]** `company/settings/update` dikirim dengan **kelima blok sekaligus** —
   `business_labels`, `operational_rules`, `ui_preferences`, `ai_preferences`,
   `reporting_preferences` — **tanpa ditunggu** (fire-and-forget)
7. Server: cari baris pengaturan; **buat baru bila belum ada**; timpa setiap blok yang dikirim
8. Server mencatat audit `company.settings.update` dengan kelima blok di `before` dan `after`
9. **[lokal]** Data pengaturan di browser diperbarui
10. **[lokal]** Kotak kuota AI diisi ulang dengan angka hasil normalisasi
11. **Tidak ada toast sukses** untuk bagian pengaturan

Bila langkah 6–7 gagal: toast danger **"Gagal menyimpan pengaturan"** tanpa alasan. Karena
langkah 5 sudah berhasil, **profil tersimpan tetapi pengaturan tidak** — dan pengguna melihat
toast sukses **dan** toast gagal bersamaan.

---

## UF-04 — Mengatur mode persetujuan koreksi stok

Dipisahkan karena ini **satu-satunya pengaturan yang punya efek terverifikasi** di modul lain.

1. Pengguna memilih **"Harus orang berbeda dari yang membuat koreksi"** pada dropdown
2. Ia menekan **"Simpan Pengaturan"** → alur UF-03 berjalan
3. Nilai `strict` tersimpan di blok aturan operasional
4. **Efek di modul Stock:** pada setiap koreksi stok berikutnya, modul Stock membaca ulang nilai
   ini dari basis data dan menolak persetujuan yang dilakukan oleh **orang yang sama** dengan
   pembuat koreksi
5. Pesan penolakan yang muncul menyebut keharusan approver berbeda

Untuk kembali ke mode longgar, pengguna memilih **"Siapapun yang punya izin persetujuan"** dan
menyimpan lagi.

**Yang perlu diketahui:** nilai ini dibaca **per operasi**, bukan di-cache. Jadi perubahan berlaku
seketika untuk koreksi stok berikutnya, tanpa perlu siapa pun login ulang.

---

## UF-05 — Mengatur identitas dokumen cetak

1. Pengguna berada di kartu **"Identitas Dokumen Cetak"**
2. Ia mengisi nama kepala dokumen — atau **membiarkannya kosong** agar memakai nama perusahaan
   (placeholder menunjukkan nama mana yang akan dipakai)
3. Ia mengisi alamat — atau membiarkan kosong agar memakai alamat cabang
4. Ia menekan **"Tambah telepon"** → satu baris kosong muncul → ia mengisi label ("TOKO") dan nomor
5. Ia menekan **"Tambah rekening"** → satu baris kosong muncul → ia mengisi bank, atas nama, nomor
6. Ia mengisi daftar barang yang dijual, **satu baris satu item**
7. Ia mengisi catatan nota dan ucapan penutup struk
8. Ia menekan **"Simpan Pengaturan"** → alur UF-03 berjalan
9. **Efek di modul Order:** nota, struk, dan surat jalan yang dicetak setelah ini memakai identitas
   baru

Baris telepon/rekening yang **tidak diisi nomornya** dibuang saat simpan — jadi menekan "Tambah
telepon" lalu tidak mengisinya tidak meninggalkan baris kosong di data.

Menekan **"Hapus"** pada satu baris hanya menghapusnya **di form**; perubahan baru tersimpan
setelah "Simpan Pengaturan" ditekan.

---

## UF-06 — Mengkalibrasi kertas kontinu untuk printer dot-matrix

Alur ini bersifat coba-dan-ukur, dan urutan itu bagian dari kontraknya.

1. Pengguna membuka kartu **"Kertas Kontinu Nota (Dot-Matrix)"**
2. Deskripsi kartu menampilkan ukuran default yang sedang berlaku (241.3mm × 139.7mm)
3. Setiap kotak menampilkan angka default sebagai **placeholder** — kotak yang kosong berarti
   "pakai default"
4. Pengguna mencentang **"Jadikan kertas kontinu sebagai default saat cetak Nota"** agar halaman
   cetak Nota otomatis memakai mode ini
5. Bila kertasnya sudah tercetak kop dari percetakan, ia mencentang
   **"Kertas sudah preprinted (kop, rekening, daftar jual) — jangan cetak ulang"**
6. Ia menekan **"Simpan Pengaturan"**
7. Ia mencetak satu Nota nyata dan mengukur hasilnya
8. Bila hasil bergeser, ia kembali ke halaman ini dan menyesuaikan margin, lalu mengulang langkah
   6–7

Lebar dan tinggi **wajib lebih dari 0** bila diisi (kertas 0mm tidak masuk akal); margin **boleh
0** (dipakai saat cetak mentok ke tepi). Nilai yang tidak sah diabaikan dan default yang berlaku.

Latar belakang angka default — termasuk mengapa margin kanan 37.91mm bukan margin visual —
terdokumentasi di [shared-data-model.md](../shared/shared-data-model.md) §7.3 dan **wajib**
dipertahankan.

---

## UF-07 — Membuka halaman Status Order

1. Pengguna membuka menu **Pengaturan → "Status Order"** (atau URL `/settings/order-status`)
2. Penjaga rute memeriksa **`company_config.manage`** — bukan `.view`
   - Pemegang izin baca-saja mendapat `/403`, meski halaman ini separuhnya hanya daftar
3. **[server]** Hook modul memuat status order bila belum termuat
4. Halaman merender **kosong** sampai data pendamping perusahaan tersedia
5. Daftar status tampil **diurutkan berdasarkan urutan tampil**; daftar aturan perpindahan tampil
   **apa adanya dari server**

---

## UF-08 — Menambah status order baru

1. Pengguna mengisi **"Nama status"** (mis. "Menunggu Konfirmasi")
2. Ia mengisi **"Kode unik"** (mis. `waiting_confirmation`)
3. **[lokal]** Bila kode itu sudah dipakai, muncul teks merah
   **"Kode ini sudah digunakan. Gunakan kode lain."** dan tombol submit dinonaktifkan
4. Ia menekan **"Tambah Status"**
5. **[lokal]** Sistem menyusun payload dengan **nilai tetap** yang tidak bisa diubah pengguna:
   - Kelompok status: **`pending`**
   - Berlaku untuk: **semua** jenis order
   - Bukan titik awal, bukan titik akhir
   - Urutan tampil: **jumlah status saat ini + 1**
   - Warna: **`#12343B`**
6. **[server]** `order-status-definitions/create` (endpoint milik modul Order)
7. **[server]** Daftar status dan aturan perpindahan dimuat ulang
8. **[lokal]** Kedua field dikosongkan
9. Status baru muncul di daftar dengan badge **"Menunggu"** — **tanpa toast sukses**

Bila gagal: toast danger **"Gagal menyimpan definisi status"** tanpa alasan.

**Batasan yang perlu diketahui:** pengguna **tidak dapat** memilih kelompok status, warna, urutan,
atau menandai status sebagai titik awal/akhir. Semua status baru lahir sebagai "Menunggu" biasa.
Untuk mengubahnya diperlukan intervensi di basis data — halaman ini tidak punya fungsi ubah sama
sekali.

---

## UF-09 — Menambah aturan perpindahan status

1. Pengguna memilih **"Dari status"** dari dropdown pencarian
2. Ia memilih **"Ke status"**
3. Ia mengisi **"Label tombol aksi"** (mis. "Konfirmasi") — inilah teks yang nanti muncul sebagai
   tombol di halaman order
4. Ia menekan **"Tambah Aturan"**
5. **[server]** `order-status-transitions/create` dengan id status asal, id status tujuan, label,
   dan penanda aktif
6. **[server]** Data dimuat ulang
7. **[lokal]** **Hanya** label yang dikosongkan — pilihan "dari" dan "ke" tetap terisi
8. Aturan baru muncul di daftar sebagai `{label}` dengan baris `{status asal} → {status tujuan}`

Langkah 7 memudahkan menambah beberapa aturan dari status yang sama secara berurutan — tampaknya
disengaja.

**Yang tidak dijaga di klien:**

| Kondisi | Perilaku |
|---|---|
| Status asal/tujuan belum dipilih | Tombol **tetap aktif**; nilai kosong dikirim sebagai angka `0` → gagal di server |
| Status asal sama dengan status tujuan | Dikirim apa adanya |
| Aturan yang sama sudah ada | Dikirim apa adanya |

Bila gagal: toast danger **"Gagal menyimpan transisi status"** tanpa alasan.

---

## UF-10 — Mengelola feature flag (hanya lewat API)

Tidak ada UI untuk alur ini. Dicatat karena endpointnya ada dan berfungsi.

1. **[server]** `company/features/list` mengembalikan seluruh feature flag perusahaan
2. **[server]** `company/features/update` dengan `feature_key`, `enabled`, dan `config` opsional
3. Server: cari baris dengan kunci itu; **buat baru bila belum ada** — tidak ada daftar kunci yang
   sah, sehingga kunci apa pun bisa dibuat
4. Server mencatat audit `company.feature.update`

Tiga kunci di-seed dalam keadaan mati: `whatsapp_assistant`, `knowledge_rag`,
`multi_location_stock`. **Tidak ada kode yang membaca nilainya**, sehingga menyalakan atau
mematikannya tidak mengubah perilaku apa pun.

---

## UF-11 — Perilaku setelah seed dijalankan ulang

Relevan untuk lingkungan dev dan test, dan berbeda perilakunya antara profil dan pengaturan.

| Data | Perilaku seed | Akibat |
|---|---|---|
| **Profil perusahaan** | "timpa bila sudah ada" | Kode, nama, nama legal, zona waktu, mata uang, locale, dan status **kembali ke nilai bawaan** — perubahan lewat UI hilang |
| **Pengaturan** | "sisipkan bila belum ada" | Pengaturan yang sudah ada **tidak disentuh** |
| **Feature flags** | "sisipkan bila belum ada" | Tidak disentuh |

Asimetri ini perlu diketahui: menjalankan seed akan mengembalikan nama perusahaan ke
"TB. SUMBER ABADI KAMOLAN" tetapi **tidak** mengembalikan identitas dokumen cetak atau mode
persetujuan koreksi stok.

---

## 12. Matriks Permission per Aksi

| Aksi | Permission | Endpoint |
|---|---|---|
| Membuka halaman Profil Perusahaan | `company_config.view` | *(tidak memanggil endpoint)* |
| Membaca profil perusahaan | `company_config.view` | `company/profile/get` |
| Mengubah profil perusahaan | `company_config.manage` | `company/profile/update` |
| Membaca pengaturan | `company_config.view` | `company/settings/get` |
| Mengubah pengaturan | `company_config.manage` | `company/settings/update` |
| Membaca feature flags | `company_config.view` | `company/features/list` |
| Mengubah feature flags | `company_config.manage` | `company/features/update` |
| Membuka halaman Status Order | **`company_config.manage`** | — |
| Menambah status / aturan perpindahan | *(permission modul Order)* | `order-status-definitions/create`, `order-status-transitions/create` |

### Kemampuan per role bawaan

| Role | `company_config.view` | `company_config.manage` | Akibatnya |
|---|---|---|---|
| `superadmin` | ✔ | ✔ | Akses penuh kedua halaman |
| `owner` | ✔ | ✔ | Akses penuh kedua halaman |
| `admin` | **✔** | **✘** | Profil Perusahaan **baca-saja**; halaman Status Order **tidak bisa dibuka** |
| `staff` | ✘ | ✘ | Kedua menu tidak muncul |
| `kasir` | ✘ | ✘ | Kedua menu tidak muncul |

Baris `admin` adalah yang paling perlu diperhatikan: ia melihat menu "Profil Perusahaan" dan bisa
membukanya dalam mode baca-saja, tetapi menu "Status Order" **juga muncul** di sidebar (karena
sidebar menyaring berdasarkan permission rute) — sehingga `admin` **tidak** melihat menu Status
Order sama sekali. Ini konsisten, bukan bug: penyaringan menu dan penjagaan rute memakai permission
yang sama.
