# User Flows — Modul 11 Delivery / Pengiriman

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** `@Post` + 200 + `{ data }` +
`BranchGuard` (controller memakai 3 guard kelas). Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Antrean, daftar SJ, URL bukti, tombol Lihat/Cetak | `order.view` | Rute diblokir (kecuali cetak SJ: login saja) |
| Terbit, Konfirmasi, Batalkan, unggah bukti (+ tombolnya) | `order.update` | Tombol hilang |
| COD `pay_now` saat konfirmasi | + `payment.create` di sesi (403 khusus konfirmasi) | — (tak ada fallback-UI; API menolak) |
| Cetak SJ | login saja (`authenticated`) | — |

## UF-01 — Konstruksi tempo: terbit → cetak → kembali → tutup (E2E 06)

1. SO tempo active (10 bata + 2 ongkir; stok 30) → antrean Buat SJ → **Buat SJ** →
   `?action=create-sj` → dialog (sisa penuh) → sopir/gudang/tanggal → **Terbitkan & Cetak** →
   nomor `SJ-...` + stok −10 saat itu + cetak berisi nama sopir/gudang.
2. SJ fisik kembali → **Konfirmasi Kembali** → arsip file + TTD-ada + nama → **Konfirmasi
   Selesai** → toast tutup-otomatis; stok tetap −10 (tanpa double); piutang > 0; nota =
   Tagihan + Sisa.

## UF-02 — COD bayar-di-kembali (E2E 06)

Terbit (3 kayu) → kembali → modal: arsip + TTD + `pay_now` tunai + catatan sopir →
konfirmasi → `payment_id` + saldo 0 + nota Lunas + bukti-bayar tercetak.

## UF-03 — Prabayar lunas-dulu (E2E 06)

Bayar penuh (modul 14) → terbit → kembali (tanpa keputusan) → `payment_id: null`; tutup.

## UF-04 — Parsial + batal-mengembalikan (E2E 06)

Terbit 1/10 → stok −1 → **Batalkan** → dialog → stok kembali (alasan `Pembatalan SJ...`) →
terbit 4/10 → konfirmasi (tanpa tutup) → terbit 6/10 → konfirmasi (tutup + tanggal-serah).

## UF-05 — Pantauan harian

`/delivery-work-queue` → tab Buat (order-terlama dulu; progress + badge tunggu) → **Buat SJ** →
dialog; tab Tunggu (kirim-terlama dulu; umur merah >7; arsip hijau/kuning) → **Cetak SJ** /
**Konfirmasi** / Lihat Order. Filter cari/tanggal/umur; reset menyisakan tab.

## UF-06 — Bukti & batal

SJ aktif → **Bukti** → modal (tombol per purpose + pratinjau) — baca saja (unggah hanya di
modal konfirmasi). SJ aktif → **Batalkan** → dialog → stok kembali + toast info (gagal → tetap
terbuka). SJ terkonfirmasi: tanpa Batalkan (API menolak).

## UF-07 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| Bukan-sales/terminal/tanpa-item/prabayar-utang | `400` terbit |
| Over-sisa/qty-0/luar-order | `400` baris |
| Stok kurang (auto/manual/duplikat/total) | `400` alokasi |
| Confirm ganda/hilang/tanpa-arsip/tanpa-alasan/pengganti | `400` konfirmasi |
| COD tanpa mode / tanpa izin bayar | `400` mode / `403` izin |
| Auto-close tanpa status/transisi | `400` tahap |
| Antrean gagal | Notice + pesan koneksi |
