# Data Model Legacy — Modul 03 Company & Settings

**Kelompok B — cukup dipahami maksudnya, JANGAN ditiru strukturnya.** Dokumen ini menjelaskan
*data apa yang perlu ada* dan *kenapa*, bukan bentuk tabel yang harus disalin.

Berkas terkait: [algorithms-legacy.md](algorithms-legacy.md) ·
[business-rules.md](business-rules.md) (di sana aturannya presisi dan wajib dipertahankan)

Konvensi kolom umum sudah dibahas di
[shared-data-model.md](../shared/shared-data-model.md) §2 — tidak diulang di sini.

---

## 1. Tabel yang Dimiliki dan Disentuh

| Tabel | Peran modul ini | Pembaca lain |
|---|---|---|
| `companies` | **Pemilik** — tulis & baca | `auth` (data perusahaan saat login) |
| `company_settings` | **Pemilik** | **Modul Stock** (mode persetujuan), **Modul WhatsApp** (preferensi asisten), **Modul Order** (profil dokumen cetak — lewat data yang dimuat frontend) |
| `company_features` | **Pemilik** | **tidak ada** |

Modul ini adalah pemilik seluruh konfigurasi tingkat perusahaan. Yang menarik: **dua dari tiga
tabel punya pembaca dari modul lain, satu tidak punya pembaca sama sekali.**

---

## 2. Relasi Antar Tabel

```
                    companies
                   (id_company)
                        │ 1
            ┌───────────┴───────────┐
            │ 1                     │ n
    company_settings          company_features
   (PK = id_company)      (PK sendiri + unik per kunci)
```

Dua bentuk hubungan yang berbeda, dan pembedaannya masuk akal:

| Hubungan | Bentuk | Alasan |
|---|---|---|
| Perusahaan ↔ Pengaturan | **Satu-ke-satu**, kunci utama sama | Setiap perusahaan punya tepat satu set pengaturan |
| Perusahaan ↔ Feature flag | **Satu-ke-banyak**, unik per pasangan | Jumlah flag bisa bertambah |

Yang perlu dipahami maksudnya: **pengaturan bukan entitas tersendiri.** Ia satelit yang tidak punya
identitas sendiri — dialamatkan lewat perusahaannya. Ini keputusan yang tepat untuk relasi
satu-ke-satu wajib, dan konsekuensinya terlihat di API: tidak ada `id` pengaturan yang pernah
dikirim atau diterima.

---

## 3. Data yang Perlu Ada — per Entitas

### 3.1 Perusahaan (`companies`)

| Kebutuhan data | Kenapa ada | Benar-benar dipakai? |
|---|---|---|
| Kode | Identitas singkat, unik | **Tidak** — tidak muncul di dokumen maupun UI |
| Nama | Identitas yang ditampilkan | **Ya** — chip topbar, fallback kepala dokumen cetak |
| Nama legal | Nama resmi badan usaha | **[PERLU KONFIRMASI]** — tersimpan, tidak ditemukan pembacanya |
| Zona waktu | Seharusnya menentukan tampilan waktu | **Tidak** — aplikasi memakai env |
| Mata uang | Seharusnya menentukan format uang | **Tidak** — aplikasi memakai `IDR` tetap |
| Locale | Seharusnya menentukan format angka/tanggal | **Tidak** — aplikasi memakai `id-ID` tetap |
| Status | Aktif/tidaknya perusahaan | **Tidak** — di-seed `active`, tidak pernah dibaca maupun bisa diubah |

**Empat dari tujuh field tidak punya pembaca.** Ini bukan sekadar kolom tak terpakai — tiga di
antaranya (zona waktu, mata uang, locale) **ditampilkan di form sebagai field yang bisa diubah**,
sehingga pengguna wajar mengira mengubahnya berpengaruh.

Keputusan untuk membuang ketiganya di sistem baru sudah diambil (lihat
[shared-business-rules.md](../shared/shared-business-rules.md) §2.5): locale dan zona waktu menjadi
konstanta aplikasi.

Untuk **kode perusahaan**, keputusan belum diambil. Ia wajib diisi dan dijaga keunikannya, tetapi
tidak dipakai untuk apa pun yang terlihat — nomor dokumen memakai kode **cabang**, bukan kode
perusahaan.

Satu ketidaksesuaian yang perlu diketahui: penjagaan keunikan kode di aplikasi membandingkan string
**apa adanya**, sementara indeks unik basis data memakai collation yang **tidak peka huruf besar/
kecil**. Untuk sistem satu-perusahaan kondisi itu tidak pernah tercapai, tetapi keduanya tidak
sepakat.

### 3.2 Pengaturan (`company_settings`)

Struktur nyatanya sederhana: kunci utama + **lima kolom JSON** + satu waktu perubahan.

| Kebutuhan data | Isi konseptual | Punya pembaca? |
|---|---|---|
| Label bisnis | Nama tampilan untuk "produk", "order", "stok", "pelanggan" | **Tidak sama sekali** |
| Aturan operasional | Kebijakan kerja lintas cabang | **Satu kunci saja** (mode persetujuan koreksi stok) |
| Preferensi tampilan | Pengaturan antarmuka | **Satu kunci saja** (profil dokumen cetak) |
| Preferensi asisten | Perilaku bot WhatsApp | **Tiga kunci** (kuota harian, mode, preferensi perilaku) |
| Preferensi pelaporan | Rentang & metrik bawaan laporan | **Tidak sama sekali** |

Yang perlu dipahami maksudnya, dan ini temuan paling penting di modul ini:

**Lima blok JSON menyimpan sekitar 18 kunci, dan hanya 5 yang punya pembaca.** Dua blok penuh
(`label bisnis` dan `preferensi pelaporan`) sepenuhnya mati. Bahkan blok yang hidup pun sebagian
besar isinya tidak dibaca — "aturan operasional" punya empat kunci di bentuk bawaan, tetapi hanya
satu yang berarti.

Bukti paling jelas bahwa label bisnis tidak pernah dibaca: seed menyimpan `itemLabel: "Barang"`,
bentuk bawaan frontend memakai `itemLabel: "Produk"`, dan seluruh antarmuka menulis **"Produk"**
secara tetap. Tiga nilai berbeda untuk satu kunci, tanpa ada yang menyadarinya — karena tidak ada
yang membacanya.

**Kenapa kolom JSON dipilih, dan kapan itu tepat.** Menyimpan konfigurasi sebagai JSON bebas
memungkinkan menambah setelan baru tanpa migrasi. Untuk profil dokumen cetak — yang berisi daftar
telepon, daftar rekening, daftar barang, dan blok kalibrasi kertas bersarang — pilihan itu **tepat**:
strukturnya bersarang, jumlahnya bervariasi, dan tidak pernah di-query per-field.

Untuk setelan skalar seperti mode persetujuan koreksi stok, pilihan itu **membawa biaya**: tidak
ada tipe, tidak ada batasan nilai, dan modul yang membacanya harus menormalkan sendiri setiap kali.
Itulah sebabnya aturan normalisasi yang sama ditulis di dua tempat (klien dan modul Stock).

**Ketiadaan waktu pembuatan.** Tabel ini punya waktu perubahan tetapi **tidak punya waktu
pembuatan** — konsekuensi wajar dari satelit satu-ke-satu, tetapi berarti tidak mungkin tahu kapan
pengaturan pertama kali dibuat.

**Ketidakcocokan nama.** Blok preferensi asisten disebut `ai_preferences` di API, tetapi kolomnya
bernama `assistant_preferences`. Adapter frontend menerima **empat ejaan** berbeda saat membacanya —
tanda bahwa nama ini pernah berubah dan kompatibilitasnya ditumpuk alih-alih dibereskan.

### 3.3 Feature Flag (`company_features`)

| Kebutuhan data | Isi |
|---|---|
| Identitas baris | Auto-increment sendiri |
| Perusahaan | Scope |
| Kunci fitur | Teks bebas, unik per perusahaan |
| Aktif/tidak | Boolean, bawaan mati |
| Konfigurasi | JSON bebas, opsional |
| Waktu dibuat & diubah | Ada keduanya |

Yang perlu dipahami maksudnya: **seluruh tabel ini mati.**

| Lapisan | Kondisi |
|---|---|
| Basis data | Tiga baris di-seed, semuanya **mati** |
| API | Dua endpoint ada dan berfungsi |
| Pemanggil API | **Nol** — tidak ada UI maupun kode yang memanggilnya |
| Pembaca nilai | **Nol** — tidak ada kode yang memeriksa apakah sebuah flag aktif |
| Jalur data ke frontend | **Diblokir** — adapter selalu memaksa daftar flag menjadi kosong |

Tiga kunci yang di-seed (`whatsapp_assistant`, `knowledge_rag`, `multi_location_stock`) menamai
fitur yang **sudah berjalan** tanpa memeriksa flag ini. Jadi flag-nya bukan sekadar belum dipakai —
ia menamai sesuatu yang sudah diputuskan lain.

Ini pola yang sama dengan prefix penyimpanan `vioni` dan kolom `guid`/`code`/`info` di envelope
permintaan: fosil dari rancangan awal yang arahnya berubah.

Dua kelemahan struktural yang perlu diketahui bila sistem flag dihidupkan kembali:

1. **Tidak ada daftar kunci yang sah.** Operasi update membuat baris untuk kunci apa pun yang
   dikirim, sehingga salah ketik menghasilkan flag baru alih-alih error.
2. **Tidak ada cara menghapus.** Tidak ada endpoint hapus, sehingga flag yang terlanjur dibuat
   tersimpan permanen.

Ketiga masalah itu — kunci bebas, tanpa hapus, tanpa pembaca — menunjukkan sistem ini tidak pernah
selesai dirancang.

---

## 4. Ketidaksesuaian Definisi Entity vs Skema Nyata

Berbeda dari modul 01 dan 02, ketiga tabel modul ini **tidak** punya masalah presisi waktu —
seluruhnya lahir di migrasi baseline dengan presisi milidetik, dan entity mendeklarasikan
mikrodetik, tetapi **tidak ada satu pun kolom waktu di modul ini yang ditampilkan atau
dibandingkan**. Jadi ketidaksesuaian itu benar-benar tanpa dampak di sini.

Yang **ada** dampaknya adalah ketidaksesuaian nama blok preferensi asisten (§3.2), dan
ketidaksesuaian penjagaan keunikan kode perusahaan (§3.1).

---

## 5. Bagaimana Data Ini Sampai ke Pengguna

Bagian ini penting karena menjelaskan mengapa dua endpoint baca tidak pernah dipanggil.

```
Saat login / pemulihan sesi:
    profil perusahaan  →  disimpan ke ringkasan sesi di browser
    pengaturan         →  dimuat sebagai bagian bootstrap aplikasi

Saat membuka halaman Pengaturan:
    form profil        ←  dibaca dari ringkasan sesi
    form pengaturan    ←  dibaca dari data bootstrap
    (tidak ada permintaan baca ke server)
```

Yang perlu dipahami maksudnya: **data konfigurasi diperlakukan sebagai bagian konteks sesi**, bukan
sebagai sumber daya yang diambil saat dibutuhkan. Alasannya masuk akal — nama perusahaan dibutuhkan
di topbar setiap halaman, dan profil dokumen cetak dibutuhkan halaman cetak yang bisa dibuka
kapan saja.

Konsekuensi yang perlu diketahui: **nilai di form bisa basi.** Perubahan dari sesi lain atau
langsung di basis data tidak terlihat sampai aplikasi dimuat ulang. Untuk sistem satu-perusahaan
dengan sedikit administrator, risikonya kecil — tetapi ia menjelaskan mengapa dua endpoint baca
menjadi tidak terpakai.

Di sistem baru, kebutuhannya tetap sama (konfigurasi harus tersedia sebelum halaman dirender), dan
pilihan yang lebih bersih adalah **satu permintaan bootstrap yang mengembalikan seluruh konteks**
— sekaligus menyelesaikan masalah serupa di modul 01 (data perusahaan yang bertahan hanya karena
tersimpan di browser).

---

## 6. Ringkasan untuk Perancangan Ulang

Kebutuhan data yang **wajib** ada, apa pun bentuknya:

| # | Kebutuhan | Kenapa |
|---|---|---|
| 1 | Identitas perusahaan yang ditampilkan (nama) | Muncul di topbar dan sebagai fallback kepala dokumen cetak |
| 2 | Kebijakan persetujuan koreksi stok | Satu-satunya setelan operasional dengan efek terverifikasi |
| 3 | Profil dokumen cetak (kop, telepon, rekening, daftar jual, catatan, penutup) | Menentukan isi nota, struk, dan surat jalan |
| 4 | Kalibrasi kertas kontinu | Terverifikasi lewat tes cetak fisik; angka-angkanya tidak boleh berubah |
| 5 | Kuota & mode asisten WhatsApp | Dibaca modul WhatsApp |
| 6 | Riwayat perubahan konfigurasi | Sudah tercapai lewat audit yang lengkap |

Kebutuhan yang **hilang** atau **berlebih**:

| # | Temuan | Arah perbaikan |
|---|---|---|
| A | Empat field perusahaan tanpa pembaca (zona waktu, mata uang, locale, status) | Tiga pertama sudah diputuskan dibuang; status perlu diputuskan |
| B | Dua blok pengaturan penuh tanpa pembaca (label bisnis, preferensi pelaporan) | Buang, atau hidupkan bila memang direncanakan |
| C | Seluruh tabel feature flag tanpa pembaca | **[PERLU KONFIRMASI]** — buang atau rancang ulang |
| D | Kode perusahaan wajib tetapi tidak dipakai | **[PERLU KONFIRMASI]** — apakah dipakai di luar sistem |
| E | Nilai skalar disimpan sebagai JSON tanpa tipe | Setelan skalar sebaiknya berkolom sendiri; yang bersarang (profil dokumen) tetap JSON |
| F | Nama blok asisten berbeda antara API dan kolom | Seragamkan satu nama |
| G | Tidak ada waktu pembuatan pengaturan | Tambahkan bila riwayat dibutuhkan |
| H | Tidak ada daftar kunci flag yang sah & tidak ada cara menghapus | Bila flag dihidupkan, kunci harus berupa daftar tertutup |
| I | Penjagaan keunikan kode aplikasi vs basis data tidak sepakat | Seragamkan (normalkan huruf, atau bandingkan tanpa peka huruf) |

Yang layak **dipertahankan sebagai konsep**:

| # | Konsep | Alasan |
|---|---|---|
| J | Pengaturan sebagai satelit satu-ke-satu tanpa identitas sendiri | Tepat untuk relasi wajib satu-ke-satu |
| K | JSON untuk struktur bersarang & berdaftar (profil dokumen cetak) | Strukturnya bervariasi dan tidak pernah di-query per-field |
| L | Konfigurasi tersedia sebagai bagian konteks sesi | Dibutuhkan sebelum halaman dirender — tetapi sebaiknya lewat satu bootstrap, bukan gabungan dua sumber |
| M | Audit mencatat seluruh isi sebelum & sesudah | Audit paling lengkap di sistem; jadikan acuan untuk modul lain |
