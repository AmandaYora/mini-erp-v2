# Test Cases — Modul 16 Stock / Inventory & Gudang

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`stock.service.spec.ts` (1400+-baris: scan/saldo/sesuaikan/lokasi/pindah/dokumen/rusak/detail!),
`stock-opening.service.spec.ts` (420+-baris: template/harga/varian/lokasi/commit/gabung!),
`use-stock-module.test.ts` (97: trigger + return!), E2E 04 (cabang-gudang!), 08 (rusak/pindah/
dokumen/approval!), 12 (buka-nyata + screenshot!). **GAP** = belum ada test (FE + batas!).

---

## TC-S — Saldo/scan/sesuaikan (TC-S-01…TC-S-12)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-S-01 | Scan kosong/tak-ada/nonaktif/non-track/varian-wajib/default-hilang | 6 status (`empty_code` tanpa-query! / `not_found` ×2 / `inactive` / `ok`-null-saldo / `variant_required` + daftar!) | Ya (spec ×6+) |
| TC-S-02 | Agregat-SQL + rincian-halaman-ini + dual-dua-kasus | Tepat (`MIN/SUM/MAX`, breakdown-hanya-halaman!) | Ya (spec) |
| TC-S-03 | `in` tambah / `out` kurang / `set` pas / saldo-baru-0 / kurang-tolak / nol-negatif / hilang / tak-track / tanpa-lokasi | Tepat (9-pesan!) | Ya (spec ×9) |
| TC-S-04 | Tanpa-modal (dua-arah!) / masuk-hanya-beli-lolos / keluar-tanpa-avg-tolak | `400` + cara / lolos / `400` | Ya (spec ×3) |
| TC-S-05 | Tanpa-kredensial/salah/tanpa-izin/strict-diri + audit | 401/401/403/403 + log-`stock.adjust` | Ya (spec ×5 + E2E-08!) |
| TC-S-06 | Lokasi: pohon-daun-depth-first / lintas-cabang-tujuan / induk-berisi-tolak / anak-berisi-tolak-arsip | Tepat | Ya (spec ×4 + E2E-04!) |
| TC-S-07 | Pindah instan atomik-2-gerak / kurang-tolak / sama-tolak | Tepat | Ya (spec ×2 + E2E-08!) |
| TC-S-08 | Dokumen: draf-tanpa-gerak / secabang-beda-lolos / sama-tolak / kirim-kurang-tolak / terima-tambah / terima-tanpa-kirim-tolak / batal-draf / batal-terkirim-tolak / lintas-cabang | Tepat | Ya (spec ×6+ + E2E-08!) |
| TC-S-09 | Daftar kritis (≤-non-null!) + rusak-sembunyi-default + tokenized-8 + limit-jepit-1–100 | Tepat | GAP (kode saja!) |
| TC-S-10 | Dual-UOM-dua-kasus + step-bulat/pecahan + label-ganda-`X (Y)` | Tepat | GAP (tampil saja!) |
| TC-S-11 | Detail-null + pohon-relevan-leluhur + 10-mutasi-server-side | Tepat | GAP (FE saja!) |
| TC-S-12 | Dialog-pemilik-420px + tombol-ganda-gate + returnTo-dinamis (`/stock` vs `/gudang`!) + toast + reset-sukses | Tepat | GAP (FE saja!) |

## TC-L — Lokasi (TC-L-01…TC-L-06)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-L-01 | Buat: kode-kosong/nama-kosong/ganda-409/ganda-arsip-lolos/induk-hilang-404 | 400/400/409/lolos/404 | Ya (spec ×1 + GAP-batas!) |
| TC-L-02 | Induk-berisi tanpa-`force` → dialog → `force` pindah-semua + 2-gerak-`transfer` | 400-`PARENT_HAS_STOCK` → 200 + audit-flag | Ya (spec ×1 + FE-dialog-GAP!) |
| TC-L-03 | Ubah: tanpa-id-400/hilang-404/ganda-409/kosong-400 + reparent-diabaikan | Tepat + audit-before/after | GAP (sebagian-spec!) |
| TC-L-04 | Arsip: beranak-400/berstok-400/arsip-200-lunak | Tepat | Ya (spec ×1!) |
| TC-L-05 | Daftar lintas-cabang: `id_branch`-tujuan-aktif vs hilang-404 | Tepat | Ya (spec ×1!) |
| TC-L-06 | Susun-ulang sesama-level otomatis + lintas-level-diabaikan + toast | Tepat | GAP (FE saja!) |

## TC-T — Transfer dokumen (TC-T-01…TC-T-06)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-T-01 | Buat-draf: kosong-400/tujuan-mati-404/tak-track-400/sama-secabang-400 | Tepat + nomor-`TRF-` | Ya (spec ×3 + E2E-08!) |
| TC-T-02 | Kirim: bukan-draf-400/kosong-400/kurang-400 + saldo-kurang + `in_transit` | Tepat | Ya (spec ×1 + E2E-08!) |
| TC-T-03 | Terima: bukan-transit-400/tanpa-out-400/cabang-salah-404 + saldo-tambah + `received` | Tepat | Ya (spec ×2 + E2E-08!) |
| TC-T-04 | Batal: draf-200-`cancelled` / terkirim-400 / stok-tak-tersentuh | Tepat | GAP (E2E-08-sebagian!) |
| TC-T-05 | Daftar: arah-keluar/masuk/semua + status + order-`createdAt`-DESC | Tepat | GAP (kode saja!) |
| TC-T-06 | Cetak-surat-tanda-3 + tombol-kondisional-cabang-status | Tepat | GAP (FE saja!) |

## TC-D — Rusak (TC-D-01…TC-D-05)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-D-01 | Masuk: yatim-tanpa-asal vs asal-kurang-400 + RUSAK-otomatis + rugi | Tepat | Ya (spec ×1 + E2E-08!) |
| TC-D-02 | Pulih: kurang-400/tujuan-daun + 2-gerak + `max(0)`-tersedia | Tepat | Ya (spec ×1 + E2E-08!) |
| TC-D-03 | Buang: kurang-400 + 1-gerak-`damaged_write_off` + final | Tepat | Ya (spec ×1 + E2E-08!) |
| TC-D-04 | Daftar: paginasi + rugi-global-lintas-halaman + hidden-null | Tepat | Ya (spec ×2!) |
| TC-D-05 | FE: alasan-bawaan-pulih + alasan-tetap-buang-tanpa-dialog + toast-6 | Tepat | GAP (FE saja!) |

## TC-O — Buka (TC-O-01…TC-O-08)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-O-01 | Konteks: saran-`existing_stock`/`single_location` + status-3 + tanpa-cabang-404 | Tepat | GAP (kode saja!) |
| TC-O-02 | Preview-valid: harga-selalu-master + nilai = qty × avg | Tepat | Ya (spec ×1!) |
| TC-O-03 | Kolom-harga-lega-diabaikan + tanpa-harga-tolak-baris | Tepat | Ya (spec ×2!) |
| TC-O-04 | Varian: kode-induk-minta-varian + contoh + tak-aktif-tolak | Tepat | Ya (spec ×2!) |
| TC-O-05 | Lokasi-tak-kenal/non-aktif/non-daun + duplikat + nama/satuan-warning | Tepat | Ya (spec ×1 + GAP!) |
| TC-O-06 | Commit: movement-`opening_stock` + finance-gabung-aditif + audit-kaya | Tepat | Ya (spec ×2!) |
| TC-O-07 | Commit-tolak: error/nol/posted/terisi (dua-lapis!) | 400 ×4-pesan | Ya (spec ×2!) |
| TC-O-08 | Wizard-E2E: tetapkan → unduh-gate → isi → cek-12 → simpan-gate → `/finance/opening` | Tepat + screenshot | Ya (E2E-12!) |

## TC-R — Saran/reservasi (TC-R-01…TC-R-05)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-R-01 | Saran: tanpa-produk-400/qty-400 + urut-petik-primer-default + `enough/shortage` | Tepat | GAP (kode saja!) |
| TC-R-02 | Hold: tanpa-kunci-400/kosong-400/TTL-jepit + kedaluwarsa-dulu + lepas-dulu + non-track-lewat + kurang-rollback | Tepat | GAP (kode saja!) |
| TC-R-03 | Release: kunci-user-sama + `released_count` + saldo-turun | Tepat | GAP (kode saja!) |
| TC-R-04 | Hold-ulang-kunci-sama = lepas + buat-baru (tanpa-ganda!) | Tepat | GAP (kode saja!) |
| TC-R-05 | Scan-modal-kasir: `additive sales availability` UOM-beda | Tepat | Ya (spec ×1!) |

## TC-M — Mutasi & halaman (TC-M-01…TC-M-04)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-M-01 | Filter: produk/tipe/tanggal/ tokenized + order-DESC + `T23:59:59` | Tepat | GAP (kode + FE!) |
| TC-M-02 | Badge-3 + Qty-+/− + Saldo-`→` + kosong-4-sumber + 25/halaman | Tepat | GAP (FE saja!) |
| TC-M-03 | Gudang-drill: tumpukan + cache-200 + expand-3-metrik + deep-link | Tepat | GAP (FE saja!) |
| TC-M-04 | Hook: muat-stok+produk-saat-idle / diam-saat-loading-ready / field-kembali | Tepat | Ya (web-test ×5!) |

## TC-U — Izin & regresi (TC-U-01…TC-U-03)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-U-01 | Rute tanpa-izin diblokir; tombol hilang per-izin (adjust/transfer/update!) | Tepat | GAP (FE saja!) |
| TC-U-02 | E2E-04 cabang-baru: gudang-sendiri + sequence + mapping + alokasi-order | Tepat | Ya (E2E-04!) |
| TC-U-03 | E2E-08/12 hijau (rusak/pindah/dokumen/approval/buka-nyata) | Tepat | Ya (E2E-08/12!) |
