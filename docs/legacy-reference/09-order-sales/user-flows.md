# User Flows — Modul 09 Order Sales

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** `@Post` + 200 + `{ data }` +
`BranchGuard`. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Daftar/detail/cetak sales | `order.view` | Rute diblokir |
| Buat SO + shortcut pelanggan | `order.create` | Tombol hilang |
| Edit/pindah status/serah langsung | `order.update` | Tombol hilang |
| Arsip SO | `order.archive` | — |
| Catat bayar / bukti | `payment.create` | Tombol hilang (`+ Catat Pembayaran`, `Edit` bukti) |
| Hapus bayar | `payment.archive` (disembunyikan bila terminal + non-net) | Tombol hilang |
| Ubah ke Tempo | `order.update` (tombol ghost di seksi bayar) | Tombol hilang |
| COD-serah bayar-kini | + `payment.create` di sesi (403 khusus) | Opsi hilang di SJ flow; deliver langsung 403 |
| Retur/Tukar, Terbitkan SJ | `sales_return.create`, `order.update` (syarat modul 11/12) | Tombol hilang |

## UF-01 — SO tunai walk-in (kasir manual)

1. Tab Penjualan → **Buat Order Baru** (default Penjualan) → customer dikosongkan → baris
   (harga saran jual, qty) → termin Bayar Saat Serah → simpan → nomor `.../PJ/...`.
2. Detail → serah langsung (tanpa SJ; butuh jalur tahap) atau Terbitkan SJ (modul 11).
   Tunai: `amount_tendered` ≥ tagihan → kembalian tampil di struk (tidak tersimpan).

## UF-02 — SO tempo + member + diskon

1. Pilih customer tempo (wajib + HP) → requote member → diskon baris (diklem) → alamat kirim →
   jatuh tempo → simpan (guard minimum dilewati baris member; baris lain dijaga).
2. **Ubah ke Tempo** hanya untuk non-net; tempo sejak awal tak perlu persetujuan.
3. Cetak Tagihan → sebagian bayar → judul berubah Bayar Sebagian → lunas → Nota Lunas.

## UF-03 — Prabayar (kunci → bayar → jalan)

1. Buat prepaid → seksi bayar tampil sejak Draft → tombol proses/SJ disembunyikan + notice →
   lunasi → aktif → SJ/serah → selesai (E2E 06/09 mengunci; blokir pesan lapis status +
   selesai-terminal).

## UF-04 — COD dua keputusan (langsung & SJ)

Serah langsung final: Bayar Sekarang (tunai + kembalian / non-tunai + referensi + bukti
opsional) atau Ubah ke Tempo (jatuh tempo). Sama di SJ terakhir (modul 11); sekali bayar =
sisa penuh (tanpa cicilan).

## UF-05 — Ubah ke Tempo manual

Seksi bayar → **Ubah ke Tempo** → isi tempo → simpan → badge Termin Tempo + toast; arsip/bayar
tetap; audit `order.approve_credit` (tanpa sumber / dengan sumber serah).

## UF-06 — Cetak & sembunyi-diskon

Nota → preliminari Notes (tak tersimpan) → kertas/preprinted/diskon → Cetak. Mode
`discount_as_price=1`: harga penuh, tanpa Disc, total/ bayar bruto, sisa riil. Bukti bayar per
pembayaran; struk POS thermal (modul 15).

## UF-07 — Edit/arsip/status SO

Edit (harga-penuh anti-ganda, requote hormat-manual) → pindah status (label transisi; selesai
terkunci pre-serah; pasca-serah hanya selesai) → arsip (guard tiga-lapis; tanpa restore).

## UF-08 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| Tempo tanpa customer/HP; pihak supplier | `400` spesifik |
| Di bawah minimum (non-member); diskon berlebih | `400` dua-nominal / batas |
| Selesai pre-serah; serah ganda; prepaid berutang | `400` lapis status/serah |
| Tunai kurang; tanpa izin bayar-serah | `400` tender / `403` izin |
| Tempo tanpa tanggal (dialog/API) | Tombol disabled / `400` |
| Quote error | Banner + submit diblokir |
| Cetak non-sales | Pesan khusus (bukan 404) |
