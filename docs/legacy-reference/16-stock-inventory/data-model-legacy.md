# Data Model Legacy — Modul 16 Stock / Inventory & Gudang

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Presisi mengikuti
entity + migrasi (001/017/019–021/034–039/044-RUSAK? + finance-buka!).

---

## 1. Tabel milik modul (6)

### `inventory_balances` — saldo per (cabang, produk, varian, lokasi)

Satu baris = satu sel. Kolom (`inventory-balance.entity.ts:9-52`): `id_inventory_balance` (PK)
+ `id_branch` + `id_product` + `id_product_variant` + `id_stock_location`
+ `on_hand_qty`/`reserved_qty`/`available_qty` (`decimal(24,12)`, default-0!)
+ `updated_at` (`UpdateDateColumn`, **tanpa `created_at`!**). Unik-DB:
`@Unique(['idBranch', 'idProductVariant', 'idStockLocation'])` (varian-bukan-produk, sejak-036;
produk via varian!). Dibuat-0 + dijumlah agregat (MIN-id/varian/lokasi + SUM + MAX-waktu!).
Relasi-ManyToOne (tanpa-FK-fisik-lintas-modul!): cabang/produk/varian/lokasi.

### `inventory_movements` — jejak berpasangan

Satu baris = satu gerak (**tak pernah edit/hapus!** — tanpa endpoint). Kolom
(`inventory-movement.entity.ts:10-54`): `id_inventory_movement` (PK) + cabang + produk +
varian + lokasi + `movement_type` (`varchar(30)`: `in`/`out`/`adjustment` — satu-satunya nilai
yang ditulis!) + `quantity` (`decimal(24,12)`) + `balance_before/after` (`decimal(24,12)`)
+ `reference_type` (`varchar(30)`, null-boleh: `transfer`, `stock_transfer`, `damaged_stock`,
`damaged_restore`, `damaged_write_off`, `manual_adjustment`, `opening_stock`, + milik-modul-lain!)
+ `id_reference` (`int`, null-boleh!) + `reason_text` (`text`, null-boleh — bebas per sumber!)
+ `id_moved_by` (null-boleh!) + `moved_at` (`datetime(6)`) + `metadata_json` (`json`, null-boleh!).
**Tanpa `created_at`/`updated_at`** — waktu = `moved_at` semata! Relasi: cabang/produk/varian/
lokasi/user-pemindah (nullable!).

### `stock_locations` — pohon per cabang

Satu baris = satu node (grup/daun!). Kolom (`stock-location.entity.ts:6-43`): `id_stock_location`
(PK) + `id_branch` + `id_parent_location` (null = akar!) + `code` (`varchar(50)`, app-UPPER!) +
`name` (`varchar(255)`) + `is_default`/`is_picking_area`/`is_primary` (default-false!) +
`sort_order` (default-0!) + `status` (`varchar(30)`, default-`active`; nilai-terlihat:
`active`/`damaged`!) + `archived_at` (null-boleh!) + `created_at`/`updated_at`.
**Tanpa-`@Unique`-DB** — unik-cabang-non-arsip hanya di aplikasi (409!; arsip-boleh-pakai-ulang!);
DB? [PERLU KONFIRMASI] cek-migrasi! Relasi: cabang + induk (self!) + anak (OneToMany!).
RUSAK-otomatis (`RUSAK`/`Barang Rusak`/9999/`damaged`, tanpa-anak-aktif!).

### `stock_reservations` — kunci POS TTL

Satu baris = satu lokasi × produk (021!). Kolom (`stock-reservation.entity.ts:10-57`):
`id_stock_reservation` (PK) + `id_branch` + `reservation_key` (`varchar(100)`) + `source_type`
(`varchar(30)`, default-`pos_cart` — satu-satunya nilai yang ditulis!) + `id_user`/`id_order`
(null-boleh!) + produk + varian + lokasi + `quantity_in_base_uom` (`decimal(24,12)`)
+ `status` (`varchar(20)`, default-`active`: `active|released|consumed|expired` — `consumed`
tanpa-pemanggil-modul-ini!) + `expires_at` + `released_at`/`consumed_at` (null-boleh!
— `expired` pakai `releasedAt`!) + `created_at`/`updated_at`. Index-aplikasi (klaim-kode):
(kunci, status, habis) + (cabang, produk, lokasi, status) — DB? [PERLU KONFIRMASI] cek-migrasi!
Relasi: cabang/produk/varian/lokasi/user/order (nullable!).

### `stock_transfers` + `stock_transfer_items` — dokumen (034!)

Header (`stock-transfer.entity.ts:9-68`): `id_stock_transfer` (PK) + `id_company` + dari-ke-cabang
+ `transfer_number` (`varchar(80)`) + `status` (`varchar(30)`, default-`draft`:
`draft|in_transit|received|cancelled`!) + `transfer_date` (`datetime(6)`) +
`dispatched_at`/`received_at`/`cancelled_at` (null-boleh!) + `driver_name` (`varchar(150)`!)
+ `vehicle_number` (`varchar(80)`) + `notes` (`text`!) + pembuat-4-peran
(`id_created_by/dispatched_by/received_by/cancelled_by`, null-boleh!) + `archived_at` (null-boleh!
— daftar filter `archived_at IS NULL`!) + `created_at`/`updated_at` (ada-`UpdateDateColumn`!).
Baris (`stock-transfer-item.entity.ts:8-39`): `id_stock_transfer_item` (PK) + transfer + produk +
varian + dari-ke-lokasi + `quantity` (`decimal(24,12)`) + `id_out_movement`/`id_in_movement`
(null-boleh! — draf-null, kirim-isi-out, terima-isi-in!) + `notes` (`text`!). Relasi baris:
transfer/produk/varian/dari-lokasi/ke-lokasi/out-movement/in-movement (nullable!).
Nomor via `branch_document_sequences` (`stock_transfer`, kunci-tulis, buat-otomatis!).

## 2. Relasi (konsep)

```
branches 1──* locations (pohon!) 1──* balances *──1 products(+varian!)
balances 1──* movements (ref-polimorfik: order/SJ/retur/transfer/sesuai/rusak/buka!)
locations 1──* reservations; transfers *──* cabang (dari-ke!) + baris-lokasi
finance states/opening ↔ saldo/modal (baca + sinkron!)
company_settings → mode-approval; users/roles → otorisasi
```

Cabang = batas (semua!); lokasi tak-antar-cabang (kecuali baca-tujuan-dokumen!); perusahaan via
cabang/produk; semua relasi = kolom-ID tanpa-FK-fisik-lintas-modul (aturan monolit!).

## 3. Jejak baca-tulis

Lihat modul 08–15 (terima/serah/retur/kasir!) + sini (sesuai/rusak/pindah/dokumen/buka/
scan/saran/reservasi/lokasi). Semua-tulis = transaksi + movement + audit (kecuali baca/saran/
scan/daftar!). Tulis-tanpa-movement: **tidak-ada** (bahkan buat-saldo-0 selalu + movement!).
Tulis-tanpa-audit: saran/scan/daftar + `stock.transfer_document`? — semua-dokumen beraudit!
Kunci-tulis (`pessimistic_write`): nomor-transfer + buka-finance + buka-saldo-per-baris!

## 4. Anomali (untuk arsitek, bukan untuk ditiru)

1. **Saldo tanpa `created_at`** (satu-satunya tanpa! — umur-sel dibuat-tanpa-jejak).
2. **Tanpa-unik-DB lokasi** (entity tanpa-`@Unique`; cek aplikasi hanya non-arsip-409!) —
   pakai-ulang kode-arsip aman-di-aplikasi (=500-DB mustahil-kecuali-race!; sekelas KI-50/KI-71! → KI baru).
3. **Movement tanpa-edit/hapus + alasan-bebas + ref-polimorfik** (jejak-abadi, teks-tak-terstruktur,
   tipe + id tanpa-FK — join-manual per sumber!).
4. **RUSAK-otomatis + status-`damaged`** (lokasi-istimewa-bukan-tipe! + auto-perbaiki-status +
   tolak-beranak!).
5. **Agregat-MIN-id** (identitas-agregat semu! — id = MIN, varian = MIN, lokasi = MIN,
   `locationCount` = berisi-saja!).
6. **Skala-desimal ganda** (`roundQty`-4 vs buka-12 + uang-2!) + toleransi-0.0001 +
   `available`-negatif-mungkin (set-dengan-reservasi!).
7. **Reservasi tanpa-kedaluwarsa-otomatis-penjadwal** (kedaluwarsa hanya saat hold-berikutnya!) +
   `consumed`/`id_order` tanpa-pemanggil + `expired`-pakai-`releasedAt`!
