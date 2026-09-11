# Business Rules — Modul 03 Company & Settings

**Kelompok A — SEMUA validasi, kondisi, dan aturan keputusan.** Setiap aturan disertai lokasi
agar bisa diverifikasi ulang. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [user-flows.md](user-flows.md) ·
[test-cases.md](test-cases.md) · [known-issues.md](../known-issues.md)

---

## 1. Validasi Input

Modul ini **tidak memakai DTO `class-validator`** — seluruh validasi manual di service, lewat dua
helper internal.

### BR-01 — Dua helper normalisasi teks

| Helper | Perilaku |
|---|---|
| **Teks wajib** | Menerima nilai apa pun; bila bukan string → jadi string kosong; di-**trim**; bila hasilnya kosong → `400` `{Label} wajib diisi` |
| **Teks opsional** | `null`/`undefined` → `null`; bila bukan string → string kosong; di-**trim**; bila hasilnya kosong → `null` |

Konsekuensi yang perlu diketahui: mengirim **angka** untuk field teks wajib (mis. `name: 123`)
menghasilkan pesan "wajib diisi", bukan "tipe salah" — karena non-string langsung dianggap kosong.

### BR-02 — Validasi `company/profile/update`

Semua field bersifat **opsional pada payload** (pola partial update); yang tidak dikirim tidak
diubah. Yang dikirim divalidasi:

| Field | Aturan | Pesan bila gagal |
|---|---|---|
| `code` | Teks wajib + **unik global** | `Kode perusahaan wajib diisi` / `Kode perusahaan sudah digunakan` |
| `name` | Teks wajib | `Nama perusahaan wajib diisi` |
| `legal_name` | Teks opsional | — |
| `timezone` | Teks wajib | `Zona waktu wajib diisi` |
| `currency_code` | Teks wajib, lalu **diubah ke huruf besar** | `Mata uang wajib diisi` |
| `locale` | Teks wajib | `Bahasa sistem wajib diisi` |

Yang **tidak** divalidasi:

- **Zona waktu tidak diperiksa** sebagai zona IANA yang sah. `Mars/Olympus` diterima dan tersimpan
- **Locale tidak diperiksa** sebagai locale yang sah. `xx-YY` diterima
- **Mata uang tidak diperiksa** terhadap daftar kode ISO 4217. `ABCDE` diterima
- Panjang nilai tidak dibatasi di aplikasi — hanya oleh kolom basis data (kode 50, nama 255,
  zona waktu 100, mata uang 10, locale 20). Melebihinya menghasilkan **500**, bukan pesan bisnis

Ketiga field tanpa validasi itu juga **tidak berpengaruh pada tampilan apa pun** (lihat BR-19),
sehingga nilai salah tidak menimbulkan gejala — ia hanya tersimpan diam-diam.

### BR-03 — Keunikan kode perusahaan

```
bila code dikirim DAN berbeda dari nilai sekarang:
    cari perusahaan lain dengan kode itu
    bila ada dan bukan dirinya sendiri → 409 "Kode perusahaan sudah digunakan"
```

Pemeriksaan **dilewati** bila nilai tidak berubah, sehingga menyimpan ulang dengan kode yang sama
tidak pernah bentrok.

Keunikan bersifat **global** (kolom `code` ber-indeks unik), bukan per-sesuatu. Untuk sistem
satu-perusahaan ini tidak berdampak.

### BR-04 — Validasi `company/settings/update`

**Tidak ada validasi sama sekali.** Kelima blok diterima sebagai objek bebas:

| Field payload | Kolom tujuan | Validasi |
|---|---|---|
| `business_labels` | `businessLabels` | tidak ada |
| `operational_rules` | `operationalRules` | tidak ada |
| `ui_preferences` | `uiPreferences` | tidak ada |
| `ai_preferences` | **`assistantPreferences`** | tidak ada |
| `reporting_preferences` | `reportingPreferences` | tidak ada |

Perhatikan baris keempat: nama di payload API adalah `ai_preferences` tetapi kolomnya bernama
`assistant_preferences`. Ketidakcocokan nama ini juga tercermin di adapter frontend, yang menerima
**empat** ejaan (`aiPreferences`, `assistantPreferences`, `assistant_preferences`,
`ai_preferences`) saat membaca.

Konsekuensi ketiadaan validasi: struktur apa pun bisa masuk. Tidak ada penjagaan bahwa
`stockAdjustmentApprovalMode` bernilai `simple`/`strict`, atau bahwa `ai_daily_limit` berupa
angka. Yang menjaga adalah **normalisasi di sisi pembaca** (BR-08, BR-09).

### BR-05 — Validasi `company/features/update`

**Tidak ada validasi.** `feature_key` diterima sebagai string apa pun, dan kunci yang belum ada
**dibuat otomatis**. Tidak ada daftar kunci yang sah.

Artinya permintaan API dapat membuat feature flag bernama apa saja, selamanya bertambah tanpa cara
menghapusnya (tidak ada endpoint hapus).

### BR-06 — Validasi klien pada halaman Profil Perusahaan

| Field | Aturan klien |
|---|---|
| Kode perusahaan, Nama perusahaan, Zona waktu, Mata uang, Bahasa sistem | `required` — diblokir browser bila kosong |
| Nama legal | tanpa aturan |
| Mata uang | `maxLength=10`, dan **diubah ke huruf besar setiap ketikan** |
| Kuota AI harian | `min=1`, `max=500` pada `input type=number` — **hanya petunjuk browser**, nilai di luar rentang tetap dinormalisasi di klien |
| Kalibrasi kertas | `min=0`, `step=0.1` |

Tidak ada pesan validasi kustom di halaman ini — seluruhnya memakai pesan bawaan browser.

### BR-07 — Validasi klien pada halaman Status Order

| Field | Aturan |
|---|---|
| Nama status | `required` + **tidak boleh kosong setelah trim** (menonaktifkan tombol) |
| Kode unik | `required` + tidak boleh kosong setelah trim + **tidak boleh sudah dipakai** |
| Label tombol aksi (aturan perpindahan) | `required` |
| Dari status / Ke status | **tidak ada validasi** |

Pemeriksaan duplikat kode terjadi **saat mengetik**, memunculkan teks
**"Kode ini sudah digunakan. Gunakan kode lain."** dan menonaktifkan tombol — sehingga tidak
pernah sampai ke server.

Sebaliknya, form aturan perpindahan **tidak** menonaktifkan tombolnya. Status yang belum dipilih
menghasilkan nilai kosong yang dikonversi menjadi angka **`0`** lalu dikirim — dan gagal di server
sebagai kesalahan teknis, bukan pesan bisnis. Lihat KI-31.

---

## 2. Aturan Normalisasi Nilai

### BR-08 — Normalisasi kuota AI harian (klien)

```
angka = Number(masukan)
bila bukan angka terhingga → 50
selain itu → dibulatkan, lalu dipangkas ke rentang [1, 500]
```

| Masukan | Hasil |
|---|---|
| *(kosong)* / `"abc"` | **50** |
| `0` / `-10` | **1** |
| `9999` | **500** |
| `12.7` | **13** |
| `250` | 250 |

Nilai hasil normalisasi **langsung ditulis ulang ke kotak input** setelah simpan, sehingga
pengguna melihat angka yang benar-benar tersimpan.

Normalisasi ini hanya ada di **klien**. Permintaan API langsung dapat menyimpan nilai apa pun,
termasuk negatif atau teks — dan modul WhatsApp yang membacanya perlu menangani itu sendiri.
**[PERLU KONFIRMASI]** apakah modul WhatsApp menormalkan ulang; ditelusuri saat analisis modul 21.

### BR-09 — Normalisasi mode persetujuan koreksi stok

Aturan yang **sama** diterapkan di tiga tempat, dan konsistensinya penting:

```
mode === 'strict' → 'strict'
selain itu       → 'simple'
```

| Lokasi | Peran |
|---|---|
| Klien (halaman pengaturan) | Menormalkan nilai yang dibaca dan yang ditulis |
| Modul Stock | Menormalkan saat membaca untuk memvalidasi persetujuan |

**Dua nama kunci diterima** di kedua tempat:

| Nama kunci | Status |
|---|---|
| `stockAdjustmentApprovalMode` | Nama utama, dipakai saat menulis |
| `stock_adjustment_approval_mode` | Nama lama, **masih dibaca** sebagai cadangan |

Toleransi dua nama ini berarti data lama tetap terbaca. Nilai apa pun selain `strict` — termasuk
salah ketik seperti `Strict` atau `stric` — **diam-diam menjadi `simple`**, yaitu mode yang lebih
longgar. Ini arah kegagalan yang perlu diketahui: salah ketik melemahkan penjagaan, bukan
memperketatnya.

### BR-10 — Normalisasi angka kalibrasi kertas (klien)

Dua aturan berbeda, dan pembedaannya disengaja:

| Jenis field | Aturan | Alasan |
|---|---|---|
| Lebar form, Tinggi per lembar | Diterima hanya bila **terhingga dan > 0**; selain itu → tidak-diisi | Kertas selebar 0mm tidak masuk akal |
| Margin atas/bawah/kiri/kanan | Diterima bila **terhingga dan ≥ 0**; selain itu → tidak-diisi | Margin 0 sah (cetak mentok ke tepi) |

Nilai "tidak-diisi" berarti field itu dihilangkan dari data, sehingga **nilai default kalibrasi
yang berlaku** saat mencetak.

| Masukan | Lebar/Tinggi | Margin |
|---|---|---|
| `""` (kosong) | tidak-diisi | tidak-diisi |
| `0` | **tidak-diisi** | **0** |
| `-5` | tidak-diisi | tidak-diisi |
| `"abc"` | tidak-diisi | tidak-diisi |
| `241.3` | 241.3 | 241.3 |

### BR-11 — Pembersihan profil dokumen cetak (klien, saat simpan)

Dijalankan sebelum data dikirim:

| Bagian | Aturan pembersihan |
|---|---|
| Nama kepala, alamat, tagline, kota, catatan, ucapan penutup | Di-**trim**; bila kosong → **dihilangkan** dari data |
| Daftar telepon | Setiap label & nomor di-trim; **baris tanpa nomor dibuang**; bila daftar jadi kosong → dihilangkan |
| Daftar rekening | Setiap bank, atas nama, nomor di-trim; **baris tanpa nomor dibuang**; bila kosong → dihilangkan |
| Daftar barang yang dijual | Dipecah **per baris**, tiap baris di-trim, **baris kosong dibuang**; bila kosong → dihilangkan |
| Blok kalibrasi kertas | Dihilangkan **seluruhnya** bila semua field di dalamnya tidak-diisi |

Perhatikan aturan telepon dan rekening: **nomor** adalah field penentu. Baris berisi label
"TOKO" tanpa nomor akan hilang tanpa peringatan; sebaliknya baris berisi nomor tanpa label
**dipertahankan** (labelnya jadi string kosong).

### BR-12 — Normalisasi dua checkbox kalibrasi

| Checkbox | Nilai tersimpan bila tercentang | Bila tidak tercentang |
|---|---|---|
| "Jadikan kertas kontinu sebagai default saat cetak Nota" | `true` | **dihilangkan** (bukan `false`) |
| "Kertas sudah preprinted — jangan cetak ulang" | **`showLetterhead: false`** | **dihilangkan** (bukan `true`) |

Checkbox kedua memakai **logika terbalik**: label berbunyi negatif ("jangan cetak ulang"),
sementara data menyimpan sisi positifnya ("tampilkan kop"). Tercentang berarti `showLetterhead`
bernilai `false`.

Pola "dihilangkan alih-alih `false`" berlaku agar nilai default kalibrasi (`isDefault: false`,
`showLetterhead: true`) yang berlaku, dan agar blok kalibrasi bisa dianggap "seluruhnya kosong"
bila tidak ada yang diisi.

### BR-13 — Normalisasi mata uang

Diubah ke huruf besar **dua kali**:

| Lokasi | Kapan |
|---|---|
| Klien, saat mengetik | Setiap perubahan input |
| Klien, saat submit | Sebelum payload dikirim |
| Server | Setelah trim, sebelum disimpan |

Sehingga `usd` selalu tersimpan sebagai `USD`, apa pun jalur masuknya.

---

## 3. Aturan Penyimpanan Pengaturan

### BR-14 — Setiap blok ditimpa seluruhnya, bukan digabung per-kunci

Di **server**, blok yang dikirim menggantikan isi blok lama sepenuhnya:

```
bila operational_rules dikirim → seluruh isi lama diganti
bila tidak dikirim            → tidak disentuh
```

Tidak ada penggabungan per-kunci di server. Konsekuensi: mengirim
`operational_rules: { stockAdjustmentApprovalMode: 'strict' }` akan **menghapus** kunci lain yang
mungkin ada di blok itu.

**Yang menyelamatkan dalam praktik:** klien selalu menggabung lebih dulu — ia mengirim seluruh isi
blok lama **plus** nilai yang berubah. Jadi penggabungan terjadi di **klien**, bukan server.

Konsekuensi yang perlu diketahui: **integrasi lain yang mengirim hanya satu kunci akan menghapus
sisanya.** Test E2E modul Stock sudah menangani ini dengan benar — ia membaca aturan operasional
lebih dulu, menyebarkannya, lalu menambahkan kunci yang ingin diubah, dan memulihkannya di akhir.
Pola itu wajib diikuti pemanggil mana pun.

### BR-15 — Baris pengaturan dibuat otomatis bila belum ada

| Operasi | Bila baris belum ada |
|---|---|
| `company/settings/get` | Mengembalikan **`null`** — bukan `404`, bukan objek default |
| `company/settings/update` | **Dibuat** lalu diisi |

Frontend menangani `null` dengan memakai bentuk data bawaannya sendiri (BR-20), sehingga UI tetap
berfungsi pada perusahaan tanpa baris pengaturan.

Perilaku yang sama berlaku untuk feature flag: `company/features/update` membuat baris bila
kuncinya belum ada.

### BR-16 — Sesi opsional pada dua operasi tulis

`company/settings/update` dan `company/features/update` menerima sesi sebagai parameter
**opsional**. Bila tidak ada sesi:

- Operasi tetap dijalankan
- **Audit log tidak ditulis**

Melalui controller, sesi **selalu** ada (diambil dari `@ActiveSession()`), sehingga jalur API selalu
teraudit. Parameter opsional itu untuk pemanggilan internal dari kode lain.

**[PERLU KONFIRMASI]** apakah ada pemanggil internal yang benar-benar memakainya. Pencarian tidak
menemukan satu pun pemanggil `CompanyService` di luar controllernya sendiri — sehingga cabang
"tanpa sesi" tampak tidak terpakai. Ada dua test khusus untuknya, jadi ia disengaja saat ditulis.

### BR-17 — Urutan penyimpanan di halaman pengaturan

Satu penekanan tombol menjalankan **dua** operasi berurutan:

```
1. company/profile/update   — DITUNGGU (await)
2. company/settings/update   — TIDAK ditunggu (fire-and-forget)
```

Konsekuensi per skenario:

| Skenario | Hasil |
|---|---|
| Keduanya berhasil | Profil & pengaturan tersimpan; **satu** toast sukses (hanya untuk profil) |
| Profil gagal | Pengecualian dilempar → **langkah 2 tidak pernah berjalan**; hanya toast danger |
| Profil berhasil, pengaturan gagal | Profil tersimpan, pengaturan **tidak**; toast sukses **dan** danger muncul bersamaan |

Skenario ketiga adalah yang paling menyesatkan: pengguna melihat konfirmasi keberhasilan sementara
sebagian besar isi halaman gagal tersimpan. Lihat KI-29.

---

## 4. Aturan Konsumsi Pengaturan oleh Modul Lain

Bagian ini penting karena menentukan pengaturan mana yang **benar-benar berarti**.

### BR-18 — Tiga pengaturan yang punya pembaca nyata

| Pengaturan | Pembaca | Cara dibaca |
|---|---|---|
| `operationalRules.stockAdjustmentApprovalMode` | **Modul Stock** | Dibaca **ulang dari basis data setiap kali** koreksi stok divalidasi — tidak di-cache |
| `uiPreferences.documentProfile` | **Modul Order** | Dibaca saat merender halaman cetak nota/struk/surat jalan |
| `assistantPreferences.ai_daily_limit`, `whatsapp_mode`, `behavior_preferences` | **Modul WhatsApp** | Dibaca saat memproses pesan |

Karena mode persetujuan dibaca per-operasi, **perubahan berlaku seketika** tanpa perlu siapa pun
login ulang atau memuat ulang aplikasi. Ini konsisten dengan pola resolusi permission di modul 01.

### BR-19 — Pengaturan yang tersimpan tetapi tidak punya pembaca

Diverifikasi lewat pencarian seluruh kode:

| Pengaturan / field | Status |
|---|---|
| `companies.timezone` | **Tidak dibaca.** Zona waktu aplikasi dari env `APP_TIME_ZONE`, bawaan `Asia/Jakarta` |
| `companies.currencyCode` | **Tidak dibaca.** Format uang memakai `IDR` tetap |
| `companies.locale` | **Tidak dibaca.** Formatter memakai `id-ID` tetap |
| `companies.status` | **Tidak dibaca.** Di-seed `active`, tidak bisa diubah lewat modul ini |
| `businessLabels` (4 kunci) | **Tidak dibaca.** UI menulis "Produk" secara tetap |
| `reportingPreferences` (2 kunci) | **Tidak dibaca** |
| `uiPreferences.compactTable`, `.dashboardHighlight` | **Tidak dibaca** |
| `operationalRules.defaultOrderKind`, `.defaultDueDays`, `.criticalStockFocus` | **Tidak dibaca** |
| `assistantPreferences.tone`, `.responseLanguage`, `.answerStyle` | **Tidak dibaca** |
| `company_features` (seluruh tabel) | **Tidak dibaca dan tidak punya UI** |

Tiga field pertama adalah yang paling menyesatkan karena **terlihat di form dan bisa diubah** —
pengguna wajar mengira mengubah zona waktu akan menggeser tampilan tanggal. Keputusan untuk
**membuang** ketiga field ini di sistem baru sudah diambil (lihat
[shared-business-rules.md](../shared/shared-business-rules.md) §2.5).

### BR-20 — Bentuk data bawaan di klien

Dipakai ketika `company/settings/get` mengembalikan `null` atau saat bootstrap:

| Blok | Isi bawaan |
|---|---|
| `businessLabels` | `itemLabel: "Produk"`, `orderLabel: "Order"`, `stockLabel: "Stok"`, `customerLabel: "Pelanggan"` |
| `operationalRules` | `defaultOrderKind: "sales"`, `defaultDueDays: 7`, `criticalStockFocus: ""`, `stockAdjustmentApprovalMode: "simple"` |
| `uiPreferences` | `compactTable: false`, `dashboardHighlight: ""` |
| `aiPreferences` | `tone: "professional"`, `responseLanguage: "id"`, `answerStyle: "concise"`, `whatsapp_mode: "rule_based"`, `ai_daily_limit: 50` |
| `reportingPreferences` | `defaultRange: "month"`, `emphasizeMetric: ""` |
| `featureFlags` | `[]` — **selalu kosong** |

Aturan penggabungan saat membaca dari server:

| Blok | Aturan |
|---|---|
| `operationalRules` | **Digabung** dengan bawaan (bawaan dulu, lalu nilai server menimpa), plus jenis order dinormalisasi |
| Blok lain | **Diganti** seluruhnya oleh nilai server bila ada; bawaan hanya dipakai bila server tidak mengirimnya |
| `featureFlags` | **Selalu** dipaksa menjadi array kosong, mengabaikan apa pun dari server |

`featureFlags` yang selalu dikosongkan adalah alasan teknis mengapa sistem feature flag tidak
pernah aktif di UI — bahkan bila endpoint `company/features/list` dipanggil, hasilnya tidak akan
sampai ke bentuk data ini.

---

## 5. Aturan Status Order (sisi pengaturan)

Aturan bisnis status order milik modul Order; di sini hanya aturan yang **ditegakkan halaman
pengaturan ini**.

### BR-21 — Nilai tetap untuk status baru

Status yang dibuat lewat halaman ini **selalu** memakai nilai berikut, tanpa pilihan bagi
pengguna:

| Properti | Nilai tetap |
|---|---|
| Kelompok status | **`pending`** |
| Berlaku untuk jenis order | **`all`** (semua) |
| Titik awal | **tidak** |
| Titik akhir | **tidak** |
| Urutan tampil | **jumlah status saat ini + 1** |
| Warna | **`#12343B`** |

Konsekuensi: setiap status baru lahir sebagai "Menunggu" berwarna sama, tidak bisa dijadikan titik
awal/akhir, dan tidak bisa diubah setelahnya lewat UI mana pun. Warna `#12343B` juga **berbeda**
dari palet warna status bawaan seed (`#9CA3AF`, `#3B82F6`, `#F59E0B`, `#10B981`, `#EF4444`).

### BR-22 — Urutan tampil dihitung dari jumlah, bukan dari nilai maksimum

`sortOrder = jumlah status saat ini + 1`.

Bila ada status yang pernah dihapus di basis data, jumlah bisa lebih kecil dari urutan tertinggi
yang sudah ada — menghasilkan **urutan tampil duplikat**. Tidak ada penjagaan.

### BR-23 — Pemeriksaan duplikat kode status hanya di klien

Pemeriksaan `kode sudah dipakai` dilakukan terhadap daftar status **yang sudah dimuat di browser**,
bukan ke server. Bila daftar itu basi (mis. status ditambahkan dari sesi lain), duplikat bisa lolos
ke server dan ditolak di sana — dengan pesan yang tidak ditampilkan (hanya toast generik).

Server modul Order sendiri memeriksa duplikat (terlihat dari pencarian `findOne({ idCompany, code })`
di layanan status order), jadi penjagaan akhirnya tetap ada.

### BR-24 — Aturan perpindahan tanpa validasi klien

| Kondisi | Dijaga? |
|---|---|
| Status asal belum dipilih | **Tidak** — dikirim sebagai angka `0` |
| Status tujuan belum dipilih | **Tidak** — dikirim sebagai angka `0` |
| Status asal sama dengan tujuan | **Tidak** |
| Aturan yang sama sudah ada | **Tidak** |
| Label kosong | Ya (`required` browser) |

Aturan perpindahan baru selalu dibuat dengan penanda **aktif**.

---

## 6. Aturan Audit

### BR-25 — Tiga `actionKey` yang ditulis modul ini

| `actionKey` | `entityType` | `idEntity` | `before` | `after` |
|---|---|---|---|---|
| `company.profile.update` | `company` | id perusahaan | 6 field identitas | 6 field identitas |
| `company.settings.update` | `company_settings` | id perusahaan | kelima blok | kelima blok |
| `company.feature.update` | `company_feature` | **id perusahaan** | `enabled`, `config` | `featureKey`, `enabled`, `config` |

Semuanya memakai `idBranch: null` (aksi level-perusahaan) dan `actorType: 'user'`.

Dua hal yang perlu diketahui:

**`company.profile.update` adalah audit paling lengkap di seluruh sistem yang sudah dianalisis** —
ia mencatat keenam field di `before` **dan** `after`. Bandingkan dengan `user.update` di modul 02
yang hanya mencatat satu field di `after`. Pola di sini yang seharusnya dijadikan acuan.

**`company.feature.update` mencatat id yang salah.** Baris feature flag punya id sendiri
(auto-increment), tetapi audit menyimpan **id perusahaan** sebagai `idEntity`. Sehingga seluruh
entri audit feature flag menunjuk entitas yang sama (perusahaan 1), dan tidak mungkin melacak
riwayat satu flag tertentu dari kolom itu — kunci flag hanya ada di dalam `after`. Untuk
`company.settings.update` hal ini **benar**, karena kunci utama tabel pengaturan memang id
perusahaan. Lihat KI-32.

### BR-26 — Penamaan `actionKey` memakai tiga segmen

Konvensi proyek adalah `resource.action` (dua segmen, snake_case). Ketiga `actionKey` modul ini
memakai **tiga** segmen: `company.profile.update`, `company.settings.update`,
`company.feature.update`.

Ini menyimpang dari konvensi, sama seperti `role.permissions.update` di modul 02. Karena
`actionKey` dipakai sebagai filter di halaman Riwayat Aktivitas, penyimpangan ini memengaruhi cara
pencarian audit ditulis.

---

## 7. Aturan Envelope & Pesan Error

### BR-27 — Pemetaan error ke envelope

| Kondisi | Kelas exception | HTTP | `code` | `info` |
|---|---|---|---|---|
| Field wajib kosong | `BadRequestException` | 400 | 200 | `validation_failed` |
| Perusahaan tidak ditemukan | `NotFoundException` | 404 | 300 | `not_found` |
| Kode perusahaan sudah dipakai | `ConflictException` | **409** | **1** | **`error`** |
| Permission kurang (guard) | `ForbiddenException` | 403 | 400 | `forbidden` |

Baris ketiga terkena masalah yang sama seperti modul 02: HTTP 409 tidak dipetakan pembungkus
respons, sehingga jatuh ke kode error generik.

### BR-28 — Pesan yang benar-benar dilihat pengguna

Seluruh notifikasi gagal di modul ini **membuang pesan server sepenuhnya** — tidak ada deskripsi
toast sama sekali:

| Penyebab nyata | Yang dilihat pengguna |
|---|---|
| "Kode perusahaan sudah digunakan" | **"Gagal menyimpan profil perusahaan"** (tanpa alasan) |
| "Nama perusahaan wajib diisi" | **"Gagal menyimpan profil perusahaan"** (tanpa alasan) |
| "Zona waktu wajib diisi" | **"Gagal menyimpan profil perusahaan"** (tanpa alasan) |
| Kegagalan menyimpan pengaturan apa pun | **"Gagal menyimpan pengaturan"** (tanpa alasan) |
| Kegagalan status order apa pun | **"Gagal menyimpan definisi status"** / **"Gagal menyimpan transisi status"** |

Ini pola yang sama dengan KI-04 (modul 01) dan KI-17 (modul 02) — **sistemik**, bukan kelalaian
satu tempat.

Perlu dicatat bahwa untuk field wajib, pesan server praktis tidak pernah muncul karena browser
sudah memblokir submit lebih dulu (`required`). Yang benar-benar terasa adalah kasus kode
perusahaan bentrok dan kegagalan menyimpan pengaturan.

---

## 8. Formula & Perhitungan

Modul ini **tidak memiliki formula bisnis** — tanpa uang, kuantitas, atau pajak. Perhitungan yang
ada:

| Perhitungan | Rumus |
|---|---|
| Normalisasi kuota AI | `min(500, max(1, round(nilai)))`, fallback 50 bila bukan angka |
| Urutan tampil status baru | `jumlah status saat ini + 1` |
| Lebar konten cetak (di modul Order) | `widthMm − marginLeftMm − marginRightMm` |
| Jumlah cabang dapat diakses | jumlah entri akses cabang pengguna |

Nilai default kalibrasi kertas dan alasan tiap angkanya (khususnya margin kanan 37.91mm sebagai
batas jangkauan print head, bukan margin visual) terdokumentasi di
[shared-data-model.md](../shared/shared-data-model.md) §7.3 dan **wajib** dipertahankan.

---

## 9. Aturan Approval

**Tidak ada aturan approval di modul ini.** Perubahan profil dan pengaturan berlaku langsung
begitu disimpan, tanpa persetujuan pihak kedua.

Namun modul ini **menentukan** aturan approval modul lain: setelan
`stockAdjustmentApprovalMode` memilih antara dua rezim persetujuan koreksi stok
(BR-09, BR-18). Jadi modul ini adalah tempat kebijakan approval diatur, bukan tempat approval
dijalankan.

Yang perlu diketahui sebagai konsekuensi: **melemahkan kebijakan approval seluruh perusahaan
memerlukan satu klik dan tidak memerlukan persetujuan siapa pun** — pemegang
`company_config.manage` dapat mengubah `strict` menjadi `simple`, lalu menyetujui koreksi stoknya
sendiri. Aksinya teraudit (`company.settings.update`), tetapi tidak dicegah.
**[PERLU KONFIRMASI]** apakah ini dapat diterima, atau perubahan kebijakan approval perlu
penjagaan lebih ketat.

---

## 10. Ringkasan Kondisi Khusus

| # | Kondisi | Aturan |
|---|---|---|
| SK-01 | Zona waktu / locale / mata uang diisi nilai tidak sah | **Diterima dan tersimpan** — tidak ada validasi, dan nilainya tidak berpengaruh apa pun |
| SK-02 | Nilai melebihi panjang kolom | **500**, bukan pesan bisnis |
| SK-03 | Baris pengaturan belum ada saat dibaca | Mengembalikan `null`; klien memakai bentuk bawaan |
| SK-04 | Baris pengaturan belum ada saat ditulis | Dibuat otomatis |
| SK-05 | Satu blok pengaturan dikirim sendirian | Blok itu **ditimpa seluruhnya** — kunci lain di dalamnya hilang |
| SK-06 | Blok dikirim sebagai objek kosong | Seluruh isinya hilang; tidak ada penjagaan |
| SK-07 | Simpan profil gagal | Pengaturan **tidak ikut disimpan** |
| SK-08 | Simpan profil berhasil, pengaturan gagal | Toast sukses **dan** gagal bersamaan; hanya profil tersimpan |
| SK-09 | Kuota AI di luar rentang lewat API langsung | **Tidak dinormalisasi** — hanya klien yang menormalkan |
| SK-10 | Mode persetujuan diisi salah ketik | Diam-diam menjadi `simple` (mode lebih longgar) |
| SK-11 | Nama kunci lama `stock_adjustment_approval_mode` | Tetap dibaca sebagai cadangan |
| SK-12 | Baris telepon/rekening tanpa nomor | **Dibuang** saat simpan |
| SK-13 | Baris telepon/rekening tanpa label tapi ada nomor | **Dipertahankan** dengan label kosong |
| SK-14 | Lebar/tinggi kertas diisi 0 | Ditolak → default berlaku |
| SK-15 | Margin diisi 0 | Diterima sebagai 0 |
| SK-16 | Checkbox kalibrasi tidak dicentang | Field dihilangkan, bukan disimpan `false` |
| SK-17 | Feature flag dengan kunci baru di-update | **Dibuat otomatis**; tidak ada daftar kunci sah dan tidak ada cara menghapus |
| SK-18 | Status order baru ditambahkan | Selalu kelompok `pending`, warna `#12343B`, bukan titik awal/akhir |
| SK-19 | Aturan perpindahan tanpa memilih status | Dikirim sebagai id `0` → gagal di server dengan pesan yang tidak ditampilkan |
| SK-20 | Seed dijalankan ulang | **Profil perusahaan kembali ke bawaan**; pengaturan dan feature flag tidak disentuh |
