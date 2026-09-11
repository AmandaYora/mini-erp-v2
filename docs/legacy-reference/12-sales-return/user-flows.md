# User Flows — Modul 12 Sales Return (Retur Penjualan)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** `@Post` + 200 + `{ data }` +
`BranchGuard` (controller kelas). Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Daftar/detail/konteks | `sales_return.view` | Rute diblokir |
| Preview/buat/dispatch/confirm | `sales_return.create` | Rute + tombol diblokir |
| Opsi refund di form | `sales_return.refund` (opsi hilang; paksa via API → 403) | Opsi hilang |
| Buka dari detail order (`Retur / Tukar`) | `sales_return.create` + sudah-serah/selesai | Tombol hilang |

## UF-01 — Retur saja, kondisi normal (uang kembali via piutang)

Order lunas 2×20rb → buat dari `?order_id=` → centang 2 normal + lokasi → alasan → live Impas?
(tanpa pengganti: retur 40rb, selisih −40rb → potong piutang 0? — lunas → sisa-eksternal penuh →
kredit/refund) → preview (notice + tabel) → simpan → stok +2, movement `in`, settlement kredit
40rb, detail (tanpa kas). [PERLU KONFIRMASI contoh angka — ilustrasi, bukan test.]

## UF-02 — Tukar POS langsung (E2E 18 pola)

Order POS → retur 2 normal + pengganti 2 (harga sama) → selisih 0 → `none` → simpan → stok
lama +2, pengganti −2 seketika, status `not_required`. Tanpa SJ.

## UF-03 — Tukar Order via SJ (E2E 18 terkunci)

Retur 2 + pengganti 2 (nilai sama) → simpan → pengganti TETAP 5 (`pending`; tanpa gerak, tanpa
guard) → seksi: sopir/plat/petugas/tanggal + lokasi → dispatch (stok 5→3, `dispatched`, SJ
normal) → confirm TTD + nama → `delivered`. Retur-lama +2 (8→10) sejak buat.

## UF-04 — Selisih mahal (tambah bayar)

Pengganti lebih mahal → notice tambah-bayar → preview (`Customer perlu tambah bayar sebelum
dokumen disimpan.`) → kasir terima tunai di luar sistem → simpan `collect_payment` (tanpa
baris `payments`! → KI baru) → settlement tercatat + piutang-efektif naik.

## UF-05 — Selisih murah bertingkat

Negatif kecil (≤ sisa) → `reduce_receivable` tanpa baris kas; negatif besar → potong + sisa
kredit/refund (izin); walk-in → refund (izin) else 403; paksa tipe salah → 400 dua pesan.

## UF-06 — Rusak & tak-kembali

Rusak → stok ke lokasi RUSAK auto (tanpa pilih); tak-kembali → tanpa gerak stok tetapi nilai
penuh dihitung (potong piutang/refund). Keduanya tanpa lokasi-field.

## UF-07 — Preview-sebelum-simpan (kontrak UI)

Tanpa preview valid → simpan ditolak warning (`Preview belum dikonfirmasi`). Setiap perubahan
(baris, alasan, settlement, tanggal, referensi, catatan) memanggil invalidasi: modal tertutup +
hasil preview dibuang → wajib `Preview & Cek` ulang. Tipe settlement yang tersimpan = hasil
resolve preview terakhir (pilihan `Otomatis` diterjemahkan server; pilihan eksplisit yang lolos
aturan dipakai). Tidak ada jalur simpan-tanpa-preview yang basi.

## UF-08 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| Order belum-serah/purchase/tak-ada | Toast `Order tidak bisa diretur` + `400/404` |
| Over-sisa/duplikat/qty-0/kondisi/lokasi | `400` baris |
| Pengganti nonaktif/di-bawah-minimum/tanpa-stok/tanpa-modal | `400` pengganti |
| Selisih-salah-tipe/refund-tanpa-izin | `400` / `403` |
| Dispatch bukan-pending/lokasi-kosong/stok-kurang | `400` dispatch |
| Confirm ganda/nama-kosong/alasan-kosong | `400` confirm |
