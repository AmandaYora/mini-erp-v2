# Business Rules — Modul 07 Member Type & Member Pricing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** SEMUA validasi, formula, kondisi
khusus. Dari `member-type.service.ts`, `member-pricing.service.ts`, controller, form FE, dan
pemakaian di `order-pricing.service.ts`/`order.service.ts`/POS. Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

---

## 1. Master rule (BR-01…BR-10)

| ID | Aturan |
|---|---|
| BR-01 | Kode: trim + **UPPERCASE**, wajib non-kosong (`400 'Kode jenis member wajib diisi'`); unik per perusahaan di antara **non-arsip** (`409 "Kode jenis member '{k}' sudah digunakan"`); arsip bisa dipakai ulang di cek → 500 DB (→ KI-71). Update boleh ganti kode (cek mengecualikan diri — beda dengan pihak/kategori) |
| BR-02 | Nama: trim, wajib non-kosong (`400 'Nama jenis member wajib diisi'`); update parsial (hanya bila dikirim) |
| BR-03 | Enum: basis ∈ {purchase_price, min_selling_price, selling_price}; aksi ∈ {add, subtract}; tipe ∈ {nominal, percent}; mode ∈ {none, up, nearest, down}; status ∈ {active, inactive} — selain itu `400 '{label} tidak valid'` (label: Basis harga/Aksi harga/Tipe nilai/Pembulatan/Status jenis member). Create memakai default bila tak dikirim (basis purchase_price, aksi add, tipe nominal, mode none, status active); update memakai nilai lama bila tak dikirim |
| BR-04 | Besaran: `Number()` harus finite dan ≥ 0 (`400 'Besaran rule member harus bernilai 0 atau lebih'`); tanpa batas atas (1000% sah — → KI-76) |
| BR-05 | Kelipatan: null/undefined/**0** → null (= tanpa pembulatan); negatif/bukan-angka → `400 'Nilai pembulatan harus bernilai 0 atau lebih'`; tanpa batas atas |
| BR-06 | Mode dan kelipatan independen: mode ≠ none + kelipatan null = tanpa pembulatan diam-diam (→ KI-73) |
| BR-07 | Deskripsi: `description ?? description_text ?? null`, trim, kosong → null; create default null |
| BR-08 | List: `limit = min(??50, 1000)` (tanpa batas bawah — sekelas KI-26/55/70); selalu non-arsip; `status` kosong/`all` = semua status; cari = LIKE nama ATAU kode (frasa); urut nama A→Z. Store memuat `limit: 1000, status: all` (seluruh rule untuk dropdown) |
| BR-09 | Update/archive id: harus integer > 0 (`400 'Jenis member tidak valid'`, sebelum query — ada test); tak dikenal/terarsip → `404 'Jenis member tidak ditemukan'` |
| BR-10 | Guard penetapan: update→inactive (hanya saat transisi) dan archive selalu menolak bila > 0 pelanggan aktif memakai (`400 'Tidak bisa {menonaktifkan\|mengarsipkan} jenis member yang masih digunakan oleh {n} pelanggan aktif. Lepaskan atau ganti jenis member pelanggan terlebih dahulu.'` — ada test 1 & 2). Mengubah isi rule (harga/basis/mode) **tidak** dijaga — selalu lolos, future-only |

## 2. Resolusi quote (BR-11…BR-15)

| ID | Aturan |
|---|---|
| BR-11 | `id_member_type` eksplisit menang atas `id_business_party` (pihak diabaikan bila keduanya dikirim — → KI-72). `""`/null/undefined/0 = tidak dikirim |
| BR-12 | Id tak valid (non-integer/≤0, kecuali pengabaian BR-11) → `400 '{Jenis member\|Customer} tidak valid'` |
| BR-13 | Member eksplisit: tak dikenal/arsip/lain-perusahaan → `404 'Jenis member tidak ditemukan'`; nonaktif → `400 'Jenis member tidak aktif'` |
| BR-14 | Via pihak: tak dikenal/bukan-customer/terarsip → `404 'Customer tidak ditemukan'`; tanpa member / member nonaktif/terarsip → **sukses dengan `member_type: null`** (degradasi diam ke standard — → KI-72) |
| BR-15 | Item: id didedup, non-positif dibuang; produk tak dikenal/lain-perusahaan/terarsip → item `{pricing_source: 'manual', unit_price: null, error: 'Produk tidak ditemukan'}`; request tetap 200 |

## 3. Rumus harga (BR-16…BR-23)

| ID | Aturan |
|---|---|
| BR-16 | Urutan: basis (UOM jual) → sesuaikan (nominal langsung; persen = basis × nilai ÷ 100) → tolak negatif → rounding → 2-desimal. `basis_price` = basis 2-desimal; `unit_price` = hasil akhir 2-desimal (`Math.round((v + EPS) × 100) / 100`) |
| BR-17 | Basis jual normal = `selling_price`; basis minimum = `min_selling_price`; basis beli = `(purchase_price ÷ purchaseToBaseFactor) × salesToBaseFactor`. Contoh terkunci: beli 120.000, box 12, jual pack 3 → basis 30.000; +1000 → 31.000. Jual 20.000 +1,5% → 20.300. Min 12.500 −500 → 12.000 |
| BR-18 | Basis kosong/null/bukan-angka → `400 "Produk '{nama}' belum memiliki {harga jual normal\|harga jual minimum\|harga beli}"`; basis negatif → `400 "{label} produk '{nama}' tidak valid"`; nol diizinkan sebagai basis |
| BR-19 | Faktor konversi basis-beli harus finite > 0, keduanya (beli & jual) → else `400 "Konversi satuan {beli\|jual} produk '{nama}' tidak valid"` |
| BR-20 | Hasil < 0 → `400 "Harga member untuk produk '{nama}' menjadi negatif"` (dicek SEBELUM rounding; ada test 1000−2000) |
| BR-21 | Rounding: mode none ATAU kelipatan null/0/NaN/≤0 → tanpa ubah; else `up = ceil(v/i)×i`, `down = floor`, `nearest = round` (atas nilai mentah, sebelum 2-desimal) |
| BR-22 | Tanpa member: harga = manual ?? jual ?? 0; source = `standard` bila tanpa ketikan atau ketikan == jual (2-desimal), else `manual` (ada test keduanya) |
| BR-23 | Gagal per item tidak menggagalkan quote: kalkulasi gagal → `{pricing_source: 'member_rule', unit_price: null, error}` (item lain tetap); `member_type` header tetap terisi |

## 4. Konsumsi di Order/POS (BR-24…BR-28; ditegakkan modul lain, kontrak milik sini)

| ID | Aturan |
|---|---|
| BR-24 | Hanya order sales memakai rule (`relatedParty.memberType` → `resolvePrice`); purchase selalu manual; sales tanpa member → `resolvePrice(product, null, manual)` (klasifikasi standard/manual tetap dicatat) |
| BR-25 | Harga rule **menimpa** `unit_price` + `unit_price_before_discount` baris (bypass guard `min_selling_price` di 2 jalur create/update); manual/non-member tetap dijaga minimum |
| BR-26 | Harga rule adalah floor diskon: klem diskon manual memakai 0 (bukan minimum) untuk baris `member_rule`; requote meng-klem ulang diskon lama agar tak ditolak backend |
| BR-27 | Snapshot 7 kolom (`pricing_source`, id/nama member, basis, arah, tipe, nilai, basis_price) final per baris; cetak/diskon/selisih POS membaca snapshot, bukan rule live |
| BR-28 | Quote butuh `order.create`; dipicu ganti pelanggan (order form + POS) dengan penjaga urutan request; baris manual-edit tak ditimpa (order form); baris quote-gagal dipertahankan ([PERLU KONFIRMASI] vs reset — kode hanya melewati) |

## 5. Audit & izin (BR-29…BR-30)

| ID | Aturan |
|---|---|
| BR-29 | Audit `member_type.create/update/archive` (`entityType: 'member_type'`, `idBranch: null`): snapshot rule penuh 10 field (update before+after; archive before+`{archivedAt}`) |
| BR-30 | List/detail-butuh-`member_type.view`; tulis-butuh-`member_type.manage`; quote-butuh-`order.create`. Seed 040 memberi view+manage ke superadmin/owner/admin. `pricing/quote` tanpa audit (read-only) |

## 6. Aturan yang TIDAK ada (verifikasi)

Tidak ada: restore/hapus; batas atas besaran/persen/kelipatan; re-price order lama/draft; member per
produk/kategori/cabang/periode; minimal harga member (boleh 0 — basis 0 + tambah 0 sah); tolak hasil
di bawah minimum (disengaja — BR-25); paginasi bawah list; Sampah member; riwayat rule di UI;
validasi silang (mis. persen > 100 ditolak — tidak ada).
