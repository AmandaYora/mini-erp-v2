# Audit Paritas Kapabilitas — Legacy vs Revamp

**Tanggal audit:** 2026-09-10 · **Verifikasi independen ke-2:** 2026-09-10 (lihat §0) · **Audit independen ke-3:** 2026-10-16
([`ANALISIS-PARITAS-2026-10-16.md`](ANALISIS-PARITAS-2026-10-16.md)) · **Dikoreksi (DOC-01):** 2026-10-19 (lihat §0A)
**Sistem lama:** `H:/dimasprasetio/SAAS/mini-erp` (NestJS + TypeORM, multi-tenant)
**Sistem baru:** `H:/dimasprasetio/SAAS/mini-erp-revamp` (Go modular monolith + MySQL, standalone)

Dokumen ini menjawab satu pertanyaan: **apakah revamp sudah punya kapabilitas yang sama dengan
sistem lama?** Bukan perbandingan kode atau arsitektur — struktur, algoritma, teknologi, dan schema
memang sengaja berbeda (lihat [PRD.md](PRD.md) dan `knowledge/decisions/`). Yang diaudit adalah
**permukaan kapabilitas**: endpoint, tabel, halaman, alur kerja, dan desain UI yang dilihat pengguna.

> **Cara membaca.** Acuan terkini: §0A (koreksi 2026-10-19 dan daftar yang masih terbuka/diterima sebagai gap),
> §3 (paritas per modul), §6 (yang sengaja tidak dibawa). §0 dan §10–§12 adalah log pengerjaan bertanggal.
> Angka proyek terkini tidak ditulis di sini — lihat [STATUS.md](STATUS.md).

---

## 0. Verifikasi Independen ke-2 — 6 Koreksi

> **Arsip.** Bagian ini mencatat keadaan 2026-09-10. Keenam temuan sudah ditindaklanjuti (Tahap D/F/I/J dan seterusnya);
> status terkini ada di §0A dan di tabel per modul §3.

Audit sebelumnya (arsip: `PARITY_AUDIT.prev.md`) diuji ulang dari nol dengan cara mencoba
**menjatuhkan klaimnya**, termasuk klaim yang bertanda ✅. Hasilnya **6 koreksi**, dan berbeda dari
putaran sebelumnya, kali ini mayoritas koreksi membuat revamp terlihat **lebih jauh** dari paritas,
bukan lebih dekat.

| # | Klaim audit sebelumnya | Hasil uji ulang |
|---|---|---|
| 1 | Modul 18 Dashboard "✅ Paritas — summary" | ❌ **Salah.** Legacy `dashboard.service.ts` = 1.075 baris dan mengembalikan **20 field**; revamp = 83 baris, **8 field**. Hilang: stok kritis, pesanan prioritas, order per status, 5 barang terlaris, 5 produk margin tertinggi, jumlah transaksi. Lihat §4.1 |
| 2 | Modul 11 Delivery "create/confirm/list/cancel/bukti ✅" | ❌ **Terlalu longgar.** Tabel `delivery_notes` revamp tidak punya `driver_name`, `vehicle_plate`, `recipient_name`, `recipient_signature_status`, `drop_location_note`, `document_kind`. Surat jalan yang sah **tidak bisa dicetak dari data yang ada**. Lihat §4.3 |
| 3 | Modul 12 Sales Return "list/detail/create/confirm/cancel ✅; context & preview belum ada" | ❌ **Jauh lebih besar.** Yang hilang bukan cuma pratinjau: **mode tukar (exchange) tidak ada sama sekali**, begitu juga 4 jenis penyelesaian (`refund`, `customer_credit`, `collect_payment`, `reduce_receivable`). Legacy 1.400 baris → revamp 403. Lihat §4.4 |
| 4 | Modul 15 POS "keranjang & bayar ✅; struk & scanner belum ada" | ❌ **Ada gap ketiga yang tidak disebut:** POS revamp mengunci `paymentTerms: "cod"` dan hanya menerima `cash`/`transfer` — **"Bayar Nanti (Piutang)" beserta jatuh temponya hilang**. Lihat §4.5 |
| 5 | §4.5 "Backend justru **lebih baik** dari sistem lama" (9 file test vs "—") | ❌ **Terbalik.** Legacy punya **54 spec backend (872 `it`)**, **53 test frontend (382 `it`)**, dan **27 spec E2E**. Revamp punya **14 fungsi test**. Revamp bukan lebih baik — revamp kehilangan ±98% jaring pengaman. Lihat §5.1 |
| 6 | §4.3 "Ekspor Excel belum ada" | ⚠️ **Sebagian salah, dan arahnya bagus.** Revamp **sudah** menghasilkan `.xlsx` asli di server (`excelize`) untuk template impor produk & template saldo awal stok. Yang benar-benar hilang adalah **ekspor daftar order** — dan legacy mengekspornya ke `xlsx` **dan `pdf`** (pdfkit), bukan xlsx saja |

Yang **tetap berdiri setelah diuji ulang**: seluruh gap finance §4.2, halaman cetak §4.6 (`grep -r
"window.print"` → nol hasil), QR scan/generate, dan paritas token UI §7 (diverifikasi ulang dengan
`diff` — **nol selisih**, klaim ini benar).

---

## 0A. Koreksi 2026-10-19 (DOC-01) — status mengikuti KODE saat ini

Audit independen 2026-10-16 ([`ANALISIS-PARITAS-2026-10-16.md`](ANALISIS-PARITAS-2026-10-16.md)) menguji dokumen ini terhadap
kode dan menemukan **14 klaim yang keliru atau terlalu longgar** serta ±70 celah (6 tinggi) yang tidak tercatat di sini. Program
penutupnya ([`plan/penyempurnaan-paritas/PLAN.md`](plan/penyempurnaan-paritas/PLAN.md), `npm run plan:check`) sudah menutup
Gelombang 1 (kritis), Gelombang 2 (kerja harian) dan Gelombang 3 (ketahanan operasional); gerbang Gelombang 2 hijau
(`node scripts/verify.mjs`, e2e 52 lulus / 2 dilewati / 0 gagal). Baris-baris di bawah dan di seluruh dokumen ini sudah
diselaraskan dengan kode **per 2026-10-19**, bukan dengan klaim lama; setiap baris yang berubah menyebut ID tugas PLAN yang
menutupnya, atau menyatakan **terbuka** / **diterima sebagai gap** beserta rujukan keputusannya
([ADR-0018](../knowledge/decisions/ADR-0018-keputusan-paritas.md) D1–D8, PLAN §7). Teks lama yang dipertahankan sebagai arsip
ditandai "arsip".

### 0A.1 Empat belas klaim yang dikoreksi

| # | Lokasi | Klaim lama | Keadaan kode 2026-10-19 | Status / rujukan |
|---|---|---|---|---|
| 1 | §3 modul 01/02 | ✅ / "✅ +" tanpa celah | `GET /roles/{id}` membawa `permissions[]`+`locked`, `RolesPage` memuat izin nyata + selisih + konfirmasi pencabutan (F-02). `resolveSession` memeriksa ulang akses cabang pada cache-miss; cabang yang dicabut mengosongkan cabang aktif, `Refresh` merekonsiliasi (F-03). Daftar user memuat Role/Cabang/Jabatan/`lastLoginAt`; audit berlabel Indonesia (`audit-labels.ts`) | **Tertutup** — K-02, K-01, U-01, U-02 |
| 2 | §3 modul 09 | "SO lengkap ✅" | Form SO punya kontrol PPN (kena pajak, termasuk pajak, tarif; F-01), alamat kirim sekali pakai, `assignedUserId`, "Dibuat Oleh", kredit disetujui oleh/kapan, kolom + filter status bayar/jatuh tempo, ringkasan keuangan bersih-retur. Semula **POS tidak punya kontrol PPN**; kini opsi PPN opsional ada di POS (backend POS menerima `taxType`) | **Tertutup** — K-03 (SO + POS), SL-01…SL-04, SL-09, SL-10. Bukti POS: `pos_vat_flow_test.go` `TestPOSOrderVATFlowReachesTaxSummary`, `CheckoutSheet.test.tsx`, e2e E02 varian PPN. Asumsi usaha PKP = ADR-0018 D4 tetap menunggu konfirmasi pemilik |
| 3 | §3 modul 12/13, §4.4 | "penyelesaian ✅ memo-level di kedua modul retur" | Salah untuk retur pembelian: kini kamus pembelian (`reduce_payable`/`refund`/`supplier_credit`). Saldo kredit **dijurnal** (Dr Piutang/Cr Uang Muka Pelanggan; Dr Uang Muka Supplier/Cr Hutang) dan dibalik saat batal; validasi arah/pihak/sisa terutang dipusatkan di `settlement_rules.go`; pratinjau menampilkan selisih + jenis sah; tambah-bayar tukar bisa `pay_now` saat SJ pengganti dikonfirmasi; pemakaian ganda ditutup (N-40) dan saldo bisa dipakai ke tagihan (`POST /payments/use-credit`, N-41) | **Tertutup** — K-04, K-04a…K-04e, N-40, N-41. **Menunggu konfirmasi akuntan:** arah debit/kredit reklasifikasi dan jurnal pemakaian saldo (PLAN §7 K-04a, N-41) |
| 4 | §3 modul 14/15 | goods-receipt/PO ✅ | Daftar penerimaan memuat No. PO/jumlah baris/diterima oleh (GR-01); daftar retur jual/beli memuat Order + pencarian nama pihak/alasan (RT-01); daftar PO memuat Dibayar/Sisa/status bayar/jatuh tempo/badge Diretur (PO-01); lokasi default saat terima (GR-02); faktur pajak/penanggung jawab/pajak per baris PO (PO-03) | **Tertutup** — GR-01, GR-02, RT-01, PO-01, PO-03 |
| 5 | §4.3 / §0 #6 | "Ekspor ✅ selesai penuh, sisa nihil" | Mekanisme benar; isi kini diperkaya: SO xlsx 3 sheet (Ringkasan, Daftar Order 28 kolom, Detail Item; margin hanya `finance.view` + rentang tanggal), PO xlsx 3 sheet (24 kolom). Uji end-to-end lewat UI (unduh + baca isi berkas) kini ada (e2e E14) | **Tertutup** — SL-04, PO-02, T-05 |
| 6 | §4.5 / `:154,329` | "toggle Sembunyikan diskon ✅"; `PosScannerModal.tsx`; "`PosPage.tsx` 744 baris" | Toggle kini ikut ke struk termal dan faktur (`?hideDiscount=1`, SL-05). Komponen pindai = `ScanDialog.tsx` (`PosScannerModal.tsx` tidak ada). `PosPage.tsx` ±50 baris; logika di `use-pos-cart.ts` + komponen `CartPanel`, `CheckoutSheet`, `CustomerSheet`, `PosCustomerPanel`, `ProductGrid`, `PosCategoryDrilldown`, `ScanDialog`, dst. Tambah pelanggan cepat di POS (PR-02), catatan transaksi + alamat kirim opt-in (SL-07), layar sukses dengan Cetak Nota/Bukti Bayar (SL-06), bunyi pindai (SL-08) | **Tertutup** — SL-05…SL-08, PR-02 |
| 7 | §4.7 | "stok termasuk yang paling sehat, endpoint setara" | Ditambah: lokasi di cabang tujuan transfer (F-06), atribut lokasi picking/utama/urutan + prioritas alokasi (F-07), sopir/kendaraan/tanggal transfer + cetak Surat Transfer, estimasi kerugian stok rusak + paginasi, saldo berjalan/catatan/tipe di kartu stok, impor saldo awal beralias Indonesia (klaim `.xls` dihapus), move-in rusak tanpa lokasi asal, filter arah transfer | **Tertutup** — K-06, ST-01…ST-06. **Diterima gap:** stok tidak ditahan pada SJ draf (hanya peringatan; ADR-0018 D2, DL-04) |
| 8 | §4.1 vs §1/§3 | dashboard "❌ 8 field" vs "✅ paritas" | Kontradiksi internal dihapus: dashboard operasional ✅ (Tahap F) membaca modul sales/purchasing/stock lewat `contracts/`, bukan hanya finance. Kini juga kartu status WhatsApp, produk dipantau (`trackedCount`), prioritas PO + alasan/umur, status basis biaya margin | **Tertutup** — Tahap F, DB-01 |
| 9 | §4.2.3 / §9 | "abaikan/pulihkan tidak bermakna tanpa antrean tersimpan"; "opening … bukan penghambat cutover" | Keliru dan sudah diperbaiki: abaikan/pulihkan sumber posting ada (`POST /finance/posting/ignore\|restore`, tabel `finance_posting_ignores`, FN-01); ikhtisar tutup hari (FN-02); saldo awal persediaan dihitung server dari ledger biaya + rekonsiliasi buku vs ledger di integrity/readiness + jalur "lengkapi" (F-05) | **Tertutup** — FN-01, FN-02, K-05, FN-10. Varian `opening/{update,import-inventory,supplement}` tetap tidak dibawa (tertutup jurnal manual/reverse + `opening/complete`) |
| 10 | §4.2.3 | "tax-package ✅", "tax-adjustments ✅" | Paket pajak kini memuat PPN Masukan (pembelian + retur beli), baris retur jual, Buku Besar, Untung Rugi rinci, rentang tanggal bebas (FN-04); ringkasan PPN memuat DPP + daftar tanpa faktur (FN-06); penyesuaian pajak punya klasifikasi (positif/negatif/final/non-objek), catatan, lampiran (FN-07); alasan buka periode wajib + diaudit (FN-05). Draft/final dan arsip **tidak** dibawa | **Tertutup** — FN-04…FN-07. **Diterima gap:** model jurnal bertipe tanpa draft/final/arsip (ADR-0018 D6). Paket pajak yang semula hanya cabang sesi kini mendukung `branch=all` + izin `finance.report.all_branches` (**N-53, tertutup**) |
| 11 | §6 Knowledge/RAG | "tercatat di kode" (komentar `engine.go:19`) | Sumber sebenarnya PRD §3/§9 (keputusan produk, bukan ADR sebelumnya) dan kini ADR-0018 D1; komentar `engine.go` merujuk keduanya. Pengenalan cabang dari teks pesan WhatsApp ada (AS-01) | **Diterima gap — menunggu konfirmasi pemilik** (D1, AS-05) |
| 12 | §5 angka test | "10 E2E; 128 file Go/434 Test; 77 web" | Dihitung ulang dari kode (2026-10-19, akhir DOC-01): 16 berkas spec e2e (E01–E14 + V00 visual + V01 a11y; gerbang Gelombang 2: 52 lulus/2 dilewati, sebelum E12–E14), 176 berkas `*_test.go` / 641 `func Test` (109 berkas / 370 test di `internal/integration`), 163 berkas test web (838 test pada hitungan vitest sebelumnya, 155 berkas; lihat §5.1) | **Dikoreksi** (angka dihitung ulang, §1 dan §5.1) |
| 13 | §3 halaman | "71 `*Page.tsx`" | 85 berkas `*Page.tsx` non-test (hitung ulang 2026-10-19; analisis 2026-10-16 mencatat 84) | **Dikoreksi** |
| 14 | PLAN A-3 / A-19 | "dibawa" / "ditagih saat SJ pengganti" | Diwujudkan: A-3 = jurnal reklasifikasi saldo kredit (K-04a); A-19 = `settlement.pay_now` pada konfirmasi SJ pengganti dan retur tukar POS satu-langkah (K-04e, RT-02) | **Tertutup** — K-04a, K-04e, RT-02 |

### 0A.2 Yang MASIH terbuka atau diterima sebagai gap (per 2026-10-19)

| Hal | Status | Rujukan |
|---|---|---|
| ~~Uji T-01…T-05~~ — **Tertutup**: matriks role (`TestRoleSeedMatrix` + e2e E12, T-01); gateway + otorisasi WhatsApp (`gateway_test.go`, `assistant_authorization_test.go`, temuan N-43/N-44, T-02); form besar (`SalesOrderFormPage*`, `ProductFormPage*`, `RolesPage`, `UsersPage*`, `SetupPage*`, `SalesReturnsPage`, `PosCustomerPanel`, `AddressFormModal`, `ProductDetailPage` foto, T-03); e2e volume + relasi (E13, T-04); e2e ekspor via UI (E14, T-05) | **Tertutup** | PLAN §8 Fase 4 |
| Program migrasi data: ETL ke produksi (Tahap K), gladi cutover (Tahap L), verifikasi cetak fisik (L3), bangun ulang APK `POS_URL` bila domain berubah | **Terbuka** (di luar kapabilitas; dikerjakan pemilik) | `docs/RUNBOOK.md` §5b/§5c, `docs/DEPLOYMENT.md` §7 |
| Tanya SOP/kebijakan via WhatsApp (Knowledge/RAG) | **Diterima gap**, menunggu konfirmasi pemilik | ADR-0018 D1, PRD §3/§9 |
| Stok keluar saat konfirmasi SJ (tidak ditahan pada SJ draf) | **Diterima gap** (peringatan lunak + pesan galat jelas) | ADR-0018 D2, DL-04 |
| Guard arsip jenis member dibuang (KI-72); dialog menampilkan pelanggan terdampak | **Diterima gap** | ADR-0018 D3, PR-09 |
| ~~Kontrol PPN di POS~~ | **Tertutup** — K-03 bagian POS (`TestPOSOrderVATFlowReachesTaxSummary`, `CheckoutSheet.test.tsx`, e2e E02 varian PPN); hanya asumsi PKP (D4) menunggu konfirmasi pemilik | PLAN K-03, ADR-0018 D4 |
| ~~Paket pajak hanya cabang sesi~~ | **Tertutup** — N-53 (`branch=all` + `finance.report.all_branches`) | PLAN N-53 |
| Penyesuaian pajak tanpa draft/final/arsip | **Diterima gap** | ADR-0018 D6, FN-07 |
| Hak menu Kategori mensyaratkan `product_categories.manage` (legacy: `product.view`) | **Diterima gap** | ADR-0018 D7, PM-01 |
| Kasir bawaan tanpa izin bayar → checkout POS tunai meninggalkan order yatim (legacy BR-18, e2e K1) | **Tertutup** 2026-10-21 — role kasir bawaan membawa `payment.create`/`payment.view`; POS menolak bayar sekarang tanpa izin sebelum membuat dokumen (e2e E02 K1). Pilihan izin menunggu konfirmasi pemilik | ADR-0018 D8 |
| Kolom inline "Info tambahan:"/"Atribut:" pada impor produk = error (bukan peringatan + migrasi otomatis) | Menunggu tinjauan pemilik | PLAN §7 PR-08 |
| Perlakuan akuntansi saldo kredit retur dan pemakaian uang muka | Default dipakai, **perlu konfirmasi akuntan** | PLAN §7 K-04a, N-41 |
| Cabang pesan WhatsApp masuk tetap seluruh perusahaan (otorisasi nomor bersifat company-wide; hanya konsol uji dibatasi akses cabang) | **Diterima gap** | PLAN §7 AS-01 |
| ~~Cabang kerja/commit per tugas tidak dibuat~~ | **Tertutup** 2026-10-20 — 5 commit per area di cabang `feat/paritas-gelombang-1-3` | PLAN G-02, §7 |
| sqlc: `finance` dan `stock` masih `pending` di ratchet (hand-written `database/sql`) | Terbuka (ADR-0012, fase terakhir) | `scripts/sqlc-migration-status.json` |
| Risiko operasional dicatat: `Readiness`/`VerifyRollups` menjalankan `Sync` ledger (menulis, idempoten); `AverageUnitCost` per posisi pada stok rusak; antrean posting memakai total pesanan induk untuk SJ/penerimaan; retur beli tanpa nomor pesanan induk di detail PPN | Dipantau | PLAN CHECKPOINT, §7 FN-04/06, FN-10, ST-03 |

---

## 1. Ringkasan Eksekutif

**Verdict (dikoreksi 2026-10-19): paritas ±98% kapabilitas operasional dan pembukuan, dengan catatan.** Angka itu benar untuk
*permukaan* (205 dari 205 endpoint masuk cakupan punya padanan; tidak ada modul legacy yang hilang), tetapi audit independen
2026-10-16 menunjukkan kedalaman di balik beberapa baris ✅ terlalu longgar: ±70 celah (6 tinggi, ±24 sedang, ±40 rendah)
tidak tercatat di dokumen ini. Celah-celah itu sudah ditutup oleh Gelombang 1–3 program penyempurnaan paritas (§0A);
yang tersisa adalah keputusan pemilik yang diterima sebagai gap (§0A.2) dan program migrasi (uji tambahan T-01…T-05 sudah selesai):
**ETL ke produksi + gladi cutover (Tahap K/L)**, serta verifikasi cetak fisik L3 oleh pemilik.

**Angka keras — cakupan endpoint:** dari 220 endpoint legacy, 8 sengaja dibuang
(`company/features/*`, `knowledge/*`) dan 7 dibuang oleh keputusan produk status-order-terkunci (D4 pada PLAN `menuju-pengganti-production`, bukan D4 PKP di ADR-0018; status order konfigurable, §6),
sehingga 205 masuk cakupan. **Finishing 2026-09-11: 205 tertutup = 100%.** Dua gap terakhir
ditutup sesi ini: `deliveries/work-queue` (J3) dan pindah seketika via
`POST /stock/transfers/move-location` (J4, pemetaan dari `stock/transfer` legacy).
Rincian per butir ada di §9 supaya angka ini bisa diperiksa, bukan dipercaya begitu saja.
Varian turunan dari tabel yang disengaja-tidak-dibuat tetap tercatat di §9 sebagai
*tidak dibawa by-design*, bukan gap.

*(Tahap 1 menutup 3 endpoint, lihat §10. Tahap E/J 2026-09-11 menutup `branches/my-access` +
`stock/detail`. Finalisasi menutup `orders/export` (xlsx+pdf di dua modul), `finance/opening`
get/validate/post, `posting-sources` list/post-batch/close-day, `reports/cash-summary`,
`tax-adjustments` list/save, `close` readiness/safe-close, dan `export/tax-package`. Finishing
menutup `deliveries/work-queue`, `stock/transfer` (via `move-location`), dan QR massal via
`products/qr-codes/export` + `products/{id}/qr` bervarian.)*

*(Arsip: pada putaran audit awal angka paritas kapabilitas tertulis ±75% vs 84% cakupan endpoint,
karena gap terbesar — dashboard, cetak, delivery — soal **kedalaman di balik endpoint**, bukan
jumlahnya. Ketiganya sudah ditutup; angka itu tidak lagi berlaku.)*

| Aspek | Status |
|---|---|
| Modul 04, 05, 06, 07, 19, 20 | ✅ Paritas (beberapa **melebihi** legacy). 05/06: tambah pelanggan cepat (PR-01/02), peringkat pencarian (PR-03), default "Aktif" + arsip (PR-04), stok/harga beli di daftar (PR-05), foto saat buat (PR-07), scan di daftar (PR-10); 07: dialog arsip menampilkan pelanggan terdampak (PR-09); 20: label Indonesia (U-02) + galat 5xx masuk audit (OP-06) |
| Modul 01 Auth, 02 User | ✅ Paritas setelah penutupan F-03 (sesi memutus cabang yang dicabut, K-01) dan F-02 (izin per role terbaca, K-02); daftar user memuat Role/Cabang/Jabatan/`lastLoginAt` (U-01). Sebelumnya ditandai ✅ tanpa celah ini |
| Modul 08 Purchasing, 10 Goods Receipt, 14 Payment, 21 Assistant | ✅ Paritas setelah PO-01…PO-03 (status bayar, ekspor, faktur pajak), GR-01/GR-02 (kolom tidak kosong, lokasi default), PY-01 (daftar semua pihak berpiutang/berhutang), AS-01/AS-02 (cabang dari teks, info produk lengkap, tool gagal tercatat) |
| Modul 03 Company | ✅ Paritas (feature flags sengaja dibuang) |
| Modul 09 Sales | ✅ Paritas setelah K-03 (PPN di form SO), SL-01…SL-04, SL-09, SL-10 (alamat sekali pakai, status bayar, ringkasan bersih-retur, ekspor, penanggung jawab). PPN di POS tertutup (K-03 bagian POS) |
| Modul 16 Stock | ✅ Paritas fungsional setelah K-06, ST-01…ST-06 (§4.7). Kebocoran otorisasi transfer per-ID ditemukan & ditutup (§11.1). **Gap diterima:** stok tidak ditahan pada SJ draf (D2) |
| Modul 11 Delivery | ✅ Paritas — kolom SJ lengkap (C1+D3) + antrean kerja (J3) + bukti bertipe (DL-01) + filter antrean (DL-02) + tanggal mutasi = tanggal kirim (DL-03), §4.3 |
| Modul 12 Sales Return | ✅ Paritas setelah K-04a…K-04e, RT-01…RT-04, N-40/N-41 — mode tukar, SJ pengganti, penyelesaian dijurnal, tukar POS satu-langkah, context & preview |
| Modul 13 Purchase Return | ✅ Paritas setelah K-04b (kamus pembelian), K-04c/K-04d (validasi + pratinjau), RT-04 (lokasi/tanggal) |
| Modul 15 POS | ✅ Paritas kanal — kredit, struk, scanner, katalog, panel member, pelanggan cepat, catatan/alamat, toggle diskon sampai ke struk (SL-05…SL-08, PR-02). PPN opsional di POS (K-03 bagian POS) |
| Modul 17 Finance | ✅ **Paritas fungsional via revisi** — saldo awal terpandu persediaan + rekonsiliasi (K-05), jurnal saldo kredit (K-04a), antrean derived + abaikan/pulihkan (FN-01) + ikhtisar tutup hari (FN-02), laba rugi rinci (FN-03), paket pajak lengkap (FN-04), alasan buka periode (FN-05), PPN DPP/tanpa faktur (FN-06), koreksi fiskal berklasifikasi + lampiran (FN-07), buku besar semua akun (FN-08), lintas cabang (FN-09), restatement biaya `costrebuild` (OP-04); kas = flag `is_cash`; penyesuaian = jurnal bertipe (D6) (§4.2) |
| Modul 18 Dashboard | ✅ Paritas operasional (Tahap F + DB-01) — stok kritis, prioritas SO+PO, status, terlaris, margin, produk dipantau, status WhatsApp, basis biaya margin; tren lewat `reporting/sales-trend` (maks 366 hari) |
| Cetak dokumen (SJ, faktur, kwitansi, struk POS, Surat Transfer, label QR) | ✅ Rute + halaman terpasang (Tahap E; Surat Transfer ST-02) — verifikasi cetak fisik L3 tetap menunggu pemilik |
| Ekspor daftar order (xlsx + pdf) | ✅ Mekanisme selesai (J1) dan isi diperkaya (SL-04, PO-02: SO 3 sheet/28 kolom, PO 3 sheet/24 kolom); uji e2e via UI sudah ada (E14, T-05) |
| QR label + cetak | ✅ Selesai (J2, finishing 2026-09-11) — QR per varian, ZIP massal per kategori, modal + halaman cetak `/print/labels` |
| Antrian kirim + pindah 1-langkah | ✅ Selesai (J3/J4, finishing 2026-09-11) — `work-queue` + halaman Antrian Kirim; `move-location` + modal |
| Jaring pengaman uji | ⚠️ **Jauh membaik, belum setara pada beberapa alur** — 176 berkas `*_test.go` / 641 `func Test` (370 integrasi ke MySQL nyata) + 163 berkas test web + 16 spec e2e (§5.1). Alur uji legacy yang dulu belum ada (T-01…T-05) kini tercakup (§0A.2) |
| Desain UI (token, layout, komponen inti) | ✅ Diport identik — diverifikasi `diff`, nol selisih (§7) |

**Ukuran relatif** — revamp mencapai cakupannya dengan kode jauh lebih sedikit. Sebagian besar itu
efek revamp yang diinginkan; sebagian sisanya adalah fitur yang memang belum ada (§4).

| | Legacy | Revamp |
|---|---|---|
| Backend (tanpa test) | 37.218 baris TS | ~59.500 baris Go di `internal/modules` (tanpa kode hasil generate sqlc; ~75.200 dengan ETL, seed, shared, server) |
| Frontend (tanpa test) | 40.603 baris TS/TSX | ~65.100 baris TS/TSX |
| Endpoint | 220 | **244 rute `/api/v1`** di router (`node scripts/contract-check.mjs`; 291 panggilan frontend, 100% cocok) |
| Tabel | 70 | **65** (18 berkas baseline per modul + migrasi bernomor 000019–000023 dan 000031; `docs/DB_SCHEMA.md`) |
| Halaman frontend (file) | 65 | **85** `*Page.tsx` |
| File test | 134 | **341** (173 Go + 155 web + 13 spec E2E) |

*Angka di tabel ini dihitung ulang 2026-10-19 (baris, berkas, rute, tabel; contract-check dan db-schema-doc dijalankan).
Hasil hijau/merah suite penuh yang terakhir diukur adalah gerbang Gelombang 2 (2026-10-19, di bawah).*

**Kesehatan build (gerbang Gelombang 2, 2026-10-19, kode final):** `node scripts/verify.mjs` ✅ HIJAU — Go build/vet,
integrasi Go penuh (±30 menit, MySQL nyata `mini_erp_test`), `tsc --noEmit`, `eslint`, `vitest`, `ui-ratchet`,
`arch-check` + self-test, `sqlc-sync`, `db-schema-doc`, `contract-check` · `npm run test:e2e` ✅ **52 lulus, 2 dilewati,
0 gagal**. Perubahan Gelombang 3 (OP-01…OP-06, PM-01) dan Fase 4 dokumen belum memiliki gerbang penuh tersendiri;
`contract-check`, `db-schema-doc --check` dan `plan-check` dijalankan hijau pada pass DOC-01/DOC-02.
*(Arsip pengukuran 2026-09-11: 73 `func Test`, 35 integrasi; 2026-10-10: 434 `func Test`, 224 integrasi; vitest 17 file / 85 test.)*

> **Angka di tabel ini pernah jauh meleset.** Versi sebelumnya mencatat 180 endpoint, 55 halaman,
> dan 9 file test — ketiganya sudah usang saat ditulis. Hasilkan ulang dengan perintah di §2
> sebelum mengutipnya.

---

## 2. Metode & Bukti

Semua angka di dokumen ini dapat direproduksi. Perbandingan dilakukan pada permukaan nyata kedua
sistem, bukan pada dokumentasi.

```bash
# 1. Endpoint legacy (220) — @Controller + @Get/@Post/... di seluruh controller
for f in $(find apps/api/src/modules -name "*.controller.ts"); do
  grep -oP "@Controller\(\s*'\K[^']*" "$f" | head -1
  grep -oP "@(Get|Post|Put|Patch|Delete)\([^)]*\)" "$f"
done

# 2. Endpoint revamp (244; atau `node scripts/contract-check.mjs`)
grep -rhoP '"(GET|POST|PUT|PATCH|DELETE) /api/v1[^"]*"' apps/api/internal --include=*.go | sort -u

# 3. Tabel — @Entity('...') vs CREATE TABLE di migrations/*.up.sql
# 4. Rute frontend — module-registry.tsx (81 path) vs route-paths.ts (45 path)
# 5. Token UI — diff token @theme kedua sisi (§7)
# 6. Test — find -name '*.spec.ts' | wc -l   vs   find -name '*_test.go' | wc -l
```

Setiap gap di §4 disertai bukti spesifik (nama kolom/endpoint/file:baris), bukan kesan.

---

## 3. Paritas per Modul

| # | Modul legacy | Modul revamp | Status | Catatan |
|---|---|---|---|---|
| 01 | auth-session | `auth` | ✅ | login, refresh, logout, me, switch-role, switch-branch. `switch-branch` memvalidasi akses cabang; **sejak K-01 (F-03)** akses cabang yang dicabut juga memutus cabang aktif sesi berjalan (`resolveSession` pada cache-miss, `Refresh` merekonsiliasi). Arsip: baris ini dulu ✅ tanpa menyebut celah itu |
| 02 | users-roles-permissions | `user` | ✅ **+** | users CRUD + status + ganti sandi (daftar memuat Role/Cabang/Jabatan/`lastLoginAt`, U-01); roles CRUD + permissions — **sejak K-02 (F-02)** `GET /roles/{id}` membaca izin nyata dan UI menampilkan selisih/konfirmasi. **Menambah** `GET /users/{id}` & `GET /permissions` |
| 03 | company-settings | `company` | ✅ | profil & settings (key-value, lebih sederhana dari legacy). Feature flags sengaja dibuang (§6) |
| 04 | branch | `branch` | ✅ | list/detail/create/update + penomoran dokumen per cabang. Catatan kecil di §5.3 |
| 05 | product-catalog | `product` | ✅ **+** | kategori, produk, varian (+`barcode`), media, search-options, impor. **Menambah** unduh template `.xlsx` server-side |
| 06 | business-party | `party` | ✅ | customer & supplier CRUD + archive/restore + alamat kirim |
| 07 | member-pricing | `party` | ✅ | member-types CRUD + restore + `pricing/quote`; ada unit test domain pricing |
| 08 | order-purchasing | `purchasing` | ✅ | PO list/detail/create/update/confirm/cancel. Pajak include/exclude/none — paritas dengan `price_includes_tax` legacy |
| 09 | order-sales | `sales` | ✅ | SO + approve-credit + alamat kirim snapshot/sekali pakai + nomor faktur pajak; **kontrol PPN di form SO (K-03/F-01)**, status bayar + jatuh tempo di daftar dan filter (SL-02), ringkasan bersih-retur (SL-03), ekspor diperkaya (SL-04), `assignedUserId`/"Dibuat Oleh"/kredit disetujui (SL-09). Arsip: dulu "SO lengkap ✅" padahal UI tak bisa PPN. PPN di POS tertutup (K-03 bagian POS) |
| 10 | goods-receipt | `goodsreceipt` | ✅ | list/detail/create; daftar memuat No. PO/jumlah baris/diterima oleh (GR-01); lokasi default + catatan `pay_now` (GR-02). Arsip: kolom daftar dulu selalu kosong |
| 11 | delivery | `delivery` | ✅ | Data SJ lengkap (C1+D3) + antrean kerja (J3); bukti bertipe scan SJ vs foto lokasi (DL-01); filter antrean + prepaid tersembunyi + jumlah item (DL-02); tanggal mutasi = tanggal kirim (DL-03); peringatan SJ draf (DL-04, D2) — §4.3 |
| 12 | sales-return | `salesreturn` | ✅ | Mode tukar, SJ pengganti, context & preview, penyelesaian **dijurnal** (`customer_credit`, K-04a), validasi arah/pihak/sisa (K-04c), pratinjau penyelesaian (K-04d), `pay_now` saat SJ pengganti (K-04e), tukar POS satu-langkah (RT-02), supir/petugas wajib (RT-03), kolom Order + pencarian (RT-01) — §4.4. Arsip: klaim "memo-level ✅" tidak benar untuk jurnal |
| 13 | purchase-return | `purchasereturn` | ✅ | Kamus pembelian `reduce_payable`/`refund`/`supplier_credit` (K-04b; dulu memakai kamus penjualan), `supplier_credit` dijurnal (K-04a), validasi + pratinjau (K-04c/d), lokasi asal bawaan + tanggal retur ≥ terima paling awal (RT-04), kolom Order terisi (RT-01) — §4.4 |
| 14 | payment | `payment` | ✅ **+** | list, detail, party-balances, party-ledger, create, bukti, cancel. **Menambah** `settle-credit`, `use-credit` + `credit-balances` (N-41) dan `open-parties` (daftar semua pihak berpiutang/berhutang, PY-01); pemakaian ganda saldo ditutup (N-40) |
| 15 | pos | kanal `sales` | ✅ | Kredit + jatuh tempo, struk termal + jembatan native, scan kamera (`ScanDialog.tsx`), katalog kategori, panel member, pelanggan cepat (PR-02), toggle diskon sampai ke struk (SL-05), catatan/alamat (SL-07), layar sukses (SL-06). PPN opsional di checkout (K-03 bagian POS) |
| 16 | stock-inventory | `stock` | ✅ | saldo/mutasi/lokasi/transfer/opname/rusak/reservasi/scan; lokasi cabang tujuan (K-06), atribut lokasi + prioritas alokasi (ST-01), sopir/kendaraan/tanggal + cetak transfer (ST-02), estimasi kerugian rusak (ST-03), kartu stok berjalan (ST-04), impor saldo awal (ST-05), ST-06. 2 penyederhanaan alur — §4.7 |
| 17 | finance | `finance` | ✅ | Paritas fungsional **via revisi** (jurnal bertipe + antrean derived + tabel penanda kecil `finance_posting_ignores`/`finance_tax_adjustment_meta`), rincian di §4.2. Arsip: dulu ❌ "6 sub-fitur + gap schema akar" |
| 18 | dashboard | `dashboard` | ✅ | Operasional via contracts-only: stok kritis, prioritas SO+PO, status, terlaris, margin, produk dipantau, status WhatsApp, basis biaya (Tahap F, DB-01); N+1 tren diperbaiki — §4.1 |
| 19 | reporting | `reporting` | ✅ | sales-trend + inventory. N+1 tren sudah ditutup — §5.2 |
| 20 | audit-log | `audit` | ✅ | list (filter `branchId`/`entity`/`entityId`/`action`/`actorId`/`from`/`to`); label aksi/entitas Indonesia di UI (U-02); galat 5xx tak terduga tercatat `error.unexpected` dengan dedupe (OP-06) |
| 21 | assistant/whatsapp | `assistant` | ✅ | 13 endpoint menutup seluruh permukaan `whatsapp/*` + `assistant/*` legacy, plus `chat` & `channel/reset` yang baru; pengenalan cabang dari teks (AS-01), info produk lengkap + tool gagal tercatat (AS-02). Knowledge/RAG sengaja dibuang (§6; menunggu konfirmasi pemilik, D1) |

---

## 4. Analisis Gap per Modul

### 4.1 Modul 18 — Dashboard (P0) — ✅ ditutup (Tahap F, DB-01)

> **Status 2026-10-19:** seluruh isi seksi ini adalah **arsip keadaan awal (2026-09-10)**. Dashboard operasional sudah
> ditutup Tahap F (2026-09-11) dan dilengkapi DB-01: kolom "Ada di revamp?" di bawah ditulis ulang menurut kode sekarang
> (`dashboard/application/{service,operations}.go`, `DashboardPage.tsx`), sehingga §1/§3 dan §4.1 tidak lagi
> bertentangan (koreksi #8, analisis §4.1).

Ini **layar pertama yang dilihat setiap pengguna setiap hari**; keadaan awalnya kehilangan seluruh sisi
operasionalnya.

Bukti: `dashboard.service.ts:44-129` (legacy) mengembalikan 20 field vs
`dashboard/application/service.go:24-83` (revamp) yang mengembalikan 8.

| Field legacy | Ada di revamp? (2026-10-19) |
|---|---|
| `today_sales_amount`, `month_sales_amount` | ✅ (sebagai omzet hari ini / bulan ini) |
| `total_payable`, `total_receivable` | ✅ (saldo buku) |
| `critical_stock_count` + `critical_stock_items` | ✅ `GET /dashboard/critical-stock` + `operations.criticalCount` (Tahap F) |
| `priority_orders` (order mendekati jatuh tempo) | ✅ `GET /dashboard/priority-orders` — SO (skor legacy) + PO lewat jatuh tempo/draf menunggu, dengan `kind`, `reason`, `ageDays` (Tahap F, DB-01) |
| `order_summary` (pending/active/completed/cancelled) | ✅ `GET /dashboard/order-status` (status terkunci di kode; keputusan produk lama, bukan D4 PKP) |
| `top_sold_products_month` (5 terlaris) | ✅ `GET /dashboard/top-sellers` |
| `top_estimated_margin_products_month` + `estimated_margin_status` | ✅ `GET /dashboard/top-margin`; status basis biaya = `operations.costBasis` + `missingCostCount` (DB-01; hambatan §4.2.1 sudah ditutup) |
| `today_order_count`, `month_sales_order_count` | ✅ di ringkasan operasional (Tahap F) |
| `total_purchase_transactions`, `total_sales_transactions` | ✅ di ringkasan operasional (Tahap F) |
| `tracked_product_count` | ✅ `operations.trackedCount`, dirender kartu (DB-01) |
| `sales_trend` (6–12 bulan, konfigurabel) | ✅ `GET /reporting/sales-trend` (`from`/`to`, maks 366 hari); dashboard menampilkan tren harian |
| `whatsapp_status` | ✅ kartu status memakai `GET /assistant/channel` (hanya peran dengan `assistant.view`, DB-01) |
| `knowledge_document_count`, `knowledge_ready_count` | ⏸️ sengaja dibuang (§6) |

Perbandingan halaman (arsip): legacy `dashboard-page.tsx` (628 baris) menampilkan "Order Perlu Aksi",
"Stok Kritis", "5 Barang Paling Laku Bulan Ini", "5 Produk Estimasi Margin Tertinggi", dan
"Pesanan Prioritas". Revamp awal `DashboardPage.tsx` (210 baris): 4 kartu laba-rugi + 4 saldo buku +
tren 30 hari; kini halaman yang sama memuat seluruh panel di atas.

**Perbedaan desain yang lebih dalam dari sekadar field (arsip, sudah diperbaiki).** Dashboard legacy membaca **tabel
operasional**; dashboard revamp awal membaca **hanya modul finance**, sehingga selama dokumen belum di-posting ke jurnal
ia menampilkan nol. Sekarang `dashboard/application/service.go` menerima klien `finance`, `stock`, `sales`, dan
`purchasing` (kontrak `*Ops`/`CriticalSummary`), jadi angka operasional tidak menunggu akuntan menutup buku. Boundary
tetap terjaga — ini persis kegunaan `contracts/`.

### 4.2 Modul 17 — Finance (P0/P1)

#### 4.2.1 Gap akar: dimensi analitik baris jurnal — ✅ **SUDAH DITUTUP** (§10)

Ini dulunya **penyebab** beberapa gap lain. Kondisi awal:

```
legacy (finance-journal-line.entity.ts) — 6 dimensi + metadata:
  id_product  id_product_variant  id_business_party  id_order  id_payment
  id_inventory_movement  metadata_json

revamp SEBELUM migrasi 000019 — tidak ada satupun:
  id  entry_id  account_id  debit  credit
```

Tiga laporan legacy karenanya **mustahil dibuat**, bukan sekadar belum dibuat: `margin` per produk,
`receivables` per pelanggan, `payables` per pemasok — plus widget "5 produk margin tertinggi" di
dashboard.

**Sekarang sudah ada** (`migrations/000019_finance_line_dimensions.up.sql`): `product_id`,
`variant_id`, `party_id`, `description` pada `finance_journal_lines`, dengan dua indeks komposit
yang mengikuti bentuk query laporan. Ketiga laporan sudah berjalan — lihat §10.

> **Kenapa 4 kolom, bukan 7 seperti legacy.** `id_order`, `id_payment`, dan
> `id_inventory_movement` sudah terwakili di level *entry* lewat `source_doc_type` +
> `source_doc_id` — menyalinnya ke tiap baris hanya menduplikasi fakta yang sama. `metadata_json`
> sengaja tidak dibawa: blob yang tidak bisa di-query bukan dimensi. Yang disimpan di baris hanya
> yang benar-benar **bervariasi di dalam satu entry**.
>
> **Jangan tertukar:** `finance/reports/margin` revamp bukan padanan `margin` legacy — ia padanan
> `gross-profit` legacy (agregat, `reports.go:429`). Padanan `margin` legacy adalah endpoint baru
> `finance/reports/margin-by-product`.
>
> ID primitif di baris jurnal **tidak melanggar** aturan modular monolith — aturannya sendiri
> berbunyi *"Cross-module relations are primitive IDs"*. Yang dilarang hanya FK dan join lintas
> modul, dan tidak satupun dipakai di sini.

#### 4.2.2 Tabel yang hilang

Ada di legacy, tidak ada di `000016_finance.up.sql`: `finance_cash_accounts`,
`finance_opening_balances`, `finance_opening_inventory_items`, `finance_posting_sources`,
`finance_tax_adjustments`, `finance_inventory_cost_states`.

*Pembaruan 2026-10-19:* enam tabel itu tetap tidak dibawa (keputusan `DB_SCHEMA.md` §6). Dua tabel **kecil pengganti**
ditambahkan migrasi `000031_finance_fn` (FN-01/FN-07/FN-10): `finance_posting_ignores` (penanda dokumen yang diabaikan
dari antrean, ganti `finance_posting_sources.ignored`) dan `finance_tax_adjustment_meta` (klasifikasi + catatan koreksi
fiskal, 1:1 ke jurnal); ditambah kolom `business_expenses.party_id`. Total tabel kini 65.

#### 4.2.3 Sub-fitur yang hilang

| Prioritas | Gap | Status (finalisasi 2026-09-11; dikoreksi 2026-10-19) |
|---|---|---|
| **P0** | **Saldo awal (opening balance)** — tanpa ini pembukuan tidak bisa dimulai dari data berjalan | ✅ via revisi: satu jurnal cutover per branch (`finance/application/opening.go` — `OpeningStatus`/`PreviewOpening`/`PostOpening`, idempoten). Endpoint `GET /finance/opening`, `POST .../opening/preview` (= validate, menolak debit ≠ kredit, P4), `POST .../opening/post`. **K-05 (2026-10-18):** nilai Persediaan dihitung server dari ledger biaya (`GET /finance/opening/inventory`), baris Persediaan tidak bisa diketik, jalur `POST .../opening/complete` memberi harga modal posisi berstok bernilai nol setelah terkunci, dan rekonsiliasi akun Persediaan buku vs ledger masuk `integrity`/`close/readiness` (`inventory_recon`, `missing_cost`). Varian `update/import-inventory/supplement` tidak dibawa by-design (tertutup jurnal manual + reverse + `opening/complete` + impor saldo awal stok). Arsip: dulu dianggap "bukan penghambat cutover" — keliru, saldo awal tanpa hitung persediaan adalah risiko akurasi cutover (analisis F-05) |
| **P0** | **Antrian posting** — akuntan harus tahu SJ mana yang belum dijurnal | ✅ derived live (`finance/application/queue.go`, P3): `GET /finance/posting-sources`, `POST .../posting/post-batch` (maks 50), `POST .../posting/close-day`; UI antrean + tutup harian di `PostingPage`. **FN-01:** `POST .../posting/ignore` (alasan wajib) dan `.../posting/restore` memakai tabel penanda `finance_posting_ignores` — dokumen macet tidak lagi memblokir safe-close selamanya (peringatan `ignored_docs`). **FN-02/FN-10:** `GET .../posting/overview` (ikhtisar tutup hari + tugas "butuh data pemilik" + backlog) dan nominal/pihak di antrean. Varian `detail` tidak dibawa (= pratinjau); `cancel-posting` = reverse jurnal. Arsip: klaim "abaikan/pulihkan tidak bermakna tanpa antrean tersimpan" keliru |
| **P1** | **Akun kas** — master rekening kas/bank terpisah dari COA | ✅ flag `is_cash` di COA + `GET /finance/reports/cash-summary`; tabel terpisah tidak dibawa by-design (`DB_SCHEMA.md` §6 *Diputuskan*) |
| **P1** | **Laporan piutang & utang dari buku** | ✅ sudah ditutup Tahap 1 (§10) — `reports/receivables`, `reports/payables` |
| **P1** | **Margin per produk** | ✅ sudah ditutup Tahap 1 (§10) — `reports/margin-by-product` |
| **P1** | **Penyesuaian pajak** | ✅ via revisi: jurnal bertipe `tax_adjustment` (`taxadjust.go`) — `GET/POST /finance/tax-adjustments`; **FN-07:** klasifikasi wajib (positif/negatif/penghasilan final/non-objek) + catatan (`finance_tax_adjustment_meta`) dan lampiran gambar (`.../{id}/attachments`). Alur draft/final dan arsip **tidak dibawa** (diterima gap, ADR-0018 D6; detail = detail jurnal, koreksi = reverse). Arsip: klaim "tax-adjustments ✅" tanpa lampiran/klasifikasi |
| **P1** | **Tutup periode aman + ekspor paket pajak** | ✅ `GET /finance/close/readiness` + `POST .../close/safe-close` (`readiness.go`); **FN-05:** alasan wajib saat buka kembali periode fiskal/pajak, tersimpan di diff audit. `GET /finance/export/tax-package` + tombol unduh; **FN-04:** paket memuat PPN Masukan (pembelian + retur beli), baris retur jual, Untung Rugi per akun, Buku Besar semua akun (maks 60.000 baris) dan rentang tanggal bebas; **FN-06:** ringkasan PPN memuat DPP + daftar transaksi kena pajak tanpa faktur. Arsip: klaim "tax-package ✅" tanpa PPN Masukan/buku besar/retur |
| ✅ Selesai | **Nomor faktur pajak** (Tahap C) — `sales_orders.tax_invoice_number/date` + `purchase_orders.supplier_invoice_number/date` tersimpan, masuk `TaxDetail` (JSON + CSV) | — |

> **Antrian posting — ✅ tertutup finalisasi 2026-09-11.** `PostingPage` kini membuka dengan
> daftar dokumen terkonfirmasi-tapi-belum-dijurnal (pilihan massal + tutup harian); input ID
> manual dipertahankan sebagai jalur darurat. Catatan arsip: sebelum revisi, halaman itu *hanya*
> punya `TextInput` ID manual (`PostingPage.tsx`) sehingga akuntan tidak punya cara mengetahui
> SJ mana yang belum masuk jurnal — itu P0 fungsional yang kini selesai.
>
> Sisi baiknya, mesin posting revamp **lebih baik** dari legacy: `Post()` idempoten (posting ulang
> dokumen yang sama mengembalikan entry lama alih-alih dobel-buku) dan `Preview()` memakai builder
> yang sama persis dengan `Post()`, jadi pratinjau tidak mungkin meleset (`posting.go:56-95`).
> Yang perlu ditambah hanyalah **daftar sumbernya**, bukan mesinnya.
>
> Paket ekspor pajak versi "dibatasi Rp4,8 M" semula **keputusan bisnis terbuka** (PRD §5 no. 7); pemilik memutuskan
> membawanya 2026-09-11 dengan mekanisme yang dirancang ulang (§12). Isi paket kemudian dilengkapi FN-04 (PPN Masukan,
> retur, Buku Besar, rentang bebas).

### 4.3 Modul 11 — Delivery (P0) — ✅ ditutup

*Pembaruan 2026-10-19:* bukti kirim kini bertipe — scan/foto SJ fisik bertanda tangan (wajib untuk konfirmasi bila setting
`delivery_require_evidence` aktif) vs foto lokasi (opsional), lewat owner type media `delivery_document`/`delivery_proof`
tanpa kolom baru (DL-01); antrean kerja memfilter tanggal di kedua tab, mencari sebelum `LIMIT`, menyembunyikan order prepaid
belum lunas, dan menampilkan jumlah item (DL-02); tanggal mutasi stok mengikuti tanggal kirim di dokumen (DL-03); membuat SJ
draf memberi peringatan stok lunak sementara reservasi pada draf **tidak dibangun** (DL-04, ADR-0018 D2 — diterima gap).

Kolom dokumen (Tahap C, selesai): `delivery_notes` kini membawa `driver_name`,
`vehicle_plate`, `warehouse_staff_name`, `recipient_name`,
`recipient_signature_status` (+ alasan bila tanpa tanda tangan),
`drop_location_note`, dan stempel `dispatched_at/by` + `confirmed_at/by`.
Alur 2-status revamp menggabung dispatch dan serah-terima di konfirmasi, jadi
kedua stempel terisi bersamaan — kolom terpisah disiapkan untuk alur 3-status
dan untuk mendaratkan data legacy apa adanya.

Yang masih hilang dari seksi ini (finishing 2026-09-11): tidak ada — tersisa
`deliveries/work-queue` ditutup J3 (`GET /deliveries/work-queue` + halaman Antrian Kirim dua tab).

| Kolom legacy | Status |
|---|---|
| `document_kind`, `id_sales_return` | ✅ Tahap D (SJ pengganti untuk retur tukar) |
| Alamat kirim di order | ✅ Tahap C (`ship_to_*` snapshot di `sales_orders`) |
| `deliveries/work-queue` | ✅ Tahap J3 (finishing: endpoint + UI, tanpa audit seperti legacy NF-06) |

### 4.4 Modul 12 & 13 — Retur (P0/P1) — ✅ ditutup (Tahap D, K-04, RT-01…RT-04, N-40/N-41)

> **Status 2026-10-19:** tabel di bawah sudah memuat penutupan Tahap D **dan** koreksi analisis 2026-10-16: klaim lama
> "4 jenis penyelesaian ✅ memo-level di **kedua** modul retur" ternyata salah — retur pembelian memakai kamus penjualan dan
> saldo kredit tidak dijurnal. Perbaikannya: K-04a (jurnal reklasifikasi), K-04b (kamus pembelian), K-04c (validasi arah/pihak/sisa),
> K-04d (pratinjau penyelesaian), K-04e (`pay_now` saat SJ pengganti), N-40 (pemakaian ganda), N-41 (pakai saldo uang muka).

**Retur penjualan revamp awal adalah retur-saja (arsip).** Bukti: `000014_salesreturn.up.sql` hanya punya
`sales_returns` + `sales_return_items`. Legacy punya `sales_return_settlements` dan kolom
`return_mode`, `replacement_delivery_status` di `sales_returns`.

| Kapabilitas legacy | Revamp |
|---|---|
| Retur biasa (barang masuk, nilai dihitung) | ✅ ada |
| **Mode tukar (`exchange`)** — baris pengganti dalam satu dokumen retur | ✅ ada (Tahap D; harga pengganti dari katalog saat buat) |
| **Surat jalan pengganti** (`replacement-deliveries` + `confirm`) untuk tukar berbasis order | ✅ ada — stok keluar saat konfirmasi SJ pengganti, tolak-sebagian-tidak-pernah, status ganda ditolak via status |
| **4 jenis penyelesaian**: `collect_payment`, `reduce_receivable`, `refund`, `customer_credit` | ✅ **Retur jual:** jenis & nominal menurut arah selisih (K-04c); `customer_credit` **dijurnal** Dr Piutang/Cr Uang Muka Pelanggan (K-04a); `reduce_receivable` otomatis (ditolak manual); tambah-bayar tukar bisa `pay_now` saat SJ pengganti (K-04e). **Retur beli:** kamus pembelian `reduce_payable`/`refund`/`supplier_credit` (K-04b — dulu memakai kamus penjualan, klaim lama salah); `supplier_credit` dijurnal Dr Uang Muka Supplier/Cr Hutang. Saldo yang sudah dikreditkan tak bisa direfund lagi (N-40) dan bisa dipakai ke tagihan lewat `payments/use-credit` (N-41). Arah debit/kredit: default legacy, **menunggu konfirmasi akuntan** (PLAN §7) |
| Guard harga modal untuk barang pengganti (`assertReplacementCostBasis`) | ✅ ada — rata-rata biaya finance dulu, lalu harga beli katalog; ditolak sebelum stok bergerak (RT-04, N-42: costing dikuras dulu); SJ pengganti dijurnal lewat builder pengganti finance (antrean posting) |
| Tukar di kanal POS | ✅ satu langkah: konfirmasi retur menyerahkan barang pengganti di konter (SJ pengganti dibuat + dikonfirmasi otomatis); order reguler tetap SJ manual dengan supir/petugas gudang wajib (RT-02, RT-03) |
| `context` (daftar item yang masih boleh diretur + sisa qty + lokasi) & `preview` (dampak sebelum simpan) | ✅ ada di **kedua** modul retur — preview memakai builder yang sama dengan create |

Ukuran (arsip 2026-09-10, sebelum Gelombang 1–2): legacy `sales-return.service.ts` 1.400 baris → revamp `salesreturn` 403 baris.
(`purchasereturn` revamp 926 baris vs legacy 867 — di sisi ini justru setara.) Angka itu kini jauh lebih besar dan tidak lagi dipakai sebagai ukuran paritas.

**Perhitungan uang (D1, selesai Tahap B).** Nilai retur kini mengikuti nilai asli:
diskon proporsional ikut, PPN ikut dibalik (`subtotal`/`discount_total`/`tax_total` di header,
per-baris `tax_base`/`tax_amount`; jurnal membalik PPN lewat kaki 2200/1400 baru). Aturan
prorata nominal dan penolakan baris tak-cocok dijaga test integrasi.

### 4.5 Modul 15 — POS (P0) — ✅ ditutup (Tahap I, Gelombang 2)

| Kapabilitas legacy | Revamp (finalisasi 2026-09-11; dikoreksi 2026-10-19) |
|---|---|
| Keranjang, tambah/kurang qty, hapus | ✅ |
| Bayar tunai + kembalian, transfer | ✅ |
| **Bayar Nanti (Piutang) + jatuh tempo** | ✅ `checkout.ts` + `pos-checkout.schema.ts` (mode `pay_later` + `dueDate`, termin net) |
| **Struk termal** + jembatan printer Android | ✅ `PosThermalReceiptPage` + `native-printer.ts` (fallback `window.print`) |
| **Scan kamera** QR/barcode | ✅ `ScanDialog.tsx` + `camera.ts` (`@zxing/browser`); bunyi sukses pindai (SL-08). Arsip: nama `PosScannerModal.tsx` tidak ada di kode saat ini |
| **Katalog per kategori (drilldown)** | ✅ `PosCategoryDrilldown.tsx` |
| Panel pelanggan + ringkasan member | ✅ `PosCustomerPanel.tsx` |
| Toggle "Sembunyikan diskon (harga penuh)" | ✅ ada di POS **dan sejak SL-05 ikut ke struk termal & faktur** (`?hideDiscount=1`). Arsip: klaim lama ✅ padahal hanya pratinjau |
| Tambah pelanggan cepat, catatan transaksi, alamat kirim opt-in, layar sukses (Cetak Nota/Bukti Bayar) | ✅ PR-02 (`CustomerSheet`), SL-07, SL-06 |
| Kontrol PPN di POS | ✅ opsional di checkout POS (K-03 bagian POS; `TestPOSOrderVATFlowReachesTaxSummary`, `CheckoutSheet.test.tsx`, e2e E02 varian PPN). Arsip: semula gap yang diterima |

Legacy `pos-page.tsx` 1.247 baris + 10 komponen pendukung; revamp `PosPage.tsx` ±50 baris (rangka halaman) dengan logika di
`hooks/use-pos-cart.ts` dan komponen `CartPanel`, `CartLineRow`, `CheckoutSheet`, `CustomerSheet`, `PosCustomerPanel`,
`ProductGrid`, `PosCategoryDrilldown`, `VariantSheet`, `ScanDialog`, `PosTopbar` (arsip: angka "744 baris tanpa komponen pendukung" tidak akurat).

### 4.6 Cetak, Ekspor, QR (P0/P1)

**Cetak — ✅ halaman & rute terpasang 2026-09-11 (Tahap E).** Klaim "nol" di
bawah ini kedaluwarsa dan dipertahankan sebagai arsip verifikasi kebutuhan:

| Dokumen | Halaman legacy |
|---|---|
| Surat jalan | `orders/pages/delivery-note-print-page.tsx` |
| Faktur / dokumen penjualan | `orders/pages/sales-document-print-page.tsx` |
| Kwitansi pembayaran | `orders/pages/payment-receipt-print-page.tsx` |
| Struk termal POS | `orders/pages/pos-thermal-receipt-print-page.tsx` |
| Jembatan printer Android | `lib/native-printer.ts` + `android-pos-shell/` |

**Ekspor — ✅ selesai penuh (J1, finalisasi 2026-09-11).** Daftar order jual & beli diunduh
`.xlsx` (streaming writer `excelize`, paging 500 baris — P3) dan `.pdf` (`fpdf`) dari server,
dengan filter list yang aktif: `GET /api/v1/sales-orders/export`,
`GET /api/v1/purchase-orders/export` + tombol "Ekspor XLSX/PDF" di kedua halaman daftar.
**Dikoreksi 2026-10-19:** klaim "selesai penuh, sisa nihil" benar untuk mekanisme tetapi isinya semula jauh lebih tipis dari legacy
(SO satu sheet 12 kolom). Kini SO xlsx 3 sheet (Ringkasan, Daftar Order 28 kolom, Detail Item; margin hanya sesi `finance.view`
dengan `from`+`to`; SL-04) dan PO xlsx 3 sheet (24 kolom; PO-02), keduanya streaming. PDF tetap 12 kolom. Uji e2e unduh via UI
dan baca isi berkas kini ada (e2e E14, T-05).
Tetap lebih baik dari legacy yang merakit workbook di browser (`xlsx-js-style`). Yang tersisa
dari seksi ini hanya nihil — QR selesai sesi finishing (lihat di bawah).

**QR — ✅ selesai penuh (J2, finishing 2026-09-11).** `GET /products/{id}/qr` bervarian
(`?variantId`, `?size`) + `GET /products/qr-codes/export` (ZIP streaming per folder kategori,
1 query katalog — P3) + modal QR (pratinjau per varian + unduh PNG + tautan cetak) + tombol
"Unduh QR (ZIP)" + halaman cetak label `/print/labels?ids=` (grid QR+nama+kode, D5
browser-print). Payload = barcode → kode mentah, selaras jalur scan (`scan-product`/POS
mencocokkan barcode persis; kode mentah lolos via pencarian — legacy F-08.2). Deliberasi
tercatat: layout label dirender browser agar bisa dikalibrasi (bukan PNG baked 600×800 legacy
F-08.3); jasa dikecualikan dari ekspor massal.

Scan kamera QR/barcode ✅ tertutup via `ScanDialog` (Tahap I).

> Dependensi `github.com/skip2/go-qrcode` **sudah ada** di `go.mod`, tapi hanya dipakai untuk
> pairing WhatsApp (`assistant/infrastructure/gateway.go:14`). Generator label QR sisi server
> tinggal dipakai ulang.
>
> Legacy sudah membuktikan pola **ekspor/cetak di server** (pdfkit). Untuk revamp, jalur itu lebih
> baik daripada memport 4 halaman cetak browser: satu sumber kebenaran, bisa diuji, dan tidak
> tergantung pengaturan cetak tiap peramban.

### 4.7 Modul 16 — Stock: dua penyederhanaan alur (P2) — celah analisis 2026-10-16 ditutup

Klaim lama "stok termasuk yang paling sehat, cakupan endpoint setara" tidak mencakup celah yang kemudian ditemukan analisis
2026-10-16. Semuanya kini ditutup: lokasi di cabang tujuan transfer (`GET /stock/locations?branchId=`, K-06/F-06); atribut lokasi
area picking/utama/urutan + kode lokasi dapat diubah + prioritas alokasi picking → utama → default → qty terbesar (ST-01/F-07);
sopir/kendaraan/tanggal transfer + cetak Surat Transfer Barang (ST-02); estimasi kerugian + paginasi stok rusak (ST-03); kartu
stok dengan saldo sebelum/sesudah, catatan, tipe, filter tipe (ST-04); impor saldo awal dengan alias sheet/header Indonesia,
preview rinci dan klaim `.xls` dihapus (ST-05); move-in rusak tanpa lokasi asal, label dual-UOM, filter arah transfer,
scan/pencarian di halaman saldo (ST-06). Dua catatan penyederhanaan alur:

1. **Pindah lokasi dalam satu gudang — ✅ selesai (J4, finishing 2026-09-11).**
   `POST /stock/transfers/move-location` + modal "Pindah Lokasi": pekerjaan 10 detik kembali
   satu layar, tetapi tetap lewat dokumen transfer (draf → kirim → terima berurutan) sehingga
   nomor, pasangan movement, dan jejak audit utuh. Stok kurang = seluruh pindah batal, draf
   tertinggal untuk ditinjau. Revamp tetap mendukung multi-item — legacy tidak.
2. **Halaman & endpoint detail item stok** — ✅ **ditutup 2026-09-11 (J5)**:
   `GET /api/v1/stock/balances/{productId}` (alias path-style di atas
   `BalancesByProduct` yang sama) + halaman `StockItemPage` (total + saldo per
   lokasi + 10 mutasi terakhir), ditautkan dari seksi Saldo `StockPage`.

### 4.8 Gap operasional lain (P2)

| Gap | Legacy |
|---|---|
| Komponen bersama (arsip: belum diport) | `async-search-select`, `hierarchical-select`, `password-input`, `field-hint`, `select-field`, `global-loader`, `party-search-select`, `product-search-select`, `stock-location-select` — ✅ **sudah diport** (§11.2 B2; kini di `shared/components/ui/` dan varian domain `PartySearchSelect`/`ProductSearchSelect`/`StockLocationSelect` di modulnya). Yang masih tanpa test render: `PasswordInput` dan address picker (T-03) |

(Riwayat/konfigurasi status order dan PWA sengaja dibuang — keputusan produk lama, D4 pada PLAN `menuju-pengganti-production` (bukan D4 PKP ADR-0018), lihat §6.)

---

## 5. Temuan Lintas Modul

### 5.1 Jaring pengaman uji — regresi berat (P1)

Revamp mengejar *maintainability*, tetapi justru di sinilah ia paling jauh mundur:

| | Legacy | Revamp (pasca Scope A/B, 2026-09-11) |
|---|---|---|
| Test backend | **54 file, 872 `it()`** (analisis 2026-10-16: 57 spec/919 `it`) | Dihitung ulang dari kode 2026-10-19 (akhir DOC-01): **176 berkas `*_test.go`, 641 `func Test`** — 109 berkas / 370 test di `internal/integration` (MySQL nyata, `mini_erp_test`) mencakup jalur uang, konkurensi, sesi, route-guard. (Arsip: 2026-10-10 128 file/434 Test/224 integrasi; 2026-09-11 31 file/73 Test/35 integrasi.) |
| Test frontend | **53 file, 382 `it()`** | Per 2026-10-19: **163 berkas** `*.test.ts(x)` Vitest (hitungan vitest sebelumnya: 155 berkas, **838 test**) (`vitest run` 2026-10-19: 833 lulus pada lari penuh paralel; 5 timeout 5 dtk akibat beban mesin lulus saat dijalankan terpisah) (arsip: 2026-10-10 77 berkas; 2026-09-11 17 file/85 test) |
| E2E | **27 spec Playwright** (`apps/e2e`) | **16 berkas spec** Playwright di `apps/e2e/specs` — E01–E11 (auth/akses, POS, rantai penjualan, pembelian, retur, stok, impor, cetak, tutup buku, smoke menu, ketahanan operasional) + E12 matriks akses role + E13 volume/relasi + E14 ekspor via UI + V00 visual + V01 a11y; gerbang Gelombang 2 (2026-10-19, sebelum E12–E14): 52 lulus, 2 dilewati |
| CI | — | `.github/workflows/ci.yml`: build, vet, gofmt, `go test` (termasuk integrasi MySQL 8.4), tsc, eslint, vitest, build web, arch-check, contract-check |

**Alur yang diuji legacy dan kini sudah tercakup (PLAN §8, T-01…T-05):** matriks akses role (`TestRoleSeedMatrix` + e2e E12), gateway/otorisasi nomor
WhatsApp (`gateway_test.go`, `assistant_authorization_test.go`; N-43/N-44), komponen form besar (`SalesOrderFormPage*`, `ProductFormPage*`, `RolesPage`, `UsersPage*`, `SetupPage*`, `SalesReturnsPage`, `PosCustomerPanel`, `AddressFormModal`, `ProductDetailPage` foto),
skenario volume + integritas relasional (e2e E13), ekspor laporan order via UI (e2e E14). Sejak 2026-10-16
sudah ditambah: pemulihan chunk-load + Error Boundary (OP-01; unit + e2e E11), kompres gambar (OP-03), audit 5xx (OP-06).

Yang sudah diuji di revamp adalah bagian yang paling rawan: `money`,
`pagination`, `timeutil`, `auth/session`, `assistant/engine`, pricing domain di `party`,
`product`, `purchasing`, `sales`, **jalur uang end-to-end** (`finance.Post` idempoten +
tolak pincang, `delivery.Confirm` + stok + penyelesaian SO, `payment.Create` + saldo —
`internal/integration`, Tahap A), serta matematika klien (format IDR, estimasi baris
PO/SO, keranjang & kembalian POS).

### 5.2 Efisiensi: N+1 — ✅ **ditutup (agregasi + dekorasi)**

**Tren penjualan — selesai.** Dulu `reporting` memanggil `finance.NetProfit()` sekali per hari
(tren setahun = 366 query berurutan). Sekarang `finance/application/trend.go` → `DailyTrend()`
memakai **3 query konstan** apa pun panjang rentangnya (ListAccounts + MappingAccount(cogs) +
satu `GROUP BY DATE(entry_date), account_id`), dan semantiknya sengaja dicerminkan persis dari
`profitLoss` supaya angkanya tidak mungkin bergeser. Tabel agregat `daily_operational_metrics`
legacy tetap tidak ditiru — tidak perlu.

**Resolusi nama lintas modul — selesai (Scope A3).** `sales.ListOrders` dan
`purchasing.ListOrders` dulu memanggil `parties.GetByID` **per baris**; keduanya kini memakai
`PartyClient.NamesByIDs(ctx, ids)` — satu pembacaan berkelompok. `payment.appendReturnBalances`
yang dulu memanggil `sales.GetByID`/`purchasing.GetByID` per baris retur kini memakai
`OrderParties(ctx, orderIDs)` + `PaidForOrders(ctx, type, ids)`.

Dijaga `internal/integration/batch_reads_test.go` (`TestBatchResolutionPaths`,
`TestPurchaseListOrdersBatchNames`).

> Metode batch ini **menambah** `contracts/`, bukan menembusnya — persis kegunaan `contracts/`.
> Nol join lintas modul, arch-check tetap lolos.

**Dekorasi laporan — selesai (2026-09-11).** Dua paragraf di atas dulu ditulis sebagai "seluruhnya
ditutup", padahal yang tertutup baru **agregasinya**. Dekorasinya — mengubah baris hasil agregat
menjadi nama yang dibaca manusia — masih satu query per baris:

| Sebelum | Sesudah |
|---|---|
| `MarginByProduct` → `products.GetByID` per produk | `ProductClient.LabelsByIDs` — satu pembacaan |
| `Receivables`/`Payables` → `parties.GetByID` per party | `PartyClient.LabelsByIDs` — satu pembacaan |
| `InventoryValue` → `products.GetByID` per posisi stok | `ProductClient.LabelsByIDs` — satu pembacaan |
| `Uncosted` → `products.GetByID` per pasangan produk-varian | idem |

`ProductClient` sebelumnya **tidak punya** metode batch sama sekali (hanya `GetByID`), jadi N+1 di
sini bukan kelalaian pemanggil melainkan bentuk kontraknya. `finance/application/decorate.go`
sekarang menjadi satu-satunya tempat dekorasi laporan, dan sifat best-effort-nya dipertahankan:
nama yang gagal dimuat tidak boleh menggagalkan laporan uang.

**Buku besar & rincian pajak — selesai (2026-09-11).** `GeneralLedger` memuat hingga 10.000 entry
lalu memanggil `GetEntry` per entry — dan `GetEntry` sendiri 2 query (header + baris), jadi satu
laporan bisa menembus ~20.000 round trip untuk data yang satu JOIN sudah punya. Kini
`repo.LinesByEntryIDs(ctx, ids)` mengambil seluruh baris sekali (di-chunk `footingChunk`), dipakai
juga oleh rincian pajak dan `ListTaxAdjustments` yang memakai pola sama. Biaya laporan turun dari
`1 + 2N` query menjadi **2**.

### 5.3 Pemilih cabang — ✅ **diperbaiki 2026-09-11 (J6)**

`GET /api/v1/branches/my-access` (login saja — tanpa `branches.view`, tanpa
guard cabang, hanya cabang aktif + penanda default, sesuai F-05 legacy)
menutup gap ini. `use-accessible-branches.ts` kini memanggilnya;

Dampak sebelumnya (kini tertutup): saat **login** daftar cabang datang dari respons login
(`pendingBranches`) sehingga benar. Yang bermasalah adalah **ganti cabang setelah login** — pengguna
tanpa `branches.view` (mis. kasir) mendapat daftar kosong lalu dialihkan ke dashboard, jadi tidak
bisa pindah cabang sama sekali. Keamanannya sendiri aman: `SwitchBranch` memvalidasi akses dengan
benar (`auth/application/service.go:424-445`).

### 5.4 Yang revamp lakukan **lebih baik** dari legacy

Supaya audit ini jujur dua arah — berikut peningkatan nyata, bukan sekadar perbedaan gaya:

- **sqlc: migrasi bertahap (ADR-0012), 16 dari 18 modul bertabel `migrated`.** `scripts/sqlc-migration-status.json` adalah ratchet (arch-check Rule C/R melarang `QueryContext`/`ExecContext` mentah di modul `migrated` kecuali yang terdaftar di `scripts/raw-sql-allow.json`); `pending` tinggal `finance` dan `stock`, `dashboard` dan `reporting` tanpa SQL. Arsip: teks lama "terkonfigurasi, belum dipakai (0 modul)" sudah tidak berlaku, dan klaim yang lebih lama "sqlc di 19 dari 20 modul" juga salah.
- **Uang sebagai integer rupiah** (ADR-0005) menggantikan `decimal(18,2)` legacy — satu kelas
  kesalahan pembulatan hilang, dan sudah ada unit test-nya.
- **Posting idempoten + preview yang tidak mungkin meleset** (§4.2.3).
- **Batas modul ditegakkan** — tidak ada join lintas modul; legacy menjoin `orders` ↔ `products` ↔
  `finance_*` bebas dalam satu query.
- **Ekspor xlsx di server** (§4.6), menggantikan perakitan workbook di browser.
- **Costing pindah ke finance** — legacy menyimpan `cogs_amount_snapshot`, `margin_amount_snapshot`
  di `order_items` (domain penjualan menyimpan angka akuntansi); revamp memusatkannya di
  `finance_inventory_cost_movements`.
- **Jalur cancel yang lengkap dan berizin** di modul retur (KI-99/KI-101), yang di legacy tidak ada.
- **`payments/settle-credit`** — tidak ada padanannya di legacy.
- **Varian produk punya `barcode` + unique key** — dasar scan POS yang lebih rapi.

---

## 6. Yang Sengaja **Tidak** Dibawa (bukan gap)

| Tidak dibawa | Alasan & rujukan |
|---|---|
| **Knowledge base / RAG** (`knowledge/documents/*`, 6 endpoint) | Keputusan produk, bukan komentar kode: sumbernya [PRD.md](PRD.md) (header "skop L9: tanpa Knowledge/RAG/AI" dan catatan Modul 21 "fase lanjut") dan dicatat sebagai D1 di [ADR-0018](../knowledge/decisions/ADR-0018-keputusan-paritas.md) (default dipakai, menunggu konfirmasi pemilik; alternatif ringan: retrieval kata kunci tanpa AI sebagai plan terpisah). Bot WA-nya sendiri **dibawa** |
| Multi-tenant / `companyId` | [ADR-0009](../knowledge/decisions/ADR-0009-single-tenant-takeout-multitenancy.md) |
| Feature flags perusahaan (`company/features/*`) | PRD §5 no. 2 — mati di 3 lapisan di sistem lama |
| Modul `order` tunggal | [ADR-0004](../knowledge/decisions/ADR-0004-order-domain-split.md) — dipecah `purchasing` + `sales` |
| Modul POS tersendiri | [ADR-0007](../knowledge/decisions/ADR-0007-pos-no-dedicated-module.md) — POS = kanal frontend |
| Localization selain id-ID/IDR/WIB | [ADR-0006](../knowledge/decisions/ADR-0006-locale-timezone-locked.md) |
| Status order konfigurable + riwayat (`order-status-definitions/*`, `order-status-transitions/*`, `orders/status-history`, 7 endpoint) | Keputusan produk lama (D4 pada PLAN `menuju-pengganti-production`, bukan D4 PKP di ADR-0018) — tetap dibuang (Tahap A); status terkunci di kode (ini juga yang membuat `order_summary` dashboard mustahil — §4.1), riwayat lama diarsip CSV saat cutover (K8) |
| Tabel `daily_operational_metrics` + cron agregasi | Bukan keputusan tercatat, tapi wajar — lihat §5.2 untuk penggantinya |
| Master `finance_cash_accounts` terpisah | Finalisasi 2026-09-11 — diganti flag `is_cash` di COA (`DB_SCHEMA.md` §6) |
| Tabel `finance_opening_*`, `finance_posting_sources`, `finance_tax_adjustments` | Finalisasi 2026-09-11 — diganti jurnal bertipe + antrean derived (§4.2.3, PLAN Tahap G/H). Diperbarui 2026-10-19: abaikan/pulihkan kini ada lewat tabel penanda kecil `finance_posting_ignores` (FN-01) dan klasifikasi koreksi fiskal lewat `finance_tax_adjustment_meta` (FN-07), tanpa mengembalikan tabel antrean/penyesuaian legacy |
| Varian `posting-sources/{detail,cancel-posting}`, `tax-adjustments/{detail,archive}`, `opening/{update,import-inventory,supplement}` | Finalisasi 2026-09-11 — tertutup pratinjau + reverse jurnal + jurnal manual/`opening/complete` (§9). (`posting-sources/{ignore,restore}` dulu masuk daftar ini; ditutup FN-01.) Draft/final + arsip penyesuaian pajak = diterima gap (ADR-0018 D6) |
| `orders/update-status` arbitrer | Status terkunci di kode (keputusan produk lama, D4 PLAN `menuju-pengganti-production`); transisi via confirm/cancel/approve-credit (§9). `deliver-goods` legacy diganti alur dua-status idempoten (ADR-0018 D2) |

---

## 7. Paritas Desain UI — ✅ Terpenuhi

Klaim ini **diuji ulang dan benar**. Token desain diport identik, bukan ditiru:

```bash
diff <(grep -oP "^\s+--(color|font|radius)[a-z-]*:\s*\K[^;]+" apps/web/src/theme/theme.css) \
     <(grep -oP "^\s+--(color|font|radius)[a-z-]*:\s*\K[^;]+" ../mini-erp/apps/web/src/tailwind.css)
# → nol selisih
```

- Palet: `ink #0f172a`, `heading #334155`, `muted #64748b`, `hairline #cbd5e1`, `brand #1e3a5f`,
  `brand-hover #16314f`, `accent #c2603a`, `ok #15803d`, `warn #b45309`, `bad #b91c1c`,
  sidebar `#0f172a` — **identik**.
- Tipografi & radius: `Inter` + `--radius-{sm,md,lg}` = 6/8/10px — **identik**.
- Variabel kompat `:root` (`--bg-canvas`, `--shadow-*`, `--topbar-height: 64px`) — **identik**.

Komponen inti sudah punya padanan (button, badge, data-table, pagination, modal, confirm-dialog,
toast, notice, empty-state, page-header, section-card, summary-card, filter-bar, action-row,
form-field, search-select, segmented-control). Komponen bersama lain sudah diport (§4.8, §11.2 B2).

> **Catatan navigasi** (konsekuensi ADR-0004, bukan regresi): `/orders` tunggal → `/purchase-orders`
> + `/sales-orders`; `/gudang/*` → `/stock/*` dengan Saldo/Mutasi/Lokasi digabung dalam satu
> `StockPage`.
>
> **Hitung halaman nyata, jangan hitung rute.** Dari 10 rute `/assistant/*`, `/knowledge/*`,
> `/whatsapp` di registry legacy, **8 hanya `<Navigate>`**; halaman nyatanya dua, dan revamp punya
> persis dua padanannya.

---

## 8. Rencana Penutupan Gap (urutan usulan)

Diurutkan berdasarkan dependensi teknis lebih dulu, lalu "tanpa ini sistem tidak bisa dipakai".

| Tahap | Isi | Alasan urutan |
|---|---|---|
| **1** | ✅ *selesai* — **Dimensi analitik di `finance_journal_lines`** (§4.2.1, §10) | Membuka margin per produk, piutang/utang dari buku, dan buku besar per baris. Tiga bug posting ikut ketahuan & diperbaiki |
| **2** | **Kolom surat jalan + alamat kirim di order** (§4.3) | Prasyarat data untuk tahap 4. Membuat halaman cetak sebelum ini = mencetak field kosong |
| **3** | **Vitest di `apps/web` + test service backend** (§5.1) | Dipasang **sebelum** finance & retur disentuh berat, bukan sesudah |
| **4** | **Cetak dokumen** (§4.6): surat jalan → faktur → kwitansi → struk termal POS | P0 lapangan. Pertimbangkan render di server (pola pdfkit legacy) alih-alih 4 halaman cetak browser |
| **5** | **Dashboard operasional** (§4.1) — stok kritis, pesanan prioritas, terlaris, hitungan order | P0 harian, murah, tidak butuh migrasi (kecuali widget margin yang menunggu tahap 1) |
| **6** | **Finance P0**: saldo awal + **daftar** dokumen belum-posting (§4.2.3) | ✅ selesai via revisi tanpa tabel baru (finalisasi 2026-09-11) |
| **7** | **POS**: bayar-nanti/kredit → struk → scan kamera → katalog kategori (§4.5) | Bayar-nanti lebih dulu: itu kapabilitas, sisanya kenyamanan |
| **8** | **Retur**: mode tukar + penyelesaian + `context`/`preview` (§4.4). **Konfirmasi dulu** kebijakan diskon-diabaikan | Butuh migrasi + keputusan bisnis |
| **9** | **Finance P1**: akun kas, piutang/utang, penyesuaian pajak, nomor faktur pajak, tutup periode aman | ✅ selesai via revisi (finalisasi 2026-09-11); piutang/utang & margin sejak Tahap 1 |
| **10** | Ekspor daftar order (xlsx+pdf) ✅, QR scan ✅ + label/ZIP/cetak ✅, antrian kerja ✅, pindah-lokasi 1 langkah ✅, detail item stok ✅ | Selesai penuh (finishing 2026-09-11) |
| **11** | ✅ *selesai* — N+1 tren & lintas modul (§5.2), komponen UI bersama (§11.2 B2), `branches/my-access` (§5.3). Riwayat status order + PWA tetap dibuang by-design (§6) | Penghalusan |

Sebelum mengerjakan tiap tahap: baca bagian modulnya di
[`docs/legacy-reference/`](legacy-reference/), lalu `known-issues.md` dan `open-questions.md` —
**replikasi perilaku, bukan replikasi cacat** (PRD §4).

---

## 9. Lampiran — Endpoint Legacy Tanpa Padanan (tersisa 0 + varian terbuka)

Dasar angka §1. **Finishing 2026-09-11: 30 butir asal seluruhnya tertutup atau dinyatakan tidak
dibawa.** Rincian per butir di bawah supaya angka ini bisa diperiksa.

**Finance (26) — 15 tertutup, 11 tidak dibawa by-design, 0 terbuka** *(dikoreksi 2026-10-19: `posting-sources/{ignore,restore}` ditutup FN-01; tiga varian `opening/*` dulu "terbuka" kini dicatat tidak dibawa)*

*Tertutup via revisi (tanpa tabel baru):*
`close/{readiness,safe-close}` · `daily-close/overview` (sebagai `POST /finance/posting/close-day`) ·
`export/tax-package` · `opening/{get,validate,post}` (sebagai `GET /finance/opening`,
`POST .../opening/preview`, `POST .../opening/post`) ·
`posting-sources/{list,post-batch,close-day}` (sebagai `GET /finance/posting-sources`,
`POST .../posting/post-batch`, `POST .../posting/close-day`) ·
`reports/cash-summary` · `tax-adjustments/{list,save}` (sebagai `GET/POST /finance/tax-adjustments`)

*Tidak dibawa by-design (`DB_SCHEMA.md` §6 *Diputuskan*; kapabilitasnya tertutup lain cara):*
`cash-accounts/{list,create,update,archive}` → flag `is_cash` + CRUD COA;
`posting-sources/detail` → pratinjau (`posting/preview`); `posting-sources/{ignore,restore}` →
**ditutup FN-01** (`POST /finance/posting/ignore|restore`, tabel penanda `finance_posting_ignores`; klaim lama "tidak
bermakna tanpa antrean tersimpan" keliru karena dokumen macet memblokir safe-close); `posting-sources/cancel-posting` →
`POST /finance/journals/{id}/reverse`; `tax-adjustments/{detail,archive}` → detail jurnal + reverse.

*Tidak dibawa (dulu "terbuka"):* `opening/{update,import-inventory,supplement}` — saldo susulan/inventaris opening tertutup via
`POST /finance/journals/manual` + impor saldo awal stok + `POST /finance/opening/complete` (K-05, memberi harga modal posisi
berstok bernilai nol setelah terkunci); bila pemilik butuh alur khusus, daftarkan sebagai enhancement. Klaim lama bahwa saldo
awal "bukan penghambat cutover" keliru untuk persediaan — kini dipandu server (K-05).

*(`reports/{margin,receivables,payables}` sudah ditutup — §10.)*

**Order & pengiriman (3) — tersisa 0**
`deliveries/work-queue` ✅ (`GET /deliveries/work-queue` + halaman Antrian Kirim, J3) ·
`orders/export` ✅ (`sales-orders/export` + `purchase-orders/export`, xlsx+pdf, J1) ·
`orders/update-status` ✅ (tercakup confirm/cancel/approve-credit; status terkunci di kode per keputusan produk lama D4 PLAN `menuju-pengganti-production`; `deliver-goods` → alur idempoten, ADR-0018 D2)

*(Retur sudah ditutup — Tahap D: `sales-returns/context`, `sales-returns/preview`,
`sales-returns/replacement-deliveries` + `confirm`, `purchase-returns/context`,
`purchase-returns/preview`.)*

**Stok (1) — tersisa 0**
`stock/transfer` ✅ (pindah seketika via `POST /stock/transfers/move-location` + modal, J4 —
pemindahan satu-langkah dengan jejak dokumen; lihat §4.7)

*(J5/J6 2026-09-11: `stock/detail` ditutup `GET /stock/balances/{productId}` +
halaman `StockItemPage`; `branches/my-access` ditutup endpoint login-only +
hook `use-accessible-branches` — keduanya keluar dari daftar ini.)*

---

## 10. Pengerjaan — Tahap 1 Selesai

**Migrasi `000019_finance_line_dimensions`** menambah `product_id`, `variant_id`, `party_id`,
`description` ke `finance_journal_lines` + dua indeks komposit (`account_id, product_id` dan
`account_id, party_id` — akun didahulukan karena selektivitasnya paling tinggi).

Setiap builder jurnal kini memasang dimensinya: pendapatan & HPP per produk, piutang & hutang per
lawan transaksi. Tiga endpoint baru berdiri di atasnya, plus UI-nya:

| Endpoint baru | UI |
|---|---|
| `GET /finance/reports/margin-by-product?from=&to=[&limit=]` | tab **Marjin** → tabel "Marjin per Produk" |
| `GET /finance/reports/receivables[?asOf=]` | tab **Piutang & Hutang** (+ menu sidebar) |
| `GET /finance/reports/payables[?asOf=]` | idem |

Biaya baca: **satu query berkelompok** untuk agregasinya, **satu lagi** untuk resolusi nama
produk/party (`LabelsByIDs`) — bukan satu per baris. Sampai 2026-09-11 kalimat ini hanya benar untuk
paruh pertamanya: agregatnya memang berkelompok, tetapi dekorasi namanya masih per baris sehingga
N+1-nya kembali lewat pintu belakang (lihat §5.2). Piutang/utang memakai batas `asOf` (kumulatif), bukan rentang:
saldo ditanyakan "per tanggal", dan filter rentang akan diam-diam membuang faktur yang lebih tua.

### 10.1 Tiga bug posting yang ikut ketahuan

Membedah builder untuk memasang dimensi membuat tiga kesalahan uang terlihat. Semuanya diperbaiki
di tahap yang sama, karena menambah dimensi di atas angka yang salah hanya melaporkan angka salah
dengan lebih rinci.

| # | Bug | Akibat | Perbaikan |
|---|---|---|---|
| 1 | **Surat jalan parsial dibukukan penuh.** `buildDelivery` memakai `so.GrandTotal`/`Subtotal`/`TaxTotal` — nilai **seluruh order** — untuk setiap SJ. Pengiriman bertahap adalah alur yang didukung (`delivery/application/service.go` membatasi tiap SJ ke sisa SO), jadi ini terjangkau, bukan teoretis | SO yang dikirim 2 kali membukukan pendapatan, PPN, dan piutang **dua kali penuh** | Nilai di-pro-rata ke baris yang benar-benar ada di SJ itu, per produk |
| 2 | **PPN dihitung dua kali pada PO harga-termasuk-pajak.** `buildGoodsReceipt` mendebit Persediaan sebesar *net* (yang untuk `tax_type = include` sudah mengandung PPN) lalu **menambahkan** PPN Masukan di atasnya | Contoh 1 unit @ Rp11.100 (PPN 11% di dalam): terbukukan Persediaan 11.100 + PPN 1.100 / Hutang 12.200. Seharusnya 10.000 + 1.100 / 11.100. Ketiga kaki salah | Persediaan didebit pada **tax base**; hutang = jumlah yang benar-benar didebit, jadi seimbang menurut konstruksi |
| 3 | **Cost ledger memakai basis yang sama salahnya.** `receiptUnitCost` memakai `lineNet`, dan komentarnya mengklaim "pre-tax" — padahal hanya benar untuk `none`/`exclude` | Rata-rata bergerak menggelembung sebesar PPN pada PO inklusif → HPP dan marjin ikut salah | Memakai tax base yang sama dengan jurnal GR, sehingga cost ledger dan buku tidak bisa berbeda |

Ketiganya sekarang dijaga unit test (`finance/application/builders_test.go`): `lineEconomics` untuk
ketiga tipe pajak termasuk kombinasi diskon, `prorate` termasuk pembagian-nol, dan satu test yang
mengirim satu order dalam dua surat jalan lalu memastikan totalnya persis sekali, bukan dua kali.

> **Catatan data lama:** jurnal yang sudah ter-posting **sebelum** perbaikan ini tetap menyimpan
> angka lamanya — posting bersifat idempoten, jadi tidak ada yang otomatis dihitung ulang. Bila
> sudah ada data produksi dengan PO/SO ber-PPN inklusif atau pengiriman bertahap, jurnal terkait
> perlu di-`reverse` lalu di-posting ulang. Di lingkungan yang masih data uji, abaikan saja.

### 10.2 Verifikasi

`go build ./...` ✅ · `go test ./...` ✅ · `tsc --noEmit` ✅ · `vite build` ✅.
*(Angka paket saat Tahap 1; kesehatan build terkini ada di §1 dan §11.)*

### 10.3 Catatan terpisah — ESLint frontend ✅ **sudah berjalan**

Arsip: dulu `npx eslint "src/**/*.{ts,tsx}"` menolak dengan *"all of the files matching the glob
pattern are ignored"* — lint tidak memeriksa satu file pun. **Sekarang lint menjangkau seluruh
`src/` dan hasilnya 0 masalah** (dari 101 warning saat Scope B dimulai; 89 di antaranya
`react-hooks/set-state-in-effect`, ditutup dengan hook `useAsyncData` yang dipakai 65 file).

---

## 11. Pengerjaan — Scope A & B (2026-09-11)

Ditutup dari [`plan/verifikasi-modul-2026-09-11/PLAN.md`](plan/verifikasi-modul-2026-09-11/PLAN.md).

### 11.1 A1 — kebocoran otorisasi transfer stok (P0, satu-satunya bug fungsional)

Empat endpoint transfer-per-ID tidak pernah membandingkan cabang dokumen dengan cabang sesi;
`GetTransfer` bahkan tidak menerima `branchID`, dan `guarded` hanyalah `RequireBranch` (memastikan
*ada* cabang aktif, bukan cabang **yang mana**). Pengguna di cabang B bisa mengirim, menerima, atau
membatalkan transfer milik cabang A dengan menebak ID numerik.

`stock` adalah satu-satunya dari 8 modul berdokumen yang tidak melakukan pemeriksaan ini — anomali,
bukan desain.

Diperbaiki dengan **aturan arah**, bukan satu perbandingan seragam (menyamaratakan akan mematahkan
transfer antar-cabang):

| Operasi | Cabang sesi yang sah |
|---|---|
| `GET` | `FromBranchID` **atau** `ToBranchID` |
| `dispatch`, `cancel` | `FromBranchID` saja |
| `receive` | `ToBranchID` saja |

Penolakan memakai `NotFound`, bukan `Forbidden` — keberadaan dokumen cabang lain tidak boleh bocor
lewat beda pesan.

Dijaga `internal/integration/transfer_scope_test.go` — termasuk kasus **"receive di cabang tujuan
berhasil"**, penjaga yang memastikan perbaikan ini tidak berubah jadi regresi lintas-cabang.
Sapuan lanjutan menemukan kasus sejenis pada lokasi stok (`TestLocationBranchScope`).

### 11.2 Sisanya

| Butir | Hasil |
|---|---|
| A2 jaring pengaman | 23 → **31 file test**, integrasi 19 → **35 test**: `stock_flows` (saldo awal, hold/release reservasi, adjust set-mode), `payment_flows` (tolak overpay, cancel hitung ulang), `user_roles` (hapus role terpakai) |
| A3 N+1 | lihat §5.2 |
| A4 toolchain | `go.mod` pin `toolchain go1.26.7` — build tidak lagi bergantung toolchain yang kebetulan ter-cache |
| A5 rahasia | `JWT_SECRET` ditolak bila = nilai contoh di luar `development`, dan bila < 32 karakter |
| B1 envelope | ✅ **tuntas** — `toast.fromServer(res.message, fallback, deskripsi)` + varian `api*Full` yang mempertahankan `{data, message}`. **98 sisi panggil tulis** memakai pesan server. Delapan `toast.success` tersisa memang bukan tulis-server: 4 unduhan berkas (mengembalikan `void`, tanpa envelope), 2 agregat batch yang dihitung klien (`Diposting N dokumen`), dan 2 pesan komposit POS yang menggabungkan beberapa keadaan klien (piutang / pembayaran tertunda / lunas) |
| B2 komponen | 9 komponen: 6 generik di `shared/components/ui/`, 3 varian domain (`PartySearchSelect`, `ProductSearchSelect`, `StockLocationSelect`) di modulnya — aturan *shared harus domain-agnostik* dihormati |
| B3 render beruntun | hook `useAsyncData` (65 file); eslint 101 → **0** |
| B4 uji frontend | 11 → **17 file**, 60 → **85 test** |

> **Kerapuhan yang ikut ketahuan dan sudah dikunci:** `React.act` hanya ada di
> `react.development.js`, sedangkan `react/index.js` memilih build dari `process.env.NODE_ENV`.
> Menjalankan test dengan `NODE_ENV=production` (lazim di CI dan Docker build) membuat **seluruh
> test yang me-render komponen** gagal dengan *"React.act is not a function"* — bukan karena
> kodenya salah. Dikunci lewat `env: { NODE_ENV: "development" }` di `apps/web/vitest.config.ts`.

**C0 (git init) — SUDAH USANG:** repo kini berisi `.git` dengan riwayat commit dan CI GitHub
Actions (`.github/workflows/ci.yml`). Catatan keamanan yang tetap berlaku: `apps/api/storage/`
(isinya `whatsapp_session.db` — sesi WhatsApp terautentikasi — dan foto produk unggahan pengguna)
tidak boleh pernah ter-commit.

---

## 12. Paket Kertas Kerja Pajak — Plafon Peredaran Bruto (2026-09-11)

Legacy punya dua versi ekspor pajak lewat parameter `layer`: `layer 1` = data riil,
`layer 2` = "Dibatasi Rp4,8 M (simulasi UMKM)". Revamp semula **tidak membawanya** (tercatat
sebagai keputusan bisnis terbuka, PRD §5 no. 7). Pemilik memutuskan membawanya
**2026-09-11**, dengan mekanisme yang dirancang ulang — bukan port 1:1.

### 12.1 Tiga koreksi terhadap perilaku legacy

| # | Legacy | Revamp | Alasan |
|---|---|---|---|
| 1 | Plafon diterapkan pada **rentang tanggal yang diminta** | **Kumulatif tahun pajak** (1 Jan → akhir bulan diminta) | Rp4,8 M adalah ambang **tahunan** (PP23). Karena paket revamp dicetak per bulan, meniru legacy membuat plafon sebulan praktis tak pernah kena — layer 2 diam-diam identik dengan layer 1 |
| 2 | Urutan exclusion **acak-stabil `CRC32(id_order)`** | **Kronologis** (tanggal, lalu id) | Untuk plafon berjalan setahun, order yang menembus plafon adalah yang **belakangan**, bukan sampel pseudo-acak yang tidak bisa dijelaskan ke pemeriksa |
| 3 | Lingkup mengikuti filter laporan | **Selalu seluruh cabang** | PP23 melekat pada **wajib pajak**, bukan cabang. Menjumlahkan plafon per cabang bisa melewati plafon perusahaan tanpa ketahuan |

### 12.2 Yang dipertahankan dari legacy — dan kenapa

Unit yang dibuang adalah **ORDER**, bukan satu jurnal. Satu penjualan tersebar di beberapa entry
(pendapatan + HPP di SJ, kas + piutang di pembayaran); membuang jurnal pendapatan saja akan
menyisakan HPP-nya sehingga marjin jadi janggal. Karena yang dibuang jurnal **utuh**
(debit = kredit), seluruh laporan tetap seimbang. Order yang menembus plafon dibuang
**seutuhnya**, tidak dipotong pas-plafon — memotong berarti mengarang penjualan yang tidak pernah
terjadi pada nilai itu.

### 12.3 Tanpa migrasi, tanpa kolom baru

Atribusi jurnal → order **diturunkan saat laporan dibuat**, lewat tiga kontrak batch baru
(`DeliveryClient.OrderIDsByNotes`, `PaymentClient.OrderIDsByPayments`,
`SalesReturnClient.OrderIDsByReturns`) plus `SalesOrderClient.SummariesByIDs` untuk pelabelan.
Tidak ada kolom `order_id` yang didenormalisasi ke `finance_journal_lines`, sehingga:

- tidak ada backfill untuk jurnal lama — data historis langsung ikut terhitung;
- tidak ada kolom yang bisa **melenceng** dari kenyataan;
- biaya query **konstan**: satu pembacaan jurnal + maksimal empat pembacaan berkelompok,
  berapa pun panjang rentangnya. Bukan satu query per order (N+1 yang justru dihindari).

Retur penjualan masuk sebagai pendapatan **negatif**, jadi peredaran bruto otomatis bersih dari
retur tanpa penanganan khusus.

### 12.4 Batas yang diakui terbuka

Satu entry pembayaran di revamp hanya punya 2 baris (kas + piutang) berapa pun jumlah alokasinya,
sehingga sebuah pelunasan bisa menyangkut order **di dalam dan di luar** plafon sekaligus.
Legacy membuang seluruh entry seperti itu — ikut menghapus kas milik order yang masih masuk.
Revamp memilih sebaliknya: **entry semacam itu dipertahankan**, lalu didaftarkan terbuka di sheet
*Transaksi Dikecualikan*. Tidak ada pilihan yang eksak di sini; yang bisa dilakukan adalah
menampakkannya, bukan menyembunyikannya.

### 12.5 Keluaran — dua paket terpisah, isi sheet SAMA

Istilah **"Layer 1"/"Layer 2" hanya hidup di level aplikasi** (konstanta `PackageActual` /
`PackageCapped`, query param `variant`). Istilah itu **tidak pernah muncul** di nama berkas, nama
sheet, maupun sel mana pun — kertas kerja yang diserahkan ke konsultan pajak harus menjelaskan
dirinya sendiri, bukan menyebut saklar internal.

**Kedua paket membawa lima sheet laporan yang sama**, dirender oleh fungsi penulis yang sama:
Untung Rugi · Posisi Harta & Hutang · Cek Saldo Akun · Rincian PPN · Rekonsiliasi Fiskal.
Yang membedakan hanya **scope** saat angkanya dihitung, bukan cara menyajikannya.

| | `...-data-riil.xlsx` | `...-peredaran-terbatas.xlsx` |
|---|---|---|
| Ringkasan | ✅ | ✅ + dasar pembatasan (plafon, terpakai, sisa) |
| Untung Rugi · Neraca · Neraca Saldo · Rincian PPN · Rekonsiliasi Fiskal | ✅ penuh | ✅ **sudah tereliminasi** |
| Peredaran Bruto (per order + kumulatif) | — | ✅ tambahan |
| Transaksi Dikecualikan (+ alasan) | — | ✅ tambahan |

Dua sheet terakhir **ditambahkan di atas**, bukan menggantikan apa pun: angka terbatas yang tidak
bisa ditelusuri balik ke aturannya bukan kertas kerja, hanya angka yang lebih kecil.

#### Bagaimana konsistensinya dijamin

Setiap laporan di modul ini membaca total lewat **satu** fungsi, `Service.footings`. Mengeliminasi
di titik itu membuat order yang dibuang hilang dari **seluruh** sheet sekaligus — laba rugi,
neraca, neraca saldo, PPN, rekonsiliasi fiskal. Tidak ada jalur kedua yang masih bisa
menghitungnya.

Di atasnya, `Service.Reports(ctx, branchID, year, month, variant)` menghitung **semua** angka satu
paket dalam satu scope, dan **kedua** builder merender dari struct yang sama. Keduanya tidak bisa
menyimpang ke rumus berbeda karena keduanya tidak punya rumus berbeda.

Eliminasi di tingkat SQL dilakukan sebagai `(penuh − yang dikecualikan)`, bukan `NOT IN (...)`
raksasa: footing adalah penjumlahan biasa sehingga pengurangannya eksak, dan sisi "dikecualikan"
dipecah per 1.000 id. Kedua sisi memakai jendela tanggal dan filter komersial yang sama persis.

### 12.6 Verifikasi

`internal/integration/turnover_test.go` (MySQL nyata, 5 test):

- `TestGrossTurnoverCeiling` — plafon menggigit, order dibuang utuh, uang tidak hilang dari pembagian
- `TestGrossTurnoverIsCumulativeYearToDate` — Februari terkena plafon **hanya** karena Januari dihitung lebih dulu (inti sifat tahunan)
- `TestGrossTurnoverIncludesLastDayOfMonth` — transaksi hari terakhir bulan tetap terhitung (batas DATETIME)
- `TestTaxPackageVariantsDiffer` — dua artefak `.xlsx` berbeda, nama berkas tidak membocorkan kosakata "layer", varian tak dikenal ditolak
- `TestCappedViewIsConsistentEverywhere` — **jaminan intinya**: order yang dikecualikan tidak terhitung di mana pun. Pendapatan turun persis sebesar yang dikecualikan, **HPP ikut turun**, neraca tetap seimbang, neraca saldo tetap balance dan ikut turun, baris PPN ikut tersaring, dan angka laba-rugi cocok dengan kertas kerja plafon.

> Fixture-nya mengambil stok dari **penerimaan barang**, bukan dari penyesuaian stok. Stok hasil
> penyesuaian tidak berbiaya, sehingga pengirimannya membukukan HPP nol — dengan fixture seperti
> itu klaim "HPP ikut turun" tidak akan pernah benar-benar teruji.
