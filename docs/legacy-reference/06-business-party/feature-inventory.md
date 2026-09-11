# Feature Inventory — Modul 06 Business Party (Customer / Supplier)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = CRUD pelanggan + pemasok + buku alamat kirim pelanggan + picker
pihak. Yang **bukan** bagian modul ini: jenis member & harga member (modul 07 — di sini hanya
kolom `id_member_type` dan badge nama member), saldo/piutang/ledger pihak (modul 14 Payment:
`payments/party-balances`, `payments/party-ledger`), order yang memakai pihak (modul 08–09),
validasi pihak di order (mis. penolakan order baru untuk pihak terarsip ditegakkan modul Order,
bukan modul ini).

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 16 — `customers/{list,detail,create,update,archive,restore}` (6), `suppliers/{list,detail,create,update,archive,restore}` (6), `customer-addresses/{list,create,update,archive}` (4) |
| Halaman | 6 rute: `/customers`, `/customers/create`, `/customers/:partyId/edit`, `/suppliers`, `/suppliers/create`, `/suppliers/:partyId/edit` (2 komponen halaman, masing-masing ganda list+form) |
| Menu sidebar | 2, grup **"Produk & Mitra"**: **"Pelanggan"** (`/customers`), **"Pemasok"** (`/suppliers`) |
| Permission | **Tidak punya kode izin sendiri.** Seluruh endpoint memakai ulang `order.view` (list/detail), `order.create` (create), `order.update` (update), `order.archive` (archive/restore) — dengan satu pengecualian: `customer-addresses/archive` memakai `order.update`, bukan `order.archive`. Tidak ada `customer.*`/`supplier.*` di `permission-code.ts` (verifikasi: nol hasil) |
| Tabel yang dimiliki | `business_parties`, `business_party_delivery_addresses` |
| Tabel yang dibaca (milik modul 07) | `member_types` (validasi + join tampilan; tulisnya milik modul 07) |
| Kolom yang ditulis di tabel modul lain | `orders.id_ship_to_address` + 4 snapshot `ship_to_*` (ditulis modul Order saat create/update order, bukan modul ini — kontrak konsumen, lihat F-05) |
| Laporan | Tidak ada laporan milik sendiri — lihat [reports-list.md](reports-list.md) |
| Penomoran dokumen | Kode otomatis `CUS-###` / `SUP-###` bila dikosongkan. Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | 11 `actionKey`: `customer.create/update/archive/restore`, `supplier.create/update/archive/restore`, `customer_address.create/update/archive` |
| Aksi yang TIDAK ada | Hapus permanen; ubah kode pihak; ubah `party_type`; pindahkan alamat antar pelanggan; `restore` alamat; nonaktifkan alamat tanpa arsip (selain menimpa utama) |

**Catatan lingkup.** Modul ini adalah **dimensi relasional** seluruh commerce: order menunjuk pihak
by-id (nama bersifat live, bukan snapshot — E2E 22 mengunci ini), payment/order menghitung saldo
per pihak, member pricing membaca `id_member_type`. Arsip pihak adalah soft-delete yang **tidak
memutus relasi**: piutang tetap tertagih, ledger tetap terbuka, order lama tetap tertaut — hanya
bisnis baru (order) yang ditolak, dan penolakan itu ditegakkan modul Order.

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Daftar Pelanggan (`/customers`) & Daftar Pemasok (`/suppliers`)

Dua halaman kembar; perbedaan hanya pada F-01.9 (kolom Member) dan seluruh teks
pelanggan↔pemasok. Deskripsi per halaman:

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Toggle Aktif \| Sampah | Tombol ganda; Sampah = `?view=trash` di URL (`status=archived` ke API). Pindah toggle me-reset halaman, menutup dialog yang terbuka, dan menghapus error tindakan |
| F-01.2 | Cari (debounce 400ms → URL `search`) | Satu frasa LIKE di 5 kolom: nama, kode, telepon, email, alamat. Placeholder `"Cari nama, kode, kontak, atau alamat"`. **Bukan** tokenized (beda dengan produk — → KI-65) |
| F-01.3 | Paginasi 20/halaman | `page` di URL; `Pagination` standar |
| F-01.4 | Kolom Nama | Nama tebal + kode redup di bawahnya |
| F-01.5 | Kolom Tipe | Badge statis: `Customer` (netral) / `Supplier` (info) — bukan data, hanya label halaman |
| F-01.6 | Kolom Kontak | `phone ?? email ?? "—"` |
| F-01.7 | Kolom Alamat | `address_text` apa adanya (bisa kosong) |
| F-01.8 | Kolom Dihapus (tab Sampah saja) | `archivedAt` format `id-ID` (`toLocaleString("id-ID")`), else `"—"` |
| F-01.9 | Kolom Member (pelanggan saja) | Badge aksen nama member (`memberType.name`) atau teks `"Non-member"`; datang dari join `memberType` di list |
| F-01.10 | Aksi baris (tab Aktif) | **[Detail]** secondary (`order.update` → halaman edit) · **[Hapus]** ghost merah (`order.archive` → dialog hapus) |
| F-01.11 | Aksi baris (tab Sampah) | **[Pulihkan]** secondary (`order.archive` → dialog pulihkan); tidak ada Detail/Edit di Sampah |
| F-01.12 | Tombol Tambah | Header + empty-state, hanya `order.create` (`Tambah Pelanggan` / `Tambah Pemasok`) |
| F-01.13 | Kartu + kosong | Judul `Data Pelanggan` / `Sampah Pelanggan` (+ varian Pemasok); deskripsi `"{n} pelanggan ditemukan."` / `"{n} pelanggan di Sampah."` / `"Memuat…"`; kosong: `Data pelanggan kosong` / `Sampah kosong` + deskripsi pasangannya |
| F-01.14 | Dialog Hapus | Judul `Hapus pelanggan/pemasok`; deskripsi `"... akan dipindahkan ke Sampah. Riwayat transaksinya tetap aman dan data bisa dipulihkan kapan saja dari tab Sampah."`; tombol `Hapus` (danger). Tanpa checkbox/tanpa ketik konfirmasi |
| F-01.15 | Dialog Pulihkan | Judul `Pulihkan ...`; deskripsi `"... akan dikembalikan ke daftar aktif."`; tombol `Pulihkan` (warning) |
| F-01.16 | Error tindakan inline | `Notice` merah `Tindakan gagal` + pesan server; fallback `"Gagal menghapus pelanggan/pemasok."` / `"Gagal memulihkan pelanggan/pemasok."`. Sukses: **tanpa toast**, daftar me-reload diam-diam |

### F-02 — Form Pelanggan (`/customers/create`, `/customers/:partyId/edit`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Mode edit by-id | Bila id tidak ada di store (di luar 100 baris / buka langsung / refresh): fetch `customers/detail`; tak valid → layar `Pelanggan tidak ditemukan` + notice `Pelanggan tidak bisa diedit` / `Data pelanggan tidak ditemukan atau sudah dihapus.`; loading → `Memuat Pelanggan` + `Memuat data pelanggan` / `Mohon tunggu sebentar.` (perbaikan anomali E2E 21 — tidak pernah jatuh ke mode tambah) |
| F-02.2 | Kartu Profil Kontak | `ID / Kode Pelanggan (opsional)` (placeholder `"Kosongkan untuk dibuatkan otomatis"`) · `Nama Lengkap / Perusahaan`* (required) · `Jenis Member` (hanya bila `member_type.view`; opsi `Non-member` + member aktif — plus member saat ini walau nonaktif; label `"nama (kode)"`) · `Telepon / WhatsApp` · `Email` (`type=email`) · `Alamat Lengkap (opsional)` (textarea) · `Catatan Tambahan (Opsional)` (textarea, placeholder `"PIC, Syarat khusus, dll..."`) |
| F-02.3 | Kartu Buku Alamat Kirim | Hanya mode edit. Judul + deskripsi `"Alamat tujuan kirim yang dapat dipilih saat membuat order atau di kasir. Satu alamat menjadi Utama (default)."` → `CustomerAddressBook` (F-04) |
| F-02.4 | Kartu Informasi Tambahan | Pasangan Nama (`"Nama Informasi (Contoh: NPWP)"`, saran dari kunci milik pelanggan lain + boleh ketik) + Keterangan (`"Isi detail informasi"`) + **Hapus** merah; tombol `+ Tambah Informasi Baru`; baris nama-kosong tidak dikirim |
| F-02.5 | Submit | `Batal` (secondary → `/customers`) + `Simpan Data Pelanggan`; sukses → pindah `/customers` **tanpa toast**; gagal → **diam total** (tetap di form, tanpa toast/notice — → KI-61) |
| F-02.6 | Kode bisa diketik saat edit tetapi diabaikan | Field kode terisi kode saat ini dan bisa diubah; server tidak menerima kode di update → perubahan hilang diam-diam (→ KI-62) |

### F-03 — Form Pemasok (kembar, selisih berikut)

| # | Selisih vs form pelanggan |
|---|---|
| F-03.1 | Tanpa field Jenis Member dan tanpa kartu Buku Alamat Kirim |
| F-03.2 | Deskripsi header: `"Mencatat mitra pengadaan dan partner stok operasional."` (pelanggan: `"Catat entitas penerima layanan dan penjualan Anda."`) |
| F-03.3 | Contoh atribut: `"Nama Informasi (Contoh: NPWP, Rekening)"`; deskripsi kartu: `"...seperti NPWP, detail bank pencairan, nama PIC, dsb."` |
| F-03.4 | Tombol kirim: `Simpan Data Pemasok`; tombol `← Kembali` memakai `navigate(-1)` (pelanggan: selalu ke `/customers`) — inkonsistensi kecil (→ KI-68) |
| F-03.5 | Selebihnya identik: by-id, diam-saat-gagal, kode-diabaikan, atribut, Batal → `/suppliers` |

### F-04 — Buku Alamat Kirim (`CustomerAddressBook`, di form edit pelanggan)

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Daftar | `"Memuat alamat…"` → baris per alamat: label (atau `"Alamat"`) + tag `Utama` (brand, bila primer) · penerima `·` telepon · teks alamat; tombol `Jadikan Utama` (ghost kecil, non-primer) · `Edit` · `Hapus` (ghost kecil). Kosong: `"Belum ada alamat kirim tersimpan."` |
| F-04.2 | Tambah/Edit inline | Form: Label (`"Label (mis. Gudang / Proyek A)"`) · penerima + telepon grid-2 · alamat* (`"Alamat lengkap *"`) · `Batal` / `Simpan alamat` (`"Menyimpan…"`; disabled bila alamat kosong). Tambah pertama otomatis Utama |
| F-04.3 | Hapus tanpa konfirmasi | Tombol `Hapus` langsung mengarsip (soft); tanpa dialog, tanpa toast; gagal → diam (→ KI-63) |
| F-04.4 | Jadikan Utama | Satu klik → demote yang lain dalam transaksi; tanpa toast |

### F-05 — Pemilih Alamat Kirim (`AddressPicker`, dipakai form order & POS)

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Muat + auto-pilih | Saat pelanggan dipilih: muat buku alamatnya (`"Memuat alamat…"`); bila form belum punya pilihan eksplisit → otomatis pilih Utama (atau pertama). Ganti pelanggan cepat: request basi diabaikan (penjaga urutan) |
| F-05.2 | Opsi dropdown | Alamat tersimpan (`"{label|Alamat} — {50 char pertama}… {(Utama)}"`) · `Alamat lain (sekali pakai)…` · `Tanpa alamat kirim` |
| F-05.3 | Pratinjau tersimpan | Di bawah dropdown: `Penerima: {nama} · {telepon}` + teks alamat |
| F-05.4 | Ad-hoc | Textarea (`"Alamat tujuan *"`) + Nama penerima + No. HP penerima (opsional) |
| F-05.5 | Tambah inline | `+ Tambah alamat baru` → form (Label/penerima/HP/alamat*) → `Simpan alamat` → daftar me-reload + alamat baru langsung terpilih. Tanpa `order.create`? — endpoint create memakai `order.create`; kasir POS yang boleh tambah pelanggan boleh tambah alamat (komentar kode, disengaja) |
| F-05.6 | Walk-in (tanpa pelanggan) | Satu textarea opsional `"Alamat kirim (opsional)"` |
| F-05.7 | Pemetaan ke order | Tersimpan → hanya `shipToAddressId` (server memvalidasi milik pihak terkait + menyalin snapshot); ad-hoc/kosong → teks + label/penerima/telepon (`""` = kosongkan). Lihat `shipToSelectionToInput` (diuji unit) |

### F-06 — Pemilih Pihak (`PartySearchSelect`, dipakai order & modul lain)

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | Cari server-side penuh | Tiap ketikan → `customers/list` / `suppliers/list` + `search` (limit 20); tidak bergantung store 100-baris (mitigasi E2E 21) |
| F-06.2 | Label opsi | Nama (+ ` (nama member)` bila member) · deskripsi telepon/email/kode |
| F-06.3 | Resolve tersimpan | Nilai id di-resolve via `detail` (atau seed `selectedParty` instan — dipakai edit order & shortcut); gagal → opsi null |
| F-06.4 | Placeholder & kosong | `"Pilih customer"` / `"Pilih supplier"`; cari `"Cari nama, kode, atau kontak..."`; boleh dikosongkan (`allowClear`, default true) |

### F-07 — Pintu masuk cepat (milik modul Order/POS, kontrak modul ini)

Tiga pintu menulis entitas yang sama via `customers/create` (dikunci E2E 20 — perilaku modul ini,
UI milik modul lain):

| Pintu | Field | Validasi khas |
|---|---|---|
| Modal shortcut Order (`Tambah Pelanggan` → `Simpan & Pilih Pelanggan`) | nama* (`#shortcut-customer-name`), telepon, alamat (opsional), member (`#shortcut-customer-member`) | `"Nama pelanggan wajib diisi"` (JS trim; lolos `required` HTML) |
| Quick-add POS (`Tambah Pelanggan Baru` → `Simpan Customer`) | nama* (placeholder), HP | `"Nama pelanggan wajib diisi"`; mode Bayar Nanti: HP wajib (`required` HTML + `"No. HP wajib diisi untuk customer yang berhutang"`) |
| Form menu Pelanggan | lengkap (F-02) | `required` HTML nama; alamat tidak wajib di semua pintu |

Semua: kode kosong → `CUS-###` otomatis & unik; yang baru langsung terpilih/dapat dibuka di menu
Pelanggan (cross-visibility).

---

## 3. Edge Case (34)

| # | Kondisi | Perilaku (dari kode) |
|---|---|---|
| E-01 | Kode manual duplikat (termasuk milik Sampah) | `409 "Kode '{k}' sudah digunakan"` + suffix `" (ada di Sampah)"` bila yang bentrok terarsip |
| E-02 | Kode dikosongkan | Otomatis `CUS-###`/`SUP-###`: suffix numerik tertinggi +1 (termasuk arsip; hanya pola murni `PREFIX+angka`; `padStart(3)`); tanpa baris cocok → `CUS-001` |
| E-03 | Kode manual vs auto-code | Manual tidak dicadangkan: auto berikutnya bisa melompati/menabrak? — tidak menabrak: max dihitung dari semua kode cocok pola, termasuk manual berpola (`CUS-007` manual → auto berikutnya `CUS-008`). Manual tak berpola diabaikan generator |
| E-04 | Dua create kosong bersamaan | Keduanya membaca max sama → kode sama → satu `500` unique-key (tanpa retry — → KI-64) |
| E-05 | `update` tanpa `id_member_type` | Keanggotaan dipertahankan (hanya `!== undefined` yang diproses). FE tanpa `member_type.view` bahkan tidak mengirim field → keanggotaan tak terlihat tapi lestari (→ KI-67) |
| E-06 | `id_member_type: null` / `0` | Melepas member (null). `0` diperlakukan sama dengan null |
| E-07 | Member untuk supplier | `400 'Jenis member hanya dapat dipakai untuk pelanggan.'` (sebelum cek keberadaan) |
| E-08 | Member tak dikenal / nonaktif / milik perusahaan lain / terarsip | `400 'Jenis member tidak ditemukan.'` / `'Jenis member tidak aktif.'` (arsip/lain-perusahaan = "tidak ditemukan") |
| E-09 | Nama kosong via API | **Lolos** — tanpa validasi nama di server (hanya FE `required` + JS trim di shortcut). `""` tersimpan (→ KI-66) |
| E-10 | Update dengan string kosong (phone/email/address/notes) | Menghapus nilai (assign langsung; `!== undefined`). Berbeda dengan alamat (trim → tolak kosong) |
| E-11 | Update `attributes: null` eksplisit | Menghapus seluruh metadata (null). `attributes` menang atas `attributes_json`; keduanya undefined → tak tersentuh |
| E-12 | Ganti kode via update | **Diabaikan** — tidak ada field kode di DTO; FE tetap menampilkan field kode (→ KI-62) |
| E-13 | Detail/archive/update id tak dikenal / tipe salah / terarsip (update/archive) | `404 'Data tidak ditemukan'` (tipe diperiksa: customer tak bisa dibuka via suppliers/*) |
| E-14 | Restore yang tidak di Sampah | `404 'Data tidak ditemukan di Sampah'` |
| E-15 | Restore menabrak kode aktif | `409 "Kode '{k}' sudah dipakai data aktif lain. Ubah kode itu dulu sebelum memulihkan."` — tetapi kode tak bisa diubah via API (E-12) → jalan buntu permanen bila terjadi (→ KI-62). Secara normal mustahil (create me-reserve lintas arsip) |
| E-16 | Arsip pihak bertransaksi | **Lolos tanpa cek.** Order lama tetap tertaut (nama live), saldo/piutang tetap terlihat & tertagih, ledger terbuka (dikunci E2E 22); order BARU ditolak modul Order (`/diarsip\|ditemukan/`) |
| E-17 | Rename pihak | Order tetap by-id; pencarian order mengikuti nama baru, nama lama tak lagi menemukan (nama live, bukan snapshot — dikunci E2E 22) |
| E-18 | Cari `"Santoso Budi"` untuk `"Budi Santoso"` | **Tidak ditemukan** — LIKE frasa tunggal, bukan tokenized (→ KI-65) |
| E-19 | `status` selain `archived` (termasuk tak dikirim) | Daftar aktif. Tidak ada status ketiga |
| E-20 | `limit` 0/negatif/string | Diteruskan ke query (hanya batas atas 100) → kosong/500 (→ KI-70, seakar KI-26/55) |
| E-21 | Alamat: teks kosong/whitespace (create & update) | `400 'Alamat wajib diisi.'` |
| E-22 | Alamat: label/penerima/telepon kosong | Disimpan `null` (bukan string kosong) |
| E-23 | Alamat pertama pelanggan | Otomatis Utama walau tak diminta; berikutnya non-utama kecuali `is_primary: true` (→ demote lain dalam transaksi) |
| E-24 | `is_primary: false` di update | **Diabaikan** — hanya `true` yang diproses; primer tak bisa diturunkan tanpa pengganti (→ KI-69) |
| E-25 | Arsip alamat Utama | Aktif lain paling awal (`sortOrder, id`) naik otomatis dalam transaksi yang sama; bila tak tersisa → pelanggan tanpa alamat & tanpa primer (diizinkan) |
| E-26 | List/create alamat untuk supplier / pelanggan arsip / tak dikenal | `404 'Pelanggan tidak ditemukan.'` (supplier tidak punya buku alamat) |
| E-27 | Update/archive alamat: pelanggan pemilik sudah diarsip | **Lolos** — hanya baris alamat yang diperiksa, tanpa `assertCustomer` (inkonsisten dengan list/create → KI-63) |
| E-28 | Order memakai `id_ship_to_address` milik pelanggan lain | Ditolak modul Order (validasi milik pihak terkait — ditegakkan di sana, bukan di sini) |
| E-29 | Alamat tersimpan diubah/diarsip setelah order dibuat | Dokumen lama tak berubah (snapshot di order; backfill `Utama` untuk alamat lama saat migrasi 044) |
| E-30 | Simpan pihak gagal (kode duplikat, server mati) | Form **diam total** — tetap di form tanpa toast/notice (→ KI-61). Beda dengan hapus/pulihkan (notice inline `Tindakan gagal`) |
| E-31 | Simpan alamat gagal (buku alamat / picker inline) | Diam — `busy` lepas, tanpa pesan (→ KI-63) |
| E-32 | Edit id tak valid (`/customers/abc/edit`) / arsip via URL | `Number` tak integer/≤0 → layar tidak-ditemukan tanpa request; arsip tak punya halaman (hanya dialog di daftar) |
| E-33 | Pihak ke-101+ | Form edit fetch by-id (benar); picker server-side (benar); dropdown lama berbasis store sudah tidak dipakai di picker (perbaikan E2E 21) |
| E-34 | Member select: member saat ini nonaktif | Tetap tampil sebagai opsi terpilih (filter: aktif ATAU id == milik pihak) — tidak hilang dari form |

---

## 4. Katalog Pesan (teks apa adanya)

**Error API:** `'Data tidak ditemukan'` (detail/update/archive lintas tipe) ·
`'Data tidak ditemukan di Sampah'` (restore salah sasaran) ·
`"Kode '{k}' sudah digunakan"` + opsional `" (ada di Sampah)"` ·
`"Kode '{k}' sudah dipakai data aktif lain. Ubah kode itu dulu sebelum memulihkan."` ·
`'Jenis member hanya dapat dipakai untuk pelanggan.'` · `'Jenis member tidak ditemukan.'` ·
`'Jenis member tidak aktif.'` · `'Alamat wajib diisi.'` · `'Pelanggan tidak ditemukan.'` (alamat:
bukan pelanggan/arsip) · `'Alamat tidak ditemukan.'`.

**Sukses: tanpa toast di seluruh modul** (simpan pihak, hapus, pulihkan, simpan/hapus/Ćjadikan-utama
alamat — semuanya diam + reload/redirect). Satu-satunya umpan balik tertulis adalah notice inline
`Tindakan gagal` (+ pesan server; fallback `Gagal menghapus/memulihkan pelanggan/pemasok.`) dan
dialog konfirmasi.

**Validasi pintu cepat (milik UI Order/POS, dikunci E2E 20):** `"Nama pelanggan wajib diisi"` ·
`"No. HP wajib diisi untuk customer yang berhutang"`.

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Kode izin `customer.*`/`supplier.*` | `permission-code.ts` nol hasil; seluruh endpoint memakai `order.*` |
| NF-02 | Hapus permanen pihak/alamat | Semua arsip = `archived_at`; restore hanya untuk pihak (alamat tanpa restore) |
| NF-03 | Ubah kode / ubah tipe pihak | DTO update tanpa kedua field |
| NF-04 | Validasi format telepon/email/alamat di server | Assign mentah; kolom bernama `phone_e164` tanpa validasi E.164 (M2-Q4 tetap terbuka) |
| NF-05 | Buku alamat untuk supplier | `assertCustomer` menolak non-customer; UI supplier tanpa kartu alamat |
| NF-06 | Pindah alamat antar pelanggan | Update tanpa field `id_business_party` |
| NF-07 | Stok/saldo/ledger di modul ini | Milik Stock/Payment; modul ini hanya dimensi |
| NF-08 | Paginasi/filter buku alamat | `find` penuh per pelanggan, urut primer→sort→id |
| NF-09 | Toast sukses/gagal simpan di slice | `parties.slice.ts` + `use-customer-addresses.ts` tanpa `notify` sama sekali |
| NF-10 | Pencarian lintas tipe | Customer & supplier endpoint terpisah; tidak ada pencarian gabungan |
