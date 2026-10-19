// Self-test for arch-check (F8-15): builds a tiny valid repo in a temp dir,
// proves it passes, then applies one deliberate violation per case and proves
// the expected rule fires. Run with: node scripts/arch-check.mjs --self-test
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { checkRepo } from "./lib.mjs";
import { renderSqlcYaml } from "../sqlc-sync.mjs";

const API = "apps/api";
const M = `${API}/internal/modules`;

/** A minimal repo that satisfies every rule. Keys are repo-relative paths. */
function baseTree() {
  const files = {
    [`${API}/migrations/000001_alpha.up.sql`]: "CREATE TABLE `alpha_items` (id bigint);\n",
    [`${API}/migrations/000002_beta.up.sql`]:
      "CREATE TABLE `beta_items` (id bigint);\nCREATE TABLE `beta_lines` (id bigint, item_id bigint, FOREIGN KEY (item_id) REFERENCES `beta_items` (id));\n",
    [`${API}/internal/shared/response/response.go`]: 'package response\nconst Ok = `{"success":true}`\n',
    [`${API}/internal/shared/money/money.go`]: "package money\nfunc prorate(a int) int { return a }\n",
    [`scripts/sqlc-migration-status.json`]: JSON.stringify({ migrated: [], pending: ["alpha", "beta"], noSql: ["gamma"] }),
    [`${API}/internal/server/server.go`]: `package server\nimport (\n\t"net/http"\n\t_ "mini-erp/internal/modules/alpha/contracts"\n)\nvar _ = http.StatusOK\n`,
    [`${API}/cmd/server/main.go`]: `package main\nimport (\n\t"mini-erp/internal/modules/alpha"\n)\nvar _ = alpha.X\n`,
    [`apps/web/src/shared/services/http-client.ts`]: 'import axios from "axios";\nexport default axios;\n',
    [`apps/web/src/app.ts`]: 'import { x } from "@/shared/services/http-client";\n',
  };
  for (const m of ["alpha", "beta"]) {
    const base = `${M}/${m}`;
    files[`${base}/${m}.module.go`] = `package ${m}\n\nvar X = 1\n\ntype Module struct{}\n\nfunc NewModule() *Module { return &Module{} }\n`;
    files[`${base}/contracts/c.go`] = `package contracts\n\ntype C struct{}\n`;
    files[`${base}/domain/d.go`] = `package domain\n\nimport "strings"\n\nvar _ = strings.ToUpper\n`;
    files[`${base}/application/a.go`] = `package application\n\nimport (\n\t"fmt"\n\n\t"mini-erp/internal/modules/${m}/contracts"\n\t"mini-erp/internal/modules/${m}/domain"\n)\n\nvar _ = fmt.Sprint\nvar _ contracts.C\nvar _ domain.X\n`;
    files[`${base}/infrastructure/r.go`] = `package infrastructure\n\nimport "database/sql"\n\nvar _ sql.DB\n\nconst q = "SELECT id FROM ${m}_items WHERE id = ?"\n`;
    files[`${base}/presentation/p.go`] = `package presentation\n\nimport "mini-erp/internal/modules/${m}/application"\n\nvar _ application.S\n`;
  }
  files[`${M}/gamma/gamma.module.go`] = "package gamma\n\ntype Module struct{}\n";
  return files;
}

/** Writes files + the generated sqlc.yaml, returns the root. */
function materialize(files) {
  const root = mkdtempSync(join(tmpdir(), "arch-selftest-"));
  const put = (rel, content) => {
    const p = join(root, rel);
    mkdirSync(dirname(p), { recursive: true });
    writeFileSync(p, content);
  };
  for (const [rel, content] of Object.entries(files)) if (content !== null) put(rel, content);
  if (!("apps/api/sqlc.yaml" in files)) put("apps/api/sqlc.yaml", renderSqlcYaml(join(root, API)));
  return root;
}

// Each case: name, mutate(files), expect = rule codes that must be reported
// (empty = the tree must be clean).
const cases = [
  { name: "baseline passes", mutate: () => {}, expect: [] },

  { name: "A: module imports another module's application", expect: ["A"],
    mutate: (f) => { f[`${M}/alpha/application/a.go`] += `import _ "mini-erp/internal/modules/beta/application"\n`; } },
  { name: "A': module imports another module's root package", expect: ["A"],
    mutate: (f) => { f[`${M}/alpha/application/a.go`] = f[`${M}/alpha/application/a.go`].replace('import (', 'import (\n\t_ "mini-erp/internal/modules/beta"'); } },
  { name: "A: non-composition-root code imports module internals", expect: ["A"],
    mutate: (f) => { f[`${API}/internal/server/server.go`] = f[`${API}/internal/server/server.go`].replace('/alpha/contracts', '/alpha/application'); } },
  { name: "A: contracts of another module stay allowed", expect: [],
    mutate: (f) => { f[`${M}/alpha/application/a.go`] += `import _ "mini-erp/internal/modules/beta/contracts"\n`; } },

  { name: "B: tenancy token", expect: ["B"],
    mutate: (f) => { f[`${M}/alpha/application/a.go`] += "var company_id = 1\n"; } },
  { name: "B: tenancy token in a comment is fine", expect: [],
    mutate: (f) => { f[`${M}/alpha/application/a.go`] += "// no company_id (ADR-0009)\n"; } },

  { name: "C: sqlc status file missing", expect: ["C"], mutate: (f) => { f["scripts/sqlc-migration-status.json"] = null; } },
  { name: "C: module not classified in the status file", expect: ["C"],
    mutate: (f) => { f["scripts/sqlc-migration-status.json"] = JSON.stringify({ migrated: [], pending: ["alpha"], noSql: ["gamma"] }); } },
  { name: "C: module with tables listed noSql", expect: ["C"],
    mutate: (f) => { f["scripts/sqlc-migration-status.json"] = JSON.stringify({ migrated: [], pending: ["alpha"], noSql: ["gamma", "beta"] }); } },
  { name: "C: sqlc.yaml schema points at all migrations", expect: ["C"],
    mutate: (f) => { f["apps/api/sqlc.yaml"] = 'version: "2"\nsql:\n  - engine: mysql\n    schema: migrations\n    queries: internal/modules/alpha/infrastructure/queries\n    gen:\n      go:\n        out: internal/modules/alpha/infrastructure/sqlc\n'; } },
  { name: "C: block reads queries of one module, writes another", expect: ["C"],
    mutate: (f) => { f["apps/api/sqlc.yaml"] = 'version: "2"\nsql:\n  - engine: mysql\n    schema:\n      - migrations/000001_alpha.up.sql\n    queries: internal/modules/alpha/infrastructure/queries\n    gen:\n      go:\n        out: internal/modules/beta/infrastructure/sqlc\n'; } },
  { name: "C: migrated module without generated files", expect: ["C"],
    mutate: (f) => { f["scripts/sqlc-migration-status.json"] = JSON.stringify({ migrated: ["alpha"], pending: ["beta"], noSql: ["gamma"] }); } },
  { name: "C: pending module that already has generated output", expect: ["C"],
    mutate: (f) => {
      f[`${M}/alpha/infrastructure/queries/q.sql`] = "-- name: Get :one\nSELECT id FROM alpha_items;\n";
      f[`${M}/alpha/infrastructure/sqlc/db.go`] = "// Code generated by sqlc. DO NOT EDIT.\npackage sqlc\n";
    } },
  { name: "C: migrated module with queries and generated db.go passes", expect: [],
    mutate: (f) => {
      f["scripts/sqlc-migration-status.json"] = JSON.stringify({ migrated: ["alpha"], pending: ["beta"], noSql: ["gamma"] });
      f[`${M}/alpha/infrastructure/queries/q.sql`] = "-- name: Get :one\nSELECT id FROM alpha_items;\n";
      f[`${M}/alpha/infrastructure/sqlc/db.go`] = "// Code generated by sqlc. DO NOT EDIT.\npackage sqlc\n";
      f[`${M}/alpha/infrastructure/r.go`] = "package infrastructure\n";
    } },

  { name: "D: axios imported outside http-client", expect: ["D"],
    mutate: (f) => { f["apps/web/src/app.ts"] = 'import axios from "axios";\n'; } },

  { name: "E: domain imports a third-party package", expect: ["E"],
    mutate: (f) => { f[`${M}/alpha/domain/d.go`] = 'package domain\n\nimport "github.com/x/y"\n\nvar _ = y.Z\n'; } },
  { name: "E: domain imports another layer", expect: ["E"],
    mutate: (f) => { f[`${M}/alpha/domain/d.go`] = 'package domain\n\nimport "mini-erp/internal/modules/alpha/infrastructure"\n\nvar _ infrastructure.R\n'; } },
  { name: "E: infrastructure imports application", expect: ["E"],
    mutate: (f) => { f[`${M}/alpha/infrastructure/r.go`] += 'import _ "mini-erp/internal/modules/alpha/application"\n'; } },
  { name: "E: application imports database/sql", expect: ["E"],
    mutate: (f) => { f[`${M}/alpha/application/a.go`] += 'import _ "database/sql"\n'; } },
  { name: "E: application imports presentation", expect: ["E"],
    mutate: (f) => { f[`${M}/alpha/application/a.go`] += 'import _ "mini-erp/internal/modules/alpha/presentation"\n'; } },
  { name: "E: presentation imports infrastructure", expect: ["E"],
    mutate: (f) => { f[`${M}/alpha/presentation/p.go`] += 'import _ "mini-erp/internal/modules/alpha/infrastructure"\n'; } },
  { name: "E: shared imports a module", expect: ["E"],
    mutate: (f) => { f[`${API}/internal/shared/money/money.go`] += 'import _ "mini-erp/internal/modules/alpha/contracts"\n'; } },

  { name: "F: cross-module FOREIGN KEY", expect: ["F"],
    mutate: (f) => { f[`${API}/migrations/000002_beta.up.sql`] += "ALTER TABLE `beta_lines` ADD FOREIGN KEY (a) REFERENCES `alpha_items` (id);\n"; } },
  { name: "F: REFERENCES to an unknown table", expect: ["F"],
    mutate: (f) => { f[`${API}/migrations/000002_beta.up.sql`] += "ALTER TABLE `beta_lines` ADD FOREIGN KEY (a) REFERENCES `ghost` (id);\n"; } },

  { name: "K: contracts import their own application", expect: ["K"],
    mutate: (f) => { f[`${M}/alpha/contracts/c.go`] += 'import _ "mini-erp/internal/modules/alpha/application"\n'; } },

  { name: "M: hand-written envelope literal", expect: ["M"],
    mutate: (f) => { f[`${M}/alpha/presentation/p.go`] += 'const body = `{"success":false,"message":"x"}`\n'; } },

  { name: "P: money math outside shared/money", expect: ["P"],
    mutate: (f) => { f[`${M}/alpha/application/a.go`] += "func lineEconomics() {}\n"; } },

  { name: "R: raw SQL call in a migrated module", expect: ["R"],
    mutate: (f) => {
      f["scripts/sqlc-migration-status.json"] = JSON.stringify({ migrated: ["alpha"], pending: ["beta"], noSql: ["gamma"] });
      f[`${M}/alpha/infrastructure/queries/q.sql`] = "-- name: Get :one\nSELECT id FROM alpha_items;\n";
      f[`${M}/alpha/infrastructure/sqlc/db.go`] = "// Code generated by sqlc. DO NOT EDIT.\npackage sqlc\n";
      f[`${M}/alpha/infrastructure/r.go`] = "package infrastructure\nfunc f(db DB) { db.QueryContext(nil, \"x\") }\n";
    } },
  { name: "R: raw SQL allowed by raw-sql-allow.json", expect: [],
    mutate: (f) => {
      f["scripts/sqlc-migration-status.json"] = JSON.stringify({ migrated: ["alpha"], pending: ["beta"], noSql: ["gamma"] });
      f[`${M}/alpha/infrastructure/queries/q.sql`] = "-- name: Get :one\nSELECT id FROM alpha_items;\n";
      f[`${M}/alpha/infrastructure/sqlc/db.go`] = "// Code generated by sqlc. DO NOT EDIT.\npackage sqlc\n";
      f[`${M}/alpha/infrastructure/r.go`] = "package infrastructure\nfunc f(db DB) { db.QueryContext(nil, \"x\") }\n";
      f["scripts/raw-sql-allow.json"] = JSON.stringify([{ file: `${M}/alpha/infrastructure/r.go`, reason: "dynamic columns" }]);
    } },

  { name: "S: module dependency cycle", expect: ["S"],
    mutate: (f) => {
      f[`${M}/alpha/application/a.go`] += 'import _ "mini-erp/internal/modules/beta/contracts"\n';
      f[`${M}/beta/application/a.go`] += 'import _ "mini-erp/internal/modules/alpha/contracts"\n';
    } },
  { name: "S: contracts-level cycle", expect: ["S"],
    mutate: (f) => {
      f[`${M}/alpha/contracts/c.go`] += 'import _ "mini-erp/internal/modules/beta/contracts"\n';
      f[`${M}/beta/contracts/c.go`] += 'import _ "mini-erp/internal/modules/alpha/contracts"\n';
    } },
  { name: "S: one-directional dependency is fine", expect: [],
    mutate: (f) => { f[`${M}/alpha/application/a.go`] += 'import _ "mini-erp/internal/modules/beta/contracts"\n'; } },

  { name: "T: module queries another module's table (Go string)", expect: ["T"],
    mutate: (f) => { f[`${M}/alpha/infrastructure/r.go`] += 'const q2 = "SELECT x FROM alpha_items a JOIN beta_items b ON b.id = a.id"\n'; } },
  { name: "T: module queries another module's table (queries/*.sql)", expect: ["T"],
    mutate: (f) => { f[`${M}/alpha/infrastructure/queries/q.sql`] = "-- name: X :one\nSELECT 1 FROM beta_lines;\n"; } },
  { name: "T: migration for a module that does not exist", expect: ["T"],
    mutate: (f) => { f[`${API}/migrations/000003_ghost.up.sql`] = "CREATE TABLE `ghost_t` (id bigint);\n"; } },
  { name: "T: a module may touch its own tables", expect: [],
    mutate: (f) => { f[`${M}/alpha/infrastructure/r.go`] += 'const q3 = "UPDATE alpha_items SET id = 1"\n'; } },

  { name: "W: Set* method on Module", expect: ["W"],
    mutate: (f) => { f[`${M}/alpha/alpha.module.go`] += "\nfunc (m *Module) SetPayments(p int) {}\n"; } },
  { name: "W: Set* allowed by arch-allow.json with a reason", expect: [],
    mutate: (f) => {
      f[`${M}/alpha/alpha.module.go`] += "\nfunc (m *Module) SetPayments(p int) {}\n";
      f["scripts/arch-allow.json"] = JSON.stringify({ allow: [{ rule: "W", file: `${M}/alpha/alpha.module.go`, match: "SetPayments", reason: "cycle" }] });
    } },
  { name: "ALLOW: stale allow entry fails", expect: ["ALLOW"],
    mutate: (f) => { f["scripts/arch-allow.json"] = JSON.stringify({ allow: [{ rule: "W", file: `${M}/alpha/alpha.module.go`, match: "SetGone", reason: "old" }] }); } },
  { name: "ALLOW: entry without a reason fails", expect: ["ALLOW"],
    mutate: (f) => {
      f[`${M}/alpha/alpha.module.go`] += "\nfunc (m *Module) SetPayments(p int) {}\n";
      f["scripts/arch-allow.json"] = JSON.stringify({ allow: [{ rule: "W", file: `${M}/alpha/alpha.module.go`, match: "SetPayments", reason: "" }] });
    } },
];

export async function runSelfTest(log = console.log) {
  let failed = 0;
  for (const c of cases) {
    const files = baseTree();
    c.mutate(files);
    const root = materialize(files);
    let got;
    try {
      got = checkRepo(root);
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
    const rules = new Set(got.map((x) => x.rule));
    let ok;
    if (c.expect.length === 0) ok = got.length === 0;
    else ok = c.expect.every((r) => rules.has(r));
    if (ok) log(`  ok   ${c.name}`);
    else {
      failed++;
      log(`  FAIL ${c.name}: expected ${c.expect.length ? c.expect.join(",") : "no violations"}, got ${got.map((x) => `[${x.rule}] ${x.file} ${x.detail}`).join(" | ") || "none"}`);
    }
  }
  return { failed, total: cases.length };
}
