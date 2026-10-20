# mini-erp — Claude Code Gateway

**mini-erp** is the revamp of a single-company, multi-branch operational ERP (purchasing,
stock/warehouse, sales & POS, delivery, payment, finance/tax, reporting, WhatsApp bot). The legacy
NestJS/TypeORM system is being replaced by a **Go modular monolith + MySQL** (`apps/api`) and a
**React 19 + Tailwind 4** frontend (`apps/web`). All 20 backend modules are implemented; what
remains is the cutover program (legacy-data ETL, rehearsal, physical print checks).

Current figures, open work, and the active plans live in **one place only:
[`docs/STATUS.md`](docs/STATUS.md)**. Do not copy figures (route/page/table/test counts, parity %)
into any other file — they go stale.

## Commands (run from the repo root)

```bash
npm run dev:web                    # frontend (Vite)
npm run dev:api                    # backend (Go via air)
node scripts/test-db.mjs up        # make sure MySQL + mini_erp_test exist before tests
npm run verify                     # everything: go build/vet/test, tsc, eslint, vitest, arch/contract/schema checks
npm run arch:check && npm run contract:check   # fast architecture + FE<->router guards
```

## Before working on anything

1. [`knowledge/INDEX.md`](knowledge/INDEX.md) — reading order per task and the **source-of-truth map**
   (which file owns which fact).
2. The relevant `.claude/rules/*.md` — loaded automatically for the paths they cover.
3. [`knowledge/MODULE_MAP.md`](knowledge/MODULE_MAP.md) — module layers, boundaries, legacy docs per module.
4. [`docs/PARITY_AUDIT.md`](docs/PARITY_AUDIT.md) §6 and `knowledge/decisions/` before assuming a
   feature is missing — several absences are deliberate.

## Critical architecture rules (never break)

- **Modular monolith.** A module exposes only `contracts/`. No cross-module imports of
  `application/`, `domain/`, `infrastructure/`; no cross-module SQL joins or foreign keys.
  Cross-module relations are primitive IDs resolved through module clients.
- **Session scope** (`branchId`/`userId`/role/permission) always comes from the `auth` session —
  never from request body/query. Standalone single-tenant: no `companyId` anywhere (ADR-0009).
- **Money is integer rupiah** (ADR-0005). Timezone/locale/currency `Asia/Jakarta`/`id-ID`/IDR are
  locked constants, not settings (ADR-0006).
- **API envelope** `{success, message, data}` under `/api/v1`; the frontend never discards `message`.
- Frontend: `@/*` alias, lazy routes, Zustand, Zod, one Axios instance, theme tokens only.
- Deployment: one app container; MySQL runs on the host.

`scripts/arch-check.mjs` mechanically checks the boundary, no-`company_id`, envelope, money-kernel and
single-Axios rules (rule list in its header); `scripts/contract-check.mjs` matches frontend calls and
permission codes to the router. Session scope and integer money are kept by review and tests.

## Keeping documentation consistent

When code changes, update the **one** owning file (see the map in `knowledge/INDEX.md`), and link to
it from elsewhere instead of restating it.

| You changed… | Update |
|---|---|
| An API route | `docs/API_CONTRACT.md` §6 by hand (`contract:check` only matches frontend calls to the router, not this table) |
| A migration / table | `node scripts/db-schema-doc.mjs` regenerates `docs/DB_SCHEMA.md` §3; add the §8 history row |
| An architectural or product decision | New/revised ADR in `knowledge/decisions/` + its line in `knowledge/INDEX.md` |
| A domain term | `knowledge/DOMAIN_GLOSSARY.md` |
| An audit action key | `apps/web/src/modules/audit/lib/audit-labels.ts` (its test fails otherwise) |
| Project status, counts, open work | `docs/STATUS.md` only |

Plans in `docs/plan/` and dated audits are **history**: read them for context, never treat them as the
current state, and do not rewrite them (append to their own log sections if a plan is still active).
Preserve each file's existing line endings (most files are LF, a few are CRLF — check with `git ls-files --eol`).
