#!/usr/bin/env node
// Builds the per-module baseline migrations (F4-1, Fase 5) from a fully migrated
// database, so the migration chain can be replaced by 18 files (one per module,
// BACKEND_GUIDE rule "one migration = one module").
//
//   node scripts/squash-baseline.mjs <dsn> [--out apps/api/migrations_baseline]
//
// <dsn> is the Go DSN form `user:pass@tcp(host:port)/db` (or `env:NAME`). The
// database must be migrated with the CURRENT chain first. The output is checked
// afterwards with scripts/schema-compare.mjs (diff must be empty).
//
// Module files keep the historical numbering/names (000001_user … 000018_assistant)
// so later migrations continue at 000019. Tables are written in foreign-key order;
// AUTO_INCREMENT counters are stripped.
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { parseDSN } from "./schema-compare.mjs";

const MODULES = [
  "user", "auth", "branch", "company", "media", "product", "party", "stock", "purchasing",
  "sales", "goodsreceipt", "delivery", "payment", "salesreturn", "purchasereturn", "finance",
  "audit", "assistant",
];

// table -> module. Rule-based, then explicit exceptions.
const EXPLICIT = {
  authz_seed_applied: "user",
  user_sessions: "auth",
  business_expenses: "finance",
  branch_document_sequences: "branch",
  company_profile: "company",
  company_settings: "company",
  media_files: "media",
  member_types: "party",
  parties: "party",
  party_delivery_addresses: "party",
  product_categories: "product",
  products: "product",
  product_variants: "product",
  purchase_orders: "purchasing",
  purchase_order_items: "purchasing",
  sales_orders: "sales",
  sales_order_items: "sales",
  goods_receipts: "goodsreceipt",
  goods_receipt_items: "goodsreceipt",
  delivery_notes: "delivery",
  delivery_note_items: "delivery",
  payments: "payment",
  payment_allocations: "payment",
  sales_returns: "salesreturn",
  sales_return_items: "salesreturn",
  sales_return_replacement_items: "salesreturn",
  sales_return_settlements: "salesreturn",
  purchase_returns: "purchasereturn",
  purchase_return_items: "purchasereturn",
  purchase_return_settlements: "purchasereturn",
  audit_logs: "audit",
  audit_log_details: "audit",
  users: "user",
  roles: "user",
  permissions: "user",
  role_permissions: "user",
  user_roles: "user",
  user_branch_access: "user",
  branches: "branch",
};

function moduleOf(table) {
  if (EXPLICIT[table]) return EXPLICIT[table];
  if (table.startsWith("stock_")) return "stock";
  if (table.startsWith("finance_")) return "finance";
  if (table.startsWith("assistant_")) return "assistant";
  return null;
}

function query(conn, sql) {
  const args = [
    "--protocol=tcp", `-h${conn.host}`, `-P${conn.port}`, `-u${conn.user}`,
    "--default-character-set=utf8mb4", "-N", "-B", "-r", conn.db, "-e", sql,
  ];
  const mysql = process.env.MYSQL_BIN || findMysql();
  const r = spawnSync(mysql, args, { encoding: "utf8", maxBuffer: 64 * 1024 * 1024, env: { ...process.env, MYSQL_PWD: conn.pass } });
  if (r.status !== 0) throw new Error(`mysql gagal: ${(r.stderr || "").replace(/password[^\n]*/gi, "***").trim()}`);
  return r.stdout;
}

function findMysql() {
  const base = "C:/laragon/bin/mysql";
  if (existsSync(base)) {
    for (const d of readdirSync(base).sort().reverse()) {
      const c = join(base, d, "bin", "mysql.exe");
      if (existsSync(c)) return c;
    }
  }
  return "mysql";
}

const argv = process.argv.slice(2);
let dsn = argv[0];
const outIdx = argv.indexOf("--out");
const outDir = outIdx >= 0 ? argv[outIdx + 1] : "apps/api/migrations_baseline";
if (!dsn) {
  console.error("usage: node scripts/squash-baseline.mjs <dsn> [--out dir]");
  process.exit(2);
}
if (dsn.startsWith("env:")) dsn = process.env[dsn.slice(4)] ?? "";
const conn = parseDSN(dsn);

const tables = query(conn, "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name")
  .split(/\r?\n/).filter(Boolean)
  .filter((t) => !/^(_it_.*|schema_migrations|legacy_.*|etl_.*)$/.test(t));

const byModule = new Map(MODULES.map((m) => [m, []]));
const unmapped = [];
for (const t of tables) {
  const m = moduleOf(t);
  if (!m) unmapped.push(t);
  else byModule.get(m).push(t);
}
if (unmapped.length) {
  console.error(`tabel tanpa modul: ${unmapped.join(", ")} — tambahkan ke EXPLICIT`);
  process.exit(1);
}

function createStatement(table) {
  // SHOW CREATE TABLE -N -B -r prints: table <TAB> create statement
  const out = query(conn, `SHOW CREATE TABLE \`${table}\``);
  const idx = out.indexOf("\t");
  let sql = out.slice(idx + 1).trim();
  sql = sql.replace(/ AUTO_INCREMENT=\d+/g, "");
  return sql + ";";
}

function fkTargets(sql) {
  return [...sql.matchAll(/REFERENCES `([^`]+)`/g)].map((m) => m[1]);
}

mkdirSync(outDir, { recursive: true });
for (const f of readdirSync(outDir)) if (/\.sql$/.test(f)) rmSync(join(outDir, f));

const crossModule = [];
MODULES.forEach((mod, i) => {
  const names = byModule.get(mod);
  const stmts = new Map(names.map((t) => [t, createStatement(t)]));
  // foreign-key order inside the module
  const ordered = [];
  const done = new Set();
  const visit = (t, stack = []) => {
    if (done.has(t)) return;
    if (stack.includes(t)) throw new Error(`siklus FK: ${[...stack, t].join(" -> ")}`);
    for (const dep of fkTargets(stmts.get(t))) {
      if (dep === t) continue;
      if (!stmts.has(dep)) {
        crossModule.push(`${t} -> ${dep}`);
        continue;
      }
      visit(dep, [...stack, t]);
    }
    done.add(t);
    ordered.push(t);
  };
  names.forEach((t) => visit(t));
  const num = String(i + 1).padStart(6, "0");
  const header = `-- Baseline modul ${mod} (F4-1): skema akhir hasil rantai migrasi 000001-000072.\n-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.\n\n`;
  writeFileSync(join(outDir, `${num}_${mod}.up.sql`), header + ordered.map((t) => stmts.get(t)).join("\n\n") + "\n");
  writeFileSync(join(outDir, `${num}_${mod}.down.sql`), [...ordered].reverse().map((t) => `DROP TABLE IF EXISTS \`${t}\`;`).join("\n") + "\n");
  console.log(`${num}_${mod}: ${ordered.length} tabel`);
});
if (crossModule.length) {
  console.error(`PERINGATAN FK lintas modul (melanggar aturan): ${crossModule.join("; ")}`);
  process.exit(1);
}
console.log(`selesai: ${tables.length} tabel -> ${outDir}`);
