# Data Model Legacy — Modul 02 Users, Roles & Permissions

**Kelompok B — cukup dipahami maksudnya, JANGAN ditiru strukturnya.** Dokumen ini menjelaskan
*data apa yang perlu ada* dan *kenapa*, bukan bentuk tabel yang harus disalin.

Berkas terkait: [algorithms-legacy.md](algorithms-legacy.md) ·
[business-rules.md](business-rules.md) (di sana aturannya presisi dan wajib dipertahankan)

Konvensi kolom umum (penamaan, timestamp, soft delete) sudah dibahas di
[shared-data-model.md](../shared/shared-data-model.md) §2 — tidak diulang di sini.

---

## 1. Tabel yang Dimiliki dan Disentuh

Modul ini adalah **pemilik** seluruh data identitas dan otorisasi. Modul lain hanya membacanya.

| Tabel | Peran modul ini | Pembaca lain |
|---|---|---|
| `users` | **Pemilik** — tulis & baca | `auth` (verifikasi login), banyak modul (nama pembuat dokumen) |
| `roles` | **Pemilik** | `auth` (resolusi role aktif) |
| `permissions` | **Baca saja** — diisi seed & migrasi | `auth` (resolusi permission per request) |
| `role_permissions` | **Pemilik** | `auth` (resolusi permission per request) |
| `user_roles` | **Pemilik** | `auth` (login, ganti role) |
| `user_branch_accesses` | **Pemilik** | `auth` (auto-pilih & verifikasi cabang) |
| `user_sessions` | **Tulis terbatas** — hanya mencabut | `auth` (pemilik penuh) |
| `branches` | Tidak dibaca sama sekali | modul 04 |

Dua catatan yang penting untuk rebuild:

**`permissions` adalah tabel referensi yang tidak dikelola UI.** Tidak ada endpoint untuk
menambah/menghapus permission. Isinya ditentukan **seed + 10 migrasi** (lihat §5), dan `roles/*`
hanya membacanya untuk validasi. Menambah permission baru di sistem ini menuntut migrasi SQL,
bukan aksi pengguna.

**Modul ini menulis ke tabel sesi milik modul lain.** Menonaktifkan pengguna dan mengganti
password mencabut sesi secara langsung. Ini kopling yang sah (perubahan kredensial harus memutus
sesi) tetapi berarti batas kepemilikan data tidak sebersih tampaknya.

---

## 2. Relasi Antar Tabel

```
companies
    │ 1                                    ┌──────────────┐
    ├──── n ──── users ──── n ──── user_roles ──── n ──── roles ──┐
    │              │                       └──────────────┘       │ 1
    │              │ n                                            │
    │              │                                              │ n
    │       user_branch_accesses                          role_permissions
    │              │ n                                            │ n
    │              │                                              │
    └──── n ──── branches                                   permissions
                                                                (referensi)
```

Tiga hubungan many-to-many, semuanya lewat tabel penghubung berkunci gabungan:

| Hubungan | Tabel penghubung | Atribut tambahan |
|---|---|---|
| Pengguna ↔ Role | `user_roles` | **tidak ada** |
| Role ↔ Permission | `role_permissions` | **tidak ada** |
| Pengguna ↔ Cabang | `user_branch_accesses` | `is_default_branch` |

**Rantai otorisasi lengkapnya:**

```
pengguna → (role yang dimilikinya) → (permission tiap role) = permission efektif
pengguna → (cabang yang boleh diakses) = ruang kerja
```

Yang perlu dipahami maksudnya: **permission tidak pernah melekat langsung ke pengguna.** Tidak ada
tabel `user_permissions`. Setiap penyesuaian hak harus lewat role — bila seorang pengguna butuh
satu izin tambahan, satu-satunya jalan adalah membuat role baru atau mengubah role yang sudah ada
(yang berdampak ke semua pemegangnya).

Konsekuensi praktisnya terlihat di data: role `kasir` lahir karena `staff` tidak cukup. Setiap
kebutuhan hak yang sedikit berbeda melahirkan satu role baru. Untuk UKM dengan sedikit peran ini
sehat; bila peran berkembang, jumlah role akan tumbuh cepat.

**[PERLU KONFIRMASI]** apakah kebutuhan "satu pengguna dengan pengecualian izin" pernah muncul di
lapangan. Bila ya, model role-only ini adalah batasan yang perlu diputuskan sadar di sistem baru.

---

## 3. Data yang Perlu Ada — per Entitas

### 3.1 Pengguna (`users`)

| Kebutuhan data | Kenapa ada |
|---|---|
| Perusahaan | Scope tenant untuk seluruh operasi |
| Nama lengkap | Identitas yang ditampilkan di mana-mana |
| Username | **Identifier login utama, unik** |
| Email | **Identifier login alternatif, unik, boleh kosong** |
| Sidik password | bcrypt |
| Nomor telepon | Kontak; tidak dipakai untuk autentikasi |
| Gelar/jabatan tampilan | Label organisasi, murni informatif |
| Status | Menentukan boleh login atau tidak |
| Waktu login terakhir | Ditulis modul auth |

Yang perlu dipahami maksudnya:

**Dua identifier login yang keduanya unik** memberi keleluasaan bagi pengguna (mengetik apa saja
yang ia ingat) tetapi menuntut dua pemeriksaan keunikan terpisah dan menghasilkan dua pesan error
berbeda. Keunikan email **nullable** menimbulkan satu masalah nyata: `NULL` boleh berulang, tetapi
**string kosong tidak** — dan jalur create tidak menormalkan kosong menjadi `NULL` sementara jalur
update melakukannya. Inilah akar KI-22. Di sistem baru, normalisasi nilai kosong harus terjadi di
**satu** tempat sebelum menyentuh basis data.

**Keunikan username bersifat global, tidak per-perusahaan.** Untuk sistem satu-perusahaan tidak
berdampak. Bila multi-perusahaan pernah menjadi target lagi, ini keputusan yang harus diambil
sadar: apakah dua perusahaan boleh punya pengguna bernama `admin`.

**Status disimpan sebagai teks bebas** dengan tiga nilai yang dikenali kode: `active`, `inactive`,
`locked`. Modul ini hanya bisa menetapkan dua yang pertama; `locked` tidak punya jalur UI.
Praktisnya seluruh kode hanya membandingkan `status === 'active'`, sehingga `inactive` dan `locked`
**berperilaku identik**. Ini menjawab pertanyaan M1-Q4 dari modul 01: dari sisi kode,
`locked` **tidak** punya arti berbeda — dan tidak ada mekanisme yang menghasilkannya. Di sistem
baru, pilih: satu status non-aktif saja, atau beri `locked` arti yang benar-benar berbeda
(mis. terkunci otomatis karena percobaan login gagal, perlu tindakan admin untuk membuka).

### 3.2 Role (`roles`)

| Kebutuhan data | Kenapa ada |
|---|---|
| Perusahaan (**boleh kosong**) | Kosong = role sistem global; terisi = role kustom perusahaan |
| Kode | Identitas fungsional, dipakai API & kebijakan |
| Nama | Label yang ditampilkan |
| Deskripsi | Penjelasan, boleh kosong |
| Penanda role sistem | Menentukan boleh diubah nama/dihapus atau tidak |

Yang perlu dipahami maksudnya:

**Dua kolom menyimpan informasi yang tumpang tindih.** "Role sistem" ditandai *dua* kali: oleh
`id_company` yang kosong **dan** oleh penanda boolean. Keduanya selalu bergerak bersama pada data
nyata, tetapi tidak ada batasan yang memaksanya. Di sistem baru, satu penanda cukup.

**Kode tidak pernah berubah setelah dibuat.** Ini keputusan yang tepat dan layak dipertahankan:
penetapan role di tabel penghubung merujuk id, sementara kebijakan (`superadmin`) merujuk kode —
mengizinkan kode berubah akan memutus keduanya. Efek sampingnya, nama dan kode bisa menyimpang
(role bernama "Manajer Gudang" dengan kode `kepala_gudang`), dan UI menampilkan keduanya
berdampingan sehingga penyimpangannya terlihat.

**Tidak ada kolom status.** Role tidak bisa dinonaktifkan sementara — hanya dihapus. Karena itu
penghapusan role menjadi **hard delete**, satu-satunya di seluruh sistem, dan dijaga oleh
pemeriksaan "sedang dipakai". Di sistem baru, status non-aktif untuk role akan menghilangkan
kebutuhan hard delete sekaligus menjaga jejak historis.

**Id role bawaan ditanam eksplisit** (1–4 untuk role sistem, 5 untuk `kasir`), dan beberapa
migrasi merujuk angka itu langsung. Jadi id role bawaan adalah bagian kontrak data, bukan nilai
yang bebas dihasilkan sistem. Ini rapuh — di sistem baru, rujuk role lewat kodenya.

### 3.3 Permission (`permissions`)

| Kebutuhan data | Kenapa ada |
|---|---|
| Kunci modul | Bagian pertama kode |
| Kunci aksi | Bagian kedua kode |
| **Kode gabungan (unik)** | Satu-satunya yang benar-benar dipakai kode |

Yang perlu dipahami maksudnya: tabel ini menyimpan **informasi yang sama tiga kali**. Kode
gabungan adalah penggabungan dua kolom lainnya, dan hanya kode gabungan yang pernah dibaca oleh
kode aplikasi. Dua kolom pecahan tidak punya satu pun pembaca.

Alasan keberadaannya bisa diduga (memudahkan pengelompokan per modul), tetapi pengelompokan yang
benar-benar dipakai UI justru **didefinisikan terpisah di frontend** dengan label bisnis
berbahasa Indonesia — bukan diturunkan dari kolom `module_key`. Jadi kolom itu tidak melayani
tujuannya. Di sistem baru, satu kode saja cukup.

Detail yang perlu diperhatikan bila kode gabungan dijadikan satu-satunya sumber: sebagian kode
punya **tiga** segmen (`finance.posting.post`, `stock.adjust.approve`), karena kunci modulnya
sendiri memuat titik. Jadi kode **tidak bisa** dipecah dengan asumsi "tepat satu titik".

### 3.4 Penetapan Role (`user_roles`)

| Kebutuhan data | Kenapa ada |
|---|---|
| Pasangan pengguna–role | Menyatakan "pengguna ini memegang role itu" |

Kunci gabungan sekaligus mencegah duplikat. Tidak ada atribut tambahan sama sekali.

Yang perlu dipahami maksudnya: **tidak ada penanda role utama.** Ini akar dua masalah yang sudah
terdokumentasi:

1. Role aktif saat login tidak deterministik untuk pengguna multi-role (modul 01 BR-06)
2. Tidak ada cara menyatakan "role ini yang utama, sisanya alternatif"

Di sistem baru, bila pengguna boleh memegang beberapa role, kebutuhan datanya jelas: **satu
penanda role default per pengguna**, atau aturan urutan yang eksplisit.

Catatan yang perlu diketahui dari data nyata: kelima akun bawaan masing-masing hanya memegang
**satu** role, sehingga kemampuan multi-role ini **belum pernah benar-benar dipakai**. Sebelum
merancangnya ulang, kepastian dari pemilik sistem lebih berharga daripada asumsi — tercatat
sebagai M1-Q1.

### 3.5 Penetapan Permission (`role_permissions`)

| Kebutuhan data | Kenapa ada |
|---|---|
| Pasangan role–permission | Menyatakan "role ini punya izin itu" |

Sama seperti `user_roles`: kunci gabungan, tanpa atribut. Tidak ada konsep izin **negatif**
(pengecualian/penolakan eksplisit) — model ini murni daftar-putih. Itu pilihan yang benar untuk
kesederhanaan dan layak dipertahankan.

### 3.6 Akses Cabang (`user_branch_accesses`)

Dibahas di [shared-data-model.md](../shared/shared-data-model.md) §3.4. Yang perlu ditambahkan
dari sudut modul ini:

**Penanda cabang default ditetapkan secara implisit dari urutan array**, bukan dipilih pengguna.
Karena penetapan selalu ganti-total, tepat satu cabang selalu bertanda default — sehingga
kekhawatiran "beberapa cabang default" di modul 01 tidak dapat terjadi lewat jalur ini.

Tetapi cara mencapainya bermasalah: pengguna tidak punya kendali atas cabang mana yang jadi
default, dan tidak ada tanda di UI. Di sistem baru, kebutuhan datanya tetap (satu default per
pengguna) tetapi **cara memilihnya harus eksplisit** — mis. radio button di samping daftar cabang.

---

## 4. Ketidaksesuaian Definisi Entity vs Skema Nyata

Sama seperti modul 01: seluruh tabel modul ini lahir di migrasi baseline yang memakai presisi waktu
**milidetik**, sementara definisi entity mendeklarasikan **mikrodetik**. Karena sinkronisasi skema
dimatikan permanen, basis data tidak pernah menyusul.

Berlaku untuk: `users`, `roles`, `user_branch_accesses`. Tabel `user_roles`, `role_permissions`,
dan `permissions` **tidak punya kolom waktu sama sekali**.

Dampak praktisnya nihil di modul ini (tidak ada kolom waktu yang ditampilkan atau dibandingkan),
tetapi perlu diketahui saat menyeragamkan presisi di sistem baru. Rincian di
[01-auth-session/data-model-legacy.md](../01-auth-session/data-model-legacy.md) §4.

---

## 5. Sumber Data Permission Tersebar di 11 Tempat

Ini temuan struktural yang paling perlu diperbaiki di sistem baru.

Isi tabel `permissions` — dan penetapan awalnya ke role — ditentukan oleh:

| Sumber | Isi |
|---|---|
| `seed-runner.ts` (daftar `allPermissions`) | **56** permission |
| Migrasi `009_payment_module` | permission pembayaran |
| Migrasi `017_stock_adjustment_approval_and_permissions` | `stock.adjust.approve` |
| Migrasi `022_finance_foundation` | permission finance dasar |
| Migrasi `029_cleanup_unused_permissions` | **menghapus** `stock.create` yang tak pernah dipakai |
| Migrasi `031_finance_business_expenses` | permission biaya usaha |
| Migrasi `032_finance_tax_adjustments` | permission penyesuaian pajak |
| Migrasi `038_sales_returns` | permission retur penjualan |
| Migrasi `040_member_pricing` | permission jenis member |
| Migrasi `041_order_export_permission` | `order.export` |
| Migrasi `042_reserved_superadmin_role_policy` | memberi `role.manage` ke role `admin` |
| Migrasi `048_finance_close_force_permission` | `finance.close.force` |
| Migrasi `050_purchase_returns` | **4** permission retur pembelian |

Sementara itu **sumber kebenaran keempat** ada di kode: union `PermissionCode` di shared package
(60 kode) dan matriks `DEFAULT_ROLE_PERMISSIONS`.

Yang perlu dipahami sebagai akibatnya:

**Daftar di seed tidak lengkap** (56 dari 60) dan itu tidak menimbulkan masalah **hanya karena**
urutan operasi: migrasi selalu dijalankan sebelum seed, sehingga saat seed memetakan
`DEFAULT_ROLE_PERMISSIONS` ke id permission, semua baris sudah tersedia. Bila seed dijalankan pada
basis data yang belum termigrasi, empat permission retur pembelian akan **dilewati diam-diam**
(pola "kode tidak ditemukan → skip" yang sama dengan KI-13).

**Seed bersifat menambah, tidak pernah mengurangi.** Penetapan permission memakai "sisipkan bila
belum ada". Konsekuensinya: permission yang sengaja **dihapus** dari role sistem lewat halaman
Role & Akses akan **muncul kembali** setiap kali seed dijalankan. Untuk lingkungan dev ini tidak
terasa; bila seed pernah dijalankan di production, konfigurasi hak yang sudah disesuaikan akan
kembali ke bawaan tanpa peringatan. Lihat KI-18.

Kebutuhan datanya sendiri sederhana: **satu daftar permission yang berlaku, dengan satu sumber
kebenaran.** Di sistem baru, daftar itu sebaiknya diturunkan dari kode (yang sudah menjadi union
bertipe) dan disinkronkan ke basis data oleh satu proses, bukan ditulis ulang di setiap migrasi
fitur.

---

## 6. Duplikasi Definisi Permission di Frontend

Di luar basis data, ada **sumber kebenaran kelima**: konfigurasi matriks di frontend yang
mengelompokkan 59 permission ke 22 kelompok modul dengan label bisnis berbahasa Indonesia.

Yang perlu dipahami maksudnya: label seperti "Tutup Buku Paksa (Force, Lewati Checklist)" atau
"Kas, Hutang/Piutang, Margin & Stok" adalah **pengetahuan bisnis** yang tidak ada di tempat lain.
Kode `finance.report.view` sendiri tidak memberi tahu bahwa izin itu mencakup empat laporan
sekaligus. Jadi konfigurasi ini bukan duplikasi sia-sia — ia menyimpan terjemahan dari kode teknis
ke bahasa pengguna.

Masalahnya adalah **tidak ada yang memaksa kedua sisi sinkron**. Menambah permission baru menuntut
edit di: union bertipe, migrasi SQL, matriks bawaan role, dan konfigurasi matriks UI. Ada satu
test yang memeriksa bahwa setiap permission bawaan muncul di matriks UI — itu jaring pengaman yang
baik, tapi hanya satu arah (tidak memeriksa arah sebaliknya).

Satu bukti nyata bahwa penjagaan itu tidak cukup: komentar di test menyebut `reporting.view`
sebagai permission yang "sengaja tidak ditampilkan di UI", padahal ia **ada** di matriks. Komentar
itu sudah tidak akurat dan tidak ada yang menangkapnya.

Di sistem baru, kebutuhan datanya jelas: **satu tempat yang mendefinisikan kode, kelompok, dan
label bisnis sekaligus**, dari mana basis data dan UI diturunkan.

---

## 7. Ringkasan untuk Perancangan Ulang

Kebutuhan data yang **wajib** ada, apa pun bentuknya:

| # | Kebutuhan | Kenapa |
|---|---|---|
| 1 | Pengguna dengan dua identifier login unik (username + email opsional) | Pengguna mengetik apa yang ia ingat |
| 2 | Status pengguna yang menentukan boleh login | Dasar penonaktifan |
| 3 | Role sebagai wadah izin, milik global atau satu perusahaan | Role sistem vs kustom (`kasir`) |
| 4 | Kode role yang stabil dan tidak berubah | Dirujuk kebijakan & API |
| 5 | Daftar izin sebagai referensi bertipe | Validasi & penegakan |
| 6 | Hubungan pengguna–role dan role–izin | Rantai otorisasi |
| 7 | Hubungan pengguna–cabang dengan satu penanda default | Auto-pilih cabang saat login |
| 8 | Penanda role sistem (tidak boleh diubah nama/dihapus) | Melindungi role bawaan |
| 9 | Kemampuan mencabut sesi dari luar modul auth | Penonaktifan & ganti password harus memutus akses |

Kebutuhan data yang **hilang** dan layak ditambahkan — masing-masing punya konsekuensi yang sudah
terdokumentasi:

| # | Yang hilang | Konsekuensi di sistem lama |
|---|---|---|
| A | Penanda role utama per pengguna | Role aktif saat login tidak deterministik (modul 01 BR-06) |
| B | Pemilihan cabang default yang eksplisit | Default ditentukan urutan array, tak terlihat (BR-14, KI-20) |
| C | Batasan "pengguna harus punya minimal satu role" | Pengguna bisa terkunci dari sistem (KI-13) |
| D | Batasan "pengguna harus punya minimal satu cabang" | Pengguna terjebak tanpa ruang kerja |
| E | Status non-aktif untuk role | Memaksa hard delete, jejak historis hilang |
| F | Riwayat perubahan role & izin | Audit tidak mencatat perubahan hak (BR-22) |
| G | Satu sumber kebenaran daftar izin | Tersebar di seed + 10 migrasi + 2 tempat di kode |
| H | Satu tempat untuk kode + kelompok + label izin | UI dan basis data bisa menyimpang tanpa terdeteksi |
| I | Arti yang berbeda untuk `locked` vs `inactive`, atau hapus salah satu | Dua nilai berperilaku identik |

Yang sebaiknya **disederhanakan**:

| # | Penyederhanaan | Alasan |
|---|---|---|
| J | Satu penanda saja untuk role sistem | Sekarang `id_company` kosong **dan** boolean, redundan |
| K | Satu kolom kode izin | Kunci modul & aksi tidak punya pembaca |
| L | Rujuk role lewat kode, bukan id tetap 1–5 | Id yang ditanam keras di migrasi itu rapuh |
