#!/usr/bin/env node
// Architecture guardrails for the Go modular monolith (PRD §6 poin 9).
// Fails (exit 1) on the first violated rule, listing every offending site.
//
// Rules:
//   A. contracts-only boundary — a .go file inside internal/modules/<m>/ may only
//      import another module's `contracts/` package. Same-module imports are free,
//      as are shared/, config/, database/, router/, server/.
//   B. single-tenant takeout (ADR-0009) — `company_id`, `companyId`, `idCompany`
//      must not appear anywhere under apps/api (docs may discuss legacy).
//   C. per-module sqlc — every `sql:` block in sqlc.yaml must read queries from
//      and write generated code to the SAME module's infrastructure/ dir, so no
//      module can import another module's generated queries.
//   D. single HTTP client — `axios` may only be imported by
//      apps/web/src/shared/services/http-client.ts.
//
// Usage: node scripts/arch-check.mjs [repo-root]   (default: repo root)
import { readFileSync, readdirSync, statSync, existsSync } from "node:fs";
import { join, sep, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = process.argv[2] ?? dirname(dirname(fileURLToPath(import.meta.url)));
const API = join(ROOT, "apps/api");
const WEB = join(ROOT, "apps/web");
const MODULES = join(API, "internal/modules");

let failures = 0;
function bad(rule, file, detail) {
  process.stdout.write(`  ✗ [${rule}] ${file}${detail ? ` — ${detail}` : ""}\n`);
  failures++;
}

function walk(dir, out = []) {
  for (const e of readdirSync(dir)) {
    const p = join(dir, e);
    if (statSync(p).isDirectory()) out = walk(p, out);
    else out.push(p);
  }
  return out;
}

// --- Rule A: contracts-only imports ---------------------------------------
if (existsSync(MODULES)) {
  const mods = readdirSync(MODULES).filter((m) => statSync(join(MODULES, m)).isDirectory());
  for (const f of walk(MODULES).filter((f) => f.endsWith(".go"))) {
    const rel = f.split(`internal${sep}modules${sep}`)[1] ?? "";
    const own = rel.split(sep)[0];
    const src = readFileSync(f, "utf8");
    for (const m of mods) {
      if (m === own) continue;
      // import of another module that is NOT its contracts/ package
      const re = new RegExp(`mini-erp/internal/modules/${m}/(?!contracts[/"\\s])([a-zA-Z0-9_/]*)`, "g");
      let hit;
      while ((hit = re.exec(src)) !== null) {
        bad("A", f, `imports ${m}/${hit[1] || "(root)"} instead of ${m}/contracts`);
      }
    }
  }
}

// --- Rule B: no tenancy leftovers in code -----------------------------------
// Comments are stripped first: discussing the takeout (e.g. "tanpa
// company_id (ADR-0009)") must not trip the guard — only real code counts.
function stripComments(src, isGo) {
  let out = src.replace(/\/\*[\s\S]*?\*\//g, "");
  const marker = isGo ? "//" : "--";
  return out
    .split("\n")
    .map((line) => {
      const idx = line.indexOf(marker);
      return idx === -1 ? line : line.slice(0, idx);
    })
    .join("\n");
}

if (existsSync(API)) {
  for (const f of walk(API).filter((f) => /\.(go|sql)$/.test(f))) {
    const src = stripComments(readFileSync(f, "utf8"), f.endsWith(".go"));
    for (const pat of ["company_id", "companyId", "idCompany", "id_company"]) {
      if (src.includes(pat)) bad("B", f, `contains forbidden tenancy token \`${pat}\``);
    }
  }
}

// --- Rule C: per-module sqlc -----------------------------------------------
{
  const yaml = readFileSync(join(API, "sqlc.yaml"), "utf8");
  const queries = [...yaml.matchAll(/queries:\s*"([^"]+)"/g)].map((m) => m[1]);
  const outs = [...yaml.matchAll(/out:\s*"([^"]+)"/g)].map((m) => m[1]);
  if (queries.length !== outs.length || queries.length === 0) {
    bad("C", "apps/api/sqlc.yaml", "queries/blocks mismatch or empty");
  } else {
    queries.forEach((q, i) => {
      const qm = (q.match(/internal\/modules\/([^/]+)\/infrastructure\/queries/) || [])[1];
      const om = (outs[i].match(/internal\/modules\/([^/]+)\/infrastructure\/sqlc/) || [])[1];
      if (!qm || !om || qm !== om) bad("C", "apps/api/sqlc.yaml", `block ${i}: queries=${q} out=${outs[i]}`);
    });
  }
}

// --- Rule D: single axios import --------------------------------------------
if (existsSync(WEB)) {
  for (const f of walk(join(WEB, "src")).filter((f) => f.endsWith(".ts") || f.endsWith(".tsx"))) {
    if (f.endsWith(join("shared", "services", "http-client.ts"))) continue;
    const src = readFileSync(f, "utf8");
    if (/from\s+["']axios["']|require\(["']axios["']\)/.test(src)) {
      bad("D", f, "imports axios directly — use shared/services/http-client.ts");
    }
  }
}

if (failures > 0) {
  process.stdout.write(`\narch-check: ${failures} violation(s).\n`);
  process.exit(1);
}
process.stdout.write("arch-check: OK (A contracts-only, B no-tenancy, C per-module sqlc, D single http-client).\n");
