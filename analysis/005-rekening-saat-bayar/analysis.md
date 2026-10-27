# Analisis 005 — Rekening dipilih saat catat pembayaran saja

**Tanggal:** 2026-10-27 · **Pemicu:** pengguna menilai rekening tujuan idealnya hanya di "Catat pembayaran", bukan di
tahap awal (order) · **Lanjutan dari:** [ADR-0026](../../knowledge/decisions/ADR-0026-rekening-tujuan.md),
[analysis/002](../002-rekening-tujuan-dan-retur-sebagian/analysis.md), [analysis/004](../004-satu-sumber-rekening/analysis.md)

Tanpa `flow.mmd`/`sequence.mmd`: tidak ada alur baru, hanya satu tingkat pilihan yang dihapus.

## 1. Keadaan sekarang (live `4b9b46c`)

ADR-0026 memodelkan rekening di **dua tingkat**:

| Tingkat | Tempat | Disimpan di | Fungsi |
|---|---|---|---|
| 1. **Rekening tujuan tagihan** (niat) | Form Order Jual ("Rekening tujuan", terisi default); detail order (baris + **Ubah**, `PUT /sales-orders/{id}/receipt-account`); Order dari SJ | `sales_orders.receipt_account_id` | Dicetak "Transfer ke" di nota; nilai awal tingkat 2 |
| 2. **Rekening pembayaran** (fakta) | Catat pembayaran (detail order, menu Pembayaran, koreksi), pelunasan SJ, Order dari SJ lunas, retur tambah bayar, penerimaan barang, POS | `payments.cash_account_id` | Jurnal ke akun rekening; laporan Kas & Rekening; wajib untuk transfer/cek |

Urutan nilai awal tingkat 2: pilihan kasir → tingkat 1 → default (`payment.resolveCashAccount`).

Sejak rilis `4b9b46c`, rekening di nota **tidak tercetak secara bawaan** ("Tampilkan rekening"). Fungsi utama tingkat 1
(memberi tahu pelanggan ke mana harus transfer) jadi jarang dipakai.

## 2. Temuan

1. **Pembukuan hanya bergantung pada tingkat 2.** Jurnal, saldo per rekening, dan rekonsiliasi rekening koran memakai
   `payments.cash_account_id`. Tingkat 1 tidak pernah dijurnal.
2. **Rekening yang benar baru diketahui saat uang masuk.** Pelanggan bisa mentransfer ke rekening mana pun yang ia tahu.
   Tingkat 1 adalah *niat* yang bisa berbeda dari *fakta*. Akibatnya ada dua isian yang bisa saling bertentangan
   ("tujuan BCA, dibayar ke Mandiri") dan membingungkan saat ditinjau.
3. **Beban di awal tanpa manfaat pembukuan.** Pembuat order melihat dan bisa mengubah rekening walau belum ada uang.
   Ditambah satu tombol Ubah, satu endpoint, dan satu jejak audit.
4. **POS sudah benar.** Di POS, checkout adalah pembayaran, jadi pilihan rekeningnya sudah di tingkat 2 (hanya labelnya
   "Rekening tujuan").
5. **Semua jalur bayar sudah punya pilihan rekening** (temuan 2 ADR-0026). Menghapus tingkat 1 tidak meninggalkan jalur
   tanpa rekening, dan nilai awal tetap terisi dari default.

## 3. Riset (diakses 2026-10-27)

| Sumber | Temuan |
|---|---|
| [Odoo — Payments](https://www.odoo.com/documentation/saas-17.4/applications/finance/accounting/payments.html) | Saat **Register Payment**, pengguna memilih *Journal* (bank/kas), metode, dan tanggal. Jurnal bank ditentukan saat pembayaran, bukan saat faktur dibuat. "Recipient Bank" di faktur bersifat opsional, terutama untuk QR/SEPA di Eropa |
| [Accurate — Penjualan](https://accurate.id/fitur/penjualan/), [Accurate — Virtual Account BCA](https://accurate.id/tips-tutorial/virtual-account-bca/) | Akun kas/bank dipilih di **Penerimaan Penjualan** (saat pembayaran dicatat). Rekening per tagihan baru muncul bila memakai virtual account |
| [Jurnal — Kas & Bank](https://www.jurnal.id/id/fitur/kas-transaksi/kas-bank), [Jurnal — prosedur penerimaan kas](https://www.jurnal.id/id/blog/prosedur-penerimaan-kas/) | Penerimaan dicatat dengan bank tujuan setoran saat uang diterima |
| Analisis 002 §3 (Odoo/ERPNext) | Rekening per tagihan berguna bila tagihan dikirim ke pelanggan B2B dengan instruksi satu rekening tertentu, atau dengan virtual account/QR per tagihan |

Kesimpulan: pola umum di ERP/akuntansi UKM Indonesia adalah **rekening dipilih saat pembayaran dicatat**. Rekening per
tagihan adalah fitur tambahan untuk kasus khusus: instruksi transfer per tagihan, VA/QR, atau rekening per cabang.

## 4. Kapan tingkat 1 tetap berguna (dan kenapa tidak di sini)

| Kasus | Di mini-erp |
|---|---|
| Tagihan B2B dikirim dengan satu rekening tujuan tertentu | Rekening di nota kini bawaan tidak tercetak. Bila dicentang, cukup daftar rekening perusahaan |
| Virtual account / QR per tagihan | Tidak ada (di luar ruang lingkup ADR-0026) |
| Rekening berbeda per cabang/pelanggan | Tidak ada (ADR-0026: tidak termasuk) |

Jadi untuk proses toko saat ini, tingkat 1 menambah isian tanpa manfaat yang terpakai.

## 5. Rekomendasi: satu tingkat, saat catat pembayaran

**Setuju dengan pengguna.** Rekening hanya dipilih saat uang dicatat (tingkat 2). Tingkat 1 dihapus.

1. **Order Jual / Order dari SJ:** hapus isian "Rekening tujuan" di form, baris + **Ubah** di detail, dan
   `ReceiptAccountModal`. Pelunasan langsung di Order dari SJ tetap memilih rekening (tingkat 2).
2. **Semua jalur bayar** (tidak berubah, kecuali nilai awal): "Ke/Dari rekening" terisi **rekening default**, bisa diganti,
   wajib untuk transfer/cek. POS: label "Rekening tujuan" diganti **"Ke rekening"** agar sama dengan jalur lain.
3. **Cetak nota:** bila "Tampilkan rekening" dicentang, cetak **daftar rekening perusahaan yang ditandai tampil di
   dokumen** (bukan satu rekening tagihan). Bukti pembayaran tetap mencetak rekening pembayaran itu ("Ke rekening"),
   karena itu fakta. Struk POS tidak berubah (rekening pembayaran bila transfer, selain itu daftar).
4. **Backend:**
   - `sales`: berhenti menulis `receipt_account_id` (tidak lagi terisi default saat order dibuat). Hapus
     `PUT /sales-orders/{id}/receipt-account` dan `SetReceiptAccount`.
   - `payment`: urutan nilai awal menjadi pilihan → default (fallback tagihan dihapus).
   - **Kolom** `sales_orders.receipt_account_id`: dihapus dengan migrasi baru `000049_sales_drop_receipt_account`
     (`.down.sql` mengembalikannya). Kolom tanpa pemakai hanya membingungkan pembaca skema; isinya hanya ada di data uji
     v2, yang akan dimuat ulang saat cutover. `sqlc` dan `DB_SCHEMA.md` diperbarui.
   - Label audit `sales_orders.receipt_account_change` dibiarkan untuk jejak lama.
5. **Uji:** integrasi Go (pembayaran tanpa rekening tagihan → default; route terhapus 404/405), vitest form/detail order,
   cetak nota dengan daftar rekening, dan e2e E26 (alurnya diganti: order tanpa rekening → catat pembayaran transfer terisi
   default → ganti ke Mandiri → jurnal ke akun Mandiri).
6. **Dokumen:** ADR-0026 direvisi (butir 2/3 tingkat tagihan dihapus, butir 6 cetak), `API_CONTRACT.md` §6 (route
   dihapus), `DB_SCHEMA.md` §8 (baris migrasi 49), RUNBOOK §6f langkah 5, STATUS, glosarium "rekening tujuan".

**Yang hilang dan diterima:** order tidak lagi "mengingat" rekening yang diharapkan sebelum dibayar. Bila suatu hari
dibutuhkan (mis. VA per tagihan), itu fitur terpisah dengan model sendiri.

## 6. Alur pengguna sesudahnya

- **Buat Order Jual:** tanpa isian rekening.
- **Catat pembayaran → Transfer:** "Ke rekening" terisi rekening default, ganti bila uang masuk ke rekening lain → Simpan.
  Jurnal ke akun rekening itu.
- **POS → Bayar → Transfer:** "Ke rekening" terisi default.
- **Cetak nota:** tanpa rekening. Centang "Tampilkan rekening" untuk mencetak daftar rekening perusahaan.
