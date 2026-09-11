# User Flows — Modul 07 Member Type & Member Pricing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua endpoint `@Post` +
`@HttpCode(200)` + `{ data }`; scope perusahaan dari sesi; tanpa BranchGuard. Bagian ambigu
ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Buka `/member-types`, cari, lihat rule | `member_type.view` | Rute diblokir |
| Tambah/Edit/Arsipkan + tombolnya + rute form | `member_type.manage` | Tombol hilang (sel aksi kosong); rute diblokir |
| Field member di form pelanggan/shortcut order | `member_type.view` | Field hilang (nilai lestari — KI-67 modul 06) |
| Harga member otomatis di order/POS (`pricing/quote`) | `order.create` (bukan member_type.*) | Tanpa harga otomatis (harga manual/standard) |
| Penetapan member ke pelanggan | mengikuti form pelanggan (`order.update`) | — |

## UF-01 — Buat jenis member

1. `/member-types` → **[Tambah Jenis Member]** (`member_type.manage`) → `/member-types/create`.
2. Kode* (`A` — tersimpan uppercase) → Nama* → Status (default Aktif) → Catatan → Basis (default
   Harga beli) → Aksi (default Tambah) → Tipe (default Nominal) → Besaran* (default 0) →
   Pembulatan (default Tanpa) → Kelipatan (mis. `100`).
3. **[Simpan Jenis Member]** → toast `"Jenis member dibuat"` → daftar; kolom Rule langsung
   menampilkan (`Harga beli + Rp 1.000`). Gagal (duplikat/negatif/enum) → toast merah + pesan,
   tetap di form.

## UF-02 — Ubah rule yang dipakai

1. Daftar → **[Edit]** → ubah (mis. besaran 10% → 15%, atau basis) → simpan → toast
   `"Jenis member diperbarui"`.
2. **Order lama/draft tidak berubah** (snapshot final). Order **baru** untuk pelanggan member
   memakai rule baru. Tanpa peringatan, tanpa re-price.
3. Nonaktifkan/arsip saat masih dipakai pelanggan aktif → toast gagal + hitungan
   (`...masih digunakan oleh {n} pelanggan aktif...`); dialog arsip tetap terbuka.

## UF-03 — Tetapkan & lepas member pelanggan

1. Form pelanggan (punya `member_type.view`) → pilih jenis → simpan → badge member muncul di
   daftar + label picker (`nama (member)`).
2. Order/POS berikutnya untuk pelanggan itu otomatis memakai harga rule (via quote).
3. Lepas (pilih `Non-member`/null) → harga kembali standard/manual. Arsip member baru bisa setelah
   semua pelanggan dilepas (guard E2E 22-B modul 06).

## UF-04 — Jual ke member di Order

1. Form order sales → pilih pelanggan member → requote: baris non-manual ditimpa harga member;
   baris manual (`priceEdited`) tidak; diskon di-klem ulang (tidak ditolak backend saat Simpan).
2. Baris error (basis kosong/negatif/konversi) → banner per produk; baris sehat tetap memakai harga
   member. Simpan → snapshot 7 kolom per baris; guard minimum dilewati untuk baris member.
3. Ganti pelanggan non-member → harga kembali saran standard (baris manual tetap).

## UF-05 — Jual ke member di POS

1. Pilih pelanggan di checkout → quote per isi keranjang → harga + badge member per baris;
   tambah barang setelahnya ikut di-quote (kunci request per pelanggan+isi).
2. Quote gagal total → banner `"Harga member belum bisa dihitung..."`; per produk gagal → banner
   agregat; baris gagal mempertahankan harga sebelumnya ([PERLU KONFIRMASI] harga lama vs standard —
   kode hanya melewati (`return`) tanpa me-reset baris itu).
3. Selesai → order tersimpan dengan snapshot; struk/nota memakai snapshot; opsi cetak
   `discount_as_price` tersedia bila ada hemat member.

## UF-06 — Arsip jenis member

1. Daftar → **[Arsipkan]** → dialog (`...tidak akan bisa dipilih untuk pelanggan baru.`) →
   **[Arsipkan]** → hilang dari daftar (tanpa Sampah) + toast sukses; atau toast gagal berhitung
   bila masih dipakai.
2. Pelanggan yang telanjur memakai (seharusnya tak ada karena guard) → quote jatuh ke standard
   diam-diam (E-21); order lama tetap bersnapshot.

## UF-07 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| Kode duplikat aktif | Toast gagal + `409` spesifik |
| Kode milik arsip | **500** tanpa pesan ramah (→ KI-71) |
| Besaran/kelipatan negatif, enum salah, kosong wajib | Toast + `400` spesifik |
| Nonaktif/arsip dipakai | Toast + `400` berhitung; dialog arsip tetap terbuka |
| Quote: member/party/produk bermasalah | Per-item `error` (200 tetap); total-gagal → banner koneksi |
| Tanpa `order.create` di POS/order | Tanpa harga member (tanpa pesan khusus — [PERLU KONFIRMASI] perlu pemberitahuan?) |
