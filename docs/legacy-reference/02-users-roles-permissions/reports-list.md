# Reports List — Modul 02 Users, Roles & Permissions

**Kelompok A.** Dokumen ini mencatat laporan yang dihasilkan modul ini beserta logika
perhitungannya.

---

## 1. Kesimpulan: Modul Ini Tidak Menghasilkan Laporan

**Tidak ada laporan di modul Users, Roles & Permissions.** Diverifikasi, bukan diasumsikan:

| Yang diperiksa | Hasil |
|---|---|
| 10 endpoint modul | Semuanya CRUD master data — tidak ada agregat, rekap, atau perhitungan |
| Halaman frontend | 4 rute: satu daftar master data, dua form, satu halaman pengaturan |
| Permission bertipe laporan | Tidak ada. Modul memakai `user.*` dan `role.manage` |
| Menu sidebar | Kedua entri masuk grup **"Pengaturan"**, bukan "Pantauan" |
| Endpoint ekspor | Tidak ada (bandingkan `orders/export`, `finance/export/tax-package`) |
| Halaman cetak | Tidak ada |
| Perhitungan | Hanya paginasi dan `Math.ceil(total/limit)` — bukan logika bisnis |

Daftar pengguna (`/users`) adalah **tampilan master data berpaginasi**, bukan laporan: tidak ada
periode, tidak ada agregasi, tidak ada total selain jumlah baris.

---

## 2. Satu-satunya Angka yang Dihitung

Untuk kelengkapan, inilah seluruh "perhitungan" yang ada di modul ini:

| Angka | Rumus | Tampil di |
|---|---|---|
| Jumlah pengguna | `COUNT(*)` hasil query daftar, **setelah** penyaringan superadmin | Deskripsi kartu: `{total} user perusahaan beserta role dan akses cabangnya.` |
| Rentang baris | `dari = (page-1)*limit + 1`, `sampai = min(page*limit, total)` | Bilah paginasi: `{dari}–{sampai} dari {total} data` |
| Total halaman | `Math.ceil(total / 20)` | Bilah paginasi: `{halaman} / {totalHalaman}` |
| Jumlah sesi tercabut | `COUNT` baris terpengaruh pencabutan | **Tidak ditampilkan** — hanya masuk metadata audit |

Perhatikan angka pertama: karena penyaringan role `superadmin` terjadi **di dalam query**, jumlah
yang dilihat sesi `owner` dan sesi `superadmin` **berbeda** untuk perusahaan yang sama. Ini
konsisten dengan baris yang tampil, jadi bukan ketidakcocokan — tetapi perlu diketahui bila angka
itu dipakai untuk rekonsiliasi.

Angka keempat layak dicatat: `users/change-password` mengembalikan `revoked_sessions` dalam
responsnya, tetapi UI **membuangnya**. Admin tidak pernah tahu berapa perangkat yang terputus
akibat aksinya.

---

## 3. Modul Ini Memasok Data untuk Laporan Modul Lain

Meski tidak menghasilkan laporan sendiri, modul ini menulis data yang **dikonsumsi** halaman
laporan modul lain. Ini yang perlu diketahui agar tidak salah kira letak fiturnya.

### 3.1 Audit log (modul 20 Observability)

Modul ini menulis **8 jenis `actionKey`** ke `audit_logs`, dan semuanya tampil di halaman
**Riwayat Aktivitas** (`/audit-logs`, permission `audit_log.view`):

| `actionKey` | Peristiwa |
|---|---|
| `user.create` | Pengguna ditambahkan |
| `user.update` | Profil pengguna diubah |
| `user.update_status` | Pengguna diaktifkan/dinonaktifkan |
| `user.change_password` | Password pengguna diganti |
| `role.create` | Role dibuat |
| `role.update` | Role diubah |
| `role.delete` | Role dihapus |
| `role.permissions.update` | Permission role diubah |

Berbeda dari modul auth (yang EXEMPT dari audit log), modul ini **wajib** mencatat setiap
operasi tulis — dan memang melakukannya di kedelapan jalur.

**Batasan yang perlu diketahui saat membaca riwayat itu** (rincian di
[business-rules.md](business-rules.md) BR-22):

| Yang tidak terjawab dari audit | Penyebab |
|---|---|
| Role apa yang ditambahkan/dicabut dari seorang pengguna | Perubahan role **tidak dicatat sama sekali** |
| Cabang apa yang ditambahkan/dicabut | Perubahan akses cabang **tidak dicatat sama sekali** |
| Nilai baru email/username/telepon/jabatan setelah diedit | `after` hanya memuat `fullName` |
| Permission apa yang **dicabut** dari sebuah role | Hanya daftar permission baru yang dicatat, bukan yang lama |

Jadi untuk pertanyaan pengawasan yang paling penting — "siapa memberi hak akses apa kepada
siapa" — audit log saat ini **tidak bisa menjawabnya**, meski peristiwanya tercatat.
**[PERLU KONFIRMASI]** apakah ini perlu dilengkapi di sistem baru.

### 3.2 Nama pengguna pada dokumen & riwayat modul lain

Daftar pengguna yang dimuat modul ini mengisi `userRecords` di store, yang dipakai lintas modul
untuk menerjemahkan `id_user` menjadi nama pada nota, struk, dan riwayat status.

Ada satu perilaku khusus yang tercatat di adapter: daftar itu **hanya terisi bila sesi punya
`user.view`**. Untuk role tanpa izin itu (mis. `kasir`), pencarian nama akan gagal — sehingga
disediakan jalan pintas: bila id yang dicari **sama dengan akun yang sedang login**, nama akun
aktif dipakai. Tujuannya agar kasir tetap melihat namanya sendiri di struk alih-alih `-`, tanpa
risiko salah atribusi ke user lain.

### 3.3 Basis data permission untuk penegakan akses

Tabel `permissions` dan `role_permissions` yang dikelola modul ini adalah sumber yang dibaca
**setiap request** oleh verifikasi sesi (modul 01 BR-08). Jadi perubahan di halaman Role & Akses
langsung memengaruhi seluruh sistem tanpa jeda.

---

## 4. Bahan Mentah bila Laporan Ingin Dibuat

Dicatat sebagai inventaris data, **bukan** usulan fitur. Jangan diimplementasikan tanpa permintaan.

| Pertanyaan operasional | Bisa dijawab sekarang? | Catatan |
|---|---|---|
| Berapa pengguna aktif vs non-aktif? | **Sebagian** — kolom status ada, tapi tidak ada endpoint yang menghitungnya; harus dihitung manual dari daftar |
| Siapa saja yang memegang role tertentu? | **Tidak langsung** — daftar user memuat rolenya, tapi tidak ada filter berdasarkan role |
| Role mana yang tidak dipakai siapa pun? | **Tidak** — hitungan pemakaian hanya dilakukan internal saat mencoba menghapus role, tidak diekspos |
| Permission apa yang dimiliki seorang pengguna (gabungan semua rolenya)? | **Tidak** — UI hanya menampilkan permission per-role, tidak per-user |
| Kapan seorang pengguna terakhir login? | **Tidak** — `users.last_login_at` tersimpan tapi **tidak pernah dikembalikan** oleh `users/list` maupun ditampilkan di UI mana pun |
| Siapa yang mengubah hak akses seseorang, kapan? | **Tidak** — lihat §3.1 |
| Berapa pengguna per cabang? | **Tidak langsung** — `branchIds` ada di daftar, tapi tidak diagregasi |

Baris `last_login_at` menjawab pertanyaan M1-Q8 dari modul 01: **tidak**, modul Users tidak
menampilkannya. Kolom itu ditulis setiap login tetapi tidak pernah dibaca oleh kode mana pun
selain penulisnya sendiri — praktis data mati.

Yang **paling sering dibutuhkan** dari daftar di atas dan paling murah ditambahkan adalah
"permission efektif seorang pengguna" — datanya sudah lengkap (`user_roles` × `role_permissions`),
hanya belum ada tampilannya.

---

## 5. Modul Lain yang Menampilkan Data Bernuansa Pengguna

Agar tidak salah kira letak fiturnya:

| Fitur | Modul pemilik |
|---|---|
| Halaman **Riwayat Aktivitas** (`/audit-logs`) | Modul 20 Observability |
| Login, sesi, ganti role & cabang aktif | Modul 01 Auth & Session |
| Daftar & pengaturan cabang | Modul 04 Branch |
| Pengaturan profil perusahaan | Modul 03 Company |
| Approval koreksi stok (memakai kredensial user) | Modul 16 Stock |
