# Business Rules — Modul 16 Stock / Inventory & Gudang

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula,
kondisi khusus. Dari 5 service + controller + FE + spec + E2E. Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

---

## 1. Invarian & angka (BR-01…BR-06)

| ID | Aturan |
|---|---|
| BR-01 | Saldo per (cabang, produk, varian, lokasi); `available = on_hand − reserved` di SEMUA pintu tulis (`stock.service.ts:426,449,508,1191`; `stock-damaged.service.ts:138,161,216,237,289`; `stock-transfer.service.ts:212,290,395,432`; `stock-opening.service.ts:335`); gerak selalu berpasangan movement (`balanceBefore/After`, `referenceType` + `idReference`-null-boleh, `reasonText`, `metadata`, `idMovedBy`, `movedAt`); unik-DB `(cabang, varian, lokasi)` |
| BR-02 | Basis-UOM simpan; dual tampil (faktor-normalisasi: null/''/≤0 → fallback-1!; label ganda-bila-beda `X transaksi (Y basis)`; step 1-bulat vs 0.0001-pecahan!); dua-kasus key (camel + snake!) di `attachUomAvailabilityFields` (`stock.service.ts:925-971`); konversi `toBase = qty × faktor`, `fromBase = qty ÷ faktor` (snap-toleransi!) |
| BR-03 | Leaf = tanpa-anak-aktif (`countActiveChildren`, arsip-diabaikan!); semua tulis-stok ke daun-aktif (`assertLeafLocation`: 404 bila hilang/arsip + 400 bila non-aktif/bukan-daun!); agregat cabang/produk menjumlah (`SUM`); lokasi tak-antar-cabang (lokasi selalu cabang-sesi, kecuali `locations/list` + tujuan-dokumen!) |
| BR-04 | Kritis = tersedia ≤ minimum (**non-null!**, `min_stock_qty IS NOT NULL`); daftar + badge + saran-badge; rusak sembunyi-default (`include_damaged` falsy → `sl.status = 'active'`!); `locationCount = SUM(on_hand > 0)` |
| BR-05 | Presisi: qty-4-desimal (`roundQty`: `Math.round((v + EPSILON) × 10000) / 10000`, `stock.service.ts:1259-1261`!) vs buka-`roundQuantity`-12-desimal + `roundMoney`-2-desimal (`stock-opening.service.ts:98-104`!); toleransi-alokasi 0.0001; limit jepit (saldo 1–100-default-20!; rusak 1–100-default-20!; dokumen/daftar/mutasi maks-100 **tanpa-bawah** — cek!); token-maks-8, AND-token/OR-kolom, `LIKE %token%` |
| BR-06 | Audit 13 kunci (`stock.adjust`, `stock.location.create/update/archive`, `stock.damaged.move_in/restore/write_off`, `stock.transfer`, `stock.transfer_document.create/dispatch/receive/cancel`, `stock.opening_import.commit`); semua-tulis = transaksi + movement + audit (kecuali baca/saran/scan/daftar!) |

## 2. Sesuaikan (BR-07…BR-10)

| ID | Aturan |
|---|---|
| BR-07 | Qty > 0 dulu (`Quantity harus lebih dari 0` — cek!: `undefined <= 0` = false sehingga tanpa-qty lolos-guard-ini!) + alasan-trim (`Alasan penyesuaian wajib diisi`) + produk-ada (404!) + tracked (400!) + varian-resolve + lokasi-wajib (`Lokasi stok wajib dipilih`, 400!) + daun-aktif; `out` cukup-on-hand (`Stok tidak mencukupi` — banding vs `onHand`, bukan `available`!); `adjustment` = **timpa-bebas** (termasuk-nol-FE, tapi BE-tolak-nol karena guard-qty-dulu!); saldo-buat-0-bila-hilang |
| BR-08 | Modal via `finance_inventory_cost_states.average_cost` + `products.purchase_price`: `in` = avg > 0 ATAU beli > 0 (lolos!); else = avg > 0 saja (2 pesan + cara-isi!: `Isi Harga Beli produk, atau posting pembelian/saldo awal...` vs `Posting pembelian atau saldo awal...`!); movement `manual_adjustment` + metadata `{requestedByUserId, approvedByUserId, approvalMode, approvedAt}` |
| BR-09 | Otorisasi: pengenal-3-nama (`identifier ?? username ?? email`, trim!) + sandi wajib (400 `Otorisasi penanggung jawab wajib diisi`); user aktif-perusahaan (username ATAU email!) + cocok-bcrypt else 401 `Credential penanggung jawab tidak valid` (**dua-sebab satu-pesan!**); izin `stock.adjust.approve` (403 `User penanggung jawab tidak memiliki izin approval stock adjustment` — peran global-`null` atau secabang dihitung!); `strict` + diri → 403 `Mode strict mewajibkan approver berbeda dari requester`; mode default-`simple` (**salah-ketik = simple!**); **tanpa-threshold-nominal** (tiap-adjust-butuh-manusia!); self-approval hanya-`simple`! |
| BR-10 | FE: tipe-3 + async-tracked + varian-kondisional (auto-bila-tunggal!) + daun-pertama + min-dinamis (koreksi-0 vs BE-400!) + dialog-pemilik-420px + tombol-ganda-gate + toast + returnTo (`/stock` vs `/gudang` di breadcrumb!) |

## 3. Lokasi (BR-11…BR-14)

| ID | Aturan |
|---|---|
| BR-11 | Kode trim-UPPER-wajib (400 `Kode lokasi wajib diisi`) + nama-trim-wajib (400!); unik-cabang-non-arsip (409 `Kode lokasi '...' sudah digunakan`; **arsip-boleh-pakai-ulang!**); induk ada-cabang-non-arsip (404 `Lokasi induk tidak ditemukan`!); buat-selalu `isDefault: false` (cek! — tanpa-cara-set-default!) + `sort_order ?? 0` + `active` |
| BR-12 | Induk-berisi (`SUM(onHand)` — reserved-diabaikan!) + sub-baru tanpa-`force_transfer`: tolak-400 `PARENT_HAS_STOCK: ...` (FE → dialog!); + `force_transfer` = transaksi pindah-semua-saldo-induk-ke-anak (lewati-nol!; induk `onHand = 0`, `available = max(0, available − qty)` — reserved-nyangkut!; movement-`out`-`transfer` + `in`-berpasangan + alasan-otomatis + metadata `{reason: 'sub_location_created'}`!) + audit-flag-`stockTransferred: true` |
| BR-13 | Ubah: id-wajib (400 `ID lokasi wajib diisi`!) + ada-cabang-non-arsip (404!) + kode-unik-kecuali-diri (409; kosong-tolak!) + nama/urut/petik/primer; **tanpa-pindah-induk via update** (`id_parent_location` diabaikan — reorder-FE lintas-level diam-tak-pindah! → temuan); audit before/after 5-field |
| BR-14 | Arsip: ada (404!) + anak-aktif (`Hapus sub-lokasi terlebih dahulu`) + stok-`SUM(onHand)` (`Lokasi masih punya stok. Kosongkan stok terlebih dahulu` — reserved-diabaikan!); soft-`archivedAt` (audit tanpa-before/after!); lintas-cabang-baca (`locations/list` + `id_branch`) untuk tujuan-transfer (aktif-saja, 404 `Cabang tujuan tidak ditemukan`!) |

## 4. Pindah & dokumen (BR-15…BR-20)

| ID | Aturan |
|---|---|
| BR-15 | Seketika: qty ≤ 0 → 400 (cek! — NaN lolos `<= 0`!) + sama → 400 `Lokasi asal dan tujuan tidak boleh sama` + produk-tracked + varian + daun-dua-secabang + asal-kosong → 400 `Produk tidak memiliki stok di lokasi asal` + kurang → 400 `Stok tidak mencukupi: tersedia X, diminta Y` (banding-`available`!); **dua-gerak** `transfer` (out + in-`paired_movement_id`!) + audit `stock.transfer`; tanpa-dokumen/nomor! |
| BR-16 | Dokumen-buat: ≥1-baris (400 `Minimal satu barang wajib dipilih`!) + tujuan-aktif-perusahaan (404!) + per-baris qty > 0 + produk-tracked + varian + asal-daun-cabang-ini + tujuan-daun-cabang-itu (per-baris-override ?? header!) + secabang-sama → 400; **draf-tanpa-gerak** + nomor-kunci-tulis + `idOut/In: null` + audit-create |
| BR-17 | Kirim: dari-cabang-sesi + `draft` (else 400 `...hanya bisa dikirim dari status dibuat`!) + punya-barang (400!) + tersedia-cukup per-baris (400 `Stok '...' tidak cukup di lokasi asal`!); kurang-saldo (`available −= qty`, `onHand −= qty`) + movement-`out`-`stock_transfer`-`in_transit` (alasan `Transfer {nomor} ke {tujuan}` + metadata `{idStockTransfer, idStockTransferItem, idToBranch, idToLocation, status}`!) + `idOutMovement` + header-`in_transit` + audit-dispatch |
| BR-18 | Terima: ke-cabang-sesi + `in_transit` (else 400 `Hanya transfer sedang dikirim yang bisa diterima`!) + per-baris wajib-`idOutMovement` (else 400 `...belum memiliki mutasi keluar yang valid`!); masuk (buat-saldo-0!) + movement-`in` (alasan `Terima transfer {nomor} dari {asal}` + `{pairedOutMovementId}`!) + `idInMovement` + header-`received` + audit-receive; **tanpa-pindah-cabang-otomatis** (user tetap ganti-cabang-manual!); E2E: terima-cabang-salah = 404! |
| BR-19 | Batal: dari-cabang + `draft` (else 400 `Hanya surat transfer yang belum dikirim yang bisa dibatalkan`!); **tanpa-gerak** (stok-tak-tersentuh, `in_transit` tak-bisa-batal!); → `cancelled` + audit-cancel; FE: tombol `Batal` hanya draf-asal + toast `Surat transfer dibatalkan` |
| BR-20 | FE-dokumen: daftar-30-`all` + arah-badge (`Dalam Cabang`/`Keluar`/`Masuk`) + status-badge-4 + aksi-cabang-status (`Cetak` selalu + `Kirim`/`Terima`/`Batal` kondisional!) + cetak-fisik (tanda-3!) + form-gate (sopir-wajib! + stok-cek + tujuan-cabang-lain-dimuat!) + notice-2 + toast-6!; FE-kirim-satu-baris (API multi-baris!); `transferStock` reset-form + reload + daftar-ulang |

## 5. Rusak (BR-21…BR-24)

| ID | Aturan |
|---|---|
| BR-21 | Daftar: rusak-non-arsip + qty > 0 + nama-ASC (+ varian-urut!); limit-jepit-1–100-default-20!; baris (unit-beli + rugi-`roundQty`-4-desimal!) + varian-hidden-jadi-null! + `total_estimated_loss` = `roundQty(SUM(onHand × beli))` **global-lintas-halaman**! |
| BR-22 | Masuk: qty > 0 (400!) + alasan-trim (400 `Alasan barang rusak wajib diisi`!) + produk-tracked + varian + asal-daun-**opsional** (**tanpa = yatim-dari-nol**, by-design: `Tidak dari stok aktif`!; ada = kurang → 400 `Stok normal tidak cukup...` banding-`available`!); RUSAK-otomatis (`RUSAK`/`Barang Rusak`/9999/`damaged`; beranak → 400!; non-`damaged` → auto-perbaiki-status!); gerak `damaged_stock` + `{toDamagedLocation/fromLocation, source: reference_type ?? 'manual'}` + `idReference: reference_id ?? null`; audit-move_in; respons-rugi |
| BR-23 | Pulih: qty + alasan (`Alasan wajib diisi`!) + produk + varian + tujuan-daun-wajib + rusak-lokasi (diberi → assert-`damaged` + daun!; tanpa → getOrCreate!) + rusak-ada-cukup (400 `Stok rusak tidak cukup`; tersedia-`max(0, ...)`!); gerak-berpasangan `damaged_restore` (`{toLocation}` / `{fromDamagedLocation}`); audit-restore; FE-bawaan (alasan `Masih layak jual` + qty-penuh!) |
| BR-24 | Buang: qty + alasan + produk + varian + rusak-cukup; **satu**-gerak-`out`-`damaged_write_off` + metadata-`estimatedLossAmount`; audit-write_off; **final** (tanpa-jalur-kembali!); FE-tetap (`Tidak layak dijual` + tanpa-dialog!) + toast-rugi |

## 6. Buka (BR-25…BR-29)

| ID | Aturan |
|---|---|
| BR-25 | Konteks: cabang-aktif (404 `Cabang tidak ditemukan`!) + produk-aktif-tracked + varian-terlihat/default + daun-aktif + saldo + saran (satu-lokasi-berisi → `existing_stock` / satu-daun → `single_location` / else-null!) + ringkasan (`products_total` = baris-varian! + `master_products_total` + lokasi + dengan/tanpa + `status: no_locations/ready/needs_location`!) |
| BR-26 | File: sheet-alias-4 + header-alias-7-kolom + angka-fleksibel (ID-vs-US-separator!) + baris-2 + kosong-lewat; wajib-3-kolom (kode-barang/lokasi/qty!) + non-negatif; kode-normalisasi (lower-tanpa-`?`-rapat-UPPER!); master (ada/arsip/track/aktif!) / varian (kode-varian-wajib + contoh! / tanpa-varian-aktif / tanpa-default!) / lokasi (ada/aktif/daun!) / nama/satuan-peringatan (warning!) / duplikat-triplet (error!) / terisi-lewati-peringatan (`already_filled`, qty-saldo-berjalan!); **tanpa-error + tanpa-lewat + ada + produk + lokasi + varian = baris-valid** (+ `average_cost` + `inventory_value`!) |
| BR-27 | Commit: tolak-error (`File stok awal masih memiliki error...`) / nol-baris-qty>0 (`Tidak ada baris...`) / posted-finance (dua-lapis: baca + kunci-tulis! `Saldo awal finance sudah diposting...`) / terisi-per-baris (lock-kedua! `...sudah terisi. Gunakan Penyesuaian Stok...`); saldo = qty (**timpa, bukan-tambah!**) + `available = qty − reserved` (roundQty!); movement-`in`-`opening_stock` (`idReference` = id-buka-keuangan! + `{source: 'stock_opening_import', row, productCode, locationCode, averageCost}`); finance-agregat-per-produk (qty-sum + nilai-sum + avg = nilai/qty!) + gabung-aditif-ke-draf (`finance_opening_items_synced`!); audit-kaya (rows/totalQty/totalValue/openingId/financeRows/movementIds!) |
| BR-28 | Template: hanya-belum-terisi per-lokasi-pilihan + sheet-Panduan (diabaikan-parser!) + gaya-wajib (font/border/header-biru/merah-putih!); FE: tetapkan-dulu-gate (massal `Terapkan ke yang kosong`/`Terapkan ke semua`/per-baris/buat-cepat-`GUD-UTAMA`!) + unduh-gate-2-pesan + file-`.xlsx/.xls` + `[Cek File]`-12-isu + `[Simpan Stok Awal]`-gate (`!file || !ok || valid ≤ 0 || imported || busy`!) + `Buka Saldo Awal Keuangan` (→`/finance/opening`!) + toast-ganda + notice-4! |
| BR-29 | Harga = `roundMoney(purchasePrice / purchaseToBaseFactor)` (base!) + nilai = `roundMoney(qty × avg)`; **tanpa-kolom-Excel** (kolom-lega-diabaikan-diam, insiden-2026-08: 241-produk 10–1000×, ±Rp3,2M!); tanpa-harga-master = tolak-baris (`...belum punya Harga Beli di Data Produk...`!); file-lama diabaikan (spec: `ignores a "Harga Modal Rata-rata" column`!) |

## 7. Scan/saran/reservasi (BR-30…BR-33)

| ID | Aturan |
|---|---|
| BR-30 | Scan-kode-pasti-exact (tanpa-fuzzy!): kosong → `empty_code` (tanpa-query!) → varian (`variant_code` ATAU `barcode`, arsip-abaikan!) → produk-pasti (`product_code`) → `not_found` → nonaktif → `inactive` → non-tracked → `ok`-tanpa-saldo (`balance: null`, `available_qty: null`!) → varian-wajib (`variant_required` + daftar-terlihat!) / default / hilang → `not_found` → agregat-cabang-lokasi-aktif + dual + label + step (**12 field**: `status, code, product(+variants), product_variant, balance(+breakdown+locationCount), available_qty(+_base/_sales), base_uom, sales_uom, sales_to_base_factor, display_stock_label, quantity_step_sales`!) |
| BR-31 | Saran: produk-wajib + qty > 0 + varian + faktor (input ?? sisi-order-`purchase ? beli : jual` ?? 1!) + lokasi-pilihan-opsional; kandidat daun-aktif-tersedia (onHand > 0 DAN available > 0!) + urut-petik-primer-default-terbanyak-sort-id (toleransi-0.0001!); serakah + cukup-flag + kurang-angka + total + per-lokasi-prioritas (`location_path: [nama]`-tunggal!); **tanpa-FIFO-tanggal** (cek!) |
| BR-32 | Reservasi: kunci-trim-wajib + ≥1-baris + TTL-jepit-60–1800-default-600! + kedaluwarsa-dulu (`expired` + saldo-turun + `releasedAt` — cek!: expired-pakai-`releasedAt`!) + lepas-kunci-**user-dulu** (`released`!) + produk-ada (404!) + **non-tracked-lewat-diam** (semua-non-tracked = 200-kosong!) + varian + qty-base > 0 + kandidat-urut + kurang-rollback-penuh (`Stok '...' tidak cukup untuk di-reserve`!) + baris-`pos_cart`-`idUser`-sesi-`idOrder-null` + saldo-naik; lepas = kunci-**user-sama** + turun + `released_count`; `consumed` tanpa-pemanggil (cek!) |
| BR-33 | FE-kasir: tombol-scan + `QRScannerModal` (mengisi-search!) + adaptor-`toStockBalance/Movement/Location`; guard-hantu + saldo-60dk? — tanpa-ghost-reserve di modul ini (hold/TTL milik alur kasir modul 15, kontrak E-04/E-05!) |

## 8. Lintas modul (BR-34…BR-35)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-34 | Saldo/mutasi/lokasi/saran/scan/reservasi menjadi terima/serah/retur/kasir/lapor (baca + tulis-berpasangan!) | Order (08–13), POS (15), Finance (17) |
| BR-35 | Modal (`finance_inventory_cost_states.average_cost` — guard, bukan tulis!) + buka-draf (`finance_opening_balance/items` — gabung-aditif!) menjadi HPP/posting/saldo-awal | Finance (17) |

## 9. Aturan yang TIDAK ada (verifikasi)

Tidak ada: edit/hapus-movement; hapus-permanen (arsip-saja!); stok-negatif-via-`out` (diblokir — kecuali `available`-negatif hasil `set`-dengan-reservasi, cek temuan!); terima-tanpa-kirim; kembalikan-
buang; edit-reservasi (hold-ulang = lepas + buat-baru!); pindah-induk-update; lokasi-antar-cabang
(kecuali baca-tujuan-dokumen!); saran-tanpa-produk; buka-tanpa-sheet/posted-gagal-diam; file >5MB
(Multer-5MB!); scan-fuzzy; saran-lebih-tanpa-flag (`enough/shortage` selalu-ada!); reservasi-
tanpa-izin-stok (`order.create`, bukan `stock.*`!); buka-tanpa-harga-Excel (kolom-dihapus!);
template-tanpa-tetapkan (gate!); threshold-nominal-approval (tiap-adjust-approve!);
flag-`allow_sales_allocation` (tak-ada-di-entity!); stok-negatif-`available`-guard (tak-ada!).

## 10. Temuan baru & pertanyaan terbuka (belum ber-ID)

- Guard qty `adjust`/`transfer` (`data.quantity <= 0`) lolos untuk qty-hilang/`NaN` (`undefined <= 0` = false) sehingga tanpa-qty bisa lanjut ke aritmetika-`NaN` — cek! (`apps/api/src/modules/stock/stock.service.ts:1156`, `apps/api/src/modules/stock/stock-transfer.service.ts:368`).
- FE izinkan qty-0 untuk Koreksi-Fisik (`min="0"`, `stock-adjustment-page.tsx:189`) tapi BE tolak qty ≤ 0 — koreksi-ke-nol via UI pasti-400 — cek!.
- `movement_type` apa pun selain `in`/`out` (mis. `set` mentah) jatuh ke cabang-`else` = perilaku-timpa-koreksi — cek! (`apps/api/src/modules/stock/stock.service.ts:1181-1188`).
- `stock/detail` cari produk tanpa filter perusahaan (`findOne({id, archived}`) sehingga baca-lintas-perusahaan mungkin — cek! (`apps/api/src/modules/stock/stock.service.ts:1106`); filternya juga hanya `status active` tanpa `archivedAt` tidak seperti daftar (`apps/api/src/modules/stock/stock.service.ts:1115-1117`).
- `stock/transfers/list` + `stock/movements`: `page` tanpa-batas-bawah dan `limit` tanpa-batas-bawah (limit-0 = take-0, page-negatif = skip-negatif) — cek! (`apps/api/src/modules/stock/stock-transfer.service.ts:67-68`, `apps/api/src/modules/stock/stock.service.ts:1264-1265`).
- `locations/update` abaikan `id_parent_location` sehingga pindah-induk dan susun-ulang lintas-level diam-tak-pindah (FE `reorderLocations` kirim `parentLocationId`!) — cek! (`apps/api/src/modules/stock/stock-location.service.ts:226-261`, `apps/web/src/store/slices/stock.slice.ts:184-189`).
- `locations/create` selalu `isDefault: false` — tak ada jalur API menjadikan lokasi-default — cek! (`apps/api/src/modules/stock/stock-location.service.ts:200-211`).
- Cek stok lokasi (`getStockQtyAtLocation`) jumlahkan `onHand` saja sehingga arsip/pindah-induk lolos saat `reserved > 0` — cek! (`apps/api/src/modules/stock/stock.service.ts:276-280`); `force_transfer` juga biarkan `reserved` nyangkut di induk yang dikosongkan (`available = max(0, available − qty)`) — cek! (`apps/api/src/modules/stock/stock-location.service.ts:123-126`).
- `adjust` mode-`adjustment` hitung `available = after − reserved` tanpa-guard sehingga `available` bisa negatif saat ada reservasi — cek! (`apps/api/src/modules/stock/stock.service.ts:1190-1192`).
- Status reservasi `consumed` + kolom `id_order`/`consumed_at` tak punya pemanggil di modul ini (konsumsi milik order/kasir di luar berkas) — cek! (`apps/api/src/modules/stock/stock.service.ts:401-437`); reservasi `expired` catat `releasedAt` (bukan kolom khusus) — cek! (`apps/api/src/modules/stock/stock.service.ts:439-456`).
- `hold` lewati produk-non-tracked diam-diam sehingga semua-non-tracked = 200 tanpa-baris — cek! (`apps/api/src/modules/stock/stock.service.ts:476`).
- Saran-alokasi `location_path` selalu `[nama]` satu-elemen (bukan path-penuh) — cek! (`apps/api/src/modules/stock/stock.service.ts:369`).
- Agregat `detail` pakai identitas baris-pertama (id/lokasi = saldo-`id ASC`-pertama) — cek! (`apps/api/src/modules/stock/stock.service.ts:1120-1131`).
- FE dokumen-transfer selalu satu-baris (`transferStock` kirim `items: [satu]`) walau API dukung multi-baris — cek! (`apps/web/src/store/slices/stock.slice.ts:133-138`).
- Entity `stock_locations` tanpa `@Unique` DB — duplikat-kode-arsip hanya dijaga guard-409 aplikasi; pakai-ulang kode-arsip aman-di-aplikasi — cek migrasi! (`apps/api/src/modules/stock/entities/stock-location.entity.ts:1-55`).
