# Status Proyek — mini-erp revamp

**Satu-satunya tempat** untuk angka, posisi pekerjaan, dan daftar yang masih terbuka. Dokumen lain
merujuk ke sini, tidak menyalin isinya. Saat angka di bawah berubah, perbarui file ini (dan tanggalnya)
— jangan menulis angka di `CLAUDE.md`, `README.md`, atau `knowledge/`.

**Terakhir diukur:** 2026-10-21, dari kode di `main`.

## 1. Posisi singkat

- **Kapabilitas:** seluruh 20 modul backend, frontend, POS, dan bot WhatsApp terimplementasi. Paritas
  kapabilitas terhadap sistem lama **±98%**; rinciannya, termasuk yang sengaja tidak dibawa, ada di
  [PARITY_AUDIT.md](PARITY_AUDIT.md) (§1 ringkasan, §6 yang sengaja tidak dibawa).
- **Program penyempurnaan paritas** (audit 2026-10-16) **selesai** 2026-10-20: 96/96 tugas
  (`npm run plan:check`), lihat [plan/penyempurnaan-paritas/PLAN.md](plan/penyempurnaan-paritas/PLAN.md).
- **Yang tersisa adalah program cutover**, bukan kapabilitas — lihat §3.

## 2. Angka (dan cara menghitung ulang)

| Ukuran | Nilai | Perintah |
|---|---|---|
| Modul backend | 20 | `ls apps/api/internal/modules` |
| Rute `/api/v1` | 244 (291 panggilan frontend, semua cocok; 22 rute tidak dipanggil web) | `node scripts/contract-check.mjs` (`--list` untuk rinci) |
| Kode izin | 81 | `node scripts/contract-check.mjs` |
| Tabel | 65 (sistem lama: 70) | `node scripts/db-schema-doc.mjs` → [DB_SCHEMA.md](DB_SCHEMA.md) §3 |
| Berkas migrasi | 24 = 18 baseline per modul + `000019`–`000023`, `000031` (nomor 24–30 sengaja kosong) | `ls apps/api/migrations/*.up.sql` |
| sqlc | 16 dari 18 modul bertabel `migrated`; `finance` dan `stock` masih `pending` | `scripts/sqlc-migration-status.json` |
| Halaman frontend | 85 berkas `*Page.tsx` | `find apps/web/src -name '*Page.tsx' ! -name '*.test.tsx'` |
| Go non-test (`internal/modules`) | ±74 ribu baris | `find apps/api/internal/modules -name '*.go' ! -name '*_test.go' \| xargs cat \| wc -l` |
| Test backend | 179 berkas `*_test.go` (112 di `internal/integration`, MySQL nyata) | `find apps/api -name '*_test.go'` |
| Test frontend | 172 berkas (942 test pada gerbang terakhir) | `find apps/web/src -name '*.test.ts*'` |
| Spec E2E | 19 (E01–E16, V00 visual, V01 a11y, V02 responsif); gerbang terakhir 100 lulus / 0 dilewati / 0 gagal | `ls apps/e2e/specs` |

Gerbang penuh terakhir: 2026-10-19/20 (`node scripts/verify.mjs`, `verify.mjs --web`, `npm run test:e2e`).
Gerbang 2026-10-21 hijau: `verify.mjs --api` termasuk integrasi Go, vitest, dan `npm run test:e2e`.
Toolchain web butuh Node 22+ (`.nvmrc`; CI ikut dinaikkan).
Sebelum rilis, ulangi sekali pada pohon kerja yang bersih.

## 3. Yang masih terbuka

| Alur kerja | Status | Sumber kebenaran |
|---|---|---|
| **Cutover**: ETL data produksi (Tahap K, `apps/api/cmd/etl`), gladi L1/L2, parallel-run atau cutover langsung, rollback | Gladi L1 pada dump produksi lulus 2026-10-16; determinisme L2 belum terbukti per checkpoint itu | Prosedur: [RUNBOOK.md](RUNBOOK.md) §5b/§5c. Rencana: [plan/menuju-pengganti-production/PLAN.md](plan/menuju-pengganti-production/PLAN.md) §7 Fase 9, §9. Catatan gladi: [CHECKPOINT-2026-10-16](plan/menuju-pengganti-production/CHECKPOINT-2026-10-16.md) |
| Verifikasi cetak fisik (5 dokumen dot-matrix, Surat Transfer, struk) + UAT per role | Belum — dikerjakan pemilik | menuju-pengganti-production PLAN §7 Fase 9 |
| **Deploy ke VPS elcodelabs** (`minierp`, port 8084, image GHCR) | Repo sudah sesuai `SERVER_PLAYBOOK` (2026-10-21). CLI migrate + server mode produksi teruji lokal. Belum dikerjakan: image pertama dari CI (perlu push ke `main`), setup server (DB/user, `~/minierp`, nginx, TLS), keputusan domain staging | [ADR-0019](../knowledge/decisions/ADR-0019-deploy-elcodelabs-ghcr.md), [DEPLOYMENT.md](DEPLOYMENT.md) §4 |
| Bangun ulang APK kasir bila domain `POS_URL` berubah | Saat cutover | [DEPLOYMENT.md](DEPLOYMENT.md) §7 |
| Konfirmasi pemilik/akuntan atas keputusan *default* (D1–D8; Dr/Cr saldo kredit retur K-04a; jurnal pemakaian uang muka N-41; PR-08; AS-01) | Default dipakai, menunggu konfirmasi | [ADR-0018](../knowledge/decisions/ADR-0018-keputusan-paritas.md), [PARITY_AUDIT.md](PARITY_AUDIT.md) §0A.2 |
| sqlc untuk `finance` dan `stock` | Terbuka, fase terakhir | [ADR-0012](../knowledge/decisions/ADR-0012-sqlc-adoption-and-raw-sql-policy.md) |
| Sisa redesain UI: uji tablet kasir sungguhan (U5-5 + uji sentuh POS bagian U7-2), logo resmi (U7-7); tema gelap ditunda. Fase 6 dan U7-2/U7-3 (keyboard E15, responsif V02) selesai 2026-10-21 | Menunggu pemilik/perangkat | [plan/redesain-ui/PLAN.md](plan/redesain-ui/PLAN.md) "Pembaruan eksekusi 2026-10-21" |
| Keterbatasan yang dipantau: DM WhatsApp format LID tanpa `SenderAlt` (N-44); `Readiness`/`VerifyRollups` menjalankan `Sync` ledger (menulis, idempoten); `AverageUnitCost` per posisi pada stok rusak (ST-03) | Dipantau | [PARITY_AUDIT.md](PARITY_AUDIT.md) §0A.2 |

Daftar rinci yang **diterima sebagai gap** (beserta alasannya) ada di [PARITY_AUDIT.md](PARITY_AUDIT.md) §0A.2.
