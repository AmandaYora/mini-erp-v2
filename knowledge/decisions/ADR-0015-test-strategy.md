# ADR-0015: test strategy and "enough for cutover"

## Status
Accepted (F8-6, 2026-10-09)

## Decision
1. **Unit tests** for pure rules (`shared/money`, domain, dberr, doclock, sequence
   keys) — no database.
2. **Integration tests** (`apps/api/internal/integration`) run the production
   module graph (`bootstrap.Wire`) against the dedicated test database
   `mini_erp_test` (restricted user, DSN guard: the name must end in `_test`).
   They TRUNCATE the database, so concurrent runs must be serialised
   (`scripts/db-lock.mjs`). Every money, stock and concurrency rule has a test
   that fails without it: two simultaneous clicks, retry after a failure in the
   middle, quota races, replay, CAS losers.
3. **Contract/architecture checks** (`scripts/contract-check.mjs`,
   `scripts/arch-check.mjs`) are part of `scripts/verify.mjs` and CI.
4. **E2E** (Playwright, a handful of meaningful flows) runs against a server
   started on the test database, never against development data.
5. Frontend: `tsc`, `eslint`, `vitest` must be clean.

## "Enough for cutover"
Every posting source has a test of debit = credit, sub-ledger = book balance and
net receivable/payable; `VerifyRollups` is green; no silent row limits; the
ETL reconciliation test detects a deliberately corrupted row; the full suite
(`go test ./... -timeout=40m`) is green. Physical print verification and the
parallel run are owner tasks outside the automated suite.
