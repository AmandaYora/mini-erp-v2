# User Flows — Modul 06 Business Party (Customer / Supplier)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Alur end-to-end per skenario.
Semua endpoint `@Post` + `@HttpCode(200)` + `{ data }`; scope perusahaan dari sesi; **tanpa
BranchGuard** (company-scoped). Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Buka `/customers`, `/suppliers`, cari, picker pihak | `order.view` | Rute diblokir |
| Tombol Tambah + `/customers/create`, `/suppliers/create` | `order.create` | Tombol hilang; rute diblokir |
| Tombol Detail + `/customers/:id/edit`, `/suppliers/:id/edit` | `order.update` | Tombol hilang; rute diblokir |
| Hapus + Pulihkan (kedua tab) | `order.archive` | Tombol hilang |
| Field Jenis Member di form | `member_type.view` (baca; nilai lama lestari bila tak terlihat — E-05) | Field hilang |
| Buku alamat: lihat/tambah | `order.view` / `order.create` | — |
| Buku alamat: ubah + arsip (termasuk Hapus & Jadikan Utama) | `order.update` (bukan `order.archive`!) | Tombol tetap tampil (tidak di-gate di UI — panggilan gagal di server bila tanpa izin). [PERLU KONFIRMASI] gate UI |
| Pintu cepat Order/POS | mengikuti izin pintu masing-masing (`order.create` untuk shortcut order) | — |

## UF-01 — Tambah pelanggan/pemasok lengkap (menu)

1. `/customers` → **[Tambah Pelanggan]** → `/customers/create`.
2. Kode **dikosongkan** (placeholder `"Kosongkan untuk dibuatkan otomatis"`) → isi Nama* → member
   (opsional) → kontak → alamat (opsional — dikunci E2E 20) → catatan → atribut (opsional).
3. **[Simpan Data Pelanggan]** → `POST customers/create` → pindah `/customers` **tanpa toast** →
   kode `CUS-###` terisi otomatis, terlihat di daftar + dapat dicari.
4. Pemasok identik via `/suppliers` (`SUP-###`, tanpa member/alamat-buku).

## UF-02 — Tambah kilat dari Order / POS (tiga pintu, satu entitas)

1. **Order:** `/orders/create` → **[Tambah Pelanggan]** → modal: nama* (`#shortcut-customer-name`),
   telepon, alamat (opsional), member → **[Simpan & Pilih Pelanggan]** → modal tutup + pelanggan
   langsung terpilih di dropdown order (`#relatedPartyId` memuat nama).
2. **POS bayar-sekarang:** checkout → **[Tambah Pelanggan Baru]** → nama* + HP → **[Simpan
   Customer]** → autocomplete terisi nama.
3. **POS bayar-nanti:** sama + HP wajib (`required` + pesan `"No. HP wajib diisi untuk customer yang
   berhutang"`).
4. Nama kosong/spasi di ketiga pintu → pesan `"Nama pelanggan wajib diisi"` (JS trim; di menu utama
   hanya `required` HTML). Kode selalu auto + unik lintas pintu (dikunci E2E 20).
5. Hasil dari pintu mana pun: terlihat & bisa diedit di menu Pelanggan tanpa wajib isi alamat.

## UF-03 — Edit pelanggan/pemasok

1. Daftar (tab Aktif) → **[Detail]** → `/customers/:id/edit` terisi penuh. Id di luar 100 store /
   URL langsung / refresh → fetch `customers/detail` (+ layar muat/tidak-ditemukan; **tidak pernah**
   jatuh ke mode tambah — perbaikan E2E 21).
2. Ubah field → **[Simpan Data Pelanggan]** → `POST customers/update` → kembali `/customers` tanpa
   toast. String kosong menghapus nilai kontak/alamat/catatan; kode yang diketik ulang **diabaikan**;
   member tak dikirim (tanpa izin lihat) = lestari.
3. Rename: order lama tetap tertaut by-id dan mengikuti nama baru di pencarian order (live, bukan
   snapshot — dikunci E2E 22); tidak ada duplikat.

## UF-04 — Hapus (arsip) & pulihkan

1. Tab Aktif → **[Hapus]** → dialog (`... dipindahkan ke Sampah. Riwayat transaksinya tetap aman
   dan data bisa dipulihkan kapan saja dari tab Sampah.`) → **[Hapus]** → baris hilang (reload
   diam-diam) atau notice `Tindakan gagal` + pesan.
2. Tab Sampah (`?view=trash`, kolom `Dihapus`) → **[Pulihkan]** → dialog (`... akan dikembalikan ke
   daftar aktif.`) → **[Pulihkan]** → kembali ke Aktif atau notice gagal (mis. bentrok kode — yang
   saran perbaikannya tak bisa dijalankan, → KI-62).
3. Efek arsip (dikunci E2E 22): hilang dari daftar + picker (bisnis baru tertutup — order baru
   ditolak modul Order); **tetap** di saldo piutang/utang, ledger terbuka, pelunasan berjalan.
   Arsip bukan penghapusan relasi.

## UF-05 — Kelola buku alamat pelanggan

1. Edit pelanggan → kartu Buku Alamat Kirim → `+ Tambah alamat` → isi (alamat wajib) → **Simpan** →
   daftar bertambah; pertama otomatis `Utama`.
2. Baris → **Jadikan Utama** (primer pindah seketika) / **Edit** (borang terisi → simpan) /
   **Hapus** (langsung arsip tanpa dialog; bila primer, aktif lain paling awal naik otomatis).
3. Semua diam (tanpa toast/notice); gagal juga diam (→ KI-63).

## UF-06 — Pilih alamat kirim di order/POS

1. Pilih pelanggan di form order → buku alamat dimuat → Utama terpilih otomatis (kecuali form sudah
   punya pilihan, mis. edit order bersnapshot).
2. Ganti ke alamat tersimpan lain → pratinjau penerima+alamat; atau `Alamat lain (sekali pakai)…`
   → ketik ad-hoc; atau `Tanpa alamat kirim`; atau `+ Tambah alamat baru` (tersimpan + langsung
   terpilih, perlu `order.create`).
3. Simpan order → server: id tersimpan divalidasi milik pihak itu lalu disalin jadi snapshot;
   ad-hoc disalin langsung; kosong mengosongkan. Cetak SJ/Nota memakai snapshot (stabil walau buku
   berubah). Walk-in: textarea opsional tanpa buku.

## UF-07 — Cari & pilih pihak (picker)

1. Ketik di dropdown order/stok/laporan → hasil server (nama/kode/telepon/email/alamat untuk daftar;
   picker menampilkan nama + kontak) → pilih → id tersimpan; label tersimpan di-resolve via detail.
2. Pihak arsip tidak muncul (pemilihan baru tertutup); pihak ke-101+ tetap ketemu (server-side).
3. Member terlihat di label (`nama (member)`) — penetapan harga member terjadi di modul Order (07).

## UF-08 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons pengguna |
|---|---|
| Kode duplikat (termasuk Sampah) | Form diam (→ KI-61); API `409` (+ ` (ada di Sampah)` bila arsip) |
| Simpan gagal apa pun (form) | Diam total, tetap di form (→ KI-61) |
| Hapus/pulihkan gagal (daftar) | Notice `Tindakan gagal` + pesan server |
| Simpan alamat gagal | Diam (→ KI-63) |
| Alamat kosong | Tombol disabled (UI); API `400 'Alamat wajib diisi.'` |
| Member supplier / tak dikenal / nonaktif | API `400` (hanya via request langsung — UI tak menawarkan) |
| Id edit tak dikenal | Layar `... tidak ditemukan` (bukan form tambah) |
| Restore bentrok kode | Notice + `409` yang tak bisa ditindaklanjuti (→ KI-62) |
