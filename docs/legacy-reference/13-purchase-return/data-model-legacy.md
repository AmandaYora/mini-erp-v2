# Data Model Legacy — Modul 13 Purchase Return (Retur Pembelian)

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Presisi mengikuti
entity + migrasi 050 (terakhir di repo!).

---

## 1. Tabel milik modul (3, lahir 050)

### `purchase_returns` — header

Satu baris = satu retur (selalu `completed` lahir). Kolom: id (perusahaan, cabang) · nomor
(100, unik per cabang) · order-asal + pihak (null) · `status` · tanggal + alasan · 3 angka
(subtotal, pajak, total) · `settlement_type` (4 nilai: none/reduce/refund/kredit — tanpa
`collect`!) · `cancels_order` (flag penuh) · pembuat + timestamps + arsip (tanpa endpoint).

### `purchase_return_items` — baris

Satu baris = satu produk × lokasi-keluar. Kolom: id-retur + order-item + produk + varian +
nomor + snapshot (nama/kode/varian/UOM) + qty + base + harga-bersih + pajak 3 + total 2 +
tracked + lokasi + movement + catatan. Tanpa diskon-terpisah/kondisi (beda sales).

### `purchase_return_settlements` — baris kas/saldo

Hanya refund/kredit + jumlah > 0. Kolom: id-retur + tipe + tanggal (= tanggal-retur) +
jumlah + metode (refund saja) + referensi + catatan + pembuat + arsip (disaring di detail!).

### Sampingan migrasi 050

Izin keempat `purchase_return.cancel` (seed + matriks, tanpa penegak — KI baru) · sequence
`purchase_return` (`RTB-...`, yearly-dekoratif) · akun 5300 `Selisih Retur Pembelian`
(expense/debit, mapping `purchase_return_difference`: nota-asli vs avg-cost!).

## 2. Relasi (konsep)

```
orders(purchase) 1──* purchase_returns ──* items ──> balances/movements(out)
purchase_returns 1──* settlements (refund/kredit; tanpa payments!)
business_parties(supplier) 1──* returns; branches (sequence); users (pembuat)
```

Cabang + perusahaan = batas + guard cabang≈perusahaan.

## 3. Jejak baca-tulis

Buat (transaksi, lock-order-dulu): rencana → nomor → header + baris (+ stok + settlement) →
penuh? (batal + history) → audit. Baca: list (order + pihak), konteks (terima + retur +
lokasi), preview, detail (+ settlement-non-arsip).

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **Izin `cancel` tanpa penegak** (seed + matriks + tipe, nol endpoint — KI baru; kembaran
   `sales_return.cancel`!).
2. **Akun 5300 tanpa baca di modul** (ditulis migrasi, dipakai finance — kontrak antar-modul).
3. **Tanpa baris `payments`** (kas tanpa bayar — KI-98 meluas).
4. **Tanpa drift presisi** (terverifikasi: entity `(18,4)`/`(18,2)`/`(9,4)` = migrasi) —
   pengecualian dari pola KI-90; catat sebagai acuan yang benar.
5. **Arsip disaring tanpa penulis** (pola penerimaan).
6. **Status `cancelled` bertipe tanpa penulis** (pola sales).
