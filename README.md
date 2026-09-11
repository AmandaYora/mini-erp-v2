# mini-erp

Revamp of a single-company, multi-branch operational ERP — purchasing, stock/warehouse, sales &
POS, delivery, payment, finance/tax, and reporting. Rebuilt from a NestJS/TypeORM system into a
**Go modular monolith + MySQL** with a **React 19 + Tailwind 4** frontend.

## Status

**20 backend modules implemented · capability parity with the legacy system ≈98%.**

| | |
|---|---|
| Backend | 217 endpoints, 26 migrations, ~33k lines Go (non-test) |
| Frontend | 65 pages, all routed, ~39k lines TS/TSX (non-test) |
| Tests | 73 backend (35 of them integration against a real MySQL) · 85 frontend |
| UI parity | Design tokens ported identically — verified by `diff`, zero difference |

**Not done yet:** legacy-data ETL (stage K), cutover rehearsal (stage L), E2E tests, and physical
print verification. These are program work, not missing capability — see
[`docs/PARITY_AUDIT.md`](docs/PARITY_AUDIT.md) for the evidence-backed breakdown and
[`docs/plan/`](docs/plan/) for the ordered closing plan.

> The figures above go stale fast. Regenerate them with the commands in `docs/PARITY_AUDIT.md` §2
> before quoting them anywhere.

## Development

Requires **Go 1.26+**, **Node 22+**, and a **MySQL 8** instance on the host.

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

Seed a development dataset (owner role, head branch, chart of accounts):

```bash
cd apps/api && go run ./cmd/seed
```

## Verification

```bash
npm run verify           # go build + vet + test, tsc, eslint, vitest, arch-check
```

Integration tests need a reachable MySQL and are run separately:

```bash
cd apps/api && go test -count=1 ./internal/integration/...
```

## Architecture

A **modular monolith**, not microservices. Boundaries are enforced by the compiler and by
`scripts/arch-check.mjs`, not by convention:

- A module exposes only `contracts/` to other modules — no cross-module service, repository, or
  domain imports.
- No cross-module SQL joins and no cross-module foreign keys. Cross-module relations are stored as
  primitive IDs and resolved through module clients.
- Session scope (branch, user, role, permission) always comes from the authenticated session,
  never from a request body or query string.
- Money is stored as **integer rupiah**. Locale, timezone, and currency (`id-ID` / `Asia/Jakarta` /
  IDR) are locked application constants, not editable settings.
- Every API response uses the `{ success, message, data }` envelope, and the frontend never
  discards `message`.

Decisions and their reasoning live in [`knowledge/decisions/`](knowledge/decisions/) as ADRs.

## Structure

- `apps/web` — React 19 + Tailwind 4 frontend
- `apps/api` — Go modular-monolith backend
- `packages/` — shared code and API contract
- `knowledge/` — project knowledge base (start at `knowledge/INDEX.md`; read before editing)
- `.claude/rules/` — path-scoped technical rules, enforced while editing
- `docs/` — PRD, system design, API contract, DB schema, deployment, parity audit
- `docs/legacy-reference/` — full analysis of the previous NestJS system (21 modules, known
  issues, open questions) — required reading before designing any module
- `infra/` — Docker and nginx

## Deployment

One Docker app container serving both the static frontend and the API on port 8080. The database
runs on the host — see [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md).
