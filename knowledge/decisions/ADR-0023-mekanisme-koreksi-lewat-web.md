# ADR-0023: Mekanisme koreksi salah input lewat web

## Status
Accepted (2026-10-21). Melengkapi ADR-0020 (invarian persediaan) dan ADR-0021 (kunci biaya lintas
proses, batas cutover `costrebuild`).

## Context
Setelah go-live, staf keuangan harus bisa memperbaiki kesalahan nilai persediaan tanpa akses shell
server. Dua alat yang ada tidak cukup:

1. Restatement HPP hanya tersedia sebagai perintah konsol `costrebuild`.
2. Harga modal non-nol yang salah (saldo awal salah ketik, barang masuk dengan harga keliru) tidak
   punya jalan perbaikan. "Lengkapi harga modal" hanya mengisi posisi bernilai nol, dan restatement
   tidak pernah mengubah barang masuk karena nilainya adalah bukti.

Selain itu, restatement memutar baris biaya-saja (id mutasi sintetis ≥ 2^62) di akhir rantai posisi.
Barang keluar yang dicatat sesudah baris itu akan dihitung ulang memakai rata-rata lama, sehingga
koreksinya terhapus diam-diam.

## Decision
- **Hitung ulang HPP di web.** `POST /finance/cost-restatement/preview` (dry run, `finance.manage`)
  dan `…/commit` (`finance.close.force`, alasan wajib). Mesin, gerbang, kunci, dan batas cutover
  sama dengan `costrebuild`. Alasan dicatat di metadata audit `finance.cost_restatement`. Perintah
  konsol tetap ada dan menerima `--reason`.
- **Revaluasi harga modal.** Per posisi berstok: `delta = round(qty × hargaBaru) − round(nilai)`.
  Delta ditulis sebagai satu baris ledger nilai saja (`ref_type=inventory_revaluation`, qty 0, id
  sintetis, `ref_id` = catatan revaluasi). Delta dibukukan dengan SATU jurnal (`inventory_revaluation`):
  Persediaan (dimensi produk/varian) lawan Selisih Stok (pemetaan `adjustment`), dibalik bila turun.
  Semuanya ditulis dalam satu transaksi bersama catatan `finance_inventory_revaluations(+_lines)`
  dan audit `finance.inventory_revaluation`, di bawah kunci biaya cabang setelah ledger disinkron.
  Buku dan ledger bergerak sama besar, jadi `unexplained` rekonsiliasi tidak berubah.
  Revaluasi ditolak bila stok ≤ 0, harga negatif, tidak ada perubahan, tanggal di masa depan atau
  di periode tertutup, atau saldo awal keuangan belum diposting. Sebelum saldo awal diposting,
  jurnal cutover sendiri yang menilai ledger, sehingga delta akan terbukukan dua kali.
  Pratinjau memakai `finance.manage`, komit memakai `finance.close.force`. `clientRef` opsional
  membuat replay idempoten.
- **Baris biaya-saja diputar di titik ia ditulis.** Restatement menempatkan baris sintetis tepat
  sesudah baris mutasi nyata terakhir yang sudah ada di ledger saat baris itu ditulis (id baris
  ledger lebih kecil), bukan di akhir rantai. Aturan ini juga berlaku bagi baris `opening_cost`.
- **Maju saja.** HPP barang yang sudah keluar sebelum revaluasi tidak diubah.
- **Revaluasi tanpa harga satuan.** Baris revaluasi tidak membawa `unit_cost`, sehingga "harga masuk
  terakhir" tetap berasal dari dokumen masuk nyata.

## Consequences
- Koreksi nilai persediaan sehari-hari tidak lagi memerlukan shell server. Komit tetap terbatas pada
  pemegang `finance.close.force` (bawaan: owner).
- Membalik jurnal `inventory_revaluation`, `cost_restatement`, atau `opening_supplement` lewat "Balik
  Jurnal" akan menggeser buku tanpa ledger. Rekonsiliasi menandainya sebagai selisih tak terjelaskan.
  Perbaikan yang benar adalah revaluasi ulang atau pemulihan backup (RUNBOOK §6b/§6c).
- Bukti: `apps/api/internal/integration/finance_corrections_test.go`,
  `apps/api/internal/modules/finance/application/costrestate_test.go`.
