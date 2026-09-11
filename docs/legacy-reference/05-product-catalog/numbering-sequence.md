# Numbering Sequence — Modul 05 Product / Catalog

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Format penomoran dokumen.

---

## 1. Keputusan: modul ini TIDAK punya penomoran dokumen

Tidak ada nomor urut, tidak ada sequence, tidak ada generator nomor di modul ini:

| Yang dicari | Hasil |
|---|---|
| Tabel `*_document_sequence` milik produk | Tidak ada (penomoran cabang milik modul 04; penomoran finance milik modul 17) |
| Endpoint / service penomoran | Tidak ada |
| Auto-number saat create | Tidak ada — `product_code`, `code` kategori, `variant_code`, `barcode` **wajib diisi manusia** (atau auto-slug untuk varian, lihat §3) |
| Nomor transaksi (`ORD-`, `PAY-`, `SJ-`, `TRF-`) | Milik modul Order/Stock/Branch, bukan modul ini |

## 2. Identitas teknis pengganti nomor (kontrak integrasi)

| Artefak | Format | Aturan |
|---|---|---|
| `products.id_product` | auto-increment INT | Stabil selamanya; dipakai FK oleh order, stok, finance, media, varian. Tidak pernah ditampilkan ke user |
| `product_categories.id_product_category` | auto-increment INT | Sama; dipakai `products.id_product_category` (nullable) |
| `product_variants.id_product_variant` | auto-increment INT | Dipakai 10 tabel transaksional (migrasi 036); varian default dibuat untuk **seluruh** produk lama |
| `media_files.id_media_file` | auto-increment INT | — |
| Kode operasional | Teks bebas: produk ≤ 50 char, kategori ≤ 50 char, varian ≤ 80 char, barcode ≤ 80 char | Unik per perusahaan (non-arsip); case-insensitive di DB; create tidak trim (→ KI-50) |

## 3. Satu-satunya generator kode: slug varian otomatis

Bila Kode Varian dikosongkan (form maupun inline-create API), server/FE membentuk:

```
{KODEPRODUK}-{SLUG(NamaVarian)}   →   huruf besar
SLUG = uppercase, non-[A-Z0-9] → "-", pangkas "-" tepi, fallback index baris bila kosong
Contoh: produk "PRD-001" + nama "Merah Marun" → "PRD-001-MERAH-MARUN"
```

Bukan sequence (tidak ada counter, tabrakan → 409/isu duplikat, bukan retry-otomatis).

## 4. Kode QR ≠ penomoran

QR yang diekspor modul ini mengenkode **kode yang sudah ada** (kode produk / kode varian), bukan
menerbitkan nomor baru. Memindai QR tidak mengubah state apa pun.
