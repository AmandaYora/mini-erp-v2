# Feature Inventory — Modul 03 Company & Settings

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [test-cases.md](test-cases.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 6 — `company/profile/{get,update}`, `company/settings/{get,update}`, `company/features/{list,update}` |
| Halaman | 2 rute: `/settings` (Profil Perusahaan), `/settings/order-status` (Status Order) |
| Menu sidebar | 2, grup **"Pengaturan"**: "Profil Perusahaan" (ikon `company_profile`), "Status Order" (ikon `order_status`) |
| Permission | `company_config.view` (baca), `company_config.manage` (ubah) |
| Tabel yang dimiliki | `companies`, `company_settings`, `company_features` |
| Laporan | Tidak ada — lihat [reports-list.md](reports-list.md) |
| Penomoran dokumen | Tidak ada; ada **kode perusahaan** yang diisi manual — lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | 3 `actionKey`: `company.profile.update`, `company.settings.update`, `company.feature.update` |

**Catatan lingkup:** halaman `/settings/order-status` berada di folder web modul ini, tetapi
seluruh API-nya (`order-status-definitions/*`, `order-status-transitions/*`) milik **modul
Order**. Di sini didokumentasikan sisi **UI dan alur pengaturannya** saja; aturan bisnis status
order akan dianalisis bersama modul 08–09. Halaman `/settings/roles` juga ada di folder yang sama
tetapi sudah didokumentasikan di [modul 02](../02-users-roles-permissions/).

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Profil Perusahaan (identitas bisnis)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Ubah kode perusahaan | Unik secara global; dipakai sebagai identitas singkat |
| F-01.2 | Ubah nama perusahaan | Wajib; muncul di chip topbar "Perusahaan" dan sebagai fallback kepala dokumen cetak |
| F-01.3 | Ubah nama legal | Opsional; dikosongkan → tersimpan `null` |
| F-01.4 | Ubah zona waktu | Wajib; **tidak divalidasi** sebagai zona waktu nyata |
| F-01.5 | Ubah mata uang | Wajib; **otomatis dijadikan huruf besar** di klien maupun server |
| F-01.6 | Ubah bahasa sistem (locale) | Wajib; **tidak divalidasi** sebagai locale nyata |
| F-01.7 | Sinkronisasi ke sesi lokal | Setelah tersimpan, ringkasan sesi di browser ikut diperbarui sehingga nama/kode perusahaan langsung berubah di topbar tanpa reload |
| F-01.8 | Mode baca-saja | Tanpa `company_config.manage`, seluruh field dinonaktifkan dan tombol simpan disembunyikan |

**Yang perlu diketahui:** keempat field terakhir (zona waktu, mata uang, locale) **tersimpan tapi
tidak berpengaruh pada tampilan** — seluruh formatter aplikasi memakai `id-ID` dan zona
`Asia/Jakarta` yang ditetapkan di kode/env. Lihat §4 NF-01.

### F-02 — Ringkasan Cabang (baca-saja)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Tampilkan cabang aktif sekarang | Nama cabang; `-` bila tidak ada |
| F-02.2 | Tampilkan jumlah cabang yang dapat diakses | Angka, dari daftar akses cabang pengguna |

Kartu ini murni informatif — tidak ada aksi. Pengelolaan cabang ada di modul 04 (`/branches`).

### F-03 — Aturan Koreksi Stok

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Pilih mode persetujuan koreksi stok | Dua pilihan: **"Siapapun yang punya izin persetujuan"** (`simple`) atau **"Harus orang berbeda dari yang membuat koreksi"** (`strict`) |
| F-03.2 | Berlaku lintas cabang | Satu setelan untuk seluruh perusahaan |

**Ini satu-satunya setelan operasional yang benar-benar dibaca modul lain.** Modul Stock
membacanya saat memvalidasi persetujuan koreksi stok; mode `strict` menolak persetujuan oleh orang
yang sama dengan pembuat koreksi.

### F-04 — Kuota AI Harian

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Atur batas jawaban AI per hari | Angka, dibatasi **1–500** |
| F-04.2 | Normalisasi nilai | Nilai di luar rentang dipangkas; bukan angka → jatuh ke **50** |
| F-04.3 | Nilai dinormalisasi ulang setelah simpan | Kotak input langsung menampilkan angka hasil normalisasi |

Dibaca oleh modul WhatsApp untuk membatasi jawaban berbasis AI per hari.

### F-05 — Identitas Dokumen Cetak

Bagian terbesar halaman ini. Seluruh isinya dipakai modul Order saat mencetak nota, struk, dan
surat jalan.

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Nama di kepala dokumen | Kosong → memakai nama perusahaan |
| F-05.2 | Alamat di kepala dokumen | Kosong → memakai alamat cabang |
| F-05.3 | Baris lokasi singkat (struk) | Contoh: "PEREMPATAN KAMOLAN" |
| F-05.4 | Kota untuk tanda tangan surat jalan | Contoh: "BLORA" |
| F-05.5 | Daftar nomor telepon berlabel | Bisa ditambah/dihapus; tiap baris punya label + nomor |
| F-05.6 | Daftar rekening transfer | Bisa ditambah/dihapus; tiap baris bank + atas nama + nomor |
| F-05.7 | Daftar barang yang dijual | Textarea, **satu baris satu item** |
| F-05.8 | Catatan pada nota & struk | Contoh: "barang yang sudah dibeli tidak dapat ditukar kembali" |
| F-05.9 | Ucapan penutup struk | Contoh: "TERIMA KASIH" |
| F-05.10 | Pembersihan otomatis saat simpan | Nilai kosong dibuang, spasi di-trim, baris tanpa nomor dibuang |

### F-06 — Kalibrasi Kertas Kontinu (Dot-Matrix)

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | Lebar form (mm) | Wajib **> 0** bila diisi; kosong → memakai default 241.3 |
| F-06.2 | Tinggi per lembar (mm) | Wajib **> 0** bila diisi; kosong → default 139.7 |
| F-06.3 | Margin atas / bawah / kiri / kanan (mm) | Boleh **0**; kosong → memakai default masing-masing |
| F-06.4 | Jadikan kertas kontinu sebagai default cetak Nota | Checkbox |
| F-06.5 | Tandai kertas sudah preprinted | Checkbox — bila dicentang, kop/rekening/daftar jual **tidak** dicetak ulang |
| F-06.6 | Placeholder menampilkan nilai default | Setiap kotak menampilkan angka default sebagai placeholder |

Pembedaan F-06.1/2 vs F-06.3 disengaja: kertas selebar 0mm tidak masuk akal, tetapi margin 0
sah (cetak mentok ke tepi).

### F-07 — Status Order (halaman terpisah)

| # | Sub-fitur | Detail |
|---|---|---|
| F-07.1 | Daftar status order | Urut berdasarkan urutan tampil; menampilkan label, kelompok, penanda titik awal/akhir |
| F-07.2 | Tambah status baru | Dua field: nama status + kode unik |
| F-07.3 | Peringatan kode duplikat | Muncul langsung saat mengetik, sebelum submit |
| F-07.4 | Tombol submit dinonaktifkan | Bila nama/kode kosong atau kode sudah dipakai |
| F-07.5 | Daftar aturan perpindahan status | Menampilkan label aksi dan `dari → ke` |
| F-07.6 | Tambah aturan perpindahan | Tiga field: dari status, ke status, label tombol aksi |
| F-07.7 | Berlaku lintas cabang | Dinyatakan eksplisit di deskripsi halaman |

**Yang tidak ada di halaman ini:** mengubah atau menghapus status yang sudah ada, mengubah
kelompok status, mengubah urutan tampil, mengubah warna, menonaktifkan aturan perpindahan, dan
menghapus aturan perpindahan. Lihat §4 NF-04.

### F-08 — Feature Flags (API tersedia, tanpa UI)

| # | Sub-fitur | Detail |
|---|---|---|
| F-08.1 | Daftar feature flag perusahaan | Endpoint `company/features/list` |
| F-08.2 | Nyalakan/matikan feature flag | Endpoint `company/features/update`, menerima `feature_key`, `enabled`, `config` opsional |
| F-08.3 | Pembuatan otomatis | Kunci yang belum ada **dibuat otomatis** saat di-update — tidak ada daftar kunci yang sah |

**Tidak ada satu pun halaman, tombol, atau pemanggilan yang memakai kedua endpoint ini.** Tiga
kunci di-seed (`whatsapp_assistant`, `knowledge_rag`, `multi_location_stock`) semuanya dalam
keadaan **mati**, dan tidak ada kode yang membacanya untuk menentukan perilaku apa pun. Lihat §4
NF-02.

---

## 3. Edge Case yang Terbukti Ada di Kode

### Profil perusahaan

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-01 | Kode perusahaan diisi spasi saja | `400` **"Kode perusahaan wajib diisi"** (di-trim lebih dulu) |
| EC-02 | Nama perusahaan diisi spasi saja | `400` **"Nama perusahaan wajib diisi"** |
| EC-03 | Zona waktu diisi spasi saja | `400` **"Zona waktu wajib diisi"** |
| EC-04 | Mata uang diisi spasi saja | `400` **"Mata uang wajib diisi"** |
| EC-05 | Bahasa sistem diisi spasi saja | `400` **"Bahasa sistem wajib diisi"** |
| EC-06 | Kode perusahaan sudah dipakai perusahaan lain | `409` **"Kode perusahaan sudah digunakan"** |
| EC-07 | Kode diubah ke nilai yang sama | Tidak dianggap bentrok (pemeriksaan dilewati bila nilai tidak berubah) |
| EC-08 | Zona waktu diisi teks sembarang (mis. `Mars/Olympus`) | **Diterima dan tersimpan** — tidak ada validasi zona waktu |
| EC-09 | Mata uang diisi `usd` | Tersimpan sebagai **`USD`** (otomatis huruf besar) |
| EC-10 | Mata uang diisi lebih dari 10 karakter | Dipotong `maxLength` di klien; lewat API langsung → gagal di basis data (kolom 10 karakter) |
| EC-11 | Bahasa sistem diisi `xx-YY` | **Diterima dan tersimpan** — tidak ada validasi locale |
| EC-12 | Nama legal dikosongkan | Tersimpan `null` |
| EC-13 | Field tidak dikirim sama sekali | Tidak diubah (pola partial update) |
| EC-14 | Perusahaan tidak ditemukan | `404` **"Perusahaan tidak ditemukan"** |

### Pengaturan (settings)

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-15 | Baris pengaturan belum ada saat dibaca | `company/settings/get` mengembalikan **`null`** — bukan `404`, bukan objek default |
| EC-16 | Baris pengaturan belum ada saat disimpan | **Dibuat otomatis** |
| EC-17 | Satu blok pengaturan dikirim | Blok itu **ditimpa seluruhnya**, bukan digabung per-kunci |
| EC-18 | Blok lain tidak dikirim | Tidak disentuh |
| EC-19 | Blok dikirim sebagai objek kosong `{}` | Seluruh isi blok itu **hilang** — tidak ada penjagaan |
| EC-20 | Simpan tanpa sesi (dipanggil internal) | Berhasil, tetapi **audit log tidak ditulis** |
| EC-21 | Kuota AI diisi `0` | Dinormalisasi menjadi **1** |
| EC-22 | Kuota AI diisi `9999` | Dinormalisasi menjadi **500** |
| EC-23 | Kuota AI diisi teks | Dinormalisasi menjadi **50** |
| EC-24 | Kuota AI diisi `12.7` | Dibulatkan menjadi **13** |
| EC-25 | Mode persetujuan diisi nilai selain `strict` | Dinormalisasi menjadi **`simple`** |
| EC-26 | Mode persetujuan tersimpan dengan nama lama `stock_adjustment_approval_mode` | **Tetap dibaca** — kedua nama kunci diterima (klien maupun modul Stock) |

### Dokumen cetak & kalibrasi kertas

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-27 | Nomor telepon diisi label saja tanpa nomor | Baris itu **dibuang** saat simpan |
| EC-28 | Rekening diisi bank + atas nama tanpa nomor | Baris itu **dibuang** saat simpan |
| EC-29 | Seluruh daftar telepon/rekening dikosongkan | Field tersimpan sebagai tidak-ada (`undefined`), bukan array kosong |
| EC-30 | Daftar barang berisi baris kosong / spasi | Baris kosong **dibuang**, tiap baris di-trim |
| EC-31 | Lebar form diisi `0` | **Ditolak menjadi tidak-diisi** → memakai default 241.3mm |
| EC-32 | Margin diisi `0` | **Diterima** sebagai 0 |
| EC-33 | Margin diisi negatif | Ditolak menjadi tidak-diisi → memakai default |
| EC-34 | Seluruh field kalibrasi dikosongkan | Blok kalibrasi tersimpan sebagai tidak-ada → seluruh default berlaku |
| EC-35 | Checkbox "preprinted" dicentang | Tersimpan sebagai `showLetterhead: false` (logika terbalik) |
| EC-36 | Checkbox "preprinted" tidak dicentang | Field itu tersimpan sebagai tidak-ada, bukan `true` |
| EC-37 | Checkbox "jadikan default" tidak dicentang | Tersimpan sebagai tidak-ada, bukan `false` |

### Status order

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-38 | Kode status sudah dipakai | Peringatan merah di bawah field **dan** tombol submit dinonaktifkan — tidak sampai ke server |
| EC-39 | Nama atau kode hanya spasi | Tombol submit dinonaktifkan |
| EC-40 | Status baru ditambahkan | Selalu dibuat dengan kelompok **`pending`**, berlaku untuk **semua** jenis order, bukan titik awal, bukan titik akhir, warna **`#12343B`**, urutan = jumlah status + 1 |
| EC-41 | Aturan perpindahan ditambahkan tanpa memilih status | Field status memakai `SearchableSelect` yang bisa bernilai kosong; nilai kosong dikirim sebagai `Number("")` = **`0`** → gagal di server |
| EC-42 | Aturan perpindahan dari status A ke status A sendiri | **Tidak dijaga di klien** — dikirim ke server |
| EC-43 | Label tombol aksi dikosongkan | Field bertanda `required`, diblokir browser |
| EC-44 | Aturan perpindahan duplikat | **Tidak dijaga di klien** |
| EC-45 | Setelah menambah aturan | Hanya label yang dikosongkan; pilihan "dari"/"ke" **tetap terisi** |

### Perilaku simpan halaman pengaturan

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-46 | Simpan berhasil keduanya | **Satu** toast sukses ("Profil perusahaan disimpan") — pengaturan tersimpan tanpa notifikasi sendiri |
| EC-47 | Simpan profil gagal | Toast danger; **pengaturan tidak ikut disimpan** (proses berhenti) |
| EC-48 | Simpan profil berhasil, pengaturan gagal | Toast sukses **dan** toast danger muncul bersamaan; profil tersimpan, pengaturan tidak |
| EC-49 | Tanpa izin `company_config.manage` | Submit diabaikan; tombol simpan tidak dirender |
| EC-50 | Data perusahaan/pengaturan belum termuat | Halaman merender **kosong** (tidak ada indikator memuat) |

---

## 4. Hal yang Tampak Tersedia Tapi Tidak Berfungsi

Semua baris di bawah **terlihat oleh pengguna** atau **tersimpan di data**, tetapi tidak
berpengaruh apa pun. Dicatat di [known-issues.md](../known-issues.md).

| # | Elemen | Kondisi aktual |
|---|---|---|
| NF-01 | Field **Zona waktu**, **Mata uang**, **Bahasa sistem** | Tersimpan dan tampil di form, tetapi **tidak ada satu pun tampilan yang membacanya**. Formatter aplikasi memakai `id-ID` dan `Asia/Jakarta` yang ditetapkan di kode/env |
| NF-02 | **Feature flags** (3 kunci di-seed) | Endpoint ada, tabel ada, data ada — tetapi **tidak ada UI dan tidak ada kode yang membacanya**. Adapter frontend selalu mengembalikan daftar kosong |
| NF-03 | **Label bisnis** (`itemLabel`, `orderLabel`, `stockLabel`, `customerLabel`) | Di-seed dengan nilai nyata (mis. `itemLabel: "Barang"`) tetapi **tidak pernah dibaca** — UI menulis "Produk" secara tetap. Tidak ada UI untuk mengubahnya |
| NF-04 | Halaman **Status Order** | Hanya bisa **menambah**; tidak bisa mengubah, menghapus, mengurutkan ulang, atau menonaktifkan. Padahal API modul Order menyediakan endpoint `update` |
| NF-05 | Kolom **status perusahaan** | Ada di tabel dan di-seed `active`, tetapi tidak pernah dibaca maupun bisa diubah |
| NF-06 | Preferensi pelaporan (`defaultRange`, `emphasizeMetric`) | Ada di bentuk data bawaan, tanpa UI dan tanpa pembaca |
| NF-07 | Preferensi UI (`compactTable`, `dashboardHighlight`) | Ada di bentuk data bawaan, tanpa UI dan tanpa pembaca |
| NF-08 | Preferensi AI (`tone`, `responseLanguage`, `answerStyle`) | Ada di bentuk data bawaan, tanpa UI di halaman ini dan tanpa pembaca |

**Ringkasan yang perlu diketahui sebelum rebuild:** dari 5 blok pengaturan yang tersimpan, hanya
**3 nilai** yang benar-benar memengaruhi perilaku sistem:

| Nilai | Dibaca oleh |
|---|---|
| `operationalRules.stockAdjustmentApprovalMode` | Modul Stock (validasi persetujuan koreksi) |
| `uiPreferences.documentProfile` | Modul Order (cetak nota/struk/surat jalan) |
| `assistantPreferences.ai_daily_limit`, `whatsapp_mode`, `behavior_preferences` | Modul WhatsApp |

Blok `businessLabels` dan `reportingPreferences` **seluruhnya tidak punya pembaca**.

---

## 5. Data Bawaan (Seed)

### Perusahaan

| Field | Nilai |
|---|---|
| `id_company` | 1 |
| Kode | `TSAK` |
| Nama | **TB. SUMBER ABADI KAMOLAN** |
| Nama legal | TB. SUMBER ABADI KAMOLAN |
| Zona waktu | `Asia/Jakarta` |
| Mata uang | `IDR` |
| Locale | `id-ID` |
| Status | `active` |

Seed memakai "timpa bila sudah ada" untuk seluruh field ini — sehingga **profil perusahaan yang
sudah diubah lewat UI akan kembali ke nilai bawaan** bila seed dijalankan ulang.

### Pengaturan bawaan

| Blok | Isi yang di-seed |
|---|---|
| `business_labels_json` | `{"itemLabel":"Barang","orderLabel":"Order","stockLabel":"Stok","customerLabel":"Pelanggan"}` |
| `operational_rules_json` | `{"stockAdjustmentApprovalMode":"simple"}` |
| `assistant_preferences_json` | `{"whatsapp_mode":"rule_based","behavior_preferences":{"tone":"Ringkas dan langsung"}}` |
| `ui_preferences_json` | **tidak di-seed** (null) |
| `reporting_preferences_json` | **tidak di-seed** (null) |

Berbeda dari profil, pengaturan memakai "sisipkan bila belum ada" — sehingga pengaturan yang sudah
diubah **tidak** ditimpa seed.

Perhatikan `itemLabel: "Barang"` sementara seluruh UI menulis **"Produk"** — bukti bahwa label
bisnis tidak pernah dibaca (NF-03).

### Feature flags bawaan

Tiga kunci, semuanya **mati** (`enabled = 0`):

| Kunci | Dugaan maksud |
|---|---|
| `whatsapp_assistant` | Mengaktifkan asisten WhatsApp |
| `knowledge_rag` | Mengaktifkan basis pengetahuan RAG |
| `multi_location_stock` | Mengaktifkan stok multi-lokasi |

Ketiganya menamai fitur yang **sudah berjalan** di sistem tanpa memeriksa flag ini.
**[PERLU KONFIRMASI]** apakah flag ini sisa rancangan awal (seperti prefix `vioni` dan konsep
multi-tenant) atau memang direncanakan dipakai.

### Nilai bawaan di frontend

Dipakai ketika pengaturan dari server kosong:

```
businessLabels        : itemLabel "Produk", orderLabel "Order", stockLabel "Stok",
                        customerLabel "Pelanggan"
operationalRules      : defaultOrderKind "sales", defaultDueDays 7,
                        criticalStockFocus "", stockAdjustmentApprovalMode "simple"
uiPreferences         : compactTable false, dashboardHighlight ""
aiPreferences         : tone "professional", responseLanguage "id", answerStyle "concise",
                        whatsapp_mode "rule_based", ai_daily_limit 50
reportingPreferences  : defaultRange "month", emphasizeMetric ""
featureFlags          : [] (selalu kosong)
```

Perhatikan `itemLabel` bawaan frontend adalah **"Produk"** sementara seed menyimpan **"Barang"** —
dua nilai berbeda untuk kunci yang sama, dan keduanya tidak pernah dipakai.
