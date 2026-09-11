# ADR-0009: Standalone single-tenant — all multi-tenancy machinery is removed

## Status
Accepted

## Context
The application is owned and run by a single tenant (standalone installation), with
multiple branches allowed. The legacy codebase carried multi-tenancy machinery from an
abandoned multi-tenant design: `id_company` on ~45 entities (seed only ever created
`id_company = 1`), dual-marked system roles (`id_company IS NULL` + boolean), per-company
role scoping, a `companies` table, `companyId` in session/JWT scope, and the `vioni`
(first-tenant) storage prefix (`docs/legacy-reference/shared/shared-services.md` §2.2).
None of it ever served more than one company, but every query, session, and uniqueness
rule paid the complexity cost.

## Decision
Take out everything that exists only for multi-tenancy:

1. **No `company_id` column** on any new table. Base columns are `id`, `branch_id`
   (where branch-bound), audit columns, `deleted_at` (where needed) — see
   `docs/DB_SCHEMA.md` §2.
2. **No `companyId` in session scope or JWT.** Session scope is
   `branchId`/`userId`/role/permission only. Branch isolation is still fully enforced.
3. **No `companies` table.** The `company` module keeps `company_profile` and
   `company_settings` as singleton rows (business need: legal identity on printed
   documents + operational settings) — not as tenant records.
4. **Roles are global**, marked by a single `is_system` flag. No per-company role
   scoping, no NULL-vs-boolean dual marker.
5. **Uniqueness is global**, or per-branch for branch-bound entities (decided per
   module) — never per-company.
6. **No tenant-prefixed storage keys.** Media keys use neutral functional paths
   (`product/…`, `order/transfer/…`), never a tenant name.
7. `company_features` (feature flags) stays out entirely (KI-33).

## Consequences
Simpler queries (one less filter everywhere), no tenant-leak bug class, smaller sessions
and tokens. Legacy data migration maps the single legacy company row onto the singleton
profile — no tenant mapping needed. If multi-tenancy is ever genuinely required, that is
a new architectural decision requiring its own ADR, not a column added back quietly.
Reintroducing any `company_id`-style scoping without an ADR violates this decision
(highest priority in `knowledge/SOURCE_PRIORITY.md`).
