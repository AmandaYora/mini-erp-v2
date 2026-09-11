# Business Rules — Modul 13 Purchase Return (Retur Pembelian)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula,
kondisi khusus. Cermin sales yang disederhanakan (komentar kode). Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

---

## 1. Kelayakan & baris (BR-01…BR-08)

| ID | Aturan |
|---|---|
| BR-01 | Alasan non-kosong + ≥1 baris (`Alasan retur wajib diisi` / `Minimal satu barang yang dikembalikan harus dipilih`) |
| BR-02 | PO: ada-cabang-non-arsip (404) + purchase (`Retur ini hanya berlaku untuk order pembelian`) + cabang≈perusahaan (404!) + sudah-terima (`...harus sudah menerima barang...`) + punya-item; urut `lineNo` |
| BR-03 | Baris: unik + milik-order + qty > 0 + ≤ sisa-terima + 0.0001 (`Qty retur '{n}' melebihi sisa yang bisa diretur. Sisa: {s} {u}`); sisa = terima − retur-completed |
| BR-04 | Tanggal ≥ terima (`...tidak boleh lebih awal dari tanggal penerimaan barang`) + ≤ kini+60dk (`...tidak boleh di masa depan`); kosong → kini; label `Tanggal retur` |
| BR-05 | Lokasi tracked: input ?? terima-terakhir ?? wajib-pilih (`Lokasi asal barang untuk retur '{n}' tidak ditemukan, pilih lokasi secara manual`); daun-aktif + tersedia-cukup (`Stok '{n}' tidak cukup di lokasi {nama}` — available, bukan on-hand!); non-tracked tanpa |
| BR-06 | Nilai = rasio(qty/order) × harga-bersih + pajak proporsional (rupiah); snapshot penuh; `lineNo` ulang; catatan trim-or-null |
| BR-07 | Stok: saldo −= base (`available` juga) + movement `out`/`purchase_return` alasan-`Retur ke supplier {nomor}` + tanggal-retur + metadata TANPA `idOrderItem` (lindungi margin sales!) |
| BR-08 | Lock `pessimistic_write` baris order sebelum rencana (klik-ganda aman; komentar: tidak mewarisi celah sales) |

## 2. Settlement 3-arah (BR-09…BR-13)

| ID | Aturan |
|---|---|
| BR-09 | Selalu kurangi-hutang-dulu: potong = min(retur, sisa-terutang-efektif); eksternal = retur − potong |
| BR-10 | Eksternal ≤ 0.009 → `reduce_payable` (paksa-lain → `400 ...belum ada kelebihan...`); else eksplisit-kredit ? `supplier_credit` : `refund` (default refund!) |
| BR-11 | Refund tanpa izin → 403; metode tunai-default hanya refund; referensi/catatan trim-or-null |
| BR-12 | Baris settlement hanya non-reduce + jumlah > 0 (tanggal = tanggal-retur); `reduce_payable` implisit (tanpa baris!) |
| BR-13 | **Tanpa baris `payments`** untuk refund (kas di luar sistem; sama dengan sales — cakupan KI-98 diperluas ke modul ini) |

## 3. Batal-otomatis (BR-14…BR-16)

| ID | Aturan |
|---|---|
| BR-14 | Penuh = semua item terima-terpenuhi-retur (terima-0 = lewati!) ± 0.0001, dicek SETELAH simpan (termasuk dokumen ini) |
| BR-15 | Penuh → `cancels_order` + order ke cancelled (kind-persis lalu `all`; tanpa-cocok → biarkan, bukan gagal) + history `Retur pembelian menghabiskan seluruh barang yang diterima` |
| BR-16 | Audit `purchase_return.create` + `cancelsOrder` boolean; respons = preview + id + nomor + settlement + flag |

## 4. Daftar & DTO (BR-17…BR-19)

| ID | Aturan |
|---|---|
| BR-17 | List: cabang + perusahaan + non-arsip; status-bebas; tanggal-mentah; cari 3 kolom; urut tanggal + id DESC; limit jepit 1–50 (seperti sales!), page ≥ 1 |
| BR-18 | Detail + konteks + preview: pola sales (konteks + terima + sisa + lokasi-terakhir + daun) |
| BR-19 | Nomor `RTB-...` (lock, fallback darurat); unik per cabang |

## 5. Lintas modul (BR-20…BR-21)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-20 | Kurang-stok + tanggal + tanpa-`idOrderItem` menjadi saldo/HPP/pos; potong-hutang + refund/kredit menjadi utang-efektif | Stock (16), Finance (17), Order-badge (08) |
| BR-21 | Penuh → cancelled via `findCancelledStatusForOrder` (dipakai bersama modul 08) | Order (08) |

## 6. Aturan yang TIDAK ada (verifikasi)

Tidak ada: tukar/pengganti/SJ/kondisi/diskon-terpisah/cicilan/`collect_payment`/baris-payments/
edit/batal/hapus/cetak-khusus/batas-tanggal-atas (selain +60dk)/tolak-nol-sisa-di-konteks
(tampil redup!).
