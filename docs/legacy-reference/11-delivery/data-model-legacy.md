# Data Model Legacy — Modul 11 Delivery / Pengiriman

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Presisi mengikuti
entity + migrasi 013/014/037/046.

---

## 1. Tabel milik modul (3, lahir 014 + 037 + 046)

### `delivery_notes` — surat jalan

Satu baris = satu SJ (order atau pengganti). Kolom: `id_delivery_note` · `id_order`
(**null untuk pengganti**, sejak 046) · `document_kind` (order/replacement, default order) ·
`id_sales_return` (pengganti saja) · `id_branch` · `sj_number` (100) · `driver_name` (100,
NOT NULL tanpa validasi isi) · `vehicle_plate` (20, null) · `warehouse_staff_name` (100) ·
`delivery_date` (**DATE**, tanpa jam) · `notes` · `dispatched_at`/`dispatched_by` ·
`confirmed_at`/`confirmed_by` (null) · TTD (status 30, nama 150, alasan-teks — sejak 037) ·
`drop_location_note` · `archived_at` · `created_at` (tanpa `updated_at`!). Relasi: order
(nullable), cabang, dispatcher, confirmer, items (cascade).

### `delivery_note_items` — baris per produk

Satu baris = satu produk × SJ. Kolom: id (SJ, order-item **null untuk pengganti**,
sales-return-item pengganti saja, varian wajib) · `quantity` + base · (tanpa snapshot
nama/UOM — dibaca dari baris order!; pengganti dibaca dari baris retur). Beda watak dengan
`order_items`/`goods_receipt_items` yang snapshotpenuh.

### `stock_issue_allocations` — alokasi gudang per baris

Satu baris = satu lokasi × baris-SJ: SJ + baris-SJ + (produk, varian, lokasi) + qty + base.
Jejak "diambil dari mana" (kebalikan penerimaan yang tanpa-alokasi). Dibaca di list
(dengan nama lokasi) + dipakai batal (pengembalian tepat-sasaran).

## 2. Relasi (konsep)

```
orders(sales) 1──* delivery_notes(order) ──* items ──* allocations ──> balances
sales_returns 1──* delivery_notes(replacement) (modul 12; id_order NULL)
delivery_notes 1──* media_files (2 purpose; bukti)
```

Cabang = batas + guard; perusahaan implisit (tanpa `id_company` di tabel SJ! — batas via
order/cabang; [PERLU KONFIRMASI] disengaja? — tercantum karena anomali).

## 3. Jejak baca-tulis

| Fitur | Tulis |
|---|---|
| Terbit | SJ + baris + alokasi + saldo− + movement-out (+ nomor) + audit |
| Konfirmasi | SJ (waktu/petugas/TTD) + (tutup: order + history + bayar/tempo + audit) + audit-confirm |
| Batal | saldo+ + movement-in + arsip + audit |
| Bukti | media + audit |

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **Tanpa `id_company`** di SJ (satu-satunya dokumen tanpa kolom perusahaan langsung).
2. **Tanpa `updated_at`** (satu-satunya tabel dokumen tanpa itu).
3. **Baris tanpa snapshot sendiri** (nama/UOM dibaca dari snapshot baris-order, bukan master
   produk — stabil terhadap rename produk, tetapi mengikuti edit order yang menulis ulang baris;
   rantai dua tingkat, bukan salinan langsung).
4. **Presisi drift** (014 `(18,4)` vs entity `(24,12)` — sekelas KI-90).
5. **Pengganti tak terlihat** di list/cetak-order (NULL tak cocok filter).
6. **Sopir/gudang NOT NULL tanpa validasi** (string kosong lolos API).
