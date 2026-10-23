# Module Map — mini-erp

Routing table untuk kerja per modul: lapisan, tanggung jawab, dependensi, dan dokumen sistem lama
yang relevan. Kontrak publik tiap modul dan alasan urutan lapisan ada di
[../docs/SYSTEM_DESIGN.md](../docs/SYSTEM_DESIGN.md) §3–§4.

Semua modul di bawah sudah terimplementasi. Urutan lapisan tetap berlaku saat menambah kemampuan:
modul di lapisan lebih rendah tidak boleh bergantung pada lapisan lebih tinggi.

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
| L5 | `delivery` | Pengiriman atas SO, surat jalan, bukti kirim; SJ Langsung tanpa order (ADR-0022) | `sales`, `stock`, `media`, `party` (pelanggan SJ Langsung) |
| L5 | `payment` | Pembayaran & alokasi, saldo/ledger pihak | `purchasing`, `sales`, `media` |
| L6 | `salesreturn` | Retur customer atas SO/SJ | `sales`, `delivery` |
| L6 | `purchasereturn` | Retur ke supplier atas PO/penerimaan | `purchasing`, `goodsreceipt` |
| L7 | `finance` | COA, jurnal, HPP, laporan, tutup periode, ekspor pajak | **semua modul di atas** (lihat SYSTEM_DESIGN §7 — konsumen data terbesar) |
| L8 | `dashboard` | Ringkasan operasional (hari ini + bulan berjalan + saldo kunci — baca finance live) dan alert kesehatan data (dihitung langsung; satu tabel penundaan alert, ADR-0025) | `finance`, `stock`, `sales`, `purchasing`, `payment`, `salesreturn`; untuk alert: `user`, `product`, `party`, `delivery` lewat `HealthClient` read-only + port integritas dari bootstrap |
| L8 | `reporting` | Tren penjualan harian + nilai persediaan (baca finance live, tanpa agregasi terjadwal — KI-126) | `finance` |
| L9 | `assistant` | Bot WA (whatsmeow) + 8 intent tooling read-only, whitelist nomor, tanpa RAG/AI/knowledge | `branch`, `product`, `sales`, `purchasing`, `stock`, `finance` |

POS bukan modul backend ([ADR-0007](decisions/ADR-0007-pos-no-dedicated-module.md)); ia kanal frontend
di atas `sales` (`channel = 'pos'`), `party`, `product`, `stock`, `payment`.

Aturan boundary: `.claude/rules/backend-modular-monolith.md` dan `.claude/rules/database.md`.
Kontrak lintas modul yang tidak boleh diubah sepihak (scope sesi, satuan stok, snapshot item,
pasangan saldo–mutasi stok, penomoran dokumen, dst.): [SYSTEM_DESIGN §7](../docs/SYSTEM_DESIGN.md#7-kontrak-lintas-modul-yang-tidak-boleh-berubah-sepihak).

## Referensi Sistem Lama per Modul

Sebelum mengubah perilaku sebuah modul, baca dokumen legacy-nya untuk memahami perilaku yang sudah
ada — dan **jangan** mengasumsikan replikasi 1:1 tanpa membaca `known-issues.md`/`open-questions.md`
bagian modul itu:

| Modul revamp | Dokumen legacy |
|---|---|
| `audit` | [../docs/legacy-reference/20-audit-log/](../docs/legacy-reference/20-audit-log/) |
| `media` | [../docs/legacy-reference/shared/shared-services.md](../docs/legacy-reference/shared/shared-services.md) §2.2–2.3 (`StorageService`, `ImageOptimizerService`) |
| `auth` | [../docs/legacy-reference/01-auth-session/](../docs/legacy-reference/01-auth-session/) |
| `user` | [../docs/legacy-reference/02-users-roles-permissions/](../docs/legacy-reference/02-users-roles-permissions/) |
| `company` | [../docs/legacy-reference/03-company-settings/](../docs/legacy-reference/03-company-settings/) |
| `branch` | [../docs/legacy-reference/04-branch/](../docs/legacy-reference/04-branch/) |
| `product` | [../docs/legacy-reference/05-product-catalog/](../docs/legacy-reference/05-product-catalog/) |
| `party` | [../docs/legacy-reference/06-business-party/](../docs/legacy-reference/06-business-party/), [../docs/legacy-reference/07-member-pricing/](../docs/legacy-reference/07-member-pricing/) |
| `stock` | [../docs/legacy-reference/16-stock-inventory/](../docs/legacy-reference/16-stock-inventory/) *(dokumen paling tipis di legacy — baca kode lama langsung juga bila perlu)* |
| `purchasing` | [../docs/legacy-reference/08-order-purchasing/](../docs/legacy-reference/08-order-purchasing/) |
| `sales` | [../docs/legacy-reference/09-order-sales/](../docs/legacy-reference/09-order-sales/), [../docs/legacy-reference/15-pos/](../docs/legacy-reference/15-pos/) |
| `goodsreceipt` | [../docs/legacy-reference/10-goods-receipt/](../docs/legacy-reference/10-goods-receipt/) |
| `delivery` | [../docs/legacy-reference/11-delivery/](../docs/legacy-reference/11-delivery/) |
| `payment` | [../docs/legacy-reference/14-payment/](../docs/legacy-reference/14-payment/) |
| `salesreturn` | [../docs/legacy-reference/12-sales-return/](../docs/legacy-reference/12-sales-return/) |
| `purchasereturn` | [../docs/legacy-reference/13-purchase-return/](../docs/legacy-reference/13-purchase-return/) |
| `finance` | [../docs/legacy-reference/17-finance/](../docs/legacy-reference/17-finance/) *(paling dalam & terverifikasi di legacy)* |
| `dashboard` | [../docs/legacy-reference/18-dashboard/](../docs/legacy-reference/18-dashboard/) |
| `reporting` | [../docs/legacy-reference/19-reporting/](../docs/legacy-reference/19-reporting/) |
| `assistant` | [../docs/legacy-reference/21-assistant-whatsapp-knowledge/](../docs/legacy-reference/21-assistant-whatsapp-knowledge/) *(hanya bagian bot WhatsApp yang dibawa)* |
