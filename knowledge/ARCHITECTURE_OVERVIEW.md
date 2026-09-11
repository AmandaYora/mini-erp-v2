# Architecture Overview — mini-erp

Ringkasan gateway. Arsitektur lengkap (stack, peta modul, urutan pembangunan, kontrak lintas
modul, risiko per modul, keamanan) ada di **[../docs/SYSTEM_DESIGN.md](../docs/SYSTEM_DESIGN.md)**.

## Ringkas

```txt
apps/web (React 19 + Tailwind 4)  →  /api/v1/*  →  apps/api (Go modular monolith)  →  MySQL (host)
```

- Backend: Go modular monolith, 20 modul (+ `shared/` teknis). Setiap modul: `contracts/ ·
  application/ · domain/ · infrastructure/ · presentation/`. Hanya `contracts/` yang bisa diimpor
  modul lain — ditegakkan compiler, bukan konvensi manual.
- Database: MySQL 8 host-level (bukan container). Migrasi lewat `golang-migrate`, akses data lewat
  `sqlc` (bukan GORM). Relasi lintas modul = primitive ID, tanpa FK fisik lintas modul.
- Deployment: satu container Docker (frontend statis + API Go), database tetap di host.
- Locale/timezone/currency (`id-ID` / `Asia/Jakarta` / IDR) adalah **konstanta aplikasi**, bukan
  setelan yang bisa diubah pengguna — lihat
  [decisions/ADR-0006-locale-timezone-locked.md](decisions/ADR-0006-locale-timezone-locked.md).

## Urutan Pembangunan (ringkas, detail di SYSTEM_DESIGN §4)

L0 fondasi (`audit`, `media`) → L1 identitas (`auth`, `user`, `company`, `branch`, dibangun sebagai
satu paket) → L2 master data (`product`, `party`) → L3 (`stock`) → L4 transaksi inti (`purchasing`,
`sales`) → L5 turunan (`goodsreceipt`, `delivery`, `payment`) → L6 retur → L7 (`finance`, paling
akhir — konsumen data terbesar) → L8 observabilitas.

## Kenapa Arsitektur Ini Dipilih

Revamp dari sistem lama yang punya satu modul backend menaungi enam domain sekaligus dan
ketergantungan data lintas modul yang tidak terlihat dari kode — lihat alasan lengkap di
[../docs/PRD.md §1](../docs/PRD.md#1-latar-belakang--tujuan-revamp) dan perbaikan arsitektur wajib
di [../docs/PRD.md §6](../docs/PRD.md#6-perbaikan-arsitektur-yang-wajib-diadopsi-bukan-opsional).

## Baca Selanjutnya

- [../docs/SYSTEM_DESIGN.md](../docs/SYSTEM_DESIGN.md) — dokumen utama, baca ini untuk detail
- [MODULE_MAP.md](MODULE_MAP.md) — routing kerja per modul
- [DATABASE_GUIDE.md](DATABASE_GUIDE.md), [API_GUIDE.md](API_GUIDE.md) — konvensi turunan
