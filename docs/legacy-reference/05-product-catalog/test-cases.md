# Test Cases — Modul 05 Product / Catalog

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Skenario input→output nyata dari
kode/data yang ada. Kolom **Sudah ada** menandai cakupan test saat ini (`product.service.spec.ts`,
`product-import.service.spec.ts`, test FE `product-form-page`, `product-category-select`,
`product-media-panel`, `product-search-select`, `product-type-config`, E2E 15 + 17); **GAP** =
belum ada test dan perlu ditulis saat rebuild.

---

## TC-C — Kategori (TC-C-01…TC-C-08)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-C-01 | `createCategory {code:'CAT-NEW', name:'Kategori Baru'}` (belum dipakai) | Tersimpan + audit `product_category.create` | Ya (spec) |
| TC-C-02 | `createCategory` kode sudah dipakai non-arsip | `409 ConflictException`, tidak menyimpan | Ya (spec) |
| TC-C-03 | `createCategory {id_parent_category: 5}` | `idParentCategory=5` tersimpan (tanpa validasi induk) | Ya (spec) |
| TC-C-04 | `updateCategory {id:1, name:'Baru'}` ada | Nama berubah + audit before/after | Ya (spec) |
| TC-C-05 | `updateCategory {id:99}` tak ada | `404 NotFoundException` | Ya (spec) |
| TC-C-06 | `updateCategory {sort_order:5}` | `sortOrder=5` | Ya (spec) |
| TC-C-07 | `archiveCategory(1)` ada | `archivedAt=Date` + audit `product_category.archive` | Ya (spec) |
| TC-C-08 | `archiveCategory(99)` tak ada | `404` | Ya (spec) |
| TC-C-09 | `createCategory` kode arsip / induk tak dikenal / `updateCategory` ganti kode | Arsip: lolos (potensi 500); induk: tersimpan mentah; kode: **diabaikan** | GAP → KI-50/51/54 |
| TC-C-10 | UI: pilih induk saat edit diri beranak | Diri + keturunan hilang dari opsi (`getDescendantCategoryIds`) | Ya (FE test) |

## TC-L — List & search (TC-L-01…TC-L-12)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-L-01 | `list {}` | `{items, meta:{page:1, limit:20, total}}` + gambar default bila tanpa foto | Ya (spec) |
| TC-L-02 | `list {limit:9999}` | `take(100)` | Ya (spec) |
| TC-L-03 | `list {search:'prd'}` | `andWhere LIKE '%prd%'` (tokenized) | Ya (spec) |
| TC-L-04 | `list {search:'SPECIAL  1 KG'}` | 3 token AND (`%SPECIAL%`, `%1%`, `%KG%`) | Ya (spec) |
| TC-L-05 | `list {status:'inactive'}` / `{product_type:'service'}` / `{id_product_category:3}` / `{stock_tracked:true/false}` | Filter masing-masing tepat; `stock_tracked` undefined → tanpa filter | Ya (spec ×5) |
| TC-L-06 | `list {page:2, limit:10}` | `skip(10) take(10)` | Ya (spec) |
| TC-L-07 | `list {prefer_stock_available:true}` + cabang | `leftJoin stock_rank` + `orderBy stock_rank_order ASC, name, code` | Ya (spec) |
| TC-L-08 | `searchOptions {search:'vioni', stock_tracked:true}` | Item ringan + harga number + `meta{limit:20, has_more:false}`, **tanpa** `getSignedUrl` | Ya (spec) |
| TC-L-09 | `searchOptions {limit:999}` (31 baris) | `take(31)` → 30 item + `has_more:true` | Ya (spec) |
| TC-L-10 | `searchOptions {search:'CAT', status:'active', stock_tracked:true}` | Filter + `CASE search_rank` + `orderBy search_rank` | Ya (spec) |
| TC-L-11 | `searchOptions {search:'CAT', prefer_stock_available:true}` + cabang | `orderBy search_rank` lalu `addOrderBy stock_rank_order` (relevansi tetap pertama) | Ya (spec) |
| TC-L-12 | `searchOptions {id_product:42, limit:1}` | `andWhere id` + `take(2)` (limit+1) | Ya (spec) |
| TC-L-13 | `detail(1)` ada / `(99)` tak ada | Item + gambar + varian / `404 'Produk tidak ditemukan'` | Ya (spec) |
| TC-L-14 | `list {limit:0}` / `-5` / `"abc"` | Diteruskan ke query (kosong/500) | GAP → KI-55 |
| TC-L-15 | E2E: produk habis vs tersedia vs jasa (3 produk, stok hanya 1) | `products/list` urut: tersedia → jasa → habis; tercermin di tabel `/products`, picker order, grid POS | Ya (E2E 17) |

## TC-P — Create produk (TC-P-01…TC-P-14)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-P-01 | Fisik berstok lengkap (`NEW-001`, pcs/box 12, harga beli 10000) | Tersimpan + audit `product.create` + varian default tersembunyi | Ya (spec ×2) |
| TC-P-02 | Fisik berstok tanpa `purchase_price` | `400 BadRequest`, `save` tidak dipanggil | Ya (spec) |
| TC-P-03 | Non-stok tanpa harga beli | Lolos | Ya (spec) |
| TC-P-04 | Kode duplikat | `409` | Ya (spec) |
| TC-P-05 | Field opsional lengkap (3 harga, min stok, summary, attributes) | Tersimpan semua | Ya (spec) |
| TC-P-06 | `summary:'   '` | `null` | Ya (spec) |
| TC-P-07 | Hanya `uom:'pcs'` (legacy) | Ketiga satuan=pcs, faktor 1 | Ya (spec) |
| TC-P-08 | `service` + `stock_tracked:true, sales_uom:'jam', min_stock_qty:10` | `stockTracked=false`, satuan=jam, faktor 1, min=null | Ya (spec) |
| TC-P-09 | `service` tanpa satuan | `400 'Satuan transaksi wajib diisi'` | Ya (spec) |
| TC-P-10 | Status tak dikirim / `'inactive'` | `active` / `inactive` | Ya (spec ×2) |
| TC-P-11 | `has_variants:true, variants:[]` | `400 'Minimal satu varian wajib diisi'` | Ya (spec) |
| TC-P-12 | `has_variants:true` + 1 varian | Varian tersimpan (`isDefault=false, isHidden=false, status active`) | Ya (spec) |
| TC-P-13 | UI: tipe fisik→jasa→fisik | Field stok hilang lalu kembali **dengan nilai** | Ya (FE test) |
| TC-P-14 | UI: tipe jasa | `"Satuan layanan"` ada + `"Jasa tidak dipantau sebagai stok gudang."` | Ya (FE test) |
| TC-P-15 | E2E: isi form `/products/create` fisik | Redirect `/products/:id` + stok detail 0 | Ya (E2E 15-real-user) |

## TC-U — Update produk (TC-U-01…TC-U-12)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-U-01 | `{id:1, product_name:'Baru'}` | Berubah + audit `product.update` | Ya (spec) |
| TC-U-02 | `{id:99}` | `404` | Ya (spec) |
| TC-U-03 | Ganti kode ke milik produk lain | `409` | Ya (spec) |
| TC-U-04 | Ganti kode ke nilai sama | Lolos (tanpa cek duplikat) | Ya (spec) |
| TC-U-05 | Update 13 field sekaligus (nama, tipe service, satuan, harga, status, atribut, summary) | Semua teraplikasi; fisik→service memaksa satuan seragam + min null | Ya (spec) |
| TC-U-06 | `summary:'   '` | `null` | Ya (spec) |
| TC-U-07 | `{uom:'lembar'}` pada produk ada | Ketiga label=lembar, faktor tetap (12/1) | Ya (spec) |
| TC-U-08 | Berstok → `product_type:'service'` | `400` | Ya (spec) |
| TC-U-09 | `base_uom:'dus'` + ada movement | `400 /riwayat pergerakan stok/`, `save` tidak dipanggil, query cek `inventory_movements` | Ya (spec) |
| TC-U-10 | Fisik non-stok → `non_stock` | Lolos + satuan seragam + min null | Ya (spec) |
| TC-U-11 | Edit harga beli berstok → 0/null; ganti label satuan beli/jual; harga negatif | **Lolos** (tak ada guard) | GAP → KI-52 |
| TC-U-12 | Ganti kode berspasi / kode varian milik sendiri | Trim + lolos untuk sendiri; spasi-create lolos tanpa trim | GAP → KI-50 |

## TC-M — Media (TC-M-01…TC-M-08)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-M-01 | `listMedia` ada foto | Item + `url` signed + `isPrimary` | Ya (spec) |
| TC-M-02 | `uploadMedia` JPEG pertama | Optimasi→WebP→`putObject(private)`→baris `isPrimary:true` + metadata optimized→audit `product_media.upload` | Ya (spec) |
| TC-M-03 | `uploadMedia` PDF | `400` dari optimizer | Ya (spec) |
| TC-M-04 | `archiveMedia` | `archivedAt=Date` + audit | Ya (spec) |
| TC-M-05 | `setPrimaryMedia` non-utama | `isPrimary=true` + audit + respons | Ya (spec) |
| TC-M-06 | `archive` produk ada/tak ada | `archivedAt` + audit / `404` (+ tidak save bila tak ada) | Ya (spec ×3) |
| TC-M-07 | Arsip produk berstok / bersejarah-kosong; arsip foto utama (promosi pengganti); upload tanpa file; >5MB | 400 berstok / lolos bersejarah / promosi otomatis / 400 tanpa file / tolak >5MB | GAP (sebagian di E2E lain) |
| TC-M-08 | FE: panel tanpa foto / tanpa `canUpdate` | Gambar default + notice / tanpa input-tombol | Ya (FE test) |

## TC-I — Import (TC-I-01…TC-I-10, dari `product-import.service.spec.ts` 716 baris + kode)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-I-01 | File contoh valid (kategori+produk+atribut) | Preview `ok:true`, counts create/update tepat | Ya (spec) |
| TC-I-02 | Sheet wajib hilang | Isu error per sheet hilang | Ya (spec, pola umum) |
| TC-I-03 | Duplikat kode dalam file (kategori/produk/varian) | Isu error menyebut baris pasangan | Ya (spec) |
| TC-I-04 | Kode arsip dipakai ulang | Isu `"ada di data arsip. Pulihkan ..."` | Ya (spec) |
| TC-I-05 | Induk tak dikenal / siklus kategori | Isu + (siklus: `"membentuk putaran: A > B"`) | Ya (spec, sebagian) |
| TC-I-06 | Tipe/status/boolean sinonim + tak dikenal | Mapping + fallback (physical/active/true) + isu bila tak dikenal | Ya (spec, sebagian) |
| TC-I-07 | Commit bersih | Counts + **satu** audit `product.bulk_import`, idempoten diupload ulang | Ya (spec) |
| TC-I-08 | Commit dengan error | `400 'File belum bisa diproses karena masih ada {n}...'` | Ya (spec) |
| TC-I-09 | Chunk gagal (lock/timeout) | Fallback per baris; pesan `Database sibuk/Bentrokan transaksi` | GAP (mock-level) |
| TC-I-10 | Varian: produk hanya-di-DB, produk non-fisik, nonaktifkan varian berstok, inline `Info tambahan:` | Masing-masing isu error; commit diblokir | Ya (spec, sebagian) + GAP |

## TC-Q — QR & picker FE (TC-Q-01…TC-Q-06)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-Q-01 | `getOperationalQRCodeTargets` bervarian aktif-terlihat | Satu target per varian (`{produk} - {varian}`), tanpa induk | Ya (FE `qr-export` test — verifikasi nama file test) |
| TC-Q-02 | Tanpa varian terlihat | Satu target kode produk | Ya (sama) |
| TC-Q-03 | `buildBulkQRCodeZipPath` karakter ilegal/duplikat/tanpa kategori | Sanitasi + sufiks `_2` + folder `"Tanpa Kategori"` | Ya (sama) |
| TC-Q-04 | `ProductSearchSelect` ketik + pilih + resolve id | Opsi `"{kode} - {nama}"`, payload `search-options` + `prefer_stock_available` | Ya (FE test) |
| TC-Q-05 | E2E picker: produk baru dipakai di order + penyesuaian stok | Order terbuat + adjustment masuk antre otorisasi | Ya (E2E 15) |
| TC-Q-06 | E2E QR massal + modal tunggal | ZIP terunduh; modal tampil untuk 1 target | GAP (E2E tak menyentuh QR) |

## GAP — celah test yang harus ditutup saat rebuild (G-01…G-10)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Kunci UOM hanya diuji untuk base_uom; tidak untuk perubahan **faktor saja** | Jalur bypass potensial tak terjaga |
| G-02 | Tidak ada test arsip produk berstok vs bersejarah-kosong | Guard integritas utama tanpa jaring |
| G-03 | Tidak ada test update harga beli → 0 (KI-52) | Menentukan apakah KI-52 diperbaiki atau dikunci sebagai perilaku |
| G-04 | Tidak ada test trim/normalisasi kode create vs update (KI-50) | Duplikat berspasi lolos diam-diam |
| G-05 | Tidak ada test kategori: induk tak dikenal, siklus via API, kode-diabaikan (KI-51/54) | API lebih longgar dari UI/import |
| G-06 | Tidak ada test import-commit: fallback per baris, pesan lock/deadlock, idempoten upload-ulang | Jalur paling rumit (chunked) tak teruji ujung-ke-ujung |
| G-07 | Tidak ada test E2E untuk QR (massal maupun modal) | Fitur terlihat-user tanpa jaring |
| G-08 | Tidak ada test batas `limit` list (0/negatif/string) | Seakar KI-26, pola berulang |
| G-09 | Tidak ada test pesan arsip-menyesatkan + tanpa-restore (KI-56) | Keputusan produk perlu dikunci test |
| G-10 | Tidak ada test aturan harga negatif via API | FE `min=0` bisa di-bypass; perlu keputusan |
