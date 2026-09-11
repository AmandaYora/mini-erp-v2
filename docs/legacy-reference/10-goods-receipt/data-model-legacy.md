# Data Model Legacy — Modul 10 Goods Receipt

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Presisi mengikuti
entity + migrasi 018 (+ 036 untuk kolom varian).

---

## 1. Tabel milik modul (2, lahir 018)

### `goods_receipts` — header batch penerimaan

Satu baris = satu batch terima (satu mobil/satu catat). Kolom: `id_goods_receipt` ·
`id_company` · `id_branch` · `id_order` · `id_stock_location` (null bila campur) ·
`supplier_delivery_number` (100, null) · `received_at` (wajib; boleh masa lalu) · `notes` ·
`id_received_by` (user aplikasi, null) · timestamps + `archived_at` (**tanpa endpoint** —
KI-91). Index: (company, branch, order), (lokasi), (arsip). FK: company, branch, order,
lokasi, penerima.

### `goods_receipt_items` — baris per produk-lokasi

Satu baris = satu produk × lokasi dalam satu batch. Kolom: id (penerimaan, order, baris-order,
produk, varian — varian dari 036, tak ada di 018) · `id_stock_location` (null) · `quantity` +
`quantity_in_base_uom` · snapshot UOM ganda · referensi sumber (`purchase_order_item` + id
baris) · `id_inventory_movement` (null untuk non-stok). Tanpa arsip/timestamps-mati? —
`created/updated_at` ada; tanpa `archived_at` (beda dengan header!). Index: (penerimaan),
(order, baris), (lokasi), (movement).

## 2. Relasi (konsep)

```
orders(purchase) 1──* goods_receipts ──* goods_receipt_items ──1:1── inventory_movements(in)
goods_receipt_items *──1 order_items (sisa = order − Σ kumulatif)
stock_locations 1──* items; users 1──* receipts (pencatat)
```

Tanpa FK ke payments (bayar COD terpisah, terhubung via order + audit); tanpa nomor; tanpa
status (final vs parsial turunan order).

## 3. Jejak baca-tulis

Terima (transaksi): baca order+item+penerimaan-lama+status+bayar+lokasi → tulis penerimaan +
item + saldo + movement (+ bayar/termin) + order + history + audit. List/detail: baca
ber-relasi. Alias: delegasi.

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **Presisi drift**: 018 `DECIMAL(18,4)` vs entity `(24,12)` (sekelas faktor-UOM) — verifikasi
   live (→ KI-90).
2. **Arsip setengah**: header punya kolom tanpa endpoint; item tanpa kolom — hapus kolom atau
   tambah endpoint (→ KI-91).
3. **Varian susulan** (036) — kolom tak ada di migrasi lahir; pola evolusi, bukan masalah.
4. **Tanpa lock sisa** (E-18) — satu-satunya agregat-kumulatif tanpa `pessimistic_write`.
5. **Tanpa nomor** (KI-89) — identitas operasional lemah untuk rujukan lisan.

