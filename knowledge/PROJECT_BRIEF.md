# Project Brief — mini-erp

## Apa ini

Revamp dari aplikasi ERP operasional satu-perusahaan multi-cabang untuk UKM
(`../mini-erp` — versi lama, NestJS + TypeORM + MySQL): pembelian, stok/gudang, penjualan & POS,
pengiriman, pembayaran, keuangan/pajak, laporan, dan bot WhatsApp operasional. Revamp ini akan
**menggantikan** sistem lama di produksi.

## Kenapa direvamp

Sistem lama sudah menjalankan operasional nyata tapi sulit dirawat: satu modul backend menaungi
enam domain sekaligus (~9.000 baris), ketergantungan data lintas modul yang tidak terlihat dari
kode (Finance membaca 12 tabel milik 8 modul lain lewat SQL mentah), dan konvensi dasar yang tidak
ditegakkan sistem (validasi payload nyaris tidak ada, tidak ada base entity, dua standar
pembulatan uang hidup bersamaan). Detail lengkap di [../docs/PRD.md §1](../docs/PRD.md#1-latar-belakang--tujuan-revamp).

## Stack baru

Go modular monolith (`apps/api`) + MySQL (host-level) + React 19/Tailwind 4 (`apps/web`) — batas
antar-modul ditegakkan oleh `scripts/arch-check.mjs`, bukan konvensi manual. Ringkasan:
[ARCHITECTURE_OVERVIEW.md](ARCHITECTURE_OVERVIEW.md).

## Posisi saat ini

Seluruh 20 modul terimplementasi dan paritas kapabilitas dengan sistem lama sudah diaudit. Yang
tersisa adalah program cutover (ETL data produksi, gladi, verifikasi cetak fisik). Angka dan daftar
pekerjaan terbuka: [../docs/STATUS.md](../docs/STATUS.md).

Perilaku sistem lama dianalisis penuh di `docs/legacy-reference/` (21 modul, known issue, pertanyaan
terbuka) dan tetap menjadi acuan saat mengubah perilaku sebuah modul.

## Baca lebih lanjut

- [../docs/PRD.md](../docs/PRD.md) — tujuan, cakupan, prinsip "replikasi perilaku, bukan cacat"
- [../docs/SYSTEM_DESIGN.md](../docs/SYSTEM_DESIGN.md) — arsitektur & kontrak lintas modul
- [MODULE_MAP.md](MODULE_MAP.md) — routing kerja per modul
