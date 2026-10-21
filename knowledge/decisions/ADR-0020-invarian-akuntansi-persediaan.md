# ADR-0020: Invarian akuntansi persediaan — ledger biaya selalu bisa dijelaskan buku

## Status
Accepted (2026-10-21). Melengkapi ADR-0005 (uang rupiah bulat), K-05 (rekonsiliasi Persediaan
buku vs ledger biaya), F2b-11 (rantai harga modal), dan OP-04 (`costrebuild`).

## Context
Gladi migrasi 2026-10-21 menunjukkan selisih Persediaan buku vs ledger biaya puluhan miliar, yang
setelah ditelusuri berasal dari data legacy (ditangani ETL, ADR-0018 D11). Audit runtime v2 pada
hari yang sama menemukan celah yang akan menghasilkan selisih **baru** setelah cutover:

1. Pembatalan (`<ref>_cancel`) dihargai pada rata-rata saat pembatalan, bukan biaya dokumen
   aslinya, sehingga pasangan asli + batal menyisakan nilai di ledger tanpa jurnal.
2. Pindah ke/dari lokasi rusak dan transfer satu cabang dihargai ulang di kaki masuk. Bila kaki
   keluar mengosongkan posisi, kaki masuk jatuh ke harga masuk terakhir (berbeda nilai). Kaki masuk
   transfer antar-cabang memakai rata-rata cabang tujuan, sehingga nilai perusahaan berubah tanpa
   transaksi.
3. Barang "datang rusak" tanpa lokasi asal (ST-06) menambah nilai ledger tanpa jurnal.
4. Ledger menyimpan sen (decimal 18,2), sedangkan jurnal membulatkan per baris, sehingga selisih
   pembulatan menumpuk. Posisi yang qty-nya habis bisa menyisakan nilai.
5. Jurnal manual bisa langsung ke akun Persediaan, dan biaya operasional bisa dicatat ke HPP.
6. Rekonsiliasi turun jadi peringatan begitu ada **satu** dokumen belum dijurnal, berapa pun
   nilainya. Selisih yang tidak berhubungan ikut tersamarkan.
7. `costrebuild` membukukan semua koreksi baris keluar ke HPP, termasuk baris yang aslinya dijurnal
   ke akun lain (selisih stok, hapus stok, retur beli), dan ikut mengoreksi pasangan netral.

## Decision
**Invarian:** setiap mutasi stok **dijurnal persis sebesar nilai yang dicatat ledger**, atau
**netral nilai** (dua kaki bernilai sama). Tidak ada kategori ketiga.

- **Rupiah bulat.** Nilai baris ledger dibulatkan ke rupiah saat costing. Keluar yang mengosongkan
  posisi mengambil **tepat sisa nilainya**, sehingga qty 0 berarti nilai 0.
- **Kaki cermin dihargai dari kaki yang dicerminkan**, bukan dari rata-rata:
  - pembatalan → baris dokumen asli (cabang, ref, produk, varian), pro-rata kumulatif, sehingga
    pasangan bernilai nol tepat setelah pembulatan;
  - kaki masuk transfer → kaki keluarnya, lintas cabang. Cabang pengirim disinkronkan lebih dulu,
    sebelum kunci costing cabang penerima diambil, sehingga dua cabang yang saling kirim tidak
    bisa deadlock;
  - kaki masuk pindah lokasi rusak → kaki keluar tepat sebelumnya.
  Bila kaki yang dicerminkan tidak ada, rantai biaya biasa dipakai dan baris ditandai estimasi.
- **Barang datang rusak** memakai ref sendiri, `damaged_arrival`, dan dijurnal seperti selisih stok
  lebih (Dr Persediaan / Cr Selisih Persediaan). Hapus bukunya kelak mengurangi nilai yang memang
  ada di buku.
- **Akun kendali.** Jurnal manual ke akun yang dipetakan `inventory` ditolak; Persediaan hanya
  bergerak bersama stok. Biaya operasional ke akun yang dipetakan `cogs` ditolak; HPP hanya berasal
  dari ledger biaya. Jurnal manual ke HPP tetap boleh untuk koreksi akuntan.
- **Rekonsiliasi menjelaskan, bukan memaafkan.** `InventoryRecon` menghitung nilai ledger dokumen
  yang belum dijurnal (`pendingValue`) dan yang sengaja diabaikan (`ignoredValue`).
  `unexplained = diff + pendingValue + ignoredValue` harus dalam toleransi. Bila tidak,
  `inventory_recon` **gagal**, ada dokumen tertunda maupun tidak. Integritas juga ikut gagal.
- **`costrebuild` hanya mengoreksi yang dijurnal dari ledger**, ke akun yang sama dengan jurnal
  aslinya:

  | Mutasi | Akun koreksi |
  |---|---|
  | `delivery` | `cogs` |
  | `adjustment` | `adjustment` |
  | `write_off` | `writeoff` |
  | `purchase_return` | `purchase_return_difference` |

  Pasangan netral, kaki cermin, dan dokumen yang dibatalkan tidak disentuh.

## Consequences
- Selisih buku vs ledger setelah cutover hanya bisa muncul dari campur tangan di luar aplikasi
  (ubah DB langsung) atau dari dokumen yang sengaja diabaikan. Keduanya terlihat dan bernama.
- Data lama di ledger (impor ETL, baris 2 desimal) tidak ditulis ulang. Aturan baru berlaku untuk
  baris yang di-costing sesudahnya, dan `costrebuild` memakai aturan pembulatan yang sama.
- Kontrak `StockClient` mendapat `TransferSourceBranch`.
- Bukti: `apps/api/internal/integration/inventory_invariants_test.go`.
