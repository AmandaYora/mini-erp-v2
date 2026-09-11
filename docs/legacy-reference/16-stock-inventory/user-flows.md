# User Flows — Modul 16 Stock / Inventory & Gudang

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** `@Post` + 200 + `{ data }` +
`BranchGuard` (+ multipart buka). 25 endpoint (E-01…E-25, lihat
[feature-inventory.md](feature-inventory.md)). Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Lihat saldo/mutasi/scan/saran/lokasi-list/rusak-list/dokumen-list (E-01/02/03/06/11/15/20/25) | `stock.view` | Rute diblokir |
| Sesuaikan (E-07) + stok-awal (E-08/09/10) + rusak-tulis (E-12/13/14) + tombolnya | `stock.adjust` | Tombol hilang |
| Otorisasi sesuaikan (bukan endpoint!) | `stock.adjust.approve` (login siapa pun!) + mode `simple`/`strict` | 401/403 |
| Lokasi tulis (E-16/17/18) (+ tombol/handle) | `stock.update` | Baca-saja (tanpa handle!) |
| Pindah seketika (E-19) + dokumen (E-21/22/23/24) (+ tombolnya) | `stock.transfer` | Tombol hilang |
| Reservasi hold/release (E-04/05) | `order.create` (kasir, bukan stok!) | 403 guard |
| Rute transfer/pindah/opening/adjust | `stock.transfer` / `stock.adjust` | Rute diblokir |

## UF-01 — Pantau & telusur

`/stock` (cari-400ms/scan-isi-search/kritis-`critical_only`) → E-01 (dua-mode!) → Detail
(metrik-4 + pohon-relevan + 10-mutasi-server-side E-25 `id_product`!) → `Lihat Semua`
(prefill-kode!) → `/stock/movements` (filter-tipe/akhir-hari-`T23:59:59`/`Hapus Filter`).
Lokasi-kosong → `balance: null` → null (bukan error). Produk-non-tracked → `ok`-tanpa-saldo.

## UF-02 — Sesuaikan berotorisasi

`/stock/adjustments/create` (`?item_id=&location_id=&returnTo=`) → tipe-3 + item-tracked +
varian (auto-tunggal!) + lokasi (daun-pertama!) + qty (min-dinamis!) + alasan →
`Lanjut Otorisasi` (gate!) → kredensial pemilik-hak (`simple`: siapa pun berhak
**termasuk-diri**; `strict`: bukan-diri!) → E-07 (guard: qty → alasan → produk → modal →
varian → lokasi → otorisasi!) → movement-`manual_adjustment` + audit → toast
`Penyesuaian stok tersimpan` + returnTo + reload. Modal-gagal: tanpa-modal
(`Isi Harga Beli produk, atau posting pembelian/saldo awal produk ini dulu` untuk-`in` vs
`Posting pembelian atau saldo awal produk ini dulu agar harga modalnya terbentuk` untuk
out/set!); `Otorisasi penanggung jawab wajib diisi` (400); `Credential penanggung jawab
tidak valid` (401 **dua-sebab**!); `User penanggung jawab tidak memiliki izin approval
stock adjustment` / `Mode strict mewajibkan approver berbeda dari requester` (403).
E2E-08: self-`strict` = 403, ganti-approver = 200 + audit-`stock.adjust` tercatat!

## UF-03 — Kelola gudang

`/gudang` (bor-tumpukan + isi-daun-cache-200 + deep-link `Pindah dari Sini`/`Pindah Lokasi`/
`Penyesuaian Stok`!) → `/gudang/locations` (E-15 pohon + tambah/edit/arsip/susun-ulang-
sesama-level + toasts!) → induk-berstok: tolak-`PARENT_HAS_STOCK` (400!) → dialog
`Pindahkan Stok Otomatis?` → `force_transfer` (E-16!) → stok-turun-semua + movement-
berpasangan-`transfer` + audit-flag. Arsip (E-18): tanpa-anak (`Hapus sub-lokasi terlebih
dahulu`) + tanpa-stok (`Lokasi masih punya stok...`); tanpa-dialog, toast
`Lokasi berhasil diarsipkan`. E2E-04: lantai1 + rakA + area-tanpa-rak; mutasi-ke-induk =
400-daun!

## UF-04 — Pindah seketika vs dokumen

Seketika (`/gudang/pindah-lokasi`: E-19; daun-dua + tersedia-gate + 9-syarat + toast
`Stok dipindahkan`, **tanpa-dokumen/nomor**, tetap-di-halaman!) vs dokumen
(`/gudang/transfer`: E-21 draf (tanpa-gerak + nomor-`TRF-`!) → E-22 Kirim (asal-berkurang,
`in_transit`!) → E-23 Terima (tujuan-bertambah, `received`!) / E-24 Batal-draf
(tanpa-gerak!); lintas-cabang lokasi-tujuan-daftar-cabang-lain (E-15-`id_branch`!) +
cetak-surat-fisik-tanda-3!). Keduanya daun + tersedia + varian-wajib. E2E-08: draf →
batal-200 + draf → kirim → terima-200; terima-cabang-salah = 404!

## UF-05 — Rusak-yatim + pulih/buang (E2E 08)

`Catat Barang Rusak` (E-12): tanpa-asal (`Tidak dari stok aktif` = **yatim-dari-nol**,
by-design!) vs berasal (kurang-normal!); alasan-wajib; RUSAK-otomatis → daftar (E-11) +
rugi-global → `Kembalikan` (E-13, alasan-bawaan `Masih layak jual`, qty-penuh, daun-tujuan!)
/ `Rugi/Buang` (E-14, alasan-tetap `Tidak layak dijual`, tanpa-dialog, final!). E2E-08:
tambah-12 → rusak-4 (rugi ≥ 120rb!) → pulih-sebagian → buang-sisa = 1-tersisa; pengganti-
rusak-jalan: keluar-15 + rusak-5!

## UF-06 — Saldo-awal migrasi (E2E 12)

E-08 Konteks (4-kartu + saran-`existing_stock`/`single_location` + status!) → tetapkan
(massal-`Terapkan ke yang kosong`/`semua`/per-baris/buat-cepat-`GUD-UTAMA`!) → template
(hanya-belum-terisi + Panduan-diabaikan + gaya!) → isi (kode-fleksibel + qty, **tanpa-kolom-
harga**!) → `[Cek File]`-E-09 (12-isu + nilai-`Rp`!) → `[Simpan Stok Awal]`-E-10
(tolak-error/nol/posted/terisi!) → movements-`opening_stock` + draft-keuangan-gabung +
toast `Stok awal tersimpan` + `Buka Saldo Awal Keuangan` (→`/finance/opening`!). Harga
SELALU master (kolom-lega-diabaikan + tanpa-harga-tolak!). E2E-12: wizard → `Migrasi Stok
Awal` → `Template siap diunduh` → `Download Template`/`...Belum Diisi` → input
`File Excel stok awal` → preview-valid → simpan → `/finance/opening`!

## UF-07 — Scan & saran & reservasi (kasir)

Scan-kode-pasti E-02 (6-status: `empty_code`/`not_found`/`inactive`/`ok`/`variant_required`/
`ok`-non-tracked! + dialog? — FE hanya isi-search + bunyi?-cek!) → saran-alokasi E-03
(cukup-`enough`/kurang-`shortage` + prioritas-petik-primer-default!) → hold-otomatis E-04
(kunci + TTL-600 + baris-`pos_cart`!) → serah-konsumsi (modul-order, `consumed` di luar
berkas!) / lepas-manual E-05 (`released_count`!). Guard-hantu + saldo-60dk? — tanpa di
modul ini (milik 15!).

## UF-08 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| Tanpa-item/lokasi/varian/alasan/qty (E-07) | `400` sesuaikan (urutan-guard!) |
| Tanpa-modal/out-tanpa-avg/kredensial/izin/strict-diri (E-07) | `400` + cara / `401` / `403` / `403` modal |
| Induk-berisi/anak-berisi/stok-berisi/kode-ganda (E-16/17/18) | `400`-`PARENT_HAS_STOCK` (+ dialog!) / `400` / `400` / `409` lokasi |
| Pindah kurang/sama/bukan-daun (E-19) | `400` pindah (3-pesan!) |
| Dokumen kosong/produk-hilang/tak-track/varian/sama/lokasi (E-21) | `400`/`404` draf |
| Kirim bukan-draf/kosong/kurang (E-22) | `400` kirim (3-pesan!) |
| Terima bukan-transit/tanpa-geran (E-23) | `400` terima (+ tanpa-pindah-cabang!) |
| Batal bukan-draf (E-24) | `400` batal |
| Rusak qty/alasan/produk/varian/kurang (E-12/13/14) | `400`/`404` rusak (+ varian-wajib-FE!) |
| Buka tanpa-sheet/posted/nol/terisi/finance-posted (E-09/10) | `400` buka (+ tanpa-file/tipe-file!) |
| File >5MB/bukan-Excel (E-09/10) | Multer-5MB/`File harus berformat Excel .xlsx atau .xls` |
| Quote/reservasi/saran salah (E-03/04/05) | `400` per-pintu (kunci/TTL/produk/qty/kurang!) |
| Lintas-cabang salah (E-15/23) | `404` cabang-tujuan/surat-tak-ditemukan |

## UF-09 — Cetak & arsip dokumen

Daftar-dokumen E-20 (`direction: all`, 30!) → `Cetak` (kapan pun!) → surat-fisik
(`window.print`, area-cetak-saja!) → tanda-tangan-3 → arsip-fisik (di luar sistem!).
Batal-draf E-24 = arsip-logis (`cancelled`, stok-tak-tersentuh); lokasi-arsip E-18 =
lunak (`archivedAt`, tanpa-hapus!); movement = abadi (tanpa-arsip!).

## UF-10 — Navigasi & guard shell

Menu-6 (izin-rute!) → halaman-10 (guard-rute + guard-tombol-dalam!) → aksi → toast →
`reloadStock` (paralel-3!) → daftar-ulang. `useStockModule` muat stok + produk
(nama-produk-tabel!). Rute-`:itemId`-terakhir (tanpa-menelan!). `returnTo` dinamis
(`/stock` vs `/gudang` di breadcrumb + tombol!). Tanpa-`activeWorkspaceData` = null
(bukan-error!).
