# Peta Dokumen `docs/`

Isi `docs/` terbagi menjadi tiga jenis. Jenisnya menentukan cara memakai dan merawatnya.

| Jenis | Arti | Cara merawat |
|---|---|---|
| **Referensi** | Mengikat dan harus cocok dengan kode hari ini | Perbarui bersama kode di commit yang sama |
| **Status** | Posisi proyek saat ini | Hanya [STATUS.md](STATUS.md) |
| **Riwayat** | Rencana, audit, dan checkpoint bertanggal | Beku; baca untuk konteks, jangan dijadikan acuan keadaan terkini |

Bila referensi dan riwayat bertentangan, referensi (dan kode) yang benar; lihat
[`knowledge/SOURCE_PRIORITY.md`](../knowledge/SOURCE_PRIORITY.md).

## Referensi

| Dokumen | Isi | Dijaga oleh |
|---|---|---|
| [PRD.md](PRD.md) | Latar belakang revamp, pengguna, cakupan, prinsip "replikasi perilaku, bukan cacat", keputusan bisnis awal | manual |
| [SYSTEM_DESIGN.md](SYSTEM_DESIGN.md) | Stack, peta modul + kontrak publik, urutan lapisan, kontrak lintas modul, keamanan | manual |
| [API_CONTRACT.md](API_CONTRACT.md) | Konvensi API, envelope, permission, **tabel semua rute** (§6), variabel environment (§7) | manual, bersama perubahan rute (`contract:check` hanya mencocokkan frontend ↔ router) |
| [DB_SCHEMA.md](DB_SCHEMA.md) | Aturan tabel, kepemilikan per modul (§3, dibangkitkan), disposisi tabel legacy, riwayat migrasi (§8) | `node scripts/db-schema-doc.mjs --check` |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Prasyarat, setup lokal, Docker, migrasi produksi, env produksi, APK kasir | manual |
| [RUNBOOK.md](RUNBOOK.md) | Operasi: deploy, backup, restore, rollback, gladi/cutover ETL, operasi harian | manual (`scripts/ops/`) |

## Status

- [STATUS.md](STATUS.md) — angka terkini, pekerjaan terbuka, rencana yang aktif.
- [PARITY_AUDIT.md](PARITY_AUDIT.md) — audit paritas kapabilitas yang hidup (terakhir diselaraskan dengan kode
  2026-10-19, §0A). Tabel per modul (§3), yang sengaja tidak dibawa (§6), dan gap yang diterima (§0A.2) adalah acuan;
  §0 dan §10–§12 adalah log pengerjaan bertanggal.

## Riwayat

### Audit bertanggal

| Dokumen | Keterangan |
|---|---|
| [ANALISIS-PARITAS-2026-10-16.md](ANALISIS-PARITAS-2026-10-16.md) | Audit independen dari kode; masukan program penyempurnaan paritas (sudah ditutup) |
| [PARITY_AUDIT.prev.md](PARITY_AUDIT.prev.md) | Versi PARITY_AUDIT per 2026-09-10, digantikan versi sekarang |

### Rencana (`plan/`)

Setiap rencana berada di `plan/<slug>/PLAN.md`, lampirannya di folder yang sama.

| Rencana | Dibuat | Status |
|---|---|---|
| [penutupan-gap-kapabilitas](plan/penutupan-gap-kapabilitas/PLAN.md) | 2026-09-10 | Tahap A–J selesai. Tahap K/L (ETL, cutover) **digantikan** oleh menuju-pengganti-production (ETL kini `apps/api/cmd/etl`) |
| [verifikasi-modul-2026-09-11](plan/verifikasi-modul-2026-09-11/PLAN.md) | 2026-09-11 | Selesai |
| [menuju-pengganti-production](plan/menuju-pengganti-production/PLAN.md) | 2026-10-08 | **Aktif**: Fase 9 (gladi, parallel-run, cutover) dan F8-18 (sqlc `finance`/`stock`). Lampiran rancangan di `design/`; catatan gladi di `CHECKPOINT-2026-10-16.md` |
| [redesain-ui](plan/redesain-ui/PLAN.md) | 2026-10-16 | Selesai kecuali yang butuh perangkat/pemilik: uji tablet kasir (U5-5), logo resmi (U7-7), tema gelap (ditunda). Lihat "Pembaruan eksekusi 2026-10-21". [PANDUAN-MIGRASI-HALAMAN.md](plan/redesain-ui/PANDUAN-MIGRASI-HALAMAN.md) tetap resep kerja yang berlaku |
| [penyempurnaan-paritas](plan/penyempurnaan-paritas/PLAN.md) | 2026-10-18 | Selesai 2026-10-20 (`npm run plan:check`); audit penutup di `AUDIT-Z01.md` |

Rencana yang masih aktif boleh diperbarui di bagian log/checkpoint-nya sendiri. Rencana yang selesai tidak disunting
lagi; pekerjaan baru = rencana baru.

### Analisis sistem lama (`legacy-reference/`)

Analisis lengkap sistem NestJS lama, disimpan di repo dengan sengaja ([ADR-0008](../knowledge/decisions/ADR-0008-legacy-analysis-retained.md)).
Baca-saja: menggambarkan sistem **lama**, bukan revamp.

- `00-overview.md` (ringkasan, temuan lintas modul §6, pola yang layak ditiru §7), `00-module-map.md`, `module-dependency-map.md`
- `NN-<modul>/` — 21 folder modul, masing-masing berisi feature-inventory, business-rules, user-flows, data-model, algoritma,
  penomoran, laporan, ui-ux-spec, test-cases
- `known-issues.md` (197 KI), `open-questions.md`, `shared/` (aturan, model data, layanan bersama)
- `PROGRESS.md` — log proses analisis (historis)

Pemetaan modul revamp → folder legacy: [`knowledge/MODULE_MAP.md`](../knowledge/MODULE_MAP.md) §"Referensi Sistem Lama per Modul".

## Menambah dokumen baru

| Kebutuhan | Tempat |
|---|---|
| Rencana kerja baru | `docs/plan/<slug>/PLAN.md` + baris di tabel di atas |
| Analisis satu kebutuhan sebelum coding | `analysis/<NNN>-<slug>/` (lihat [`analysis/README.md`](../analysis/README.md)) |
| Keputusan arsitektur/produk | ADR baru di `knowledge/decisions/` |
| Konvensi kerja yang berlaku umum | Panduan yang sesuai di `knowledge/` |
| Status / angka | [STATUS.md](STATUS.md) |
