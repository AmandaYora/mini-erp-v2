# Module Map — mini-erp

Routing table untuk kerja per modul. Detail arsitektur & urutan pembangunan ada di
[../docs/SYSTEM_DESIGN.md](../docs/SYSTEM_DESIGN.md) — file ini adalah versi ringkas untuk
navigasi cepat saat menyentuh kode.

**Status: 20 modul sudah terisi**, bukan skeleton kosong lagi (~30k baris Go non-test, 26
migrasi, 65+ halaman frontend, 23 file uji backend + 11 file uji frontend). Paritas kapabilitas
terhadap sistem lama **±98%** (cakupan endpoint 100% dari target 205) — tersisa program migrasi:
ETL data legacy (Tahap K), gladi cutover (Tahap L), E2E, dan verifikasi cetak fisik. Tahap
G/H-revisi (tanpa tabel baru) dan J2/J3/J4 (antrean kirim, label QR, pindah 1-langkah) selesai
2026-09-11. Semuanya terdaftar dengan bukti `file:baris` dan urutan penutupan berbasis dependensi di
[../docs/PARITY_AUDIT.md](../docs/PARITY_AUDIT.md).

Urutan lapisan di bawah tetap berlaku saat menambah kemampuan baru: jangan menyentuh modul di
lapisan lebih tinggi dengan mengasumsikan dependensinya belum ada — cek kodenya dulu.

| Lapisan | Modul (`internal/modules/<x>`) | Tanggung jawab | Bergantung pada |
|---|---|---|---|
| L0 | `audit` | Catat siapa mengubah apa/kapan, dipanggil semua modul | — |
| L0 | `media` | Penyimpanan file (foto produk, bukti bayar/kirim) | — |
| L1 | `auth` | Login, sesi JWT, ganti role/cabang aktif, resolusi permission | `user`, `branch` (kontrak, bukan import langsung — lihat SYSTEM_DESIGN §4) |
| L1 | `user` | Akun, role, permission (RBAC) | — (dipakai `auth` untuk sesi; hanya impor middleware sesi untuk actor) |
| L1 | `company` | Profil & pengaturan perusahaan tunggal | `auth` |
| L1 | `branch` | Direktori cabang + penomoran dokumen per cabang | `auth`, `user` |
| L2 | `product` | Katalog, kategori, varian, UOM/konversi | L1 |
| L2 | `party` | Customer/supplier, alamat kirim, member type & pricing | L1, `product` |
| L3 | `stock` | Lokasi, saldo, mutasi, transfer, opname | L1, `product` |
| L4 | `purchasing` | Purchase order ke supplier | L1, `party`, `product`, `stock` |
| L4 | `sales` | Sales order ke customer (kanal reguler & POS) | L1, `party`, `product`, `stock` |
| L5 | `goodsreceipt` | Penerimaan barang atas PO | `purchasing`, `stock` |
| L5 | `delivery` | Pengiriman atas SO, surat jalan, bukti kirim | `sales`, `stock`, `media` |
| L5 | `payment` | Pembayaran & alokasi, saldo/ledger pihak | `purchasing`, `sales`, `media` |
| L6 | `salesreturn` | Retur customer atas SO/SJ | `sales`, `delivery` |
| L6 | `purchasereturn` | Retur ke supplier atas PO/penerimaan | `purchasing`, `goodsreceipt` |
| L7 | `finance` | COA, jurnal, HPP, laporan, tutup periode, ekspor pajak | **semua modul di atas** (lihat SYSTEM_DESIGN §7 — konsumen data terbesar) |
| L8 | `dashboard` | Ringkasan operasional (hari ini + bulan berjalan + saldo kunci — baca finance live, tanpa tabel) | `finance` |
| L8 | `reporting` | Tren penjualan harian + nilai persediaan (baca finance live, tanpa agregasi terjadwal — KI-126) | `finance` |
| L9 | `assistant` | Bot WA (whatsmeow) + 8 intent tooling read-only, whitelist nomor, tanpa RAG/AI/knowledge | `branch`, `product`, `sales`, `purchasing`, `stock`, `finance` |

## Aturan Boundary (ringkas — detail di `.claude/rules/backend-modular-monolith.md`)

- Hanya `contracts/` milik modul lain yang boleh diimpor. Dilarang mengimpor
  `application/`, `domain/`, `infrastructure/` modul lain.
- Relasi lintas modul = primitive ID di kolom DB (`party_id`, `product_id`, dst.), **tanpa** FK
  fisik lintas modul dan **tanpa** join SQL lintas modul.
- Orkestrasi lintas modul terjadi di `application/` modul pemicu, memanggil `contracts/` modul lain
  secara eksplisit.

## Kontrak Lintas Modul yang Tidak Boleh Berubah Sepihak

Daftar lengkap 12 kontrak ada di
[../docs/SYSTEM_DESIGN.md §7](../docs/SYSTEM_DESIGN.md#7-kontrak-lintas-modul-yang-tidak-boleh-berubah-sepihak).
Yang paling sering relevan saat implementasi:

1. Scope sesi selalu dari token (`auth`), tidak pernah dari body request.
2. Satuan stok + faktor konversi adalah kontrak `product` — jangan hardcode konversi di modul lain.
3. Item dokumen transaksi (`purchase_order_items`, `sales_order_items`, dst.) menyimpan **snapshot**
   nama/harga produk saat itu — bukan join ke `products` hidup.
4. Setiap perubahan `stock_balances` wajib disertai baris `stock_movements` (kontrak milik `stock`).
5. Nomor dokumen (PO/SO/SJ/pembayaran/retur) dialokasikan lewat `BranchClient.NextDocumentNumber`,
   bukan dihitung manual di modul masing-masing.

## Modul yang Belum Dijadwalkan

Tidak ada — semua 20 modul L0–L9 terimplementasi (L9 dijadwalkan dan dibangun
atas permintaan pemilik; skop: bot WA + tooling, tanpa Knowledge/RAG/AI).

## Referensi Sistem Lama per Modul

Saat mendesain modul baru, baca dokumen legacy yang relevan untuk memahami perilaku yang sudah ada
(fitur, known issue, pertanyaan terbuka) — **jangan** mengasumsikan replikasi 1:1 tanpa membaca
`known-issues.md`/`open-questions.md` bagian modul itu:

| Modul baru | Dokumen legacy |
|---|---|
| `auth` | [../docs/legacy-reference/01-auth-session/](../docs/legacy-reference/01-auth-session/) |
| `user` | [../docs/legacy-reference/02-users-roles-permissions/](../docs/legacy-reference/02-users-roles-permissions/) |
| `company` | [../docs/legacy-reference/03-company-settings/](../docs/legacy-reference/03-company-settings/) |
| `branch` | [../docs/legacy-reference/04-branch/](../docs/legacy-reference/04-branch/) |
| `product` | [../docs/legacy-reference/05-product-catalog/](../docs/legacy-reference/05-product-catalog/) |
| `party` | [../docs/legacy-reference/06-business-party/](../docs/legacy-reference/06-business-party/), [../docs/legacy-reference/07-member-pricing/](../docs/legacy-reference/07-member-pricing/) |
| `purchasing` | [../docs/legacy-reference/08-order-purchasing/](../docs/legacy-reference/08-order-purchasing/) |
| `sales` | [../docs/legacy-reference/09-order-sales/](../docs/legacy-reference/09-order-sales/), [../docs/legacy-reference/15-pos/](../docs/legacy-reference/15-pos/) |
| `goodsreceipt` | [../docs/legacy-reference/10-goods-receipt/](../docs/legacy-reference/10-goods-receipt/) |
| `delivery` | [../docs/legacy-reference/11-delivery/](../docs/legacy-reference/11-delivery/) |
| `salesreturn` | [../docs/legacy-reference/12-sales-return/](../docs/legacy-reference/12-sales-return/) |
| `purchasereturn` | [../docs/legacy-reference/13-purchase-return/](../docs/legacy-reference/13-purchase-return/) |
| `payment` | [../docs/legacy-reference/14-payment/](../docs/legacy-reference/14-payment/) |
| `stock` | [../docs/legacy-reference/16-stock-inventory/](../docs/legacy-reference/16-stock-inventory/) *(dokumen paling tipis di legacy — baca kode lama langsung juga bila perlu)* |
| `finance` | [../docs/legacy-reference/17-finance/](../docs/legacy-reference/17-finance/) *(paling dalam & terverifikasi di legacy)* |
| `dashboard` | [../docs/legacy-reference/18-dashboard/](../docs/legacy-reference/18-dashboard/) |
| `reporting` | [../docs/legacy-reference/19-reporting/](../docs/legacy-reference/19-reporting/) |
| `audit` | [../docs/legacy-reference/20-audit-log/](../docs/legacy-reference/20-audit-log/) |
