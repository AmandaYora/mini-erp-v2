# mini-erp

Revamp of a single-company, multi-branch operational ERP — purchasing, stock/warehouse, sales &
POS, delivery, payment, finance/tax, reporting, and a WhatsApp operations bot. Rebuilt from a
NestJS/TypeORM system into a **Go modular monolith + MySQL** with a **React 19 + Tailwind 4** frontend.

All 20 backend modules are implemented; the remaining work is the cutover program (legacy-data ETL,
rehearsal, physical print checks). Current figures and open work: [`docs/STATUS.md`](docs/STATUS.md).

## Development

Requires **Go 1.26** (toolchain pinned in `apps/api/go.mod`), **Node 22+** (`.nvmrc`; the pinned vitest/jsdom refuse Node 20), and **MySQL 8.0+** on the host (production: 8.0.46).
Tool install commands: [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) §1.

```bash
cp .env.example .env     # then set DB_DSN and a real JWT_SECRET (min. 32 chars)
npm install
npm run migrate:up       # apply database migrations
```

Run frontend and backend separately from the project root:

```bash
npm run dev:web          # Vite dev server
npm run dev:api          # Go API via air
```

Seed a development dataset (catalog, system roles, chart, head branch):

```bash
cd apps/api && go run ./cmd/seed init   # idempotent
go run ./cmd/seed admin                 # first owner user (prints a generated
                                        # password once, or set BOOTSTRAP_ADMIN_PASSWORD)
```

## Verification

```bash
node scripts/test-db.mjs up   # MySQL + mini_erp_test for integration tests
npm run verify                # go build/vet/test (incl. integration), tsc, eslint, vitest, arch/contract/schema checks
npm run verify:fast           # skips Go integration tests and Vitest (no MySQL needed)
npm run test:e2e              # Playwright suite, see apps/e2e/README.md
```

## Architecture

A **modular monolith**, not microservices: modules talk only through `contracts/`, with no cross-module
joins or foreign keys — checked by `scripts/arch-check.mjs`, while `scripts/contract-check.mjs` keeps
frontend calls in line with the router. Session-derived scope, integer-rupiah money, and the
`{success, message, data}` envelope apply everywhere. Summary:
[`knowledge/ARCHITECTURE_OVERVIEW.md`](knowledge/ARCHITECTURE_OVERVIEW.md); decisions:
[`knowledge/decisions/`](knowledge/decisions/).

## Structure

- `apps/web` — React frontend · `apps/api` — Go backend · `apps/e2e` — Playwright suite
- `packages/` — shared workspace code
- `knowledge/` — project knowledge base; start at [`knowledge/INDEX.md`](knowledge/INDEX.md)
- `docs/` — product, design, API, schema, deployment, runbook, status, plans; map in [`docs/README.md`](docs/README.md)
- `docs/legacy-reference/` — full analysis of the previous NestJS system
- `analysis/` — per-requirement analysis artefacts
- `.claude/rules/` — path-scoped rules for AI-assisted editing
- `infra/` — Docker and nginx · `scripts/` — checks, migrations, ops

## Deployment

One Docker app container serving both the static frontend and the API on port 8080 (published on the VPS as `127.0.0.1:8084`, see `docs/DEPLOYMENT.md`). The database
runs on the host — see [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) and [`docs/RUNBOOK.md`](docs/RUNBOOK.md).
