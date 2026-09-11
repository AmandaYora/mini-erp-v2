# UI/UX Spec — Modul 06 Business Party (Customer / Supplier)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Tiap screen & interaksi apa adanya
dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Rute (`module-registry.tsx`, semua `access: "protected"`, `shell: true`):

| Rute | Izin | Elemen | Menu |
|---|---|---|---|
| `/customers` | `order.view` | `CustomersListPage` | Produk & Mitra → Pelanggan |
| `/customers/create` | `order.create` | `CustomerFormPage` | — |
| `/customers/:partyId/edit` | `order.update` | `CustomerFormPage` | — |
| `/suppliers` | `order.view` | `SuppliersListPage` | Produk & Mitra → Pemasok |
| `/suppliers/create` | `order.create` | `SupplierFormPage` | — |
| `/suppliers/:partyId/edit` | `order.update` | `SupplierFormPage` | — |

Tidak ada halaman detail baca-saja: tombol **Detail** membuka halaman edit.

---

## 1. Screen `/customers` — Daftar Pelanggan (+ kembaran `/suppliers`)

**Header** (`PageHeader`): breadcrumb `Dashboard > Pelanggan` (`> Pemasok`); judul `Daftar Pelanggan`
(`Daftar Pemasok`); deskripsi `"Daftar pelanggan baik perorangan maupun korporasi / perusahaan."`
(`"Daftar pemasok atau principal sumber stok Anda."`); aksi kanan **[Tambah Pelanggan]** (`Tambah
Pemasok`) bila `order.create` → `/customers/create`.

**FilterBar** (2 field): `Tampilan` — dua tombol `[Aktif | Sampah]` (aktif = `primary`, sampah =
`ghost`; sampah menulis `?view=trash`); `Cari pelanggan` (`customer-search`, placeholder `"Cari nama,
kode, kontak, atau alamat"`, debounce 400ms → `?search`, reset `page`). Tidak ada filter lain
(tanpa filter member, tanpa filter tanggal, tanpa sortir — urutan selalu nama A→Z / arsip terbaru).

**Notice error** (bila ada): merah `Tindakan gagal` + pesan server.

**Kartu** (`SectionCard`): judul `Data Pelanggan` / `Sampah Pelanggan` (+ varian Pemasok); deskripsi
`"Memuat…"` / `"{n} pelanggan ditemukan."` / `"{n} pelanggan di Sampah."`. Tabel (`DataTable`):

| Kolom | Isi |
|---|---|
| Nama | Nama tebal + kode redup |
| Tipe | Badge statis `Customer` (netral) / `Supplier` (info) |
| Member (pelanggan saja) | Badge aksen nama member, atau teks `Non-member` |
| Kontak | `phone ?? email ?? "—"` |
| Alamat | teks apa adanya (bisa kosong) |
| Dihapus (tab Sampah saja) | `archivedAt` → `toLocaleString("id-ID")`, else `"—"` |
| Aksi | Aktif: `[Detail]` secondary (`order.update`) + `[Hapus]` ghost merah (`order.archive`); Sampah: `[Pulihkan]` secondary (`order.archive`). Tanpa izin: sel aksi kosong |

Kosong: `Data pelanggan kosong` / `Sampah kosong` (+ Pemasok); deskripsi `"Belum ada pelanggan yang
cocok dengan pencarian ini."` / `"Tidak ada pelanggan di Sampah."`; tombol Tambah (tab aktif +
`order.create`). Bawah: `Pagination` 20/halaman.

**Dialog Hapus** (`ConfirmDialog` danger): judul `Hapus pelanggan` (`Hapus pemasok`); deskripsi
`"... {nama} akan dipindahkan ke Sampah. Riwayat transaksinya tetap aman dan data bisa dipulihkan
kapan saja dari tab Sampah."`; tombol `Hapus`. **Dialog Pulihkan** (warning): judul `Pulihkan ...`;
deskripsi `"... {nama} akan dikembalikan ke daftar aktif."`; tombol `Pulihkan`. Tanpa tombol batal
berlabel khusus (batal = tutup dialog standar).

## 2. Screen `/customers/create` & `/customers/:partyId/edit` — Form Pelanggan

**Header**: breadcrumb `Dashboard > Pelanggan > Tambah|Edit`; judul `Tambah Pelanggan` / `Edit
Pelanggan`; deskripsi `"Catat entitas penerima layanan dan penjualan Anda."`; aksi `[← Kembali]`
ghost → **selalu `/customers`** (bukan halaman sebelumnya).

**Keadaan by-id** (edit id di luar store / langsung / refresh): fetch `customers/detail` → loading:
header `Memuat Pelanggan` + notice biru `Memuat data pelanggan` / `Mohon tunggu sebentar.`; gagal:
header `Pelanggan tidak ditemukan` + notice merah `Pelanggan tidak bisa diedit` / `Data pelanggan
tidak ditemukan atau sudah dihapus.` + tombol kembali.

**Kartu Profil Kontak** (`"Informasi identifikasi utama"`, grid 280px): urutan field —
`ID / Kode Pelanggan (opsional)` (`code`, placeholder `"Kosongkan untuk dibuatkan otomatis"`) →
`Nama Lengkap / Perusahaan`* (`name`, required) → `Jenis Member` (`member-type-id`; hanya bila
`member_type.view`; opsi `Non-member` + `"nama (kode)"` member aktif + member saat ini) →
`Telepon / WhatsApp` (`phone`) → `Email` (`email`, `type=email`) → `Alamat Lengkap (opsional)`
(textarea 80px, `col-span-full`) → `Catatan Tambahan (Opsional)` (textarea 80px, placeholder
`"PIC, Syarat khusus, dll..."`, `col-span-full`).

**Kartu Buku Alamat Kirim** (edit saja): judul + deskripsi `"Alamat tujuan kirim yang dapat dipilih
saat membuat order atau di kasir. Satu alamat menjadi Utama (default)."` → daftar/borang alamat
(§4). Tambah pelanggan tidak punya kartu ini (alamat pertama dibuat setelahnya atau inline di order).

**Kartu Informasi Tambahan** (`"Catat data spesifik pelanggan seperti NPWP, detail bank, nama PIC,
dsb."`): baris `Nama Informasi (Contoh: NPWP)` (dropdown saran kunci pelanggan lain + ketik baru,
`"Pilih atau ketik..."`) + `Keterangan` (`"Isi detail informasi"`) + `[Hapus]` merah; `[+ Tambah
Informasi Baru]`.

**ActionRow**: `[Batal]` (secondary → `/customers`) + `[Simpan Data Pelanggan]` (submit; tanpa label
sibuk/disabled — klik ganda memungkinkan kirim ganda, [PERLU KONFIRMASI] anti-double-submit).

## 3. Screen pemasok — selisih dari pelanggan

Header deskripsi `"Mencatat mitra pengadaan dan partner stok operasional."`; judul `Tambah/Edit
Pemasok`; **tanpa** field Jenis Member dan **tanpa** kartu Buku Alamat; contoh atribut
`"Nama Informasi (Contoh: NPWP, Rekening)"` + deskripsi `"...seperti NPWP, detail bank pencairan,
nama PIC, dsb."`; tombol kirim `Simpan Data Pemasok`; `[← Kembali]` memakai `navigate(-1)`
(beda dengan pelanggan yang selalu ke `/customers` — → KI-68). ID field: `supplier-search`,
`supplier-attribute-name-{id}`, dst. (pola sama, prefix `supplier-`).

## 4. Komponen `CustomerAddressBook` (kartu di form edit)

Keadaan: `"Memuat alamat…"` → kosong: `"Belum ada alamat kirim tersimpan."` → baris (grid-2,
garis-bawah putus): kiri label tebal (atau `"Alamat"`) + tag `Utama` (brand 0.72rem) + penerima
(`nama · telepon`, bila ada) + teks alamat; kanan `[Jadikan Utama]` (ghost kecil, non-primer saja)
+ `[Edit]` + `[Hapus]` (ghost kecil). Borang (ganti tombol `+ Tambah alamat`): `Label (mis. Gudang /
Proyek A)` · penerima + HP grid-2 · `Alamat lengkap *` · `[Batal]` / `[Simpan alamat]`
(`"Menyimpan…"`, disabled bila alamat kosong; `busy` menonaktifkan semua tombol). Edit mengisi
borang dari baris; simpan pertama (`addresses.length===0`) mengirim `isPrimary: true`.

## 5. Komponen `AddressPicker` (form order & POS)

Dropdown (`SearchableSelect`): placeholder `"Memuat alamat…"` / `"Pilih alamat kirim"`; opsi alamat
(`"{label|Alamat} — {≤50 char}… {(Utama)}"`) + `"Alamat lain (sekali pakai)…"` + `"Tanpa alamat
kirim"`. Di bawahnya: pratinjau tersimpan (`Penerima: {nama} · {telepon}` + alamat, 0.8rem redup)
atau borang ad-hoc (textarea `"Alamat tujuan *"` + `Nama penerima (opsional)` + `No. HP penerima
(opsional)`) atau borang tambah (`Label (mis. Gudang / Proyek A)`, penerima, HP, `Alamat lengkap *`,
`[Batal]` / `[Simpan alamat]` disabled-kosong). Selalu ada tombol `+ Tambah alamat baru` (secondary
kecil). Walk-in: satu textarea `"Alamat kirim (opsional)"` 2 baris.

## 6. Komponen `PartySearchSelect` (dropdown pihak di order & modul lain)

Trigger placeholder `"Pilih customer"` / `"Pilih supplier"` (atau custom); cari `"Cari nama, kode,
atau kontak..."`; opsi `nama (+ (member))` + deskripsi kontak; kosong dimungkinkan (silang hapus,
default). Resolve instan bila seed tersedia, else fetch `detail` senyap (gagal → kosong).

## 7. Perilaku visual & format yang mengikat

- Sampah adalah **tab**, bukan halaman: URL `?view=trash`, judul/kolom/deskripsi berganti, dialog
  tak terbawa pindah tab.
- Tanggal hanya satu: kolom `Dihapus` format lokal `id-ID` (mis. `9/9/2026, 10.00.00` — format
  `toLocaleString`, [PERLU KONFIRMASI] apakah perlu format tanggal baku `dd-mm-yyyy hh:mm`).
- Tidak ada badge status aktif/nonaktif (pihak tidak punya status — hanya ada/tidak di Sampah).
- Tidak ada foto, tidak ada tab riwayat transaksi di modul ini (riwayat ada di modul Order/Payment).
- Member selalu badge aksen di daftar + label `(kode)` di opsi form; `Non-member` selalu teks polos.
