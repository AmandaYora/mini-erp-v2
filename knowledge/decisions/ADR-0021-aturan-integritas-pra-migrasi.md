# ADR-0021: Aturan integritas pra-migrasi — tanggal, awal buku, pihak di akun kendali

## Status
Accepted (2026-10-21). Hasil audit akhir sebelum migrasi data produksi. Melengkapi ADR-0006
(zona waktu WIB terkunci), ADR-0016 (atomisitas lewat idempoten + CAS), ADR-0018 D11 (koreksi
migrasi), dan ADR-0020 (invarian persediaan).

## Context
Audit akhir pra-migrasi (fitur, alur data, dokumen) menemukan beberapa jalan masuk data yang
membuat buku, dokumen, dan ledger tidak lagi saling menjelaskan setelah cutover:

1. Dokumen dan jurnal boleh bertanggal di masa depan. Mutasi stok dicap "sekarang", sedangkan
   jurnalnya memakai tanggal dokumen, sehingga keduanya jatuh di hari berbeda. Saldo seumur hidup
   (dashboard) juga ikut menghitung jurnal yang tanggalnya belum tiba.
2. Bulan sebelum periode pertama yang tercatat dianggap terbuka. Jurnal bertanggal sebelum saldo
   awal migrasi mengubah setiap saldo sesudahnya tanpa masuk periode yang bisa ditinjau atau
   ditutup.
3. Jurnal manual dan saldo awal bisa memakai akun Piutang, Hutang, atau Uang Muka tanpa pihak.
   Neraca bergerak, tetapi laporan per pihak dan umur dokumen tidak pernah melihatnya.
4. `costrebuild` berjalan sebagai proses terpisah. Kunci dalam-proses (`doclock`) tidak
   menghalangi penghitungan biaya server. Alat ini juga menulis ulang histori legacy yang dimigrasi.
5. Pembatalan pasangan offset otomatis retur, pembatalan/ubah order, dan pembayaran memakai kunci
   berbeda. Uang bisa masuk ke order yang sedang dibatalkan, atau retur menyisakan cermin offset
   yang sudah dibatalkan.

## Decision
- **Tanggal ≤ hari ini (WIB).** Tanggal SJ, pembayaran (termasuk pelunasan SJ COD), retur,
  penyelesaian retur, transfer stok, biaya, dan jurnal (manual, saldo awal, penyesuaian fiskal,
  serta jurnal dari dokumen) tidak boleh setelah hari ini. Dokumen legacy yang terlanjur bertanggal
  ke depan tetap di antrean posting dan dijurnal oleh tutup harian tanggalnya sendiri. Tutup
  harian menolak tanggal yang belum tiba.
- **Awal pembukuan.** Setelah ada bulan yang ditutup, bulan yang lebih awal dari baris periode
  pertama cabang berada di luar buku. Jurnal ke sana ditolak, baik di pra-cek maupun di dalam
  transaksi insert, tanpa membuat baris periode baru. Cabang yang belum pernah menutup bulan belum
  punya awal buku yang tetap.
- **Pihak di akun kendali.** Baris jurnal manual dan saldo awal ke akun yang dipetakan
  `receivable`/`customer_advance` wajib memilih pelanggan. Baris ke `payable`/`supplier_advance`
  wajib memilih pemasok. Baris akun lain tidak boleh membawa pihak. Penyesuaian fiskal boleh
  membawa pihak tanpa diwajibkan, karena jurnal fiskal tidak dibaca laporan per pihak.
- **Kunci biaya lintas proses.** Semua penulis ledger biaya cabang (sinkron server, `costrebuild`,
  pelengkapan saldo awal) memegang `doclock` **dan** kunci MySQL `GET_LOCK('<db>:fincost:<cabang>')`.
  `costrebuild` hanya merestatement baris keluar dengan id mutasi **di atas batas cutover**
  (`finance_cutover.stock_movement`). Histori migrasi adalah bukti dan tidak ditulis ulang
  (ADR-0020).
- **Kunci pembayaran.** Batal dan ubah SO/PO memegang `paymentcontracts.LockKey` (pihak, atau order
  walk-in) di dalam kunci dokumennya, dengan urutan kunci dokumen lalu kunci pembayaran, sama dengan
  alur pembayaran yang sudah ada. Penyelesaian retur diserialkan per retur.
- **Offset otomatis retur milik retur.** Membatalkan pasangan `offset:sr:<id>` / `offset:pr:<id>`
  melepas cerminnya di retur (pengurangan piutang/hutang dan memo otomatis). Konfirmasi ulang tidak
  memasangnya kembali.

## Consequences
- Bukti hari-H (RUNBOOK §5c) bisa menyisakan satu blocker `unposted` untuk dokumen legacy
  bertanggal setelah hari-H. Ini memang benar, karena bulannya belum boleh ditutup sebelum berakhir.
- `negative_inventory` yang berasal dari histori legacy tidak lagi bisa "diperbaiki" `costrebuild`.
  Posisinya dicatat untuk akuntan.
- Selama `costrebuild` berjalan, posting yang datang menunggu maks. 30 detik, lalu ditolak dengan
  pesan "sedang dihitung ulang".
- Tes integrasi tidak boleh lagi memakai tanggal setelah hari eksekusi, dan tidak boleh memakai
  akun kendali tanpa pihak.
