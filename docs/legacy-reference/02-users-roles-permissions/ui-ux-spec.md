# UI/UX Spec — Modul 02 Users, Roles & Permissions

**Kelompok A — kontrak tampilan yang harus SAMA PERSIS di sistem baru.** Semua teks, urutan
field, dan label dikutip apa adanya dari kode.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md)

**Catatan bahasa:** berbeda dari modul 01 (halaman auth campur Inggris–Indonesia), seluruh UI
modul ini **berbahasa Indonesia** — kecuali satu titik: nilai status pengguna tampil sebagai
`active` / `inactive` mentah (lihat §2.6).

Token warna dan komponen bersama (`Badge`, `Button`, `DataTable`, `SectionCard`, `PageHeader`,
`Notice`, `EmptyState`, `FilterBar`, `ActionRow`, `Pagination`, `PasswordInput`) dijelaskan di
[shared-services.md](../shared/shared-services.md) §5 dan
[01-auth-session/ui-ux-spec.md](../01-auth-session/ui-ux-spec.md) §1.

---

## 1. Peta Screen

| Rute | Judul halaman | Permission | Menu sidebar |
|---|---|---|---|
| `/users` | Pengguna | `user.view` | Pengaturan → **"Pengguna"** (ikon `users`) |
| `/users/create` | Pengguna | `user.create` | — |
| `/users/:userId/edit` | Pengguna | `user.update` | — |
| `/settings/roles` | Pengaturan | `role.manage` | Pengaturan → **"Role & Akses"** (ikon `roles`) |

Ketiga rute `/users*` memakai judul halaman yang sama ("Pengguna"), sehingga label di topbar tidak
berubah saat masuk form. Rute `/settings/roles` berjudul "Pengaturan" — bukan "Role & Akses"
seperti label menunya.

---

## 2. Screen: `/users` — Daftar Pengguna

Tata letak vertikal, jarak antar-blok `gap-6`: PageHeader → FilterBar → SectionCard(tabel).

### 2.1 PageHeader

| Elemen | Isi |
|---|---|
| Breadcrumb | **"Dashboard > Pengguna"** |
| Judul | **"Pengguna"** |
| Deskripsi | **"Kelola anggota tim, role yang dimiliki, dan cabang yang dapat mereka akses."** |
| Aksi | Tombol primary **"Tambah Pengguna"** → `/users/create` — **hanya bila punya `user.create`** |

### 2.2 FilterBar

Satu kotak, kartu ber-border `hairline`, latar `surface`, padding `px-6 py-4`.

| Elemen | Isi |
|---|---|
| Label | **"Cari pengguna"** — 0.85rem, `font-medium`, warna `heading` |
| `id` / `htmlFor` | `user-search` |
| Placeholder | **"Cari nama, username, atau email"** |
| Perilaku | Setiap ketikan langsung memicu pemuatan ulang **dan mereset halaman ke 1** |

### 2.3 SectionCard tabel

| Elemen | Isi |
|---|---|
| Judul | **"Anggota Tim"** |
| Deskripsi saat memuat | **"Memuat…"** (memakai karakter ellipsis tunggal `…`) |
| Deskripsi saat selesai | `{total} user perusahaan beserta role dan akses cabangnya.` |

### 2.4 Kolom tabel (urutan WAJIB dipertahankan)

| # | Header | Isi |
|---|---|---|
| 1 | **"Nama"** | Dua baris: nama lengkap (`<strong>`, warna `ink`) di atas, email (warna `muted`) di bawah. Bila email kosong → **`-`** |
| 2 | **"Jabatan"** | Nilai `display_title` |
| 3 | **"Role"** | Satu `Badge tone="neutral"` per role, label lewat `formatRoleLabel`, `flex-wrap`, `gap-2` |
| 4 | **"Cabang"** | Satu `Badge tone="info"` per cabang berisi **kode** cabang |
| 5 | **"Status"** | Badge status + tombol toggle (lihat §2.6) |
| 6 | **"Aksi"** | Tombol secondary **"Detail"** → `/users/{id}/edit` — hanya bila punya `user.update`; bila tidak, sel kosong |

Baris tabel **tidak** bisa diklik — navigasi hanya lewat tombol "Detail".

### 2.5 Format label role

`formatRoleLabel` — sama seperti modul 01:

| Kode | Tampilan |
|---|---|
| `superadmin` | Superadmin |
| `owner` | Owner |
| `admin` | Admin |
| `staff` | Staff |
| lain, mis. `kasir` | Kasir |
| lain, mis. `kepala_gudang` | Kepala Gudang |

### 2.6 Kolom Status

Dua elemen berdampingan (`flex-wrap`, `gap-2`):

| Elemen | Aturan |
|---|---|
| Badge | Tone **`success`** (hijau) bila status `active`, **`warning`** (kuning) bila selain itu |
| Teks badge | **Nilai mentah** — `active` atau `inactive`, dalam bahasa Inggris |
| Tombol | Variant **`ghost`**. Label **"Nonaktifkan"** bila status aktif, **"Aktifkan"** bila tidak. Hanya tampil bila punya `user.archive` |

Tombol bekerja **tanpa dialog konfirmasi** — satu klik langsung mengubah status.

### 2.7 Empty state

Muncul bila pemuatan selesai dan tidak ada baris.

| Elemen | Isi |
|---|---|
| Judul | **"Belum ada pengguna"** |
| Deskripsi | **"Tambahkan anggota tim pertama agar akses cabang dan role bisa diatur."** |
| Aksi | Tombol **"Tambah Pengguna"** — hanya bila punya `user.create` |

Saat empty state tampil, **paginasi tidak dirender**.

### 2.8 Paginasi

Batas tetap **20 baris per halaman**. Bilah di bawah tabel, border atas `hairline`, latar
`surface`, padding `px-4 py-3`.

| Bagian | Isi |
|---|---|
| Teks kiri saat memuat | **"Memuat…"** |
| Teks kiri saat kosong | **"Tidak ada data"** |
| Teks kiri normal | `{dari}–{sampai} dari {total} data` — memakai **en dash** (`–`), bukan tanda hubung |
| Tombol kiri | **"← Prev"** — nonaktif di halaman 1 atau saat memuat |
| Indikator tengah | `{halaman} / {totalHalaman}` — `min-w-[80px]`, tebal, terpusat |
| Tombol kanan | **"Next →"** — nonaktif di halaman terakhir atau saat memuat |

Blok navigasi hanya dirender bila total halaman > 1 **atau** halaman saat ini > 1.

### 2.9 Perilaku kegagalan

Bila pemuatan daftar gagal, tabel dikosongkan **tanpa pesan, tanpa toast, tanpa notice**. Yang
terlihat user: empty state "Belum ada pengguna" — seolah perusahaan tidak punya pengguna.

Sebaliknya, kegagalan toggle status **memunculkan** toast danger **"Gagal mengubah status
pengguna"** tanpa deskripsi.

---

## 3. Screen: `/users/create` dan `/users/:userId/edit` — Form Pengguna

Satu komponen melayani kedua mode; yang membedakan hanya keberadaan data user.

### 3.1 PageHeader

| Elemen | Mode tambah | Mode edit |
|---|---|---|
| Breadcrumb | **"Dashboard > Pengguna > Tambah"** | **"Dashboard > Pengguna > Edit"** |
| Judul | **"Tambah Pengguna"** | **"Edit Pengguna"** |
| Deskripsi | **"Atur profil user, role yang dimiliki, dan daftar cabang yang boleh diakses."** | sama |
| Aksi | Tombol ghost **"← Kembali"** → kembali satu langkah riwayat | sama |

Panah pada tombol memakai entitas `&larr;`.

### 3.2 Kartu 1 — "Data Pengguna"

Deskripsi kartu: **"Profil utama anggota tim."**

Grid responsif: `repeat(auto-fit, minmax(280px, 1fr))`, `gap-5`.

**Urutan field (WAJIB dipertahankan):**

| # | Label | `id`/`name` | Tipe | Wajib | Catatan |
|---|---|---|---|---|---|
| 1 | **"Nama lengkap"** | `fullName` | text | ✔ | |
| 2 | **"Jabatan"** | `displayTitle` | text | ✔ | |
| 3 | **"Email"** | `email` | **email** | ✔ | Validasi format oleh browser |
| 4 | **"Username"** | `username` | text | ✔ | |
| 5 | **"No. WhatsApp"** | `phone` | text | ✘ | Satu-satunya field opsional |
| 6 | **"Password awal"** | `password` | password | ✔ | **Hanya mode tambah**; `minLength 8`, `autoComplete="new-password"` |
| 7 | **"Konfirmasi password"** | `passwordConfirm` | password | ✔ | **Hanya mode tambah**; `minLength 8` |

Label memakai gaya seragam: 0.85rem, `font-medium`, warna `heading`, jarak `gap-1.5` ke input.
Input memakai kelas `INPUT` kanonik design system.

Field 6 dan 7 memakai komponen `PasswordInput` dengan toggle ikon mata (`aria-label`
"Tampilkan password" / "Sembunyikan password").

### 3.3 Kartu 2 — "Ubah Password" (hanya mode edit)

| Elemen | Isi |
|---|---|
| Judul | **"Ubah Password"** |
| Deskripsi | **"Isi hanya jika password pengguna perlu diganti."** |
| Field 1 | Label **"Password baru"**, `name="password"`, placeholder **"Kosongkan jika tidak diubah"**, `minLength 8`, **tidak** wajib |
| Field 2 | Label **"Konfirmasi password baru"**, `name="passwordConfirm"`, placeholder sama, `minLength 8`, **tidak** wajib |

Tata letak grid sama seperti kartu 1.

### 3.4 Kartu 3 — "Role yang Dimiliki"

| Elemen | Isi |
|---|---|
| Judul | **"Role yang Dimiliki"** |
| Deskripsi | **"Satu user dapat memiliki lebih dari satu role aktif."** |
| Bentuk | Checkbox pill, `flex-wrap`, `gap-2` |

Style pill: `inline-flex`, `rounded-full`, border `hairline`, `px-3 py-1.5`. Saat tercentang
(`has-[:checked]`): border `brand`, latar `brand/10`, `font-medium`, teks `brand`.

Isi label pill = **`role.name`** (mis. "Kepala Gudang"), bukan kodenya.

| Aturan | Detail |
|---|---|
| Role yang ditampilkan | Semua role perusahaan **kecuali `superadmin`** |
| Centang bawaan (mode edit) | Role yang saat ini dimiliki user |
| Centang bawaan (mode tambah) | **Role `staff`** |

### 3.5 Kartu 4 — "Akses Cabang"

| Elemen | Isi |
|---|---|
| Judul | **"Akses Cabang"** |
| Deskripsi | **"Pilih cabang yang boleh diakses user ini."** |
| Bentuk | Checkbox pill, style identik kartu 3 |

Isi label pill = **`branch.name`** (bukan kode, berbeda dari kolom Cabang di daftar yang memakai
kode).

| Aturan | Detail |
|---|---|
| Centang bawaan (mode edit) | Cabang yang saat ini bisa diakses |
| Centang bawaan (mode tambah) | Cabang yang bertanda default perusahaan |

**Yang tidak terlihat di UI:** cabang pertama pada urutan terpilih menjadi **cabang default user**.
Tidak ada penanda, tidak ada penjelasan, dan tidak ada cara memilihnya secara sadar. Lihat
[known-issues.md](../known-issues.md) KI-20.

### 3.6 ActionRow

| Tombol | Variant | Aksi |
|---|---|---|
| **"Batal"** | secondary | Pindah ke `/users` |
| **"Tambah Pengguna"** (mode tambah) / **"Simpan Perubahan"** (mode edit) | primary, `type="submit"` | Simpan |

### 3.7 Pesan validasi klien

Ditampilkan lewat **balon validasi native browser** (`reportValidity`), bukan teks di bawah field:

| Kondisi | Pesan |
|---|---|
| Password < 8 karakter, mode tambah | **"Password awal minimal 8 karakter."** |
| Password < 8 karakter, mode edit | **"Password baru minimal 8 karakter."** |
| Password ≠ konfirmasi | **"Konfirmasi password tidak sama."** |

Pesan dibersihkan setiap kali field diketik ulang. Field wajib yang kosong dan format email salah
memakai pesan bawaan browser (bahasa mengikuti setelan browser, bukan aplikasi).

### 3.8 Perilaku setelah simpan

| Hasil | Yang terjadi |
|---|---|
| Berhasil | Pindah ke `/users` — **tanpa toast sukses** |
| Gagal | Tetap di form. Toast danger **"Gagal menyimpan pengguna"** **tanpa deskripsi** — alasan spesifik dari server tidak ditampilkan |

---

## 4. Screen: `/settings/roles` — Role & Akses

Tata letak: PageHeader → grid dua kartu berdampingan → SectionCard matriks (lebar penuh).

Grid dua kartu: `repeat(auto-fit, minmax(min(100%, 420px), 1fr))`, `gap-6` — menumpuk vertikal di
layar sempit.

### 4.1 PageHeader

| Elemen | Isi |
|---|---|
| Breadcrumb | **"Dashboard > Pengaturan > Role & Akses"** |
| Judul | **"Role & Akses"** |
| Deskripsi | **"Kelola role perusahaan dan permission yang melekat pada setiap role."** |

### 4.2 Kartu kiri — "Daftar Role"

Deskripsi: **"Role sistem dan role kustom perusahaan."**

| # | Header kolom | Isi |
|---|---|---|
| 1 | **"Role"** | Dua baris: `role.name` (`<strong>`, `ink`) di atas, `formatRoleLabel(role.code)` (`muted`) di bawah |
| 2 | **"Tipe"** | `Badge tone="neutral"` berteks **"Sistem"** bila role sistem; `Badge tone="info"` berteks **"Kustom"** bila bukan |
| 3 | **"Aksi"** | Tombol secondary **"Atur"** selalu; tombol danger **"Hapus"** hanya untuk role kustom |

Kolom "Role" menampilkan nama dan label kode yang **sering identik** (mis. nama "Admin" dan label
"Admin"), sehingga baris kedua tampak mengulang baris pertama untuk role sistem.

Empty state (bila tidak ada role sama sekali):

| Elemen | Isi |
|---|---|
| Judul | **"Belum ada role"** |
| Deskripsi | **"Role akan menentukan hak akses setiap pengguna di perusahaan."** |

### 4.3 Kartu kanan — "Tambah / Edit Role"

Deskripsi: **"Role baru berlaku di level perusahaan."**

| # | Label | `id` | Bentuk | Wajib |
|---|---|---|---|---|
| 1 | **"Nama role"** | `role-name-input` | input text | ✔ |
| 2 | **"Deskripsi"** | `role-description-input` | textarea, `min-h-[80px]`, `resize-y` | ✘ |

ActionRow:

| Tombol | Variant | Aksi |
|---|---|---|
| **"Reset"** | secondary, `type="button"` | Mengosongkan kedua field **dan membatalkan pemilihan role** |
| **"Simpan Role"** (bila ada role terpilih) / **"Tambah Role"** | primary, submit | Simpan |

**Tidak ada field kode role di UI** — kode selalu diturunkan dari nama (lihat
[numbering-sequence.md](numbering-sequence.md)).

### 4.4 SectionCard — "Matriks Hak Akses"

| Elemen | Isi |
|---|---|
| Judul | **"Matriks Hak Akses"** |
| Deskripsi | **"Pilih role terlebih dahulu, lalu nyalakan izin yang dibutuhkan."** |
| Saat belum ada role terpilih | `Notice tone="info"` berteks **"Pilih salah satu role untuk mulai mengatur permission."** |

Saat role terpilih: daftar vertikal (`gap-3`) berisi **22 kotak kelompok modul**. Setiap kotak:
`rounded-md`, border `hairline`, latar `surface`, padding 4, kelas legacy `list-item`.

Isi kotak: satu baris `flex-wrap justify-between` — label modul (`<strong>`, `ink`) di kiri,
kumpulan pill permission di kanan.

Style pill permission identik pill role/cabang di form pengguna, dengan tambahan latar `surface`
dan ukuran teks 0.85rem.

`aria-label` setiap pill: **`{Label Modul}: {Label Permission}`** — mis. `"Dashboard: Lihat"`,
`"Produk & Item: Tambah"`, `"Role & Akses: Kelola"`. Format ini dipakai test E2E dan unit test —
**jangan diubah**.

### 4.5 Daftar lengkap 22 kelompok modul dan labelnya

Urutan di bawah adalah urutan tampil, dan harus dipertahankan.

| # | Label kelompok | Label permission (urut) |
|---|---|---|
| 1 | **Dashboard** | Lihat |
| 2 | **Produk & Item** | Lihat · Tambah · Ubah · Arsipkan |
| 3 | **Jenis Member** | Lihat · Kelola |
| 4 | **Penjualan / Order** | Lihat · Buat Order · Ubah Order · Arsipkan · Export Report |
| 5 | **Retur & Tukar Barang** | Lihat Retur · Buat Retur/Tukar · Proses Refund · Batalkan Retur |
| 6 | **Retur Pembelian** | Lihat Retur · Buat Retur Pembelian · Proses Refund · Batalkan Retur |
| 7 | **Pembayaran** | Lihat · Catat Pembayaran · Batalkan/Arsipkan |
| 8 | **Inventori Gudang** | Lihat Stok · Kelola Lokasi · Pindah/Transfer Stok · Koreksi Stok · Setujui Koreksi |
| 9 | **Laporan Operasional** | Lihat Laporan |
| 10 | **Keuangan - Setup** | Lihat Setup · Kelola Setup, Saldo Awal & Periode · Tutup Buku Paksa (Force, Lewati Checklist) |
| 11 | **Keuangan - Rekap Posting** | Lihat Rekap · Posting, Abaikan, Pulihkan & Batalkan |
| 12 | **Keuangan - Jurnal** | Lihat Jurnal · Balikkan Jurnal |
| 13 | **Keuangan - Laporan** | Kas, Hutang/Piutang, Margin & Stok · Pajak |
| 14 | **Keuangan - Biaya Usaha** | Lihat Biaya · Catat Biaya · Batalkan Biaya |
| 15 | **Keuangan - Penyesuaian Pajak** | Lihat Penyesuaian Pajak · Kelola Penyesuaian Pajak |
| 16 | **Pengguna & Tim** | Lihat · Tambah · Ubah · **Nonaktifkan** |
| 17 | **Pengaturan Perusahaan** | Lihat · Kelola |
| 18 | **Cabang & Lokasi** | Lihat · Kelola |
| 19 | **Role & Akses** | Kelola |
| 20 | **Asisten WA - Kanal & Bot** | Lihat · Kelola |
| 21 | **Asisten WA - Basis Pengetahuan** | Lihat · Tambah · Ubah · Arsipkan |
| 22 | **Riwayat Aktivitas** | Lihat |

Total **59 pill**. Perhatikan kelompok 16: label permission `user.archive` adalah
**"Nonaktifkan"**, bukan "Arsipkan" — mencerminkan bahwa aksinya memang menonaktifkan, bukan
mengarsipkan.

Satu permission sistem **sengaja tidak ditampilkan**: `whatsapp.simulate`. Alasannya tercatat di
test — agar admin tidak bisa memberikan permission developer itu ke role kustom.

### 4.6 Perilaku matriks

| Aspek | Perilaku |
|---|---|
| Penyimpanan | **Otomatis per klik.** Tidak ada tombol simpan untuk matriks |
| Umpan balik visual | Optimistis — pill langsung berubah sebelum server menjawab |
| Bila gagal | Pill dikembalikan ke kondisi sebelumnya, data dimuat ulang, dan muncul toast danger **"Gagal memperbarui permission role"** dengan deskripsi berisi **kode mesin** |
| Setelah menyimpan role dari form | Pilihan role dikosongkan → matriks **menutup kembali** ke notice |

---

## 5. Katalog Notifikasi Toast Modul Ini

Semua hilang otomatis setelah 3400 ms.

| Pemicu | Judul | Deskripsi | Tone |
|---|---|---|---|
| Role ditambahkan | **"Role berhasil ditambahkan"** | *(tidak ada)* | success |
| Role diperbarui | **"Role berhasil diperbarui"** | *(tidak ada)* | success |
| Role dihapus | **"Role berhasil dihapus"** | *(tidak ada)* | success |
| Gagal menyimpan role | **"Gagal menyimpan role"** | **kode mesin** (mis. `error`) | danger |
| Gagal menghapus role | **"Gagal menghapus role"** | **kode mesin** | danger |
| Gagal memperbarui permission | **"Gagal memperbarui permission role"** | **kode mesin** | danger |
| Mencoba mengubah role sistem | **"Role sistem tidak dapat diubah"** | **"Gunakan role kustom untuk kebutuhan tambahan."** | warning |
| Gagal menyimpan pengguna | **"Gagal menyimpan pengguna"** | *(tidak ada)* | danger |
| Gagal mengubah status pengguna | **"Gagal mengubah status pengguna"** | *(tidak ada)* | danger |

**Tidak ada toast sukses** untuk aksi pengguna (tambah, edit, ubah status) — umpan baliknya adalah
perpindahan halaman atau perubahan badge di tabel. Ini asimetri sadar-atau-tidak dengan aksi role
yang semuanya bertoast sukses. **[PERLU KONFIRMASI]** apakah perlu diseragamkan.

---

## 6. Format & Konvensi Tampilan

| Aspek | Aturan di modul ini |
|---|---|
| Format tanggal | **Modul ini tidak menampilkan tanggal apa pun.** `created_at`, `updated_at`, `last_login_at` tersimpan tapi tidak pernah ditampilkan |
| Format angka | Hanya pada teks paginasi (bilangan bulat apa adanya, tanpa pemisah ribuan) |
| Kode cabang | Ditampilkan apa adanya di daftar pengguna (badge info) |
| Nama cabang | Ditampilkan di form pengguna (pill checkbox) |
| Label role | Selalu lewat `formatRoleLabel` di daftar pengguna; **nama role apa adanya** di form pengguna dan daftar role |
| Status pengguna | **Nilai mentah bahasa Inggris** (`active`/`inactive`) |
| Password | Selalu tersembunyi secara default, toggle per field |
| Kelas CSS legacy | `filter-bar`, `list-item`, `field` — dipertahankan sebagai hook selector test |
| Pola aksi destruktif | **Tanpa dialog konfirmasi** untuk hapus role maupun nonaktifkan user |
