# mini-erp — Claude Code Gateway

**mini-erp** — revamp of a single-company, multi-branch operational ERP (purchasing,
stock/warehouse, sales & POS, delivery, payment, finance/tax, reporting) previously built in
NestJS/TypeORM, now rebuilt as a Go modular monolith + MySQL. Backend: Go (`apps/api`). Frontend:
React 19 + Tailwind 4 (`apps/web`).

**Status: 20 backend modules implemented** (~60k lines Go in `internal/modules`, 244 routes, 18 per-module baseline
migrations + numbered migrations 000019–000023/000031 / 65 tables) and 85 frontend pages. **sqlc: incremental migration
(ADR-0012)** — 16 of 18 table-owning modules migrated (`finance`, `stock` still `pending`; `scripts/sqlc-migration-status.json`
is the ratchet), hand-written `database/sql` remains only in `pending` modules or `scripts/raw-sql-allow.json` exceptions.
Capability parity with the legacy system is **~98%** (100% endpoint coverage of the 205 target); the independent audit of
2026-10-16 found deeper gaps and the parity-completion program (`docs/plan/penyempurnaan-paritas/PLAN.md`,
`npm run plan:check`) has closed waves 1-3 — what is still open is listed in `docs/PARITY_AUDIT.md` §0A.2. The remaining work is
the program, not capability: legacy-data ETL (stage K), cutover rehearsal (stage L), extra tests (PLAN T-01..T-05), and physical print verification.
Finance P0/P1 landed via table-less revisions, and J2/J3/J4 (QR labels, delivery work queue, one-step
location move) closed 2026-09-11. Stage 1 of the closing plan is done: journal lines
carry product/party dimensions, and margin-per-product plus receivables/payables now report from
the books (three posting bugs were found and fixed on the way — see the audit's §10). All of it is listed with file:line evidence and a dependency-ordered closing plan in
[`docs/PARITY_AUDIT.md`](docs/PARITY_AUDIT.md). Read that before assuming a feature is missing by
design — several things that look absent are deliberate, and the figures above go stale fast.
Module order and boundaries: `knowledge/MODULE_MAP.md`.

## Commands (run from root)

```bash
npm run dev:web   # start frontend
npm run dev:api   # start backend
npm run arch:check && npm run contract:check   # architecture + FE<->router guards (npm run verify = everything)
node scripts/test-db.mjs up   # pastikan MySQL + mini_erp_test sebelum verify/test
```

## Sebelum Mengerjakan Apa Pun

1. Baca [`knowledge/INDEX.md`](knowledge/INDEX.md) — indeks lengkap dan routing task→domain.
2. Baca [`.claude/rules/`](.claude/rules/) yang relevan — ditegakkan saat menyunting kode.
3. Baca [`knowledge/MODULE_MAP.md`](knowledge/MODULE_MAP.md) untuk urutan pengerjaan modul dan
   dokumen legacy yang relevan per modul.
4. Sebelum mendesain sebuah modul: baca bagiannya di
   [`docs/PRD.md` §5](docs/PRD.md#5-keputusan-bisnis-yang-perlu-dikonfirmasi-sebelumselagi-desain-modul)
   (keputusan bisnis terbuka) dan `docs/legacy-reference/known-issues.md` (perbaiki vs replikasi).

## Critical architecture rules

- Backend is a **modular monolith**. A module exposes only `contracts/` to other modules.
  No cross-module service/repository/domain imports, no cross-module DB joins or foreign keys.
- Cross-module relations are stored as primitive IDs and resolved via module clients.
- Session scope (`branchId`/`userId`/role/permission) always comes from the
  authenticated session (`auth` module) — never from request body/query.
  Standalone single-tenant: no `companyId` anywhere (see
  `knowledge/decisions/ADR-0009-single-tenant-takeout-multitenancy.md`).
- Money is stored as integer rupiah. Timezone/locale/currency (`Asia/Jakarta`/`id-ID`/IDR) are
  locked application constants, not editable settings — see
  `knowledge/decisions/ADR-0005-money-as-integer-rupiah.md` and
  `knowledge/decisions/ADR-0006-locale-timezone-locked.md`.
- API responses always use the `{success, message, data}` envelope; `message` is never discarded
  by the frontend.
- Frontend uses the `@/*` alias, lazy routes, Zustand, Zod, Axios. Theme color is centralized.
- Docker = one app container; the database runs on the host.

## Before changing code

Read the relevant file in `knowledge/` first (start at `knowledge/INDEX.md`), and follow
the path-scoped rules in `.claude/rules/`. Do not duplicate that knowledge here — this file
is a gateway, not a documentation dump.
