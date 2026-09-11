# Test Cases — Modul 03 Company & Settings

**Kelompok A — skenario input→output nyata, diturunkan dari kode dan test yang sudah ada.**

Sumber: `company.service.spec.ts` (**19 kasus**), `use-company-module.test.ts` (5),
`01-sidebar-and-menu-smoke.spec.ts` (2 E2E), `08-inventory-return-transfer-adjustment.spec.ts`
(1 E2E), plus kasus turunan pembacaan kode (ditandai **[dari kode]**).

Data bawaan: lihat [feature-inventory.md](feature-inventory.md) §5.

---

## 1. `company/profile/get`

### TC-PG01 — Mengembalikan profil perusahaan ✅ *ada di spec*

Diharapkan: objek perusahaan lengkap (kode, nama, nama legal, zona waktu, mata uang, locale,
status).

### TC-PG02 — Perusahaan tidak ada ✅ *ada di spec*

Diharapkan: **404** **"Perusahaan tidak ditemukan"**.

### TC-PG03 — Tidak pernah dipanggil UI **[dari kode]**

Diharapkan: pencarian seluruh kode frontend & E2E menemukan **nol** pemanggil. Halaman Profil
Perusahaan mengisi formnya dari ringkasan sesi, bukan dari endpoint ini.

---

## 2. `company/profile/update`

### TC-PU01 — Mengubah identitas dan menulis audit ✅ *ada di spec*

```
Request : { data: { code: "TSAK2", name: "Nama Baru", timezone: "Asia/Makassar",
                    currency_code: "idr", locale: "en-US" } }
```
Diharapkan:
- Nilai tersimpan; `currency_code` menjadi **`IDR`** (huruf besar)
- Audit `company.profile.update` tercatat dengan **keenam field** di `before` dan `after`

### TC-PU02 — Menolak nilai wajib yang kosong ✅ *ada di spec*

Diharapkan: **400** dengan pesan sesuai field:

| Field kosong | Pesan |
|---|---|
| `code` | **"Kode perusahaan wajib diisi"** |
| `name` | **"Nama perusahaan wajib diisi"** |
| `timezone` | **"Zona waktu wajib diisi"** |
| `currency_code` | **"Mata uang wajib diisi"** |
| `locale` | **"Bahasa sistem wajib diisi"** |

### TC-PU03 — Menolak kode perusahaan duplikat ✅ *ada di spec*

Diharapkan: **409** **"Kode perusahaan sudah digunakan"**.

### TC-PU04 — Nilai hanya spasi dianggap kosong **[dari kode]**

```
Request : { data: { name: "   " } }
```
Diharapkan: **400** "Nama perusahaan wajib diisi" — nilai di-trim lebih dulu.

### TC-PU05 — Nilai non-string dianggap kosong **[dari kode]**

```
Request : { data: { name: 123 } }
```
Diharapkan: **400** "Nama perusahaan wajib diisi" — bukan pesan "tipe salah".

### TC-PU06 — Kode diubah ke nilai yang sama **[dari kode]**

```
Request : { data: { code: "TSAK" } }   // nilai saat ini juga "TSAK"
```
Diharapkan: **berhasil** — pemeriksaan keunikan dilewati karena nilai tidak berubah.

### TC-PU07 — Nama legal dikosongkan **[dari kode]**

```
Request : { data: { legal_name: "" } }
```
Diharapkan: tersimpan **`null`**.

```
Request : { data: { legal_name: null } }
```
Diharapkan: tersimpan **`null`**.

### TC-PU08 — Field tidak dikirim tidak berubah **[dari kode]**

```
Request : { data: { name: "Nama Baru" } }
```
Diharapkan: kode, nama legal, zona waktu, mata uang, locale **tidak tersentuh**.

### TC-PU09 — Zona waktu tidak divalidasi **[dari kode]**

```
Request : { data: { timezone: "Mars/Olympus" } }
```
Diharapkan (perilaku **aktual**): **berhasil tersimpan**. Tidak ada validasi zona IANA, dan
nilainya tidak berpengaruh pada tampilan apa pun. Menguji BR-19 & KI-28.

### TC-PU10 — Locale tidak divalidasi **[dari kode]**

```
Request : { data: { locale: "xx-YY" } }
```
Diharapkan: **berhasil tersimpan**, tanpa efek apa pun pada format tanggal/angka.

### TC-PU11 — Mata uang tidak divalidasi terhadap ISO 4217 **[dari kode]**

```
Request : { data: { currency_code: "abcde" } }
```
Diharapkan: tersimpan sebagai **`ABCDE`**.

### TC-PU12 — Nilai melebihi panjang kolom **[dari kode]**

```
Request : { data: { code: "<51 karakter>" } }
```
Diharapkan: **500** dari basis data, bukan pesan bisnis.

### TC-PU13 — Kode dengan huruf besar/kecil berbeda **[dari kode]**

Data: perusahaan lain berkode `TSAK`.
```
Request : { data: { code: "tsak" } }
```
Diharapkan (perilaku **aktual**): pemeriksaan aplikasi **meloloskannya** (membandingkan string apa
adanya), tetapi indeks unik basis data (`utf8mb4_unicode_ci`, tidak peka huruf) **menolaknya** →
**500** bukan `409`. Menguji ketidaksesuaian di
[numbering-sequence.md](numbering-sequence.md) §2.3. **Belum ada test.**

---

## 3. `company/settings/get`

### TC-SG01 — Mengembalikan pengaturan bila ada ✅ *ada di spec*

Diharapkan: objek pengaturan dengan kelima blok.

### TC-SG02 — Mengembalikan `null` bila belum ada ✅ *ada di spec*

Diharapkan: **`null`** — bukan `404`, bukan objek default. Klien yang memakai bentuk bawaannya
sendiri.

### TC-SG03 — Dipakai E2E untuk baca-lalu-pulihkan ✅ *ada di E2E*

Pola yang dipakai test modul Stock:
```
1. settings = company/settings/get
2. originalRules = settings.operationalRules ?? settings.operational_rules ?? {}
3. ... ubah dan uji ...
4. finally: company/settings/update dengan operational_rules: originalRules
```
Menguji BR-14: pemanggil **wajib** membaca-lalu-menyebarkan agar tidak menghapus kunci lain.
Perhatikan langkah 2 menerima **dua ejaan** nama blok.

---

## 4. `company/settings/update`

### TC-SU01 — Memperbarui baris yang sudah ada ✅ *ada di spec*

Diharapkan: nilai tersimpan; audit `company.settings.update` tercatat dengan kelima blok di
`before` dan `after`.

### TC-SU02 — Membuat baris baru bila belum ada ✅ *ada di spec*

Diharapkan: baris pengaturan **dibuat** lalu diisi — tidak melempar `404`.

### TC-SU03 — Tidak menulis audit bila tanpa sesi ✅ *ada di spec*

Diharapkan: pengaturan tersimpan, tetapi audit **tidak** dipanggil. Menguji BR-16.

### TC-SU04 — Memperbarui aturan operasional ✅ *ada di spec*

```
Request : { data: { operational_rules: { stockAdjustmentApprovalMode: "strict" } } }
```
Diharapkan: blok aturan operasional tersimpan berisi nilai itu.

### TC-SU05 — Blok yang tidak dikirim tidak diubah ✅ *ada di spec*

```
Request : { data: { ui_preferences: { ... } } }
```
Diharapkan: keempat blok lain **tidak tersentuh**.

### TC-SU06 — Blok yang dikirim ditimpa SELURUHNYA **[dari kode]**

Data awal: `operationalRules = { stockAdjustmentApprovalMode: "strict", defaultDueDays: 14 }`.
```
Request : { data: { operational_rules: { stockAdjustmentApprovalMode: "simple" } } }
```
Diharapkan (perilaku **aktual**): hasil akhir **hanya** `{ stockAdjustmentApprovalMode: "simple" }`
— `defaultDueDays` **hilang**. Tidak ada penggabungan per-kunci di server. Menguji BR-14 & KI-30.
**Belum ada test.**

### TC-SU07 — Blok dikirim sebagai objek kosong **[dari kode]**

```
Request : { data: { operational_rules: {} } }
```
Diharapkan: seluruh isi blok itu **hilang**; tidak ada penjagaan.

### TC-SU08 — Nama payload `ai_preferences` menyimpan ke kolom asisten **[dari kode]**

```
Request : { data: { ai_preferences: { ai_daily_limit: 100 } } }
```
Diharapkan: tersimpan di kolom **`assistant_preferences_json`**, dan saat dibaca kembali muncul
sebagai `assistantPreferences`. Menguji ketidakcocokan nama di BR-04.

### TC-SU09 — Tanpa validasi struktur **[dari kode]**

```
Request : { data: { operational_rules: { stockAdjustmentApprovalMode: 12345 } } }
```
Diharapkan: **tersimpan apa adanya**. Yang menjaga adalah normalisasi di sisi pembaca — modul
Stock akan membacanya sebagai `simple` karena nilainya bukan `'strict'`.

### TC-SU10 — Kuota AI di luar rentang lewat API langsung **[dari kode]**

```
Request : { data: { ai_preferences: { ai_daily_limit: -50 } } }
```
Diharapkan: **tersimpan sebagai -50**. Normalisasi ke rentang 1–500 hanya ada di klien.
Menguji BR-08 & SK-09. **Belum ada test.**

---

## 5. `company/features/list`

### TC-FL01 — Mengembalikan seluruh feature flag perusahaan ✅ *ada di spec*

Diharapkan: array baris flag dengan `featureKey`, `enabled`, `config`.

### TC-FL02 — Array kosong bila belum ada ✅ *ada di spec*

Diharapkan: `[]`.

### TC-FL03 — Tidak pernah dipanggil UI **[dari kode]**

Diharapkan: pencarian seluruh kode menemukan **nol** pemanggil. Diperkuat oleh fakta bahwa adapter
frontend **selalu memaksa** daftar flag menjadi kosong. Menguji NF-02 & KI-33.

---

## 6. `company/features/update`

### TC-FU01 — Menyalakan flag yang sudah ada ✅ *ada di spec*

```
Request : { data: { feature_key: "whatsapp_assistant", enabled: true } }
```
Diharapkan: `enabled` menjadi `true`; audit `company.feature.update` tercatat.

### TC-FU02 — Membuat baris baru bila kunci belum ada ✅ *ada di spec*

Diharapkan: baris **dibuat** dengan kunci itu — tidak melempar `404`.

### TC-FU03 — Memperbarui konfigurasi flag ✅ *ada di spec*

```
Request : { data: { feature_key: "knowledge_rag", enabled: true, config: { limit: 10 } } }
```
Diharapkan: `config` tersimpan.

### TC-FU04 — Tidak menulis audit bila tanpa sesi ✅ *ada di spec*

Diharapkan: flag tersimpan, audit **tidak** dipanggil.

### TC-FU05 — Kunci apa pun diterima **[dari kode]**

```
Request : { data: { feature_key: "kunci_ngawur_apa_saja", enabled: true } }
```
Diharapkan (perilaku **aktual**): **berhasil**, baris baru dibuat. Tidak ada daftar kunci yang sah
dan **tidak ada endpoint hapus** — sehingga kunci itu tersimpan permanen. Menguji BR-05.
**Belum ada test.**

### TC-FU06 — `config` tidak dikirim tidak diubah **[dari kode]**

```
Request : { data: { feature_key: "knowledge_rag", enabled: false } }
```
Diharapkan: `enabled` berubah, `config` **tetap** nilai sebelumnya.

### TC-FU07 — Audit mencatat id perusahaan, bukan id flag **[dari kode]**

Diharapkan (perilaku **aktual**): entri audit memiliki `idEntity` = **id perusahaan** (mis. `1`),
bukan id baris flag. Sehingga seluruh entri audit flag menunjuk entitas yang sama. Menguji BR-25 &
KI-32. **Belum ada test.**

---

## 7. Normalisasi Klien — Kuota AI Harian

Seluruh kasus **[dari kode]**; belum ada test.

| # | Masukan | Hasil yang diharapkan |
|---|---|---|
| TC-AI01 | *(kosong)* | **50** |
| TC-AI02 | `"abc"` | **50** |
| TC-AI03 | `0` | **1** |
| TC-AI04 | `-10` | **1** |
| TC-AI05 | `1` | 1 |
| TC-AI06 | `250` | 250 |
| TC-AI07 | `500` | 500 |
| TC-AI08 | `9999` | **500** |
| TC-AI09 | `12.7` | **13** (dibulatkan) |
| TC-AI10 | `12.4` | **12** |

Setelah simpan, kotak input **ditulis ulang** dengan nilai hasil normalisasi — pengguna melihat
angka yang benar-benar tersimpan.

---

## 8. Normalisasi Klien — Mode Persetujuan Koreksi Stok

Seluruh kasus **[dari kode]**.

| # | Nilai tersimpan | Dinormalisasi menjadi |
|---|---|---|
| TC-AM01 | `"strict"` | **`strict`** |
| TC-AM02 | `"simple"` | `simple` |
| TC-AM03 | `"Strict"` (huruf besar) | **`simple`** — perbandingan peka huruf |
| TC-AM04 | `"stric"` (salah ketik) | **`simple`** |
| TC-AM05 | `undefined` | `simple` |
| TC-AM06 | `12345` | `simple` |

Aturan yang sama diterapkan di klien **dan** di modul Stock. Arah kegagalannya perlu diketahui:
salah ketik apa pun **melemahkan** penjagaan menjadi mode longgar, bukan memperketatnya.

### TC-AM07 — Nama kunci lama tetap dibaca **[dari kode]**

Data: `operationalRules = { stock_adjustment_approval_mode: "strict" }` (nama lama).
Diharapkan: **dibaca sebagai `strict`** — baik oleh klien maupun modul Stock. Menguji BR-09.

---

## 9. Normalisasi Klien — Kalibrasi Kertas

Dua aturan berbeda; seluruh kasus **[dari kode]**.

### Lebar form & Tinggi per lembar (wajib > 0)

| # | Masukan | Hasil |
|---|---|---|
| TC-KP01 | `""` | tidak-diisi → **default berlaku** |
| TC-KP02 | `0` | **tidak-diisi** → default berlaku |
| TC-KP03 | `-5` | tidak-diisi |
| TC-KP04 | `"abc"` | tidak-diisi |
| TC-KP05 | `241.3` | 241.3 |

### Margin atas/bawah/kiri/kanan (boleh ≥ 0)

| # | Masukan | Hasil |
|---|---|---|
| TC-KP06 | `""` | tidak-diisi → default berlaku |
| TC-KP07 | `0` | **0** (diterima) |
| TC-KP08 | `-5` | tidak-diisi |
| TC-KP09 | `37.91` | 37.91 |

Pembedaan TC-KP02 vs TC-KP07 adalah inti aturannya dan **wajib** dipertahankan.

### TC-KP10 — Blok kalibrasi dihilangkan bila seluruhnya kosong **[dari kode]**

Aksi: mengosongkan keenam angka dan tidak mencentang kedua checkbox.
Diharapkan: blok kalibrasi **tidak ada** di data tersimpan → seluruh nilai default berlaku saat
mencetak.

### TC-KP11 — Checkbox tidak dicentang menghilangkan field **[dari kode]**

| Checkbox | Tidak dicentang → tersimpan |
|---|---|
| "Jadikan kertas kontinu sebagai default" | **dihilangkan** (bukan `false`) |
| "Kertas sudah preprinted" | **dihilangkan** (bukan `true`) |

### TC-KP12 — Logika terbalik checkbox preprinted **[dari kode]**

Aksi: mencentang **"Kertas sudah preprinted"**.
Diharapkan: tersimpan sebagai **`showLetterhead: false`**.
Aksi: menghapus centang.
Diharapkan: field `showLetterhead` **dihilangkan**.

---

## 10. Pembersihan Profil Dokumen Cetak

Seluruh kasus **[dari kode]**; belum ada test.

### TC-DP01 — Teks di-trim dan yang kosong dihilangkan

| Masukan | Hasil |
|---|---|
| `"  TB SUMBER  "` | `"TB SUMBER"` |
| `"   "` | **dihilangkan** dari data |
| `""` | dihilangkan |

Berlaku untuk: nama kepala, alamat, tagline, kota, catatan nota, ucapan penutup.

### TC-DP02 — Baris telepon tanpa nomor dibuang

| Baris | Hasil |
|---|---|
| label `"TOKO"`, nomor `"0813..."` | **dipertahankan** |
| label `"TOKO"`, nomor `""` | **dibuang** |
| label `""`, nomor `"0813..."` | **dipertahankan** dengan label kosong |
| Seluruh daftar kosong setelah pembersihan | field daftar **dihilangkan** |

### TC-DP03 — Baris rekening tanpa nomor dibuang

Aturan identik TC-DP02 — **nomor rekening** adalah field penentu; bank dan atas nama boleh kosong.

### TC-DP04 — Daftar barang dipecah per baris

```
Masukan (textarea):
"  - BAHAN BANGUNAN  \n\n  - KERAMIK  \n   \n- GRANIT"
```
Diharapkan: `["- BAHAN BANGUNAN", "- KERAMIK", "- GRANIT"]` — tiap baris di-trim, baris kosong
dibuang.

```
Masukan: "\n\n   \n"
```
Diharapkan: field **dihilangkan** dari data.

---

## 11. Halaman Profil Perusahaan (UI)

### TC-UI01 — Render kosong sebelum data siap **[dari kode]**

Data: perusahaan / data pendamping / pengaturan belum tersedia.
Diharapkan: halaman merender **kosong sama sekali** — tanpa indikator memuat, tanpa pesan.
Menguji EC-50 & KI-34.

### TC-UI02 — Mode baca-saja tanpa izin kelola **[dari kode]**

Data: sesi tanpa `company_config.manage`.
Diharapkan:
- **Semua** input, select, textarea, checkbox **dinonaktifkan**
- Tombol "Hapus", "Tambah telepon", "Tambah rekening" **tidak dirender**
- Tombol "Simpan Pengaturan" **tidak dirender**
- Teks penjelas: **"Anda sedang melihat pengaturan dalam mode baca saja."**

### TC-UI03 — Teks penjelas saat punya izin kelola **[dari kode]**

Diharapkan: **"Perubahan akan langsung berlaku untuk seluruh cabang."** + tombol
**"Simpan Pengaturan"**.

### TC-UI04 — Mata uang diubah ke huruf besar saat mengetik **[dari kode]**

Aksi: mengetik `usd` di kotak Mata uang.
Diharapkan: kotak langsung menampilkan **`USD`**.

### TC-UI05 — Placeholder nama kepala dokumen bersifat dinamis **[dari kode]**

Data: nama perusahaan `TB. SUMBER ABADI KAMOLAN`.
Diharapkan placeholder: `Kosongkan untuk memakai "TB. SUMBER ABADI KAMOLAN"`.

### TC-UI06 — Urutan penyimpanan: profil ditunggu, pengaturan tidak **[dari kode]**

Diharapkan: `company/profile/update` dipanggil dan **ditunggu**, lalu
`company/settings/update` dipanggil **tanpa ditunggu**.

### TC-UI07 — Profil gagal menghentikan penyimpanan pengaturan **[dari kode]**

Data: `company/profile/update` menolak.
Diharapkan: `company/settings/update` **tidak pernah dipanggil**; hanya toast danger
**"Gagal menyimpan profil perusahaan"**. Menguji SK-07. **Belum ada test.**

### TC-UI08 — Profil berhasil + pengaturan gagal menampilkan dua toast **[dari kode]**

Diharapkan: toast success **"Profil perusahaan disimpan"** **dan** toast danger
**"Gagal menyimpan pengaturan"** muncul bersamaan; hanya profil tersimpan. Menguji SK-08 & KI-29.
**Belum ada test.**

### TC-UI09 — Tidak ada toast sukses untuk pengaturan **[dari kode]**

Aksi: mengubah **hanya** mode persetujuan koreksi stok, lalu simpan.
Diharapkan: **satu** toast sukses — dan judulnya **"Profil perusahaan disimpan"**, bukan tentang
pengaturan. Bagian terbesar halaman tersimpan tanpa notifikasi sendiri.

### TC-UI10 — Nama perusahaan baru langsung tampil di topbar **[dari kode]**

Aksi: mengubah nama perusahaan lalu simpan.
Diharapkan: chip **"Perusahaan"** di topbar menampilkan nama baru **tanpa reload** — karena
ringkasan sesi di browser ikut diperbarui.

---

## 12. Halaman Status Order (UI)

Seluruh kasus **[dari kode]**; belum ada test unit.

### TC-OS01 — Peringatan kode duplikat muncul saat mengetik

Data: sudah ada status berkode `draft`.
Aksi: mengetik `draft` di kotak "Kode unik".
Diharapkan:
- Teks merah **"Kode ini sudah digunakan. Gunakan kode lain."** muncul di bawah field
- Tombol **"Tambah Status"** **dinonaktifkan**

### TC-OS02 — Tombol dinonaktifkan bila nama/kode kosong

| Kondisi | Tombol |
|---|---|
| Nama `"   "`, kode terisi | dinonaktifkan |
| Nama terisi, kode `"   "` | dinonaktifkan |
| Keduanya terisi, kode unik | aktif |

### TC-OS03 — Status baru memakai nilai tetap

Aksi: menambah status bernama "Menunggu Konfirmasi" berkode `waiting_confirmation`, sementara sudah
ada 5 status.
Diharapkan payload:
```
status_group          : "pending"
applicable_order_kind : "all"
is_initial            : false
is_terminal           : false
sort_order            : 6        (jumlah status + 1)
color_hex             : "#12343B"
```
Menguji BR-21. Perhatikan warna itu **tidak ada** di palet 5 status bawaan.

### TC-OS04 — Field dikosongkan setelah menambah status

Diharapkan: kedua field ("Nama status", "Kode unik") menjadi kosong.

### TC-OS05 — Kode tidak di-trim saat dikirim

Aksi: mengetik `" draft2 "` (dengan spasi) — tombol aktif karena tidak kosong setelah trim.
Diharapkan (perilaku **aktual**): kode dikirim **dengan spasi** apa adanya. Menguji asimetri di
[numbering-sequence.md](numbering-sequence.md) §3.

### TC-OS06 — Aturan perpindahan tanpa memilih status

Aksi: mengisi hanya label, lalu menekan **"Tambah Aturan"**.
Diharapkan (perilaku **aktual**): tombol **tetap aktif**; nilai kosong dikirim sebagai
`id_from_status: 0` dan `id_to_status: 0` → gagal di server → toast danger
**"Gagal menyimpan transisi status"** tanpa alasan. Menguji BR-24 & KI-31.

### TC-OS07 — Aturan perpindahan dari status ke dirinya sendiri

Aksi: memilih status yang sama untuk "Dari" dan "Ke".
Diharapkan: **tidak dijaga di klien** — dikirim ke server.

### TC-OS08 — Hanya label dikosongkan setelah menambah aturan

Diharapkan: pilihan "Dari status" dan "Ke status" **tetap terisi**; hanya label kosong.
Perilaku ini memudahkan menambah beberapa aturan dari status yang sama.

### TC-OS09 — Terjemahan kelompok status

| Nilai | Label badge | Tone |
|---|---|---|
| `pending` | **"Menunggu"** | warning |
| `active` | **"Aktif"** | info |
| `completed` | **"Selesai"** | success |
| `cancelled` | **"Dibatalkan"** | danger |
| `sesuatu_lain` | `sesuatu_lain` (mentah) | info |

### TC-OS10 — Daftar status diurutkan, daftar aturan tidak

Diharapkan: daftar status diurutkan berdasarkan **urutan tampil** naik; daftar aturan perpindahan
tampil **apa adanya dari server** tanpa pengurutan.

### TC-OS11 — Status asal/tujuan tidak ditemukan tampil kosong

Data: aturan perpindahan merujuk id status yang tidak ada di daftar.
Diharapkan: baris `→` tampil dengan sisi yang hilang **kosong** — bukan `-`, bukan pesan.

---

## 13. Hook Modul Pengaturan

### TC-HK01 — Memanggil 3 pemuatan saat terautentikasi dan semua idle ✅ *ada di test*

Diharapkan: `reloadUsers`, `reloadRoles`, `reloadStatuses` ketiganya dipanggil.

### TC-HK02 — Tidak memanggil apa pun bila belum terautentikasi ✅ *ada di test*

Diharapkan: nol pemanggilan.

### TC-HK03 — Tidak memanggil ulang bila status sudah `ready` ✅ *ada di test*

Diharapkan: `reloadRoles` **tidak** dipanggil.

### TC-HK04 — Tidak memanggil ulang bila status `loading` ✅ *ada di test*

Diharapkan: `reloadStatuses` **tidak** dipanggil.

### TC-HK05 — Mengembalikan seluruh field yang dibutuhkan ✅ *ada di test*

Diharapkan: hook mengekspos perusahaan aktif, data pendamping, cabang aktif, daftar cabang,
pemeriksa izin, serta fungsi simpan profil/pengaturan/cabang/role/status.

### TC-HK06 — Memuat data yang tidak dipakai halaman ini **[dari kode]**

Diharapkan: membuka `/settings` memicu pemuatan **daftar pengguna** dan **daftar role** — padahal
halaman Profil Perusahaan tidak memakai keduanya. Hook yang sama melayani tiga halaman
pengaturan. Menguji UF-01 langkah 3.

---

## 14. End-to-End

### TC-E01 — Halaman `/settings` merender "Profil Perusahaan" ✅ *ada di E2E*

```
1. Buka /settings
2. Pastikan teks cocok /Profil Perusahaan/i terlihat
```

### TC-E02 — Halaman `/settings/order-status` merender "Status Order" ✅ *ada di E2E*

```
1. Buka /settings/order-status
2. Pastikan teks cocok /Status Order/i terlihat
```

### TC-E03 — Mode `strict` menolak persetujuan diri sendiri ✅ *ada di E2E*

Ini **satu-satunya** verifikasi end-to-end bahwa pengaturan modul ini benar-benar berpengaruh:

```
1. settings = company/settings/get
2. originalRules = settings.operationalRules ?? settings.operational_rules ?? {}
3. company/settings/update dengan operational_rules:
     { ...originalRules, stockAdjustmentApprovalMode: 'strict' }
4. stock/adjust dengan approval oleh 'superadmin' (= pembuat koreksi)
   → GAGAL, pesan cocok /berbeda|strict|approver/i
5. stock/adjust dengan approval oleh 'owner' (orang berbeda)
   → BERHASIL
6. finally: company/settings/update dengan operational_rules: originalRules
```

Langkah 2 dan 6 adalah pola wajib yang dituntut BR-14 — membaca lalu memulihkan agar kunci lain di
blok itu tidak hilang.

---

## 15. Kasus yang Belum Punya Test dan Layak Ditambahkan

| # | Kasus | Kenapa penting |
|---|---|---|
| GAP-01 | Blok pengaturan ditimpa seluruhnya (TC-SU06) | Integrasi yang mengirim satu kunci akan menghapus sisanya (KI-30) |
| GAP-02 | Profil berhasil + pengaturan gagal (TC-UI08) | Pengguna melihat sukses padahal sebagian besar gagal (KI-29) |
| GAP-03 | Profil gagal menghentikan penyimpanan pengaturan (TC-UI07) | Perilaku yang tak terlihat dan tak terduga |
| GAP-04 | Aturan perpindahan tanpa memilih status (TC-OS06) | Gagal teknis tanpa pesan yang bisa dipahami (KI-31) |
| GAP-05 | Audit flag mencatat id perusahaan (TC-FU07) | Riwayat flag tidak bisa dilacak per-flag (KI-32) |
| GAP-06 | Kuota AI di luar rentang lewat API (TC-SU10) | Normalisasi hanya di klien |
| GAP-07 | Feature flag dengan kunci sembarang (TC-FU05) | Data bertambah permanen tanpa cara menghapus |
| GAP-08 | Kode perusahaan beda huruf besar/kecil (TC-PU13) | Penjagaan aplikasi dan basis data tidak sepakat |
| GAP-09 | Normalisasi kuota AI & kalibrasi kertas (§7, §9) | Aturan bernuansa (0 ditolak untuk lebar, diterima untuk margin) tanpa satu pun test |
| GAP-10 | Pembersihan profil dokumen (§10) | Menentukan isi dokumen cetak; tanpa test sama sekali |
| GAP-11 | Halaman merender kosong sebelum data siap (TC-UI01) | Tampak seperti halaman rusak (KI-34) |
| GAP-12 | Status baru selalu `pending` dan warna `#12343B` (TC-OS03) | Nilai tetap yang tidak bisa diubah pengguna (KI-35) |
