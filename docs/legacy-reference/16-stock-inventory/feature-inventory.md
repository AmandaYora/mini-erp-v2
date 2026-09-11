# Feature Inventory — Modul 16 Stock / Inventory & Gudang

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode (controller **25** endpoint, 5 service, 6 entity, 10 halaman web, spec, E2E 04/08/12).
Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | **25** — `stock/balances`, `stock/scan-product`, `stock/allocations/suggest`, `stock/reservations/{hold,release}`, `stock/detail`, `stock/adjust`, `stock/opening/{context,preview,commit}`, `stock/damaged/{list,move-in,restore,write-off}`, `stock/locations/{list,create,update,archive}`, `stock/transfer`, `stock/transfers/{list,create,dispatch,receive,cancel}`, `stock/movements` |
| Halaman | 10 rute: `/stock`, `/stock/movements`, `/stock/opening`, `/stock/adjustments/create`, `/stock/:itemId`, `/gudang`, `/gudang/locations`, `/gudang/transfer`, `/gudang/pindah-lokasi`, `/gudang/damaged` |
| Menu sidebar | Grup **"Stok & Gudang"**: Stok, Gudang, Transfer Cabang, Pindah Lokasi, Barang Rusak, Stok Awal |
| Permission | `stock.view` (8 endpoint: balances/scan/suggest/detail/damaged-list/locations-list/transfers-list/movements), `stock.adjust` (7: adjust/opening-×3/damaged-×3), `stock.update` (3: locations create/update/archive), `stock.transfer` (5: transfer + transfers create/dispatch/receive/cancel), `order.create` (2: reservations hold/release — milik kasir, bukan stok!), `stock.adjust.approve` (otorisasi penyesuaian, **bukan** guard endpoint — dicek di `verifyStockAdjustmentApproval`) |
| Tabel yang dimiliki | `inventory_balances`, `inventory_movements`, `stock_locations`, `stock_reservations`, `stock_transfers`, `stock_transfer_items` |
| Tabel yang dibaca/ditulis lintas | `finance_inventory_cost_states` (guard modal!), `finance_opening_balance/items` (sinkron saldo-awal!), `company_settings` (mode approval!), `users/roles` (otorisasi!) |
| Penomoran | Transfer-dokumen `TRF-{cabang}/{tahun}/{5 digit}`; tanpa nomor lain. Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | 13 `actionKey` (`stock.adjust`, `stock.location.create/update/archive`, `stock.damaged.move_in/restore/write_off`, `stock.transfer` + `stock.transfer_document.create/dispatch/receive/cancel`, `stock.opening_import.commit`) |
| Aksi yang TIDAK ada | Edit/hapus movement; hapus permanen lokasi/saldo; edit reservasi; batal transfer-terkirim; kembalikan write-off; terima-tanpa-kirim; lokasi-antar-cabang (lokasi milik cabang!); saran-tanpa-produk; stok-negatif via `out` (diblokir — kecuali `available` hasil `set`, lihat BR-07!) |

**Catatan lingkup.** Invarian inti: **setiap `InventoryBalance` berpasangan dengan
`InventoryMovement`, resolve ke leaf `StockLocation`** (aturan arsitektur). Saldo disimpan
selalu dalam base-UOM; tampil dual-UOM. Tiga angka: `on_hand` (fisik) − `reserved` (dipesan)
= `available` (siap). Modul ini pemilik angka; modul lain hanya memanggil (terima/serah/
retur/transfer/adjust/buka/rusak/reservasi).

**Konvensi transport (semua 25).** `@Post` + `@HttpCode(200)` + body `{data}` (kecuali
`opening/preview|commit` = multipart field `file` + tetap `200`); guard
`JwtAuthGuard, PermissionGuard, BranchGuard` di kelas controller
(`stock.controller.ts:16-18`); scope cabang/perusahaan/user selalu dari sesi
(`s.idActiveBranch!`, `s.idCompany`, `s.idUser`) — tidak pernah dari body.

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Saldo (`stock/balances` E-01, `/stock`, `/stock/:itemId`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Dua mode daftar | `id_stock_location` → per-lokasi (QB, nama-ASC); tanpa → agregat-per-produk SQL (jumlah + lokasi-hitung + rincian-opsional) |
| F-01.2 | Filter | Cari tokenized (kode/nama, maks-8-token, AND-token/OR-kolom!) · `critical_only` (tersedia ≤ minimum, non-null!) · `include_damaged` (default sembunyikan-rusak!) · lokasi/produk/varian/daftar-produk · paginasi (halaman ≥1, limit jepit 1–100, default-20!) |
| F-01.3 | Cakupan | Cabang-sesi + produk-perusahaan-non-arsip + tracked-saja + lokasi-non-arsip (+ aktif kecuali rusak-diikutkan) |
| F-01.4 | Dual-UOM | Tiap saldo + `Base/Sales/UOM/faktor/label/step` dua-kasus (camel + snake!); label `5 meter (0.5 roll)` (transaksi + basis!); step 1-bulat vs 0.0001-pecahan |
| F-01.5 | Daftar `/stock` | Cari (`stock-search`, `Kode atau nama produk...`, debounce 400ms!) + scan + `Semua Stok`/`Stok Kritis Saja` + 20/halaman; kolom Produk (nama + kode!) · Tersedia-besar (+ `Kritis` bila ≤ minimum!) · Fisik · Dipesan · Status (`Kritis`-merah/`Aman`-hijau!) · Aksi-Detail; kosong `Tidak ada stok`; header `Stok Barang` + `Pantau ketersediaan barang secara menyeluruh.` + `[Riwayat Mutasi]` + `[Penyesuaian Stok]` (izin!) |
| F-01.6 | Detail `/stock/:itemId` | Header nama + kode + Kembali-`navigate(-1)` + Penyesuaian (`?item_id=&returnTo=/stock`); 4 metrik 2×2 (`Tersedia (Siap Jual)`/`Fisik Aktual`/`Dipesan`/`Minimum Stok`, 2xl!); Posisi-Gudang (pohon relevan: leluhur + daun-berstok-tebal + badge + varian-chip; kosong `Tidak ada stok fisik di gudang manapun.`); Riwayat-10-terakhir server-side (`stock/movements` + `id_product` + `limit: 20`, slice-10!) + `Lihat Semua` (prefill kode!); tanpa-produk + selesai-muat = halaman kosong (null!) |
| F-01.7 | Kontrak E-01 | `POST stock/balances` + `stock.view`; body `{search?, critical_only?, include_location_breakdown?, include_damaged?, id_stock_location?, id_product?, id_product_variant?, id_products?, page?, limit?}`; respons `{items, meta:{page,limit,total}}`; agregat: `MIN(id)`, `SUM(on/res/avail)`, `MAX(updated)`, `locationCount = SUM(on_hand>0)`; rincian opsional hanya untuk produk di halaman-ini! |

### F-02 — Mutasi (`stock/movements` E-25, `/stock/movements`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Kontrak E-25 | `POST stock/movements` + `stock.view`; body `{id_product?, movement_type?, date_from?, date_to?, search?, page?, limit?}`; `page` default-1 **tanpa-bawah!**, `limit` = min(input ?? 20, 100) **tanpa-bawah!** (cek! — limit-0 = take-0); order `moved_at` DESC; filter exact produk/tipe + `moved_at >= dari` + `<= sampai` + tokenized (kode/nama); respons `{items, meta}` |
| F-02.2 | Halaman | Filter Cari (`search-item`, `Kode atau nama produk...`, debounce-400!) + scan (mengisi-search!) + Tipe (`Semua Tipe`/`Terima Barang (Masuk)`/`Keluarkan Barang (Keluar)`/`Koreksi Fisik`) + Tanggal (dari–sampai, sampai +`T23:59:59`!) + `Hapus Filter` (merah, reset-total!); kartu `{n} mutasi ditemukan.`/`Memuat…`; tabel Produk (nama + kode + lokasi!) · Waktu · Tipe (badge!) · Qty (+/−/polos + UOM!) · Saldo (`sebelum → sesudah` + UOM!) · Catatan; kosong `Belum ada mutasi stok` + deskripsi-4-sumber; paginasi 25 |
| F-02.3 | Nilai `movement_type` | `in`/`out`/`adjustment` (satu-satunya nilai yang ditulis semua alur!); `reference_type` polimorfik terpisah (`transfer`, `stock_transfer`, `damaged_stock`, `damaged_restore`, `damaged_write_off`, `manual_adjustment`, `opening_stock`, + milik modul lain: PO/SO/retur!) |

### F-03 — Scan (`stock/scan-product` E-02)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Kontrak E-02 | `POST stock/scan-product` + `stock.view`; body `{code?}` (trim!); **exact-match saja, tanpa-fuzzy!**: kosong → `empty_code`; varian (`variant_code` ATAU `barcode`, arsip-abaikan!) → produk-pasti; else produk (`product_code`, non-arsip) → nonaktif → `inactive`; non-tracked → `ok` + `balance: null` + `available_qty: null`!; varian-wajib → `variant_required` (+ daftar varian-terlihat!); default-hilang → `not_found`; else agregat-cabang-lokasi-aktif + dual + label + step (12 field respons!) |
| F-03.2 | Konsumen | Tombol scan (`Scan Barcode / QR`) di `/stock` + `/stock/movements` → `QRScannerModal` → mengisi-search (bukan navigasi langsung!) |

### F-04 — Saran alokasi (`stock/allocations/suggest` E-03)

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Kontrak E-03 | `POST stock/allocations/suggest` + `stock.view`; body `{id_product!, id_product_variant?, quantity!, quantity_in_base_uom?, uom_to_base_factor?, order_kind?: sales|purchase, selected_location_ids?}`; produk-wajib (400) + qty > 0 (400) + produk-ada (404!) + varian-resolve; faktor = normalisasi(input ?? (purchase ? beli : jual) ?? 1); minta-base = input ?? `toBaseQuantity`; kandidat = daun-aktif-tersedia (onHand > 0 DAN available > 0!) + filter-pilihan; urut = petik → primer → default → terbanyak (toleransi-0.0001!) → sort → id; serakah per-lokasi (+ `priority{ picking, primary, default }`, `location_path: [nama]`-tunggal!); respons `{enough, shortage_quantity(+base), total_available(+base), items[], requested(+base), base_uom, transaction_uom, uom_to_base_factor}` — **tanpa-FIFO-tanggal!** (cek! — "FIFO" = urutan-prioritas ini, bukan umur-stok) |
| F-04.2 | Batas | Tanpa-produk = 400 (tidak ada saran-tanpa-produk!); lokasi non-daun/non-aktif dilewati diam-diam (bukan error!) |

### F-05 — Reservasi (`stock/reservations/hold` E-04, `/release` E-05)

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Kontrak E-04 | `POST stock/reservations/hold` + **`order.create`** (kasir!); body `{reservation_key!, ttl_seconds?, items: [{id_product!, id_product_variant?, quantity!, quantity_in_base_uom?, uom_to_base_factor?}]}`; kunci-trim-wajib (400) + ≥1-baris (400!) + TTL-jepit-60–1800-default-600!; transaksi: kedaluwarsa-dulu (aktif + habis → `expired` + saldo-turun!) + lepas-kunci-user-dulu (`released`!) → per-baris: produk-ada (404!) + **non-tracked-dilewati-diam!** + varian + qty-base > 0 (400!) + kandidat-urut + kurang → 400 `Stok '...' tidak cukup untuk di-reserve` (rollback-penuh!); baris-`pos_cart` + `idUser` sesi + `idOrder: null` + saldo-naik (`reserved +=`, `available = onHand − reserved`); respons `{reservation_key, expires_at, items: created[]}` |
| F-05.2 | Kontrak E-05 | `POST stock/reservations/release` + `order.create`; body `{reservation_key!}` (trim!); lepas = kunci + **user-sama!** → `released` + saldo-turun + `released_at`; respons `{reservation_key, released_count}` |
| F-05.3 | Status | `active` → `released` (manual / hold-ulang) / `expired` (otomatis saat hold-berikutnya!); `consumed` terdefinisi tapi **tanpa-pemanggil** (cek!) — konsumsi milik modul order di luar berkas ini |

### F-06 — Penyesuaian + approval (`stock/adjust` E-07, `/stock/adjustments/create`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | Kontrak E-07 | `POST stock/adjust` + `stock.adjust`; body `{id_product!, id_product_variant?, id_stock_location!, movement_type: in|out|adjustment, quantity!, reason_text!, approval: {username|email|identifier, password}}`; urutan-guard: qty > 0 (400 — string `set` apa pun = koreksi!) → alasan-trim (400!) → produk-ada (404!) → tracked (400!) → **guard-modal** (`in`: avg > 0 ATAU beli > 0; else: avg > 0; 400 + cara-isi!) → varian → lokasi-wajib (400!) + daun-aktif (404/400!) → **otorisasi-manusia**; transaksi: saldo-buat-0 + `in`: tambah / `out`: kurang (kurang → 400 `Stok tidak mencukupi`!) / `adjustment`: **timpa-bebas termasuk-nol** (cek! — BE tetap minta qty > 0, jadi nol via API = 400!) + `available = after − reserved`; movement `manual_adjustment` + metadata `{requestedByUserId, approvedByUserId, approvalMode, approvedAt}`; audit `stock.adjust` + before/after; respons = movement |
| F-06.2 | Otorisasi | Pengenal-3-nama (`identifier ?? username ?? email`, trim!) + sandi-wajib (400 `Otorisasi penanggung jawab wajib diisi`!); aktif + cocok-bcrypt else 401 `Credential penanggung jawab tidak valid` (**dua-sebab satu-pesan!**); izin `stock.adjust.approve` else 403; `strict` + diri-sendiri → 403 `Mode strict mewajibkan approver berbeda dari requester`; mode dari `company_settings.operationalRules.stockAdjustmentApprovalMode ?? stock_adjustment_approval_mode`, default-`simple` (**salah-ketik = simple!**); **tanpa-threshold-nominal** — setiap adjust wajib manusia-approve!; self-approval diizinkan hanya di `simple`! |
| F-06.3 | Halaman | `?item_id=&location_id=&returnTo=`; 3-tipe-besar (Terima-hijau + Keluar-merah + Koreksi-kuning + deskripsi-contoh!); Item-async-tracked + Varian-kondisional (auto-bila-tunggal!) + Lokasi-default-daun-pertama + Qty-default-1/min-dinamis (koreksi-min-0 vs BE->400, lihat temuan!) + Alasan (`Contoh: koreksi hasil stock opname pagi...`); `[Batal]` (→returnTo!) + `[Lanjut Otorisasi]` (disabled tanpa-item/lokasi/varian-wajib!); modal `Otorisasi Penanggung Jawab` (420px; `Masukkan akun owner, admin, atau superadmin...`; username/email + password + `[Batal]`/`[Otorisasi & Simpan]`/`Memverifikasi...`; sukses → reset + returnTo; gagal → toast, tetap-buka) |

### F-07 — Lokasi (`stock/locations/*` E-15…E-18, `/gudang`, `/gudang/locations`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-07.1 | Kontrak E-15 | `POST stock/locations/list` + `stock.view`; body `{id_branch?}` (cabang-lain untuk tujuan-transfer!); lintas-cabang: harus ada-aktif-perusahaan else 404 `Cabang tujuan tidak ditemukan`; tanpa-paginasi; respons `{items}` pohon-datar-depth-first + `{isLeaf, depth, path[]}` (nama-dari-akar!), urut `sortOrder,id` |
| F-07.2 | Kontrak E-16 | `POST stock/locations/create` + `stock.update`; body `{code?, name?, id_parent_location?, sort_order?, is_picking_area?, is_primary?, force_transfer?}`; kode-trim-UPPER-wajib (400!) + nama-trim-wajib (400!) + unik-cabang-non-arsip (409, **arsip-boleh-pakai-ulang!**); induk ada-cabang-non-arsip (404!); induk-berisi (`SUM(onHand)` — reserved-diabaikan!) + tanpa-`force_transfer` → 400 `PARENT_HAS_STOCK: ...` (+ dialog-FE!); + `force_transfer` → transaksi pindah-semua-saldo-induk-ke-anak + movement-berpasangan (`transfer`!) + audit-flag; buat-normal: `isDefault: false`-selalu! (cek! — tanpa-cara-buat-default via API), `sort_order ?? 0`, `active`; audit `stock.location.create`; respons = lokasi |
| F-07.3 | Kontrak E-17 | `POST stock/locations/update` + `stock.update`; body `{id_stock_location!, code?, name?, sort_order?, is_picking_area?, is_primary?}`; id-wajib (400!) + ada (404!) + kode-unik-kecuali-diri (409; kosong-tolak!); **tanpa-pindah-induk** (`id_parent_location` diabaikan diam-diam — reorder lintas-level tak-berfungsi, lihat temuan!); audit `stock.location.update` + before/after; respons = lokasi |
| F-07.4 | Kontrak E-18 | `POST stock/locations/archive` + `stock.update`; body `{id_stock_location}`; ada (404!) + anak-aktif → 400 `Hapus sub-lokasi terlebih dahulu` + stok (`SUM(onHand)`!) → 400 `Lokasi masih punya stok. Kosongkan stok terlebih dahulu`; soft-`archivedAt` (tanpa-sebelum/sesudah di audit!); respons = lokasi |
| F-07.5 | Hierarki & flag | Pohon per-cabang (tanpa-antar-cabang!); tulis-stok hanya daun (`Lokasi stok harus berupa lokasi akhir (tanpa sub-lokasi)`); flag prioritas-alokasi `isPickingArea → isPrimary → isDefault` + `sortOrder`; status `active` vs `damaged` (RUSAK!); **tanpa-flag-`allow_sales_allocation`** (cek! — kelayakan-alokasi = aktif + daun + tersedia, bukan flag) |
| F-07.6 | Halaman `/gudang` | Drill-down (tumpukan-id!): kartu-lokasi (📦-daun/🗂️-grup + nama + kode + badge anak/`{n} jenis barang`/`Kosong` + ›!); breadcrumb (`Semua Gudang` + rantai + `← Kembali ke ...`!); isi-daun (`Isi: {nama}`; `Memuat…`/`Gudang ini kosong.`/`{n} jenis barang tersimpan di sini.` + `Pindah dari Sini`!); baris-expand (nama + kode + tersedia + `Kritis` + ▲/▼ → 3-metrik + `Pindah Lokasi`/`Penyesuaian Stok` deep-link!); cache-per-daun (`limit: 200`!); kosong `Belum ada lokasi` + `Tambahkan lokasi melalui halaman Kelola Lokasi.`; header + `[Kelola Lokasi]` + `[Pindah Lokasi]` (transfer!) |
| F-07.7 | Halaman `/gudang/locations` | Lihat = `stock.view`, kelola = `stock.update` (tanpa-handle bila baca-saja!); tree (`{n} lokasi aktif pada cabang ini.`; `▾/▸/—`; badge `Lokasi Akhir`/`Grup`; handle `⠿` + `Geser untuk mengubah urutan`!; drag-sesama-level → simpan-otomatis!); baris `[+ Sub-lokasi]` + `[Edit]` + `[Arsipkan]` (tanpa-dialog!); modal tambah/edit (badge Tambah/Edit; induk-readonly-path; Kode* + Nama*; Batal/Simpan/`Menyimpan...`); modal `Pindahkan Stok Otomatis?` (notice + `tidak dapat dibatalkan`; `[Batal]`/`[Ya, Pindahkan & Buat]`/`Memproses...`); kosong-contoh Toko/Gudang A/B; deskripsi `Kelola hierarki lokasi penyimpanan. Barang hanya bisa disimpan di Lokasi Akhir (tanpa sub-lokasi di bawahnya).` |

### F-08 — Pindah seketika (`stock/transfer` E-19, `/gudang/pindah-lokasi`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-08.1 | Kontrak E-19 | `POST stock/transfer` + `stock.transfer`; body `{id_from_location!, id_to_location!, id_product!, id_product_variant?, quantity!, notes?}`; qty ≤ 0 → 400 (cek! — `NaN <= 0` = false, lolos!); sama → 400 `Lokasi asal dan tujuan tidak boleh sama`; produk-tracked + varian + daun-dua-secabang; asal kosong → 400 `Produk tidak memiliki stok di lokasi asal`; kurang → 400 `Stok tidak mencukupi: tersedia X, diminta Y`; **dua-movement** (`out`: `Transfer ke {tujuan}` + `{fromLocation,toLocation}`; `in`: `Transfer dari {asal}` + `{paired_movement_id,from_location,to_location}`); audit `stock.transfer`; respons `{out_movement, in_movement}`; **tanpa-dokumen/nomor!** |
| F-08.2 | Halaman | `?from=&item_id=&variant_id=&returnTo=`; deskripsi `Pindahkan stok antar lokasi dalam cabang aktif tanpa surat jalan.`; asal + tujuan (beda-paksa!) + produk-tracked + varian + Qty (min-1, maks-tersedia!) + Catatan (`Contoh: pindah ke rak picking, relokasi display, atau susun ulang gudang`); notice: `Pilih lokasi akhir` / `Mengecek stok` / `Stok tidak tersedia` / `Qty melebihi stok tersedia` (`Stok tersedia: X`!) / `Stok tersedia` info; `[Batal]` + `[Pindahkan]`/`Memindahkan...` (gate: daun-dua + beda + varian + 0 < qty ≤ tersedia!) |

### F-09 — Dokumen transfer (`stock/transfers/*` E-20…E-24, `/gudang/transfer`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-09.1 | Kontrak E-21 (buat) | `POST stock/transfers/create` + `stock.transfer`; body `{id_to_branch?, id_to_location!, transfer_date?, driver_name?, vehicle_number?, notes?, items: [{id_product!, id_product_variant?, id_from_location!, id_to_location?, quantity!, notes?}]}`; ≥1-baris (400!) + tujuan-aktif-perusahaan (404!) + per-baris: qty > 0 + produk-tracked + varian + asal-daun-cabang-ini + tujuan-daun-cabang-itu (per-baris-override ?? header!) + secabang-sama → 400 `Lokasi asal dan tujuan tidak boleh sama`; **draf-tanpa-gerak** + nomor (kunci-tulis!) + `idOut/In: null`; audit `stock.transfer_document.create`; respons `{...transfer, items}` |
| F-09.2 | Kontrak E-22 (kirim) | `POST stock/transfers/dispatch` + `stock.transfer`; body `{id_stock_transfer}`; harus dari-cabang + `draft` (else 400 `Surat transfer hanya bisa dikirim dari status dibuat`!) + punya-barang (400!) + tersedia-cukup per-baris (400 `Stok '...' tidak cukup di lokasi asal`!); kurang + movement-`out`-`stock_transfer` (`Transfer {nomor} ke {tujuan}`, metadata `{..., status: 'in_transit'}`!) + `idOutMovement`; header → `in_transit` + `dispatchedAt/By`; audit dispatch |
| F-09.3 | Kontrak E-23 (terima) | `POST stock/transfers/receive` + `stock.transfer`; body `{id_stock_transfer}`; harus ke-cabang + `in_transit` (else 400 `Hanya transfer sedang dikirim yang bisa diterima`!); per-baris wajib `idOutMovement` (else 400 `Transfer belum memiliki mutasi keluar yang valid`!); masuk + movement-`in` (`Terima transfer {nomor} dari {asal}`, metadata `{..., pairedOutMovementId}`!) + `idInMovement`; header → `received` + `receivedAt/By`; audit receive; **tanpa-pindah-cabang-otomatis!** |
| F-09.4 | Kontrak E-24 (batal) | `POST stock/transfers/cancel` + `stock.transfer`; body `{id_stock_transfer}`; harus dari-cabang + `draft` (else 400 `Hanya surat transfer yang belum dikirim yang bisa dibatalkan`!); **tanpa-gerak** (stok-tak-tersentuh!); → `cancelled` + `cancelledAt/By`; audit cancel |
| F-09.5 | Kontrak E-20 (daftar) | `POST stock/transfers/list` + `stock.view`; body `{status?, direction?: outgoing|incoming|all, page?, limit?}`; arah vs cabang-sesi (keluar/masuk/semua!); `page` tanpa-bawah + `limit` tanpa-bawah (cek!); order `createdAt` DESC; respons `{items (+cabang/baris/lokasi), meta}` |
| F-09.6 | Mesin status | `draft` → `in_transit` → `received` / `draft` → `cancelled`; final tanpa-kembali; kirim = cabang-asal, terima = cabang-tujuan, batal = cabang-asal-draf-saja! |
| F-09.7 | Halaman | Daftar-30 (`direction: all`!) + kolom Tanggal/Nomor/Arah (`Dalam Cabang`/`Keluar`/`Masuk`)/Status (`Draft`/`Sedang Dikirim`/`Diterima`/`Batal`)/Dari/Tujuan/Barang (`Nama (qty UOM)` koma!)/Aksi (`Cetak` selalu + `Kirim` bila draf-asal + `Terima` bila transit-tujuan + `Batal` bila draf-asal!); form (Dari-Cabang-readonly + Ke-Cabang + Dari-Lokasi + Produk + Varian + Ke-Lokasi-cabang-tujuan + Jumlah + Sopir-wajib + Kendaraan + Catatan!; notice stok/tujuan-kosong!; `[Batal]` + `[Simpan Surat Transfer]`); cetak-surat-fisik (`Surat Transfer Barang`; Nomor/Tanggal/Status; Dari/Tujuan/Sopir/Kendaraan; tabel Barang/Jumlah/Lokasi-Asal/Lokasi-Tujuan; Catatan; tanda-3: Gudang-Asal/Sopir/Gudang-Tujuan + `Nama & Tanda Tangan`!); toast kirim/terima/batal + `Gagal memuat lokasi cabang tujuan` |

### F-10 — Rusak (`stock/damaged/*` E-11…E-14, `/gudang/damaged`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-10.1 | Kontrak E-11 | `POST stock/damaged/list` + `stock.view`; body `{page?, limit?}` (jepit-1–100-default-20!); rusak-non-arsip + qty > 0 + nama-ASC (+ varian-urut!); baris `{id_product, product_code/name, variant(jadi-null bila hidden!), base_uom, id_stock_location, location_name, quantity, estimated_unit_cost(beli!), estimated_loss_amount(roundQty-4-desimal!)}` + `total_estimated_loss` **global-lintas-halaman**!; respons `{items, meta, total_estimated_loss}` |
| F-10.2 | Kontrak E-12 | `POST stock/damaged/move-in` + `stock.adjust`; body `{id_product!, id_product_variant?, quantity!, id_from_location?, reason_text!, reference_type?, reference_id?}`; qty > 0 + alasan (`Alasan barang rusak wajib diisi`!) + produk-tracked + varian + asal-daun-opsional (**tanpa = yatim-dari-nol!**, ada = kurang `Stok normal tidak cukup untuk dipindahkan ke barang rusak`!); RUSAK-otomatis (`RUSAK`/`Barang Rusak`/9999/`damaged`; beranak → 400!; non-damaged → auto-perbaiki!); gerak: asal-`out`-`damaged_stock` (+ `{toDamagedLocation, source}`) + rusak-`in` (+ `{fromLocation, source}`); audit `stock.damaged.move_in`; respons `{movement(in), damaged_location, estimated_loss_amount}` |
| F-10.3 | Kontrak E-13 | `POST stock/damaged/restore` + `stock.adjust`; body `{id_product!, id_product_variant?, id_damaged_location?, id_to_location!, quantity!, reason_text!}`; qty + alasan (`Alasan wajib diisi`!) + produk + varian + tujuan-daun + rusak-ada-cukup (`Stok rusak tidak cukup`!; tersedia-`max(0,...)`!); gerak-berpasangan `damaged_restore` (`{toLocation}` / `{fromDamagedLocation}`); audit restore; respons `{movement(in)}` |
| F-10.4 | Kontrak E-14 | `POST stock/damaged/write-off` + `stock.adjust`; body `{id_product!, id_product_variant?, id_damaged_location?, quantity!, reason_text!}`; rusak-cukup; **satu**-gerak-`out`-`damaged_write_off` + metadata-rugi (`estimatedLossAmount`!); audit write_off; respons `{movement, estimated_loss_amount}`; **final-tanpa-kembali!** |
| F-10.5 | Halaman | Kartu (`Jenis Barang Rusak` + `Potensi Nilai`!); tabel Produk (+ varian!) · Lokasi (badge!) · Qty · Potensi-Nilai · Aksi (`Kembalikan` + `Rugi/Buang`, izin!) + paginasi-20; form (`Catat Barang Rusak`; `Kosongkan lokasi asal jika barang rusak kembali dari pengiriman dan sebelumnya sudah keluar dari stok.`; Lokasi-asal-opsional-`Tidak dari stok aktif`!; Qty + Alasan-`Contoh: rusak di jalan, patah saat bongkar, atau cacat dari supplier`; notice `Barang rusak tidak ikut stok jual...`; `[Simpan Barang Rusak]`); modal `Kembalikan ke stok normal` (alasan-bawaan `Masih layak jual` + qty-penuh!); buang = alasan-tetap `Tidak layak dijual` + tanpa-dialog!; toast 6-pesan! |

### F-11 — Saldo awal (`stock/opening/*` E-08…E-10, `/stock/opening`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-11.1 | Kontrak E-08 | `POST stock/opening/context` + `stock.adjust`; tanpa-body; cabang-aktif (404 `Cabang tidak ditemukan`!); produk-aktif-tracked + varian-terlihat/default + daun-aktif + saldo + saran (`existing_stock` bila satu-lokasi-berisi / `single_location` bila satu-daun!); baris per-varian-template (`product_code`, `suggested_*`, `filled_location_ids`, `current_on_hand_qty`, `opening_status: filled|empty`!); ringkasan (`products_total` = baris-varian!, `master_products_total`, `locations_total`, dengan/tanpa-lokasi, dengan/tanpa-terisi, `status: no_locations|ready|needs_location`!) |
| F-11.2 | Kontrak E-09 | `POST stock/opening/preview` + `stock.adjust`; multipart `file` (memori, **5MB!**; tanpa-file → 400 `File Excel wajib diunggah`!; bukan-`.xlsx/.xls` → 400 `File harus berformat Excel .xlsx atau .xls`!); sheet-alias (`stok awal`/`stock opening`/`opening stock`/`migrasi stok`, else 400 `Sheet "Stok Awal" tidak ditemukan`!); header-alias per-kolom + angka-fleksibel (`1.234,5` vs `1,234.5`!) + baris-2 + kosong-lewat; 12-jenis-isu (wajib-3-kolom + non-negatif + kode-normalisasi-spasi-UPPER + master/varian/lokasi/daun/aktif + **harga-SELALU-master** (tanpa-harga = tolak!) + varian-wajib-kode + nama/satuan-peringatan + duplikat + terisi-lewati-peringatan!); respons `{ok (tanpa-error!), summary{rows_total/valid, products_total, locations_total, total_quantity, total_value, errors, warnings, skipped}, issues[{severity, sheet: 'Stok Awal', row, column, message}], items[{row, id_product, id_product_variant, product_code/name, base_uom, id_stock_location, location_code/name, quantity_on_hand, average_cost, inventory_value, note}], skipped[]}` |
| F-11.3 | Kontrak E-10 | `POST stock/opening/commit` + `stock.adjust`; file-sama; tolak-error (`File stok awal masih memiliki error...`) / nol-baris (`Tidak ada baris stok awal dengan jumlah lebih dari 0.`) / finance-posted (dua-lapis: cek-dulu + kunci-tulis! `Saldo awal finance sudah diposting...`) / terisi (`Stok ... di lokasi ... sudah terisi...` — lock!); saldo = qty (**timpa, bukan-tambah!**) + `available = qty − reserved`; movement-`in`-`opening_stock` (`idReference` = id-buka-keuangan! + `{source: 'stock_opening_import', row, productCode, locationCode, averageCost}`); finance-agregat-per-produk + gabung-aditif-ke-draf; audit `stock.opening_import.commit` (rows/totalQty/totalValue/openingId/financeRows/movementIds!); respons = preview + `imported{rows_imported, movements_created, finance_opening_items_synced, total_quantity, total_value}` |
| F-11.4 | Harga-modal | **Kolom harga di Excel dihapus permanen** (insiden-2026-08: 241-produk 10–1000×, ±Rp3,2M!); `average_cost = roundMoney(purchasePrice / purchaseToBaseFactor)` + `inventory_value = roundMoney(qty × avg)`; file-lama-berkolom-harga = diabaikan-diam! |
| F-11.5 | Halaman/wizard | `Stok Awal` (`Migrasi jumlah fisik barang per lokasi sebelum transaksi berjalan dicatat.` + `[Buka Wizard]`); 4-kartu (Barang-Stok/Lokasi-Aktif/Belum-Dipilih-Lokasi/Sudah-Diisi!); wizard `Migrasi Stok Awal` (Excel!): metrik-5 + tetapkan-lokasi (massal-`Terapkan ke yang kosong`/`Terapkan ke semua` + per-baris + buat-cepat-`GUD-UTAMA`!) + unduh-gate (`Pilih lokasi untuk semua barang sebelum download template.` / `Semua barang pada lokasi yang dipilih sudah punya stok awal...`!) + file (`Pilih file Excel stok awal`, `.xlsx/.xls`, `Pilih File`/`Ganti File`) + `[Cek File]` + preview-12-isu + `[Simpan Stok Awal]`-gate + toast-ganda + `Buka Saldo Awal Keuangan` (→`/finance/opening`!) |

### F-12 — Rute, menu & guard FE

| # | Sub-fitur | Detail |
|---|---|---|
| F-12.1 | 10 rute + izin | `/stock` (view) · `/gudang` (view) · `/stock/movements` (view, tanpa-menu!) · `/gudang/locations` (view-rute, kelola-butuh-`stock.update` di-dalam!) · `/gudang/transfer` (`stock.transfer`!) · `/gudang/pindah-lokasi` (`stock.transfer`!) · `/gudang/damaged` (view-rute, tulis-butuh-`stock.adjust` di-dalam!) · `/stock/opening` (`stock.adjust`!) · `/stock/adjustments/create` (`stock.adjust`!) · `/stock/:itemId` (view, **terakhir** agar tak-menelan rute-khusus!) — lazy-route semua |
| F-12.2 | Toast katalog | Penyesuaian (`Penyesuaian stok tersimpan`/`Gagal menyimpan penyesuaian stok`); lokasi (`Lokasi berhasil dibuat/diperbarui/diarsipkan` + gagal-3! + `Urutan lokasi diperbarui`); pindah (`Stok dipindahkan` + `Lokasi asal dan tujuan sudah diperbarui.`/`Gagal memindahkan stok`); dokumen (`Surat transfer antar cabang disimpan` + `Cetak surat lalu tekan Kirim...`/`Gagal membuat surat transfer`; `Barang ditandai sedang dikirim` + `Stok asal sudah berkurang...`/`Gagal mengirim transfer`; `Transfer diterima, stok tujuan bertambah`/`Gagal menerima transfer`; `Surat transfer dibatalkan`/`Gagal membatalkan transfer`); rusak (6!); buka (2-sukses + 7-gagal!) |
| F-12.3 | Store preload | `reloadStock`: paralel `locations/list` + `balances(limit-100-include_breakdown)` + `movements(limit-100)`; `useStockModule` memuat stok + produk saat modul-dikunjungi (nama-produk untuk tabel-stok!) |
