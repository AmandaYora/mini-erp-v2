# Data Model Legacy — Modul 12 Sales Return (Retur Penjualan)

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Presisi mengikuti
entity + migrasi 038/046.

---

## 1. Tabel milik modul (3, lahir 038)

### `sales_returns` — header dokumen

Satu baris = satu retur/tukar (selalu `completed` lahir; `cancelled` bertipe tanpa penulis).
Kolom: id (perusahaan, cabang) · `return_number` (100, unik per cabang) · order-asal + pihak
(null walk-in) · `source` (pos/sales_delivery) · `status` · `return_mode` (return_only/exchange)
· tanggal + alasan-teks · 7 angka (retur 3 + pengganti 3 + selisih) · `settlement_type` ·
`replacement_delivery_status` (4 nilai, sejak 046) · pembuat + timestamps · `cancelled_*`
(mati — E-22) + `archived_at` (disaring di list/detail/plan, tetapi tanpa endpoint penulis —
tulis-tanpa-jalan seperti penerimaan).

### `sales_return_items` — baris dua-tipe

Satu baris = retur (`returned`, + order-item + kondisi + lokasi + movement) atau pengganti
(`replacement`, + produk/varian + lokasi + movement). Kolom: id-retur + tipe + order-item
(null pengganti) + produk + varian + nomor-baris + snapshot (nama/kode/varian/UOM) + qty +
base + harga (penuh, bersih, diskon, persen) + pajak (kena, tarif, dasar, nilai) + total 3 +
tracked + kondisi (null pengganti) + lokasi + movement + catatan. Presisi: qty `(18,4)`,
uang `(18,2)`, persen `(9,4)` (seperti order).

### `sales_return_settlements` — baris kas/saldo

Hanya tipe-riil + jumlah > 0 (collect/refund/kredit; tanpa none/reduce). Kolom: id-retur +
tipe + tanggal (= tanggal-retur) + jumlah + metode (collect/refund saja) + referensi + catatan
+ pembuat + arsip (disaring di detail!).

## 2. Relasi (konsep)

```
orders(sales) 1──* sales_returns ──* items (2 tipe) ──> balances/movements
sales_returns 1──* settlements (parsial-tipe) ──(tanpa payments!)
sales_returns 1──0/1 delivery_notes(replacement) (id_order NULL)
business_parties 1──* returns (null-boleh); users (pembuat)
```

Cabang + perusahaan = batas (+ guard cabang≈perusahaan saat buat).

## 3. Jejak baca-tulis

Buat (transaksi): rencana → nomor → header + baris (+ stok + settlement) + audit. Dispatch:
SJ + stok + baris + status + audit. Confirm: SJ + status + audit. Baca: list (order + pihak),
konteks (sisa + lokasi), preview (rencana), detail (+ SJ + lokasi-bila-pending).

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **`cancelled_*` mati** (tipe + filter tanpa penulis — E-22).
2. **Arsip disaring tanpa penulis** (list menyaring arsip yang tak bisa dibuat — di atas).
3. **Tanpa baris `payments`** (kas tanpa bayar — E-16 feature).
4. **`return_date` vs `created_at`** (tanggal-dokumen bebas vs catat; backdate seperti terima).
5. **Pajak-pengganti tanpa input** (mengikuti order — konsisten, tetapi implisit).
6. **Presisi drift terverifikasi**: migrasi `(18,4)` vs entity `(24,12)` untuk qty/faktor
   (uang tetap `(18,2)` di keduanya) — sekelas KI-90, verifikasi live.
