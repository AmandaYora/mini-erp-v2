# Architecture Overview — mini-erp

Ringkasan gateway. Arsitektur lengkap (stack, peta modul + kontrak publik, urutan pembangunan,
kontrak lintas modul, risiko per modul, keamanan) ada di **[../docs/SYSTEM_DESIGN.md](../docs/SYSTEM_DESIGN.md)**.

## Ringkas

```txt
apps/web (React 19 + Tailwind 4)  →  /api/v1/*  →  apps/api (Go modular monolith)  →  MySQL 8.4 (host)
```

- **Backend:** satu binary Go, 20 modul + `shared/` teknis. Setiap modul: `contracts/ · application/ ·
  domain/ · infrastructure/ · presentation/`; hanya `contracts/` yang boleh diimpor modul lain.
  Go sendiri tidak melarang impor itu — yang menegakkan adalah `scripts/arch-check.mjs` (dibuktikan
  `--self-test`). Detail: [BACKEND_GUIDE.md](BACKEND_GUIDE.md).
- **Database:** MySQL host-level (bukan container). Setiap tabel milik tepat satu modul; relasi lintas
  modul = primitive ID tanpa FK. Migrasi `golang-migrate`, akses data `sqlc` (bertahap, ADR-0012).
  Detail: [DATABASE_GUIDE.md](DATABASE_GUIDE.md).
- **Wiring:** `internal/bootstrap.Wire` merakit semua modul dan diakhiri `Modules.Verify()`.
- **Frontend:** SPA React; satu instance Axios, lazy routes, Design System v2. Detail:
  [FRONTEND_GUIDE.md](FRONTEND_GUIDE.md).
- **Deployment:** satu container Docker (frontend statis + API Go, port 8080), database tetap di host; produksi di VPS elcodelabs, image dari GHCR ([ADR-0019](decisions/ADR-0019-deploy-elcodelabs-ghcr.md)).
  Detail: [../docs/DEPLOYMENT.md](../docs/DEPLOYMENT.md).
- **Konstanta terkunci:** `id-ID` / `Asia/Jakarta` / IDR, uang integer rupiah —
  [ADR-0006](decisions/ADR-0006-locale-timezone-locked.md), [ADR-0005](decisions/ADR-0005-money-as-integer-rupiah.md).

Lapisan modul (L0 fondasi → L9 asisten) dan dependensinya: [MODULE_MAP.md](MODULE_MAP.md).
Pola lintas modul yang dipakai di seluruh kode (kernel uang, audit dalam transaksi, transaction
script, idempotensi + CAS): ADR-0010, ADR-0011, ADR-0014, ADR-0016 di [decisions/](decisions/).

## Kenapa Arsitektur Ini Dipilih

Revamp dari sistem lama yang punya satu modul backend menaungi enam domain sekaligus dan
ketergantungan data lintas modul yang tidak terlihat dari kode — lihat alasan lengkap di
[../docs/PRD.md §1](../docs/PRD.md#1-latar-belakang--tujuan-revamp) dan perbaikan arsitektur wajib
di [../docs/PRD.md §6](../docs/PRD.md#6-perbaikan-arsitektur-yang-wajib-diadopsi-bukan-opsional).
