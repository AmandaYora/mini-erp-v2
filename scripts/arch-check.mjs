#!/usr/bin/env node
// Architecture guardrails for the Go modular monolith (PRD §6 poin 9, F8-15).
// Exit 1 when any rule is violated; every offending site is listed.
//
// Rules:
//   A  contracts-only boundary: a module (and anything outside the composition
//      roots) may import another module only through its `contracts/` package;
//      the bare module-root package is not public either (A').
//   B  single-tenant takeout (ADR-0009): no company_id/companyId/idCompany tokens.
//   C  per-module sqlc: sqlc.yaml has one block per SQL-owning module whose
//      schema is exactly that module's baseline migration(s); modules marked
//      "migrated" in scripts/sqlc-migration-status.json must have queries and
//      generated sqlc/db.go; "pending" ones must not (so the status ratchets).
//   D  single HTTP client: axios only inside shared/services/http-client.ts.
//   E  layer direction: domain<-(stdlib, shared/money|timeutil only);
//      infrastructure !-> application|presentation; application !-> presentation
//      and database/sql; presentation !-> infrastructure; shared !-> modules.
//   F  no cross-module FOREIGN KEY in migrations.
//   K  contracts/ never import application|infrastructure|presentation|domain.
//   M  the {"success": envelope literal lives only in shared/response.
//   P  single money kernel (ADR-0010): lineEconomics|CalcLine|prorate only in shared/money.
//   R  no raw QueryContext/ExecContext/... in sqlc-migrated modules (allow-list
//      scripts/raw-sql-allow.json).
//   S  the module dependency graph (auth/user/audit ignored) is acyclic.
//   T  table ownership: a module's SQL may only name tables created by its own
//      baseline migration (000NNN_<module>.up.sql).
//   W  no Set* methods on a module's Module type (late-bound wiring).
//
// Known exceptions live in scripts/arch-allow.json (each needs a reason; a
// stale entry fails the check, so the list only shrinks).
//
// Usage: node scripts/arch-check.mjs [repo-root]
//        node scripts/arch-check.mjs --self-test   (proves every rule fires on fixtures)
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { checkRepo } from "./arch-check/lib.mjs";
import { runSelfTest } from "./arch-check/self-test.mjs";

const args = process.argv.slice(2);
if (args.includes("--self-test")) {
  const { failed, total } = await runSelfTest((l) => process.stdout.write(l + "\n"));
  process.stdout.write(failed ? `\narch-check self-test: ${failed}/${total} FAILED.\n` : `arch-check self-test: OK (${total} cases).\n`);
  process.exit(failed ? 1 : 0);
}

const ROOT = resolve(args.find((a) => !a.startsWith("--")) ?? dirname(dirname(fileURLToPath(import.meta.url))));
const violations = checkRepo(ROOT);
for (const x of violations) process.stdout.write(`  ✗ [${x.rule}] ${x.file}${x.detail ? ` — ${x.detail}` : ""}\n`);
if (violations.length > 0) {
  process.stdout.write(`\narch-check: ${violations.length} violation(s).\n`);
  process.exit(1);
}
process.stdout.write("arch-check: OK (A contracts-only, B no-tenancy, C sqlc, D http-client, E layers, F no cross-FK, K contracts leak, M envelope, P money, R raw SQL, S acyclic, T table ownership, W no setters).\n");
