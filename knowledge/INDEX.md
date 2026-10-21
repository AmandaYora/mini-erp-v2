# Knowledge Index — mini-erp

Pintu masuk basis pengetahuan proyek. Tiga bagian: apa yang dibaca untuk sebuah tugas, file mana
yang **memiliki** sebuah fakta, dan daftar keputusan (ADR).

## 1. Baca sesuai tugas

| Tugas | Baca, berurutan |
|---|---|
| Memahami proyek dari nol | [PROJECT_BRIEF.md](PROJECT_BRIEF.md) → [ARCHITECTURE_OVERVIEW.md](ARCHITECTURE_OVERVIEW.md) → [MODULE_MAP.md](MODULE_MAP.md) → [../docs/STATUS.md](../docs/STATUS.md) |
| Fitur/perubahan di modul tertentu | [MODULE_MAP.md](MODULE_MAP.md) (lapisan, dependensi, folder legacy) → [../docs/SYSTEM_DESIGN.md](../docs/SYSTEM_DESIGN.md) §7 → `docs/legacy-reference/known-issues.md` bagian modul itu → baris modul di [../docs/PARITY_AUDIT.md](../docs/PARITY_AUDIT.md) §3 |
| Endpoint baru / ubah endpoint | [API_GUIDE.md](API_GUIDE.md) → [../docs/API_CONTRACT.md](../docs/API_CONTRACT.md) → [BACKEND_GUIDE.md](BACKEND_GUIDE.md) |
| Tabel/kolom/migrasi | [DATABASE_GUIDE.md](DATABASE_GUIDE.md) → [../docs/DB_SCHEMA.md](../docs/DB_SCHEMA.md) |
| Halaman/komponen frontend | [FRONTEND_GUIDE.md](FRONTEND_GUIDE.md) → [../docs/plan/redesain-ui/PANDUAN-MIGRASI-HALAMAN.md](../docs/plan/redesain-ui/PANDUAN-MIGRASI-HALAMAN.md) |
| Daftar, laporan, impor, apa pun yang berulang per baris | [PERFORMANCE_GUIDE.md](PERFORMANCE_GUIDE.md) |
| Test | [ADR-0015](decisions/ADR-0015-test-strategy.md) → [DATABASE_GUIDE.md](DATABASE_GUIDE.md) §DB test → [../apps/e2e/README.md](../apps/e2e/README.md) |
| Deploy, operasi, cutover | [../docs/DEPLOYMENT.md](../docs/DEPLOYMENT.md) → [../docs/RUNBOOK.md](../docs/RUNBOOK.md) → [../docs/STATUS.md](../docs/STATUS.md) §3 |
| "Fitur X kok tidak ada?" | [../docs/PARITY_AUDIT.md](../docs/PARITY_AUDIT.md) §6 + §0A.2 dan [decisions/](decisions/) — banyak ketiadaan yang disengaja |

## 2. Peta sumber kebenaran

Setiap fakta punya **satu** pemilik. Dokumen lain cukup menautkannya. Bila dua sumber bertentangan,
ikuti [SOURCE_PRIORITY.md](SOURCE_PRIORITY.md) dan laporkan konfliknya.

| Fakta | Pemilik |
|---|---|
| Status, angka (rute/tabel/test/halaman), pekerjaan terbuka | [../docs/STATUS.md](../docs/STATUS.md) |
| Tujuan produk, cakupan, di luar cakupan, keputusan bisnis awal | [../docs/PRD.md](../docs/PRD.md) |
| Stack, kontrak publik per modul, kontrak lintas modul, keamanan | [../docs/SYSTEM_DESIGN.md](../docs/SYSTEM_DESIGN.md) |
| Lapisan & dependensi modul, folder legacy per modul | [MODULE_MAP.md](MODULE_MAP.md) |
| Daftar rute, envelope, permission, variabel environment | [../docs/API_CONTRACT.md](../docs/API_CONTRACT.md) |
| Daftar tabel & pemiliknya, riwayat migrasi | [../docs/DB_SCHEMA.md](../docs/DB_SCHEMA.md) |
| Cara menulis migrasi, sqlc, DB test | [DATABASE_GUIDE.md](DATABASE_GUIDE.md) |
| Struktur modul Go, toolchain, wiring, penjaga arsitektur | [BACKEND_GUIDE.md](BACKEND_GUIDE.md) |
| Konvensi frontend, Design System v2 | [FRONTEND_GUIDE.md](FRONTEND_GUIDE.md) |
| Biaya query, cache in-process, bentuk indeks | [PERFORMANCE_GUIDE.md](PERFORMANCE_GUIDE.md) |
| Istilah domain, role bawaan | [DOMAIN_GLOSSARY.md](DOMAIN_GLOSSARY.md) |
| Keputusan arsitektur & produk | [decisions/](decisions/) (daftar §3) |
| Paritas vs sistem lama, yang sengaja tidak dibawa | [../docs/PARITY_AUDIT.md](../docs/PARITY_AUDIT.md) |
| Deploy & env produksi / operasi harian & cutover | [../docs/DEPLOYMENT.md](../docs/DEPLOYMENT.md) / [../docs/RUNBOOK.md](../docs/RUNBOOK.md) |
| Perilaku sistem lama | `docs/legacy-reference/` (peta: [../docs/README.md](../docs/README.md)) |
| Aturan teknis yang ditegakkan saat menyunting | `.claude/rules/` |

Jenis dokumen di `docs/` (referensi, status, riwayat) dan status tiap rencana: [../docs/README.md](../docs/README.md).

## 3. Keputusan (ADR)

| ADR | Keputusan |
|---|---|
| [0001](decisions/ADR-0001-monorepo-structure.md) | Struktur monorepo `apps/web` + `apps/api` + `packages/` |
| [0002](decisions/ADR-0002-modular-monolith.md) | Backend modular monolith, bukan microservices |
| [0003](decisions/ADR-0003-frontend-standard.md) | Standar frontend (React 19, Tailwind 4, Zustand, Zod, Axios) |
| [0004](decisions/ADR-0004-order-domain-split.md) | Modul `order` lama dipecah menjadi enam modul |
| [0005](decisions/ADR-0005-money-as-integer-rupiah.md) | Uang disimpan sebagai integer rupiah |
| [0006](decisions/ADR-0006-locale-timezone-locked.md) | Locale, zona waktu, mata uang adalah konstanta, bukan setelan |
| [0007](decisions/ADR-0007-pos-no-dedicated-module.md) | POS tidak punya modul backend sendiri |
| [0008](decisions/ADR-0008-legacy-analysis-retained.md) | Analisis sistem lama disimpan di `docs/legacy-reference/` |
| [0009](decisions/ADR-0009-single-tenant-takeout-multitenancy.md) | Standalone single-tenant; tanpa `company_id` |
| [0010](decisions/ADR-0010-shared-money-kernel.md) | `shared/money` sebagai kernel aritmetika uang (pengecualian aturan `shared/`) |
| [0011](decisions/ADR-0011-audit-in-transaction.md) | Tulis audit di dalam transaksi pemanggil (pengecualian terbatas) |
| [0012](decisions/ADR-0012-sqlc-adoption-and-raw-sql-policy.md) | sqlc untuk akses data; SQL tulisan tangan hanya lewat pengecualian |
| [0013](decisions/ADR-0013-twin-modules-stay-separate.md) | Modul kembar tetap terpisah (tanpa mesin dokumen generik) |
| [0014](decisions/ADR-0014-application-layer-transaction-scripts.md) | Application layer = transaction script; `contracts` sekaligus model data |
| [0015](decisions/ADR-0015-test-strategy.md) | Strategi test dan "cukup untuk cutover" |
| [0016](decisions/ADR-0016-atomicity-by-idempotency-and-cas.md) | Atomisitas lintas modul = langkah idempoten + CAS |
| [0017](decisions/ADR-0017-design-system-v2.md) | Design System v2 "Bingkai Navy" di atas shadcn/ui (Radix) |
| [0018](decisions/ADR-0018-keputusan-paritas.md) | Keputusan pemilik D1–D11 untuk program paritas & cutover (D9 tanggal saldo awal, D10 cutover langsung, D11 koreksi akuntansi migrasi) |
| [0019](decisions/ADR-0019-deploy-elcodelabs-ghcr.md) | Deploy produksi ke VPS elcodelabs: image dibangun CI ke GHCR, server hanya pull & run (`minierp`, port 8084) |
| [0020](decisions/ADR-0020-invarian-akuntansi-persediaan.md) | Invarian akuntansi persediaan: tiap mutasi dijurnal sebesar nilai ledger atau netral; kaki cermin, rupiah bulat, akun kendali, rekonsiliasi menjelaskan selisih |

ADR baru: nomor berikutnya, format `## Status / Context / Decision / Consequences`, lalu tambahkan
barisnya di tabel ini. ADR yang diganti tidak dihapus; tandai statusnya `Superseded by ADR-NNNN`.
