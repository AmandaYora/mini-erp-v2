# ADR-0022: Koreksi order beli — batal beralasan, tutup sisa order, koreksi harga setelah penerimaan

## Status
Accepted (2026-10-21). Melengkapi ADR-0016 (idempoten + CAS), ADR-0020 (invarian persediaan), dan
ADR-0021 (aturan integritas pra-migrasi).

## Context
Menjelang go-live, pengguna harus bisa membetulkan salah input order beli sendiri lewat web, tanpa
merusak buku, ledger biaya, atau dokumen. Audit menemukan:

1. Ubah PO terkonfirmasi yang sudah punya penerimaan/pembayaran/retur hanya menyimpan kepala
   dokumen. Perubahan harga/diskon di payload yang sama lolos dan **hilang diam-diam**.
2. Batal PO menyuruh "batalkan penerimaannya dulu", padahal penerimaan terposting tidak bisa
   dibatalkan. PO yang sebagian diterima tidak pernah bisa ditutup.
3. Harga beli yang salah setelah penerimaan terposting tidak bisa dibetulkan sama sekali. Jurnal
   penerimaan, ledger biaya, dan saldo pemasok semuanya membaca ekonomi baris PO.

## Decision
- **Tidak ada yang dibuang diam-diam.** Selama ada yang bergantung pada PO, perubahan pemasok,
  termin, pajak, tanggal order, atau baris ditolak dengan nama field-nya. Info dokumen (jatuh
  tempo, faktur, penanggung jawab, catatan) tetap bisa diubah, juga lewat `PUT …/header`. Setiap
  perubahan PO terkonfirmasi diaudit dengan nilai sebelum/sesudah dan alasan opsional.
- **Batal wajib beralasan.** `cancel_reason`/`cancelled_at`/`cancelled_by` disimpan, audit
  ikut transaksi. PO dengan penerimaan terposting tidak bisa dibatalkan. Barangnya kembali lewat
  retur pembelian, atau sisanya ditutup.
- **Tutup sisa order.** PO terkonfirmasi dengan ≥1 penerimaan terposting dan tanpa penerimaan
  pending: qty baris = qty diterima. DPP dan PPN baris **diprorata dari split tersimpan**, persis
  seperti jurnal penerimaan memproratanya, bukan dihitung ulang dari harga. Total PO jadi sama
  dengan hutang yang sudah dijurnal per penerimaan. Tanpa jurnal baru.
- **Koreksi harga beli setelah penerimaan.** PO mengambil harga baru (PO = buku pembantu pemasok).
  Riwayat baris lama/baru disimpan di `purchase_order_corrections`/`…_lines`. Selisih bagian yang
  sudah diterima per baris = `Prorate(baru, diterima, qty) − Prorate(lama, diterima, qty)` (DPP dan
  PPN), menjadi dokumen posting finance `purchase_price_correction`:
  - Persediaan menerima bagian barang yang masih di gudang, yaitu
    `min(qty posisi, qty diterima) / qty diterima`. Nilainya tidak boleh membuat posisi negatif.
    Jurnal dan **baris nilai-saja** di ledger biaya (`ref_type = purchase_price_correction`,
    qty 0, id sintetis) ditulis dalam satu transaksi di bawah kunci biaya cabang, sehingga buku =
    ledger (ADR-0020).
  - HPP menerima bagian barang yang sudah keluar. Baris jasa ke Beban.
  - PPN Masukan menerima selisih pajak. Hutang (dengan pihak pemasok) menerima jumlahnya.
  - Membalik jurnal ini ikut menulis baris nilai-saja kebalikannya. Posting ulang tidak pernah
    menghitung dua kali.
  - Ditolak bila PO batal/draf, ada penerimaan pending, ada retur hidup (retur memakai harga
    lama), atau bulan hari ini tertutup / sebelum awal buku.
- **Penerimaan lama tetap dinilai seperti saat dibukukan.** Tiap koreksi mencatat qty yang sudah
  diterima per baris. Jurnal penerimaan dan `receiptValue` memakai ekonomi **lama** untuk jendela
  penerimaan yang sudah tercakup (`contracts.LineAt`), dan ekonomi baru sesudahnya. Membangun ulang
  jurnal penerimaan sesudah koreksi atau tutup sisa order tidak pernah menghitung selisihnya dua
  kali. Penerimaan + koreksi selalu berjumlah total PO terkini.

## Consequences
- Izin dipakai ulang: ubah/info dokumen/koreksi harga = `purchasing.update`; batal/tutup sisa =
  `purchasing.archive`.
- Pembagian Persediaan/HPP dihitung saat jurnal diposting (bukan saat koreksi dibuat). Barang yang
  sudah dipindah ke cabang lain dihitung "sudah keluar" (HPP), karena ledger cabang tujuan memakai
  nilai lama.
- Baris nilai-saja memakai id sintetis (seperti pelengkap harga modal saldo awal). Restatement
  biaya mengurutkannya sesudah mutasi nyata, keterbatasan yang sama dengan `opening_cost`.
- Purchasing bergantung lambat pada `financecontracts.PurchaseCorrectionGate` (cek periode +
  pembagian). Siklus konstruksi ditutup `bootstrap.Wire` dan diperiksa `Verify`.
- Bukti: `apps/api/internal/integration/purchasing_corrections_test.go`.
