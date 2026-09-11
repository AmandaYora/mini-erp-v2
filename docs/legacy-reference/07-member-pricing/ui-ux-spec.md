# UI/UX Spec — Modul 07 Member Type & Member Pricing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Tiap screen & interaksi apa adanya.
Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Rute (`module-registry.tsx`, semua `access: "protected"`, `shell: true`, satu komponen):

| Rute | Izin | Mode |
|---|---|---|
| `/member-types` | `member_type.view` | Daftar (+ menu Produk & Mitra → Jenis Member) |
| `/member-types/create` | `member_type.manage` | Form tambah |
| `/member-types/:memberTypeId/edit` | `member_type.manage` | Form edit |

---

## 1. Screen `/member-types` — Daftar

**Header**: breadcrumb `Dashboard > Jenis Member`; judul `Jenis Member`; deskripsi `"Master aturan
harga untuk pelanggan member."`; aksi **[Tambah Jenis Member]** (hanya `member_type.manage`).

**FilterBar** (1 field, state lokal — bukan URL): `Cari jenis member` (`member-type-search`,
placeholder `"Cari nama atau kode"`; ketik langsung me-reset ke halaman 1, tanpa debounce).

**Kartu** `Daftar Jenis Member`: deskripsi `"Memuat…"` / `"{n} jenis member tersedia."` (selalu
`status: "all"` = aktif + nonaktif campur). Tabel:

| Kolom | Isi |
|---|---|
| Nama | Nama tebal + kode redup |
| Rule | `"Harga beli + Rp 1.000"` — basis + `+`/`-` + `Rp`/ `%` (id-ID, maks 2 desimal) |
| Pembulatan | `Tanpa pembulatan` atau `"Ke atas Rp 100"` (kelipatan kosong → label + kosong) |
| Status | Badge hijau `Aktif` / kuning `Tidak aktif` |
| Aksi | `[Edit]` secondary + `[Arsipkan]` ghost (hanya `member_type.manage`; tanpa izin: sel kosong) |

Kosong: `Belum ada jenis member` / `Buat jenis member pertama untuk mulai mengaitkannya ke
pelanggan.` (+ Tambah bila berhak). `Pagination` 20/halaman (state lokal).

**Dialog arsip** (warning): `Arsipkan jenis member` / `"Jenis member {nama} tidak akan bisa dipilih
untuk pelanggan baru."` / `Arsipkan`. Gagal (masih dipakai) → dialog tetap terbuka + toast merah
berpesan; sukses → tutup + reload + toast hijau.

## 2. Screen create/edit — Form

**Header**: breadcrumb `Dashboard > Jenis Member > Tambah|Edit`; judul `Tambah/Edit Jenis Member`;
deskripsi **`"Aturan harga berlaku global untuk semua produk dan dihitung pada UOM jual."`**; aksi
`[← Kembali]` ghost → `/member-types`.

**Keadaan edit**: data dari store (limit 1000). Belum siap → `Memuat Jenis Member` + kartu `Memuat
data` / `Mohon tunggu sebentar.` / `Mengambil data jenis member...`. Siap tapi tak ada (arsip/salah
id) → `Jenis Member Tidak Ditemukan` + `EmptyState` + `[Lihat Daftar]`. Tanpa fetch by-id (→ KI-75).

**Kartu Identitas** (`"Kode dan nama yang tampil pada master pelanggan."`, grid 280px): `Kode`*
(placeholder `"A"`, required) → `Nama jenis member`* (placeholder `"Member A"`, required) → `Status`
(Aktif default / Tidak aktif) → `Catatan` (textarea 80px, `col-span-full`, tanpa placeholder).

**Kartu Aturan Harga** (`"Basis harga dipilih sekali untuk jenis member ini."`): `Basis harga`
(default Harga beli: Harga beli / Harga jual minimum / Harga jual normal) → `Aksi` (default Tambah:
Tambah / Kurang) → `Tipe nilai` (default Nominal: Nominal / Persen) → `Besaran`* (number, default 0,
`min=0 step=0.01`, required) → `Pembulatan` (default Tanpa pembulatan: + Ke atas / Terdekat / Ke
bawah) → `Kelipatan pembulatan` (number `min=0 step=0.01`, placeholder `"100"`, kosong = tanpa
pembulatan).

**ActionRow**: `[Batal]` (secondary → daftar) + `[Simpan Jenis Member]` (`"Menyimpan..."` + disabled
saat simpan). Gagal → toast merah + pesan, tetap di form.

## 3. Permukaan modul ini di layar modul lain (kontrak tampilan)

| Lokasi | Wujud |
|---|---|
| Form pelanggan + shortcut order (`member_type.view`) | Dropdown `Jenis Member` / opsi `Non-member` + `"nama (kode)"`; pelanggan: `#member-type-id`; order: `#shortcut-customer-member` |
| Daftar pelanggan | Kolom Member: badge aksen nama / `Non-member`; picker: label `nama (member)` |
| Baris POS (`member_rule` + nama) | Badge/label nama member di baris keranjang (komponen POS; detail milik modul 15) |
| Tombol checkout POS | `Harga di bawah minimum` / `Harga minimum` bila ada baris invalid; opsi cetak diskon muncul bila ada hemat member (`basisPrice > unitPrice`) |
| Cetak nota | Baris diskon disembunyikan bila `discount_as_price=1`; hemat member dilipat ke harga (milik modul cetak) |
| Banner quote | Error per produk (`{nama}: {pesan}`) diagregasi; gagal total → pesan koneksi/master (POS vs order beda kalimat — §4 feature) |

## 4. Perilaku visual & format yang mengikat

- Bahasa rule selalu Indonesia (label opsi + kolom Rule + pesan); nilai persen tanpa `Rp`, nominal
  dengan `Rp`; angka `id-ID` (titik ribuan, koma desimal, maks 2 digit).
- Status selalu `Aktif` (hijau) / `Tidak aktif` (kuning) — kata "Tidak aktif" (bukan "Nonaktif"
  seperti produk/pihak — inkonsistensi kecil, [PERLU KONFIRMASI] diseragamkan atau tidak).
- Kode ditampilkan apa adanya (sudah uppercase dari server); tidak ada badge/prefix khusus.
- Tidak ada tab Sampah, tidak ada kolom tanggal, tidak ada pratinjau hitungan di form (efek rule
  hanya terlihat via quote di order/POS).
