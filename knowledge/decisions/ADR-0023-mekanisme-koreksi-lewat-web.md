# ADR-0023: Mekanisme koreksi salah input lewat web

## Status
Accepted (2026-10-21). Diputuskan atas mandat pemilik: "jika kedepannya terjadi salah input atau hal
yang berkaitan dengan human error maka harus ada mekanisme memperbaiki atau meluruskannya melalui
web". Melengkapi ADR-0016 (idempoten + CAS), ADR-0020 (invarian persediaan), dan ADR-0021 (aturan
integritas pra-migrasi).

## Context
Inventarisasi pra-go-live menemukan salah input yang tidak bisa diperbaiki lewat web, atau hanya
bisa diperbaiki dengan cara yang membuat buku, stok, dan dokumen tidak lagi cocok:

1. Pembayaran pada order COD/prabayar yang sudah selesai (termasuk semua penjualan POS) tidak bisa
   dikoreksi. Batal ditolak karena akan membuka order lagi, dan pembayaran pengganti ditolak karena
   kelebihan bayar.
2. Order yang terkirim/diterima sebagian tidak pernah bisa ditutup. Nilai sisanya terus tertagih di
   buku pembantu.
3. Harga beli yang salah setelah barang diterima tidak punya jalan perbaikan. Ubah PO bahkan
   membuang perubahan harga **diam-diam**.
4. Harga modal non-nol yang salah dan HPP basi hanya bisa diperbaiki lewat shell server
   (`costrebuild`).
5. SJ pengganti retur tukar yang salah tidak bisa ditarik.
6. Tidak satu pun pembatalan dokumen meminta atau menyimpan alasan.
7. Tidak ada alat untuk melihat bahwa buku dan dokumen sebuah pihak sudah tidak cocok.

## Decision
**Prinsip yang berlaku untuk setiap koreksi:**
- **Dokumen tidak pernah diedit diam-diam setelah berdampak.** Perbaikan dilakukan dengan dokumen atau
  aksi koreksi yang terlihat. Yang tidak diizinkan ditolak dengan pesan yang menyebut field-nya dan
  menunjuk jalan yang benar.
- **Alasan wajib di web, tersimpan di dokumen, dan diaudit dalam transaksi yang sama.** Kolom yang
  dipakai: `cancel_reason`, `cancelled_at`, dan `cancelled_by`. Audit mencatat nilai sebelum/sesudah.
  Pemanggil internal boleh mengirim alasan kosong.
- **Buku = ledger = dokumen tetap terjaga.** Koreksi yang menggeser nilai persediaan menulis jurnal dan
  baris ledger nilai-saja dalam satu transaksi, di bawah kunci biaya (ADR-0020/0021). Koreksi yang
  menyentuh piutang/hutang membawa pihaknya.
- **Idempoten.** Langkah dikunci dengan client ref atau CAS. Mengulang permintaan yang sama tidak
  menggandakan apa pun (ADR-0016).
- **Tidak ada tanggal masa depan dan tidak ada periode tertutup** (ADR-0021).

**Mekanisme per area:**

| Salah input | Mekanisme web | Ringkas |
|---|---|---|
| Metode/jumlah/tanggal/referensi pembayaran | **Koreksi pembayaran** `POST /payments/{id}/correct` | Pengganti dibuat dan aslinya dibatalkan dalam satu transaksi modul. Pihak dan order tetap sama, dihitung seolah aslinya sudah batal. Jurnal asli dibalik dan pengganti masuk antrean. Pasangan offset dan pemakaian saldo ditolak. Batas: dua pembayaran terpisah di satu order lunas belum bisa diseimbangkan ulang |
| Order jual/beli terkonfirmasi tanpa dokumen turunan | Form lengkap | Server tetap menolak perubahan ekonomi bila sudah ada SJ, penerimaan, pembayaran, atau retur |
| Info dokumen (jatuh tempo, faktur, catatan, alamat, penanggung jawab) | "Ubah info order/dokumen" | Tidak pernah menghitung ulang harga |
| Order terkirim/diterima sebagian | **Tutup sisa order** (`close-short`, beralasan) | Qty baris = qty terkirim/diterima. DPP/PPN diprorata seperti jurnal per dokumen. Order jadi selesai. Tanpa jurnal baru, buku tidak berubah. Kelebihan bayar tampil sebagai refund |
| Harga beli salah setelah penerimaan | **Koreksi harga beli** (`purchase_price_correction`) | PO mengambil harga baru, riwayat lama/baru disimpan. Selisih bagian yang sudah diterima dijurnal: Persediaan untuk bagian yang masih ada (dengan baris ledger), HPP untuk yang sudah keluar, PPN Masukan, dan Hutang pemasok. Penerimaan lama tetap dinilai seperti saat dibukukan |
| Harga modal salah (saldo awal, harga masuk) | **Revaluasi harga modal** (beralasan) | Satu baris ledger nilai-saja + satu jurnal Persediaan lawan Selisih Stok. Berlaku ke depan |
| HPP basi | **Hitung ulang HPP** (dry run, lalu komit beralasan) | Mesin `costrebuild` lewat web. Hanya sesudah batas cutover |
| SJ pengganti retur tukar yang salah | **Batalkan pengiriman pengganti** (dari halaman retur) | Draf SJ pengganti dibatalkan lewat kontrak sempit `CancelReplacement`, lalu retur kembali `pending` |
| Dokumen salah (SJ draf, penerimaan pending, retur, transfer, order, pembayaran) | Batal **beralasan** | Stok/jurnal dibalik seperti sebelumnya. Alasan tampil di detail |
| Jurnal salah | Balik Jurnal (beralasan) + jurnal manual berpihak (ADR-0021) | **Ditolak** untuk jurnal yang berpasangan dengan nilai ledger (revaluasi, hitung ulang HPP, pelengkap saldo awal, koreksi migrasi persediaan): membaliknya hanya menggeser buku |

**Deteksi:** **Rekonsiliasi Pihak** (`GET /reporting/party-reconciliation`) membandingkan, per pihak,
akun kendali (Piutang/Hutang + uang muka) dengan saldo dokumen modul pembayaran. Pihak tanpa order
berjalan wajib cocok persis. Selisihnya adalah salah input yang ditelusuri lalu dikoreksi dengan
mekanisme di atas. Dipakai juga sebagai bukti hari-H (RUNBOOK §5c).

Izin dipakai ulang, tanpa kode baru:
- Koreksi pembayaran: `payment.create` + `payment.archive`.
- Order jual/beli: `*.update` untuk ubah/koreksi, `*.archive` untuk batal/tutup sisa.
- Keuangan: `finance.manage` untuk pratinjau, `finance.close.force` untuk komit.
- Rekonsiliasi: `finance.report.view`.

## Consequences
- Pengguna bisa memperbaiki sendiri hampir semua salah input harian. Yang tersisa sengaja manual:
  menggabung pihak ganda (pakai arsip), mengubah satuan/konversi produk sesudah ada histori stok
  (produk baru + koreksi stok), dan menyeimbangkan ulang beberapa pembayaran sekaligus pada satu
  order lunas.
- Setiap koreksi meninggalkan jejak (alasan, siapa, kapan, sebelum/sesudah) di dokumen dan Jejak
  Audit.
- Penanda waktu: rekonsiliasi pihak menandai pihak yang masih punya order berjalan sebagai
  "bisa karena waktu", karena buku menagih per pengiriman sedangkan dokumen menagih per order.
- Bukti: `apps/api/internal/integration/{payment_correct,sales_corrections,purchasing_corrections,
  finance_corrections,corrections_cancel,party_recon}_test.go`.
