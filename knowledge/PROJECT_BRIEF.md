# Project Brief — mini-erp

## Apa ini

Revamp dari aplikasi ERP operasional satu-perusahaan multi-cabang untuk UKM
(`../mini-erp` — versi lama, NestJS + TypeORM + MySQL): pembelian, stok/gudang, penjualan & POS,
pengiriman, pembayaran, keuangan/pajak, dan laporan.

## Kenapa direvamp

Sistem lama sudah menjalankan operasional nyata tapi sulit dirawat: satu modul backend menaungi
enam domain sekaligus (~9.000 baris), ketergantungan data lintas modul yang tidak terlihat dari
kode (Finance membaca 12 tabel milik 8 modul lain lewat SQL mentah), dan konvensi dasar yang tidak
ditegakkan sistem (validasi payload nyaris tidak ada, tidak ada base entity, dua standar
pembulatan uang hidup bersamaan). Detail lengkap di [../docs/PRD.md §1](../docs/PRD.md#1-latar-belakang--tujuan-revamp).

## Stack baru

Go modular monolith (`apps/api`) + MySQL (host-level) + React 19/Tailwind 4 (`apps/web`) — batas
antar-modul ditegakkan lewat package boundary, bukan konvensi manual.

## Status saat ini

**Seluruh 20 modul backend + frontend + POS + bot WA terimplementasi.** Dokumentasi arsitektur & rencana sudah lengkap
(`docs/`), dan legacy sistem lama sudah dianalisis penuh (`docs/legacy-reference/` — 21 modul, 149
known issue, 130 pertanyaan terbuka). Langkah berikutnya: desain & implementasi modul satu per
satu, mengikuti urutan di [MODULE_MAP.md](MODULE_MAP.md).

## Baca lebih lanjut

- [../docs/PRD.md](../docs/PRD.md) — tujuan, cakupan, keputusan bisnis yang masih terbuka
- [../docs/SYSTEM_DESIGN.md](../docs/SYSTEM_DESIGN.md) — arsitektur & urutan pembangunan
- [MODULE_MAP.md](MODULE_MAP.md) — routing kerja per modul
