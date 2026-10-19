// Architecture rules for the Go modular monolith. Pure functions over a repo
// tree: checkRepo(root) -> Violation[]. The CLI (scripts/arch-check.mjs) and
// the self-test (scripts/arch-check/self-test.mjs) both call it.
//
// Rules (see scripts/arch-check.mjs header for the catalogue):
//   A  contracts-only imports between modules (+ A' no module-root imports)
//   B  no tenancy tokens
//   C  sqlc config/generated files per module, ratcheted by sqlc-migration-status.json
//   D  single axios client
//   E  layer direction inside a module
//   F  no cross-module FOREIGN KEY in migrations
//   K  contracts never import application|infrastructure|presentation|domain
//   M  the {"success": envelope literal lives only in shared/response
//   P  single money kernel
//   R  no raw database/sql calls in sqlc-migrated modules
//   S  module dependency graph is a DAG (auth/user/audit ignored)
//   T  a module's SQL touches only the tables its own baseline migration creates
//   W  no Set* methods on a module's Module type
import { readFileSync, readdirSync, statSync, existsSync } from "node:fs";
import { join, sep, relative, isAbsolute } from "node:path";

const LAYERS = ["contracts", "domain", "application", "infrastructure", "presentation"];
// Cross-cutting modules every other module may depend on (Rule S ignores them).
const CROSS_CUTTING = new Set(["auth", "user", "audit"]);
// Directories outside internal/modules that may import module internals.
const COMPOSITION_ROOTS = ["cmd/", "internal/bootstrap/", "internal/integration/", "internal/etl/", "internal/testutil/"];

export function walk(dir, out = []) {
  if (!existsSync(dir)) return out;
  for (const e of readdirSync(dir)) {
    const p = join(dir, e);
    if (statSync(p).isDirectory()) walk(p, out);
    else out.push(p);
  }
  return out;
}

export function stripComments(src, isGo) {
  const out = src.replace(/\/\*[\s\S]*?\*\//g, "");
  const marker = isGo ? "//" : "--";
  return out
    .split("\n")
    .map((line) => {
      const idx = line.indexOf(marker);
      return idx === -1 ? line : line.slice(0, idx);
    })
    .join("\n");
}

const posix = (p) => p.split(sep).join("/");
const readJson = (p, fallback) => (existsSync(p) ? JSON.parse(readFileSync(p, "utf8")) : fallback);

/** Import paths of a Go file (comments stripped), one per import line. */
export function goImports(src) {
  const out = [];
  const re = /^\s*(?:[\w.]+\s+)?"([^"]+)"\s*$/gm;
  const body = stripComments(src, true);
  // restrict to import declarations: `import "x"` and `import ( ... )`
  const blocks = [...body.matchAll(/^import\s*\(([\s\S]*?)^\)/gm)].map((m) => m[1]);
  for (const m of body.matchAll(/^import\s+(?:[\w.]+\s+)?"([^"]+)"/gm)) out.push(m[1]);
  for (const b of blocks) for (const m of b.matchAll(re)) out.push(m[1]);
  return out;
}

const MOD_IMPORT = /^mini-erp\/internal\/modules\/([^/]+)(?:\/(.*))?$/;

/** Splits a module import into {module, sub} (sub "" = module root package). */
function parseModuleImport(path) {
  const m = MOD_IMPORT.exec(path);
  if (!m) return null;
  return { module: m[1], sub: m[2] ?? "" };
}

// ---------------------------------------------------------------------------

export function checkRepo(root) {
  const v = [];
  const api = join(root, "apps/api");
  const web = join(root, "apps/web");
  const modulesDir = join(api, "internal/modules");
  const rel = (f) => posix(relative(root, f));
  const bad = (rule, file, detail) => v.push({ rule, file: isAbsolute(file) ? rel(file) : file, detail: detail ?? "" });

  const modules = existsSync(modulesDir)
    ? readdirSync(modulesDir).filter((m) => statSync(join(modulesDir, m)).isDirectory())
    : [];
  const goFiles = walk(api).filter((f) => f.endsWith(".go"));
  const prodGo = goFiles.filter((f) => !f.endsWith("_test.go"));
  const modOf = (f) => {
    const r = posix(relative(modulesDir, f));
    if (r.startsWith("..")) return null;
    return r.split("/")[0];
  };
  const layerOf = (f, m) => {
    const parts = posix(relative(join(modulesDir, m), f)).split("/");
    return parts.length > 1 && LAYERS.includes(parts[0]) ? parts[0] : "root";
  };

  // --- migrations: table ownership ------------------------------------------
  const tableOwner = new Map(); // table -> module
  const migDir = join(api, "migrations");
  const migFiles = existsSync(migDir)
    ? readdirSync(migDir).filter((f) => f.endsWith(".up.sql")).sort()
    : [];
  const migModule = (f) => /^\d+_([a-z0-9]+)/.exec(f)?.[1] ?? null;
  const moduleMigs = new Map(); // module -> [file]
  for (const f of migFiles) {
    const mod = migModule(f);
    if (!mod) continue;
    if (!modules.includes(mod)) {
      bad("T", join(migDir, f), `migration names module "${mod}" which does not exist under internal/modules`);
      continue;
    }
    if (!moduleMigs.has(mod)) moduleMigs.set(mod, []);
    moduleMigs.get(mod).push(f);
    const sql = stripComments(readFileSync(join(migDir, f), "utf8"), false);
    for (const m of sql.matchAll(/CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?`?([A-Za-z0-9_]+)`?/gi)) {
      const t = m[1].toLowerCase();
      if (tableOwner.has(t) && tableOwner.get(t) !== mod) {
        bad("T", join(migDir, f), `table ${t} already owned by ${tableOwner.get(t)}`);
      }
      tableOwner.set(t, mod);
    }
    // Rule F: foreign keys stay inside the module
    for (const m of sql.matchAll(/REFERENCES\s+`?([A-Za-z0-9_]+)`?/gi)) {
      const t = m[1].toLowerCase();
      const owner = tableOwner.get(t);
      if (owner === undefined) bad("F", join(migDir, f), `REFERENCES ${t}: table not created by any earlier migration`);
      else if (owner !== mod) bad("F", join(migDir, f), `cross-module FOREIGN KEY: ${mod} -> ${t} (owned by ${owner})`);
    }
  }

  // --- Rule A / A' / E / K ---------------------------------------------------
  for (const f of goFiles) {
    const m = modOf(f);
    const src = readFileSync(f, "utf8");
    const imports = goImports(src);
    const fileRel = rel(f);
    const isTest = f.endsWith("_test.go");
    if (m === null) {
      // outside internal/modules: only composition roots may touch module packages
      const inRoot = COMPOSITION_ROOTS.some((r) => fileRel.includes(`apps/api/${r}`));
      const inShared = fileRel.includes("apps/api/internal/shared/");
      for (const imp of imports) {
        const p = parseModuleImport(imp);
        if (!p) continue;
        if (inShared) bad("E", f, `shared/ must not import modules (${imp})`);
        else if (!inRoot && p.sub !== "contracts" && !p.sub.startsWith("contracts/")) {
          bad("A", f, `imports ${p.module}/${p.sub || "(root)"}: only composition roots may import module internals`);
        } else if (!inRoot && p.sub === "") {
          bad("A", f, `imports module root ${p.module}`);
        }
      }
      continue;
    }
    const layer = layerOf(f, m);
    for (const imp of imports) {
      const p = parseModuleImport(imp);
      if (p && p.module !== m) {
        // Rule A / A'
        if (p.sub === "") bad("A", f, `imports module root ${p.module} (only contracts/ is public)`);
        else if (p.sub !== "contracts" && !p.sub.startsWith("contracts/")) bad("A", f, `imports ${p.module}/${p.sub} instead of ${p.module}/contracts`);
      }
      if (isTest) continue; // layer rules apply to production code
      const own = p && p.module === m ? p.sub.split("/")[0] : null;
      if (layer === "contracts") {
        if (p && p.sub !== "contracts" && !p.sub.startsWith("contracts/")) {
          bad("K", f, `contracts must not import ${p.module}/${p.sub || "(root)"}`);
        }
      }
      if (layer === "domain") {
        const std = !imp.includes(".") && !imp.startsWith("mini-erp");
        const okShared = /^mini-erp\/internal\/shared\/(money|timeutil)(\/|$)/.test(imp);
        if (!std && !okShared && own !== "domain") bad("E", f, `domain may import only stdlib, own domain, shared/money|timeutil (${imp})`);
      }
      if (layer === "infrastructure" && (own === "application" || own === "presentation")) {
        bad("E", f, `infrastructure must not import ${own}`);
      }
      if (layer === "application") {
        if (own === "presentation") bad("E", f, "application must not import presentation");
        if (imp === "database/sql") bad("E", f, "application must not import database/sql (ADR-0011)");
      }
      if (layer === "presentation" && own === "infrastructure") bad("E", f, "presentation must not import infrastructure");
    }
  }

  // --- Rule B ----------------------------------------------------------------
  for (const f of walk(api).filter((f) => /\.(go|sql)$/.test(f))) {
    const src = stripComments(readFileSync(f, "utf8"), f.endsWith(".go"));
    for (const pat of ["company_id", "companyId", "idCompany", "id_company"]) {
      if (src.includes(pat)) bad("B", f, `contains forbidden tenancy token \`${pat}\``);
    }
  }

  // --- Rule D ----------------------------------------------------------------
  for (const f of walk(join(web, "src")).filter((f) => f.endsWith(".ts") || f.endsWith(".tsx"))) {
    if (posix(f).endsWith("shared/services/http-client.ts")) continue;
    // Tests of the client itself must build axios errors (CanceledError, adapters).
    if (/shared\/services\/http-[\w-]+\.test\.ts$/.test(posix(f))) continue;
    if (/from\s+["']axios["']|require\(["']axios["']\)/.test(readFileSync(f, "utf8"))) {
      bad("D", f, "imports axios directly — use shared/services/http-client.ts");
    }
  }

  // --- Rule P ----------------------------------------------------------------
  for (const f of goFiles) {
    if (posix(f).includes("/shared/money/")) continue;
    const src = stripComments(readFileSync(f, "utf8"), true);
    if (/\b(lineEconomics|CalcLine)\b|\bprorate\s*\(/.test(src)) {
      bad("P", f, "defines money math outside shared/money — use money.LineAmounts/Round/Prorate (ADR-0010)");
    }
  }

  // --- Rule M ----------------------------------------------------------------
  for (const f of prodGo) {
    if (posix(f).includes("/internal/shared/response/")) continue;
    const src = stripComments(readFileSync(f, "utf8"), true);
    if (/\{\s*\\?"success\\?"\s*:/.test(src)) {
      bad("M", f, 'hand-written {"success": envelope — use shared/response (api-standard)');
    }
  }

  // --- Rule W ----------------------------------------------------------------
  for (const f of prodGo.filter((f) => f.endsWith(".module.go"))) {
    const src = stripComments(readFileSync(f, "utf8"), true);
    for (const m of src.matchAll(/^func\s+\(\w+\s+\*?Module\)\s+(Set[A-Z]\w*)\s*\(/gm)) {
      bad("W", f, `late-bound setter Module.${m[1]} (pass it to NewModule, or allow-list with a reason)`);
    }
  }

  // --- Rule T: table access ---------------------------------------------------
  const sqlFiles = [];
  for (const m of modules) {
    for (const f of walk(join(modulesDir, m))) {
      if (f.endsWith("_test.go")) continue;
      if (f.endsWith(".go") || (f.endsWith(".sql") && posix(f).includes("/queries/"))) sqlFiles.push([m, f]);
    }
  }
  for (const [m, f] of sqlFiles) {
    const src = stripComments(readFileSync(f, "utf8"), f.endsWith(".go"));
    const seen = new Set();
    for (const hit of src.matchAll(/\b(?:FROM|JOIN|INTO|UPDATE|TABLE)\s+`?([a-z][a-z0-9_]*)`?/g)) {
      const t = hit[1];
      const owner = tableOwner.get(t);
      if (owner && owner !== m && !seen.has(t)) {
        seen.add(t);
        bad("T", f, `module ${m} touches table ${t} owned by ${owner}`);
      }
    }
  }

  // --- Rule S: module graph ---------------------------------------------------
  {
    const edges = new Map(modules.map((m) => [m, new Set()]));
    const cEdges = new Map(modules.map((m) => [m, new Set()]));
    for (const f of prodGo) {
      const m = modOf(f);
      if (m === null) continue;
      for (const imp of goImports(readFileSync(f, "utf8"))) {
        const p = parseModuleImport(imp);
        if (!p || p.module === m || !modules.includes(p.module)) continue;
        if (CROSS_CUTTING.has(m) || CROSS_CUTTING.has(p.module)) continue;
        edges.get(m).add(p.module);
        if (layerOf(f, m) === "contracts") cEdges.get(m).add(p.module);
      }
    }
    for (const [label, g] of [["module", edges], ["contracts", cEdges]]) {
      for (const comp of sccs(g)) {
        if (comp.length > 1) bad("S", "apps/api/internal/modules", `${label} dependency cycle: ${comp.sort().join(",")}`);
      }
    }
  }

  // --- Rule C / R: sqlc --------------------------------------------------------
  checkSqlc({ root, api, modulesDir, modules, moduleMigs, prodGo, bad, modOf, layerOf });

  return applyAllowList(root, v);
}

function sccs(g) {
  // Tarjan
  let idx = 0;
  const index = new Map(), low = new Map(), on = new Set(), stack = [], out = [];
  const strong = (n) => {
    index.set(n, idx); low.set(n, idx); idx++;
    stack.push(n); on.add(n);
    for (const w of g.get(n) ?? []) {
      if (!index.has(w)) { strong(w); low.set(n, Math.min(low.get(n), low.get(w))); }
      else if (on.has(w)) low.set(n, Math.min(low.get(n), index.get(w)));
    }
    if (low.get(n) === index.get(n)) {
      const comp = [];
      let w;
      do { w = stack.pop(); on.delete(w); comp.push(w); } while (w !== n);
      out.push(comp);
    }
  };
  for (const n of g.keys()) if (!index.has(n)) strong(n);
  return out;
}

/** Parses the (fixed-format) sqlc.yaml into blocks {schema[], queries, out}. */
export function parseSqlcYaml(text) {
  const blocks = [];
  let cur = null;
  let inSchema = false;
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.replace(/#.*$/, "");
    if (/^\s*-\s*engine:/.test(line)) { cur = { schema: [], queries: "", out: "" }; blocks.push(cur); inSchema = false; continue; }
    if (!cur) continue;
    const q = (s) => s.trim().replace(/^["']|["']$/g, "");
    let m;
    if ((m = /^\s*schema:\s*(.*)$/.exec(line))) {
      const val = m[1].trim();
      inSchema = val === "";
      if (val.startsWith("[")) cur.schema = val.replace(/^\[|\]$/g, "").split(",").map(q).filter(Boolean);
      else if (val) cur.schema = [q(val)];
      continue;
    }
    if (inSchema && (m = /^\s+-\s*(.+)$/.exec(line))) { cur.schema.push(q(m[1])); continue; }
    if ((m = /^\s*queries:\s*(.+)$/.exec(line))) { cur.queries = q(m[1]); inSchema = false; continue; }
    if ((m = /^\s*out:\s*(.+)$/.exec(line))) { cur.out = q(m[1]); inSchema = false; continue; }
    if (/^\s*\w[\w_]*:/.test(line)) inSchema = false;
  }
  return blocks;
}

function checkSqlc({ root, api, modulesDir, modules, moduleMigs, prodGo, bad, modOf, layerOf }) {
  const yamlPath = join(api, "sqlc.yaml");
  const statusPath = join(root, "scripts/sqlc-migration-status.json");
  if (!existsSync(yamlPath)) return bad("C", "apps/api/sqlc.yaml", "missing");
  if (!existsSync(statusPath)) return bad("C", "scripts/sqlc-migration-status.json", "missing (lists migrated/pending/noSql modules)");
  const status = JSON.parse(readFileSync(statusPath, "utf8"));
  const lists = { migrated: status.migrated ?? [], pending: status.pending ?? [], noSql: status.noSql ?? [] };
  const where = new Map();
  for (const [k, arr] of Object.entries(lists)) {
    for (const m of arr) {
      if (!modules.includes(m)) bad("C", "scripts/sqlc-migration-status.json", `${k} lists unknown module ${m}`);
      if (where.has(m)) bad("C", "scripts/sqlc-migration-status.json", `module ${m} listed in both ${where.get(m)} and ${k}`);
      where.set(m, k);
    }
  }
  for (const m of modules) {
    const hasTables = moduleMigs.has(m);
    const k = where.get(m);
    if (!k) bad("C", "scripts/sqlc-migration-status.json", `module ${m} is not classified (migrated|pending|noSql)`);
    else if (hasTables && k === "noSql") bad("C", "scripts/sqlc-migration-status.json", `module ${m} owns tables but is listed noSql`);
    else if (!hasTables && k !== "noSql") bad("C", "scripts/sqlc-migration-status.json", `module ${m} owns no tables but is listed ${k}`);
  }

  const blocks = parseSqlcYaml(readFileSync(yamlPath, "utf8"));
  const byModule = new Map();
  blocks.forEach((b, i) => {
    const qm = /internal\/modules\/([^/]+)\/infrastructure\/queries$/.exec(b.queries)?.[1];
    const om = /internal\/modules\/([^/]+)\/infrastructure\/sqlc$/.exec(b.out)?.[1];
    if (!qm || !om || qm !== om) { bad("C", "apps/api/sqlc.yaml", `block ${i}: queries=${b.queries} out=${b.out} must be the SAME module's infrastructure/queries and infrastructure/sqlc`); return; }
    if (byModule.has(qm)) bad("C", "apps/api/sqlc.yaml", `module ${qm} has more than one block`);
    byModule.set(qm, b);
  });
  for (const m of modules) {
    const k = where.get(m);
    const b = byModule.get(m);
    if (k === "noSql") { if (b) bad("C", "apps/api/sqlc.yaml", `module ${m} has no SQL but has a sqlc block`); continue; }
    if (!k) continue;
    if (!b) { bad("C", "apps/api/sqlc.yaml", `module ${m} (${k}) has no sqlc block`); continue; }
    const want = (moduleMigs.get(m) ?? []).map((f) => `migrations/${f}`);
    if (JSON.stringify([...b.schema].sort()) !== JSON.stringify([...want].sort())) {
      bad("C", "apps/api/sqlc.yaml", `module ${m}: schema must list exactly its own baseline migration(s) ${want.join(", ")} (got ${b.schema.join(", ") || "none"}) so cross-module queries fail at generate time`);
    }
    const qdir = join(api, b.queries);
    const odir = join(api, b.out);
    const qsql = existsSync(qdir) ? readdirSync(qdir).filter((f) => f.endsWith(".sql")) : [];
    const gen = join(odir, "db.go");
    const hasGen = existsSync(gen) && /Code generated by sqlc/.test(readFileSync(gen, "utf8"));
    if (k === "migrated") {
      if (qsql.length === 0) bad("C", b.queries, `migrated module ${m} has no queries/*.sql`);
      if (!hasGen) bad("C", b.out, `migrated module ${m} has no generated sqlc/db.go (run sqlc generate)`);
    } else if (qsql.length > 0 || hasGen) {
      bad("C", "scripts/sqlc-migration-status.json", `module ${m} already has sqlc queries/output: move it from pending to migrated`);
    }
  }

  // Rule R: raw database/sql calls are forbidden in migrated modules
  const allow = readJson(join(root, "scripts/raw-sql-allow.json"), []);
  const allowed = new Set((Array.isArray(allow) ? allow : allow.allow ?? []).map((e) => e.file));
  for (const f of prodGo) {
    const m = modOf(f);
    if (!m || !lists.migrated.includes(m)) continue;
    const r = posix(relative(root, f));
    if (r.includes("/infrastructure/sqlc/") || allowed.has(r)) continue;
    const src = stripComments(readFileSync(f, "utf8"), true);
    if (/\.(QueryContext|ExecContext|QueryRowContext|PrepareContext)\(/.test(src)) {
      bad("R", f, `raw SQL call in sqlc-migrated module ${m} (use generated queries or add to scripts/raw-sql-allow.json with a reason)`);
    }
  }
}

/**
 * scripts/arch-allow.json: { "allow": [ { rule, file, match, reason } ] }
 * An entry silences violations of `rule` in `file` whose detail contains
 * `match`. Every entry needs a reason, and an entry that no longer silences
 * anything is itself a violation (the allow-list only ever shrinks).
 */
function applyAllowList(root, violations) {
  const path = join(root, "scripts/arch-allow.json");
  const cfg = readJson(path, { allow: [] });
  const entries = cfg.allow ?? [];
  const used = new Array(entries.length).fill(false);
  const out = [];
  for (const x of violations) {
    const i = entries.findIndex((e) => e.rule === x.rule && x.file === e.file && x.detail.includes(e.match ?? ""));
    if (i >= 0) used[i] = true;
    else out.push(x);
  }
  entries.forEach((e, i) => {
    if (!e.reason || !String(e.reason).trim()) out.push({ rule: "ALLOW", file: "scripts/arch-allow.json", detail: `entry ${e.rule} ${e.file} has no reason` });
    if (!used[i]) out.push({ rule: "ALLOW", file: "scripts/arch-allow.json", detail: `stale entry (no longer needed): ${e.rule} ${e.file} ${e.match ?? ""}` });
  });
  return out;
}
