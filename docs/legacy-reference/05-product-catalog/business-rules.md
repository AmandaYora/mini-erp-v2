# Business Rules — Modul 05 Product / Catalog

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula, kondisi
khusus, dan approval. Diturunkan dari `product.service.ts`, `product-import.service.ts`,
`product.controller.ts`, form FE, dan `product-type-config.ts`. Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

---

## 1. Identitas & keunikan (BR-01…BR-07)

| ID | Aturan |
|---|---|
| BR-01 | Kode produk unik per perusahaan di antara produk **non-arsip**. Cek create memakai nilai mentah (tanpa trim); cek update memakai nilai **trim**; perbandingan mengikuti collation DB (`utf8mb4_unicode_ci`, case-insensitive). Pelanggaran → `409 "Kode produk '{k}' sudah digunakan"`. Kode produk arsip **boleh dipakai ulang** tanpa peringatan (tidak seperti import yang menolak kode arsip). |
| BR-02 | Ganti kode produk (update) yang menabrak kode/barcode **varian** → `409 "...sudah digunakan sebagai kode atau barcode varian"`, kecuali varian default tersembunyi milik produk itu sendiri. |
| BR-03 | Kode varian disimpan **huruf besar** selalu; duplikat dicek case-insensitive dalam payload (400), vs kode produk lain (409), vs kode/barcode varian lain (409). Barcode kosong → `null` (bukan string kosong). |
| BR-04 | Kode kategori unik per perusahaan di antara kategori **non-arsip** (`409 "Kode kategori '{k}' sudah digunakan"`); nilai mentah tanpa trim; kategori arsip boleh dipakai ulang (berisiko tabrakan unique key → KI-50). |
| BR-05 | Nama produk selalu diciutkan (`normalizeWhitespace`: spasi ganda/leading/trailing → tunggal/rapi) di create, update, dan import; nama varian dari import juga. Nama kategori **tidak** dinormalkan. |
| BR-06 | Kode kategori **tidak bisa diubah** — endpoint update tidak menerima field kode (form menampilkannya tetapi perubahan hilang → KI-51). |
| BR-07 | Resolusi referensi kategori (import + dropdown FE) menerima **kode atau nama**; import memprioritaskan kode eksak (existing aktif → draft file → nama). Kode produk varian harus merujuk produk **di file yang sama** (DB saja tidak cukup). |

## 2. Tipe produk & pelacakan stok (BR-08…BR-12)

| ID | Aturan |
|---|---|
| BR-08 | `product_type` ∈ {`physical`, `service`, `bundle`, `non_stock`}; selain itu → `400 'Tipe produk tidak valid'`. Tidak ada tipe `goods` di mana pun (istilah lama sudah mati). |
| BR-09 | Non-fisik selalu `stock_tracked=false` (dipaksa, mengabaikan input), satu satuan transaksi wajib (`400 'Satuan transaksi wajib diisi'` bila kosong), ketiga satuan diseragamkan, kedua faktor = 1, `min_stock_qty=null`. Prioritas satuan: `sales_uom → base_uom → purchase_uom → uom → current`. |
| BR-10 | Fisik baru default `stock_tracked=true` bila tak dikirim (`input ?? current ?? true`); update mempertahankan nilai lama bila tak dikirim. `has_variants` hanya bisa true bila fisik + berstok, else dipaksa false. |
| BR-11 | Produk berstok **tidak bisa** diubah menjadi tipe non-fisik: `400 'Produk yang sedang dilacak stoknya tidak bisa diubah menjadi tipe non-stok. Arsipkan lalu buat produk baru.'`. Produk fisik **non-stok** bebas berubah tipe (satuan diseragamkan, min stok di-null-kan). |
| BR-12 | `min_stock_qty` hanya bermakna bila berstok (selalu null untuk non-stok); FE menyembunyikan field-nya (`showMinStockField = stockTracked`). Tidak ada peringatan stok-rendah di modul ini (konsumsi ada di Stock). |

## 3. Satuan & konversi — model dual-UOM (BR-13…BR-19)

| ID | Aturan |
|---|---|
| BR-13 | Fisik wajib `base_uom` (`400 'Satuan stok (base_uom) wajib diisi'`); kolom legacy `uom` diterima sebagai fallback untuk ketiga satuan; `purchase_uom`/`sales_uom` default = `base_uom`; faktor default 1. |
| BR-14 | Kedua faktor harus > 0 dan finite (`400 '{field} harus lebih besar dari 0'`); presisi kolom `DECIMAL(24,12)` di entity (migrasi 008 menulis `DECIMAL(18,4)` — nilai live mengikuti entity; **[PERLU KONFIRMASI]** migrasi penyelaras mana yang dianggap benar). |
| BR-15 | **Kunci UOM:** bila `baseUom`/`purchaseToBaseFactor`/`salesToBaseFactor` berubah (perbandingan `Number()`) DAN ada ≥1 baris `inventory_movements` untuk produk itu (cabang mana pun dalam perusahaan) → `400 'UoM atau faktor konversi tidak dapat diubah...'`. Mengubah **label** `purchaseUom`/`salesUom` saja tidak mengunci. |
| BR-16 | Semantik faktor (kontrak yang berlaku di seluruh sistem): `purchaseToBaseFactor` = jumlah satuan stok per 1 satuan beli (`1 box = 12 pcs → 12`); `salesToBaseFactor` = jumlah satuan stok yang berkurang per 1 satuan jual (`jual 1 meter mengurangi 0.1 roll → 0.1`); bahasa operasional import/FE adalah kebalikannya `Isi Jual per Satuan Stok` (`1 roll = 10 meter → 10`) dengan rumus `salesToBaseFactor = 1 / isiJualPerStok`. |
| BR-17 | Kolom legacy import (`Konversi Jual ke Stok`, `sales_to_base_factor`, `Isi per Satuan Jual`) tetap diterima sebagai faktor teknis langsung (kompatibilitas mundur); bila `Isi Jual per Satuan Stok` diisi, ia yang menang. |
| BR-18 | Kolom `uom` legacy selalu ditulis = `baseUom` di setiap simpan (migrasi 035 menjaganya writable + backfill). Pembaca live memakai `base/purchase/sales_uom`; `uom` hanya fosil tulis. |
| BR-19 | Rumus kuantitas (shared-types, dipakai FE preview + seluruh modul): `toBase = qty × factor`, `fromBase = qty ÷ factor`, `normalizeUomFactor` (null/''/≤0 → fallback 1), toleransi nol `0.000001`, ambang stok `> 0.0001`. Label: `formatConversionLabel` (`1 box = 12 pcs`), `formatBaseToTransactionLabel` (`1 roll = 10 meter`). |

## 4. Harga (BR-20…BR-22)

| ID | Aturan |
|---|---|
| BR-20 | Produk berstok **wajib** `purchase_price > 0` saat **create** dan saat **import** (pesan: `'Harga beli wajib diisi (lebih dari 0)... dibutuhkan untuk menghitung HPP saat barang terjual.'`). Alasan tercatat di kode: tanpa ini produk tertahan `"Menunggu data"` di Keuangan. |
| BR-21 | Guard BR-20 **tidak** berjalan di **update** (→ KI-52): produk berstok bisa diedit harga belinya menjadi 0/null. `selling_price`/`min_selling_price` selalu opsional, tanpa relasi yang ditegakkan (`min ≤ normal` tidak dicek — [PERLU KONFIRMASI] sengaja atau belum). |
| BR-22 | Presisi harga `DECIMAL(18,2)`; `summary` kosong/whitespace → `null`; `attributes` = map string→string bebas (baris nama-kosong dibuang FE; duplikat nama di FE menimpa diam-diam — last-write-wins). |

## 5. Varian (BR-23…BR-29)

| ID | Aturan |
|---|---|
| BR-23 | Varian hanya untuk fisik berstok. Menyimpan produk `has_variants=true` tanpa ≥1 baris varian bernama → `400 'Minimal satu varian wajib diisi'` (baris nama-kosong dibuang dulu, tanpa error). |
| BR-24 | Mengaktifkan varian (`has_variants` false→true) mensyaratkan total `on_hand` seluruh cabang ≤ 0.0001, else 400 (menyebut "varian default" + "split stok"). Menonaktifkan (true→false) mensyaratkan total `on_hand` varian **non-default** ≤ 0.0001. |
| BR-25 | Menghapus daftar varian / menonaktifkan varian (aktif→nonaktif) / menghapus baris varian mensyaratkan stok varian itu = 0 (≤ 0.0001), else `400 'Varian masih punya stok...'`. |
| BR-26 | Kode varian kosong → otomatis `{KODEPRODUK}-{SLUG(NAMA)}` uppercase (`slug`: non-alnum → `-`, pangkas `-` tepi, fallback index). Status selain `inactive` → `active`. `sort_order` non-finite → index baris. |
| BR-27 | Setiap produk non-varian memiliki/mendapat 1 varian default tersembunyi (`Standar`, kode=nama=barcode=kode produk, status mengikuti produk); dibuat di create produk, create via import, migrasi 036 (backfill seluruh produk lama), dan dipertahankan di setiap update non-varian. Diarsipkan saat varian terlihat pertama diaktifkan (dengan cek stok). |
| BR-28 | Tabrakan pindai (scan-key): dalam satu simpan, tidak boleh ada dua varian memakai string yang sama sebagai kode maupun barcode (case-insensitive) → `400 "Kode/barcode varian '{k}' duplikat"`. Barcode varian tidak boleh sama dengan kode produk mana pun (409). |
| BR-29 | Import menandai produk ber-`hasVariants=true` otomatis bila ada baris variannya, tetapi **tidak pernah** mematikan flag; produk tanpa baris varian di file mempertahankan flag lamanya. Import varian menimpa `variantName/status/sortOrder`, memaksa `isDefault=false, isHidden=false`, dan mengarsipkan varian default lama (dengan cek stok). |

## 6. Arsip (BR-30…BR-33)

| ID | Aturan |
|---|---|
| BR-30 | Arsip produk mensyaratkan total `on_hand` seluruh cabang ≤ 0.0001 → else `400 "Produk masih punya saldo stok {qty}..."`. Riwayat movement **tidak** menghalangi (soft-delete; histori tetap). |
| BR-31 | Cek BR-30 memakai `on_hand_qty`, **bukan** `available_qty` — stok yang seluruhnya ter-reservasi (available 0, on_hand > 0) tetap menghalangi; sebaliknya tidak ada пья cek reservasi khusus ([PERLU KONFIRMASI] arti "saldo" yang dimaksud). |
| BR-32 | Arsip kategori **tanpa syarat apa pun** (boleh dipakai produk, boleh punya anak). Tidak ada arsip varian via API modul ini (varian diarsipkan hanya sebagai efek samping sync). |
| BR-33 | Arsip bersifat satu arah: tidak ada endpoint restore untuk produk/kategori/varian/media. Pesan import yang menyuruh "Pulihkan ... terlebih dahulu" menunjuk aksi yang tidak ada (→ KI-56). |

## 7. Media (BR-34…BR-38)

| ID | Aturan |
|---|---|
| BR-34 | Upload: `owner_type='product'`, `purpose='product_image'`, `visibility='private'` selalu; `is_primary` default = true hanya bila foto pertama; `sort_order` default = maks+1; optimasi WebP wajib via backend (FE compress boleh di-bypass — server yang menentukan); kunci objek dari `buildProductImageKey(company, product, webp)`. |
| BR-35 | `set-primary` dan `upload(is_primary)` menurunkan semua foto lain dalam **satu transaksi** (tidak pernah ada 2 utama). |
| BR-36 | Arsip foto utama mempromosikan foto tersisa paling awal (`sortOrder ASC, createdAt ASC`) dalam transaksi yang sama; bila tak tersisa, produk kembali ke gambar default. |
| BR-37 | Seluruh endpoint media mensyaratkan produk **non-arsip** (`assertProductExists`); media selalu dilingkup `(company, product, purpose)` — foto produk lain tak terjangkau walau id diketahui. |
| BR-38 | Tidak ada endpoint ubah `sort_order`/ganti file: urutan = urutan upload selamanya; mengganti gambar = arsip + upload baru (→ KI-59). Batas 5MB di multer; tipe di optimizer (JPG/PNG/WebP). |

## 8. Aturan lintas modul yang dijaga dari sini (BR-39…BR-42)

| ID | Aturan | Konsumen |
|---|---|---|
| BR-39 | `baseUom` + faktor adalah sumber kebenaran inventori; Stock menyimpan saldo dalam base dan mengonversi saat tampil/transaksi | Stock, POS, Order |
| BR-40 | Order menyalin **snapshot** (kode/nama/UOM/tipe/stok/varian) saat dibuat; perubahan master tidak mengubah order lama (alasan `MUST NOT` di knowledge: jangan join live untuk histori) | Order, cetak nota, retur |
| BR-41 | `purchase_price` produk berstok adalah basis HPP; tanpanya jurnal tertahan `"Menunggu data"` di Finance | Finance, Stock opening |
| BR-42 | `Kode Varian` adalah payload scan/QR operasional untuk produk bervarian; QR induk tidak dipakai bila varian terlihat-aktif ada | POS scan, picking, delivery |

## 9. Sinonim & parsing import (BR-43…BR-46)

| ID | Aturan |
|---|---|
| BR-43 | Tipe: kosong→`physical`; `barang fisik|barang|produk stok|stok|physical`→physical; `jasa|layanan|service`→service; `paket|bundle|paket/bundle`→bundle; `non-stok|non stok|tanpa stok|tidak pakai stok|non_stock`→non_stock; lain → isu error + fallback physical. |
| BR-44 | Status: kosong/`aktif`/`active`→active; `nonaktif|non aktif|tidak aktif|inactive`→inactive; lain → isu + active. Pantau stok: kosong→**true**; ya/y/iya/true/1→true; tidak/n/no/false/0→false; lain → **true** (fallback!) — [PERLU KONFIRMASI] default-true untuk sampah ketikan. |
| BR-45 | Angka fleksibel id/en (`Rp`, `.`/`,` ribuan/desimal); gagal → isu `"Kolom "{kolom}" harus berisi angka."` + fallback (faktor→1, opsional→null). Duplikat kode dalam file dicek per kunci ternormalkan (lower + single-space). |
| BR-46 | Sheet dikenali via alias (Inggris/Indonesia, `?` dan spasi berlebih diabaikan); header per kolom punya 2–6 alias; sheet `Panduan` (dan sheet tak dikenal) diabaikan; baris yang seluruh sel kuncinya kosong dilewati; nomor isu = nomor baris Excel. |

## 10. Kondisi khusus & formula tampilan (BR-47…BR-50)

| ID | Aturan |
|---|---|
| BR-47 | Peringkat stok (`prefer_stock_available` + ada cabang sesi): berstok-tersedia (0) < non-stok/jasa (1) < berstok-habis (2); relevansi teks selalu di atas peringkat stok. Stok dihitung dari `inventory_balances.available_qty` per cabang, hanya lokasi `active` non-arsip. |
| BR-48 | Peringkat relevansi search-options: kode persis (0) < awalan kode (1) < awalan nama (2) < lain (3). |
| BR-49 | Pasangan FE `Isi jual per stok ⇄ Dampak jual ke stok`: `factor = 1/units`, `units = 1/factor`; input tak-valid → pasangan dikosongkan (`""`), submit memakai fallback `"1"`; presisi tampil 12 desimal dipangkas. |
| BR-50 | Audit: `product.create/update` mencatat `{id, name}` (+`before.name` di update); kategori mencatat nama before/after; media mencatat kunci objek/flag; bulk import **satu** entri ringkasan counts (tanpa per-baris). Semua dengan `idBranch: null`. Harga/UOM/varian tidak tercatat di audit (→ NF-10). |

## 11. Aturan yang TIDAK ada (verifikasi)

Tidak ada: batas panjang UOM (selain kolom 30 char); validasi `min_selling_price ≤ selling_price`;
validasi `purchase_price ≤ selling_price`; larangan harga negatif di API (FE `min=0` hanya di browser —
API menerima negatif; [PERLU KONFIRMASI] disengaja atau celah); batas jumlah varian/foto/atribut;
larangan arsip kategori dipakai; larangan hapus atribut (t terbesar: atribut ditimpa ganti-total tiap
update — atribut yang tidak dikirim hilang); rate-limit upload; approval untuk perubahan master
(berlaku seketika bagi pemegang izin — kontras dengan Stock yang punya mode approval).
