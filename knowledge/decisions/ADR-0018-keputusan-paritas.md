# ADR-0018: Keputusan pemilik untuk program penyempurnaan paritas (D1–D7)

## Status
Accepted (2026-10-18). Rencana: `docs/plan/penyempurnaan-paritas/PLAN.md` §1. Temuan:
`docs/ANALISIS-PARITAS-2026-10-16.md` §5.

## Context
Audit paritas independen menemukan beberapa perbedaan dari sistem legacy yang bukan bug,
melainkan keputusan produk yang belum pernah dikonfirmasi pemilik. Program ini tidak boleh
menunggu jawaban, jadi tiap keputusan diberi *default* yang aman dan dapat dibalik. Tidak ada
jawaban pemilik yang tercatat sampai tanggal ADR ini; seluruhnya berstatus **Default dipakai**
dan dapat diubah dengan menerbitkan revisi ADR ini.

## Decision

| ID | Topik | Status | Keputusan yang dipakai |
|---|---|---|---|
| D1 | Tanya SOP/kebijakan via WhatsApp (Knowledge/RAG) | Default dipakai | Tetap **dibuang**. Dasarnya PRD §3/§9 (bukan komentar kode, bukan ADR sebelumnya). Alternatif ringan bila pemilik menginginkannya kembali: retrieval kata kunci atas dokumen SOP tanpa AI, dikerjakan sebagai plan sendiri. **Mohon konfirmasi pemilik.** |
| D2 | Stok keluar saat konfirmasi SJ | Default dipakai | Alur dua-status dipertahankan (stok keluar saat **konfirmasi**). Mitigasi: cek ketersediaan lunak + peringatan saat membuat SJ draf, dan pesan galat konfirmasi yang menjelaskan penyebab (DL-04). Reservasi pada SJ draf tidak dibangun. |
| D3 | Guard jenis member yang dipakai pelanggan aktif (KI-72) | Default dipakai | Jenis member tetap boleh diarsipkan; dialog menampilkan jumlah pelanggan aktif terdampak dan konsekuensi harga (PR-09). Tidak ada blokir. |
| D4 | Usaha PKP (PPN) | Default dipakai | Dianggap PKP: kontrol PPN tersedia di order jual (K-03) dan laporan PPN dilengkapi (FN-04, FN-06). Default order baru tetap "tanpa pajak". |
| D5 | Domain `android-pos-shell` saat cutover | Default dipakai | Didokumentasikan saja; tidak ada perubahan kode (OP-05). |
| D6 | Penyesuaian pajak: model jurnal bertipe vs draft/final | Default dipakai | Model jurnal bertipe dipertahankan (`DB_SCHEMA.md:304`). Lampiran, catatan, dan klasifikasi koreksi ditambahkan hanya sebagai kolom pelengkap (FN-07). |
| D7 | Hak menu "Kategori" | Default dipakai | Menu tetap mensyaratkan `product_categories.manage`; pemilih kategori di form produk tetap lewat `products.view`. Perbedaan dengan legacy (`product.view`) didokumentasikan (PM-01). |

## Consequences
- Tugas terdampak mengerjakan *default* di atas; bila pemilik menjawab lain, tugas dibuka ulang
  lewat §6 PLAN (`N-xx`) dan ADR ini direvisi.
- D1 adalah satu-satunya keputusan yang menghapus kemampuan legacy; ia tetap tercatat sebagai
  "Dibuang sengaja" di tabel penutupan (Z-01) sampai pemilik mengonfirmasi.
- D4 mengubah kelengkapan laporan pajak, bukan perilaku default order.

## Perbedaan dengan legacy yang diterima (PM-01 / D7)

- Legacy: menu dan halaman "Kategori" terlihat oleh siapa pun yang punya `product.view`.
- Revamp: menu, halaman, dan semua perubahan kategori (`POST/PUT /product-categories`, arsip) memakai `product_categories.manage`.
  `GET /product-categories` tetap `products.view`, jadi pemilih kategori di form produk dan filter daftar produk bekerja untuk kasir tanpa membuka halaman kelola.
- Dampak: peran yang hanya punya `products.view` (mis. kasir) tidak lagi melihat menu Kategori; mereka tetap dapat memilih kategori saat mengubah produk bila punya izin ubah produk.
- Bukti perilaku: `apps/web/src/app/routes/registry.tsx` (perm menu), `apps/api/internal/modules/product/presentation/routes.go` (izin per rute), `apps/web/src/app/routes/route-access.test.ts`.
- Bila pemilik menginginkan perilaku legacy, ubah izin menu menjadi `products.view` dan tambahkan kontrol baca-saja pada halaman; itu perubahan terpisah.
