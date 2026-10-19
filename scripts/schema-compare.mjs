#!/usr/bin/env node
// Schema fingerprint / comparer (design F4 Lampiran C). Used to PROVE that the
// squashed baseline (F4-1, Fase 5) builds a schema identical to the migration chain.
//
//   node scripts/schema-compare.mjs dump <dsn> [--out file.json]
//   node scripts/schema-compare.mjs diff <a.json> <b.json> [--expected manifest.txt]
//   node scripts/schema-compare.mjs compare <dsn|a.json> <dsn|b.json> [--expected manifest.txt]
//   node scripts/schema-compare.mjs hash <dsn|a.json>
//
// <dsn> is the Go DSN form `user:pass@tcp(host:port)/db` (the form TEST_DB_DSN
// uses); a dsn may be replaced by `env:NAME` to read it from an environment
// variable (keeps passwords out of shell history). The password is handed to the
// mysql client through MYSQL_PWD, never on the command line, and is never printed.
//
// Dumped (all from information_schema, normalised, deterministic):
//   tables   engine, collation, row_format, create_options
//   columns  type, nullability, default, extra, charset, collation, generation expr
//            (ordinal_position is NOT compared: code uses explicit column lists)
//   indexes  per index: uniqueness, type, visibility, ordered column list (+sub_part/collation)
//   foreign keys / CHECK constraints
// Normalised away: AUTO_INCREMENT counters, ordinal positions, auto-generated constraint
// names (`<table>_ibfk_N`, `<table>_chk_N`), the integration harness marker tables
// (`_it_*`) and golang-migrate's `schema_migrations`.
//
// diff output: one line per difference, `kind|table|object|detail`, sorted. With
// `--expected FILE` (one line per APPROVED difference, `#` comments allowed) the
// diff must equal the manifest exactly (stage 2 of the squash). Exit 0 = identical
// (or equal to manifest), 1 = different, 2 = usage/connection error.
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const IGNORE_TABLE = /^(_it_.*|schema_migrations)$/;

// ---------------------------------------------------------------- mysql client
function findMysql() {
  if (process.env.MYSQL_BIN) return process.env.MYSQL_BIN;
  const dir = process.env.LARAGON_MYSQL_DIR;
  const cands = [];
  if (dir) cands.push(join(dir, "bin", "mysql.exe"));
  const base = "C:/laragon/bin/mysql";
  if (existsSync(base)) {
    for (const d of readdirSync(base).sort().reverse()) cands.push(join(base, d, "bin", "mysql.exe"));
  }
  for (const c of cands) if (existsSync(c)) return c;
  return "mysql"; // PATH
}

export function parseDSN(dsn) {
  const m = /^([^:@]*)(?::([^@]*))?@tcp\(([^:)]+)(?::(\d+))?\)\/([^?]+)/.exec(dsn);
  if (!m) throw new Error("DSN tidak dikenali; pakai user:pass@tcp(host:port)/db");
  return { user: m[1], pass: m[2] ?? "", host: m[3], port: m[4] ?? "3306", db: m[5] };
}

function unescapeField(s) {
  return s.replace(/\\(.)/g, (_, c) => (c === "n" ? "\n" : c === "t" ? "\t" : c === "0" ? "\0" : c));
}

function query(conn, sql) {
  const args = [
    "--protocol=tcp", `-h${conn.host}`, `-P${conn.port}`, `-u${conn.user}`,
    "--default-character-set=utf8mb4", "-N", "-B", conn.db, "-e", sql,
  ];
  const r = spawnSync(findMysql(), args, {
    encoding: "utf8", maxBuffer: 256 * 1024 * 1024,
    env: { ...process.env, MYSQL_PWD: conn.pass },
  });
  if (r.error) throw new Error(`gagal menjalankan klien mysql: ${r.error.message}`);
  if (r.status !== 0) throw new Error(`mysql gagal: ${(r.stderr || "").replace(/password[^\n]*/gi, "***").trim()}`);
  return r.stdout.split(/\r?\n/).filter((l) => l.length > 0).map((l) => l.split("\t").map(unescapeField));
}

// ----------------------------------------------------------------------- dump
const Q_TABLES = `SELECT table_name, IFNULL(engine,''), IFNULL(table_collation,''), IFNULL(row_format,''), IFNULL(create_options,'')
  FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name`;
const Q_COLUMNS = `SELECT table_name, column_name, column_type, is_nullable, IFNULL(column_default,'<NULL>'), IFNULL(extra,''),
  IFNULL(character_set_name,''), IFNULL(collation_name,''), IFNULL(generation_expression,'')
  FROM information_schema.columns WHERE table_schema = DATABASE() ORDER BY table_name, column_name`;
const Q_INDEXES = `SELECT table_name, index_name, non_unique, seq_in_index, IFNULL(column_name,''), IFNULL(expression,''),
  IFNULL(sub_part,''), IFNULL(collation,''), index_type, is_visible
  FROM information_schema.statistics WHERE table_schema = DATABASE() ORDER BY table_name, index_name, seq_in_index`;
const Q_FKS = `SELECT k.table_name, k.constraint_name, k.column_name, k.referenced_table_name, k.referenced_column_name, r.update_rule, r.delete_rule
  FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r
    ON r.constraint_schema = k.constraint_schema AND r.constraint_name = k.constraint_name AND r.table_name = k.table_name
  WHERE k.table_schema = DATABASE() AND k.referenced_table_name IS NOT NULL
  ORDER BY k.table_name, k.constraint_name, k.ordinal_position`;
const Q_CHECKS = `SELECT t.table_name, c.constraint_name, c.check_clause
  FROM information_schema.check_constraints c JOIN information_schema.table_constraints t
    ON t.constraint_schema = c.constraint_schema AND t.constraint_name = c.constraint_name
  WHERE t.table_schema = DATABASE() AND t.constraint_type = 'CHECK' ORDER BY t.table_name, c.constraint_name`;

const autoName = (table, name) => new RegExp(`^${table}_(ibfk|chk)_\\d+$`).test(name);

export function dumpSchema(conn) {
  const tables = {};
  const t = (n) => tables[n];
  for (const [name, engine, collation, row_format, create_options] of query(conn, Q_TABLES)) {
    if (IGNORE_TABLE.test(name)) continue;
    // AUTO_INCREMENT is not part of create_options in information_schema, but strip defensively.
    const opts = create_options.replace(/\bAUTO_INCREMENT=\d+\s*/gi, "").trim();
    tables[name] = { engine, collation, row_format, create_options: opts, columns: {}, indexes: {}, foreign_keys: {}, checks: {} };
  }
  for (const [tb, col, type, nullable, def, extra, cs, coll, gen] of query(conn, Q_COLUMNS)) {
    if (!t(tb)) continue;
    t(tb).columns[col] = { type, nullable, default: def, extra, charset: cs, collation: coll, generated: gen };
  }
  for (const [tb, idx, nonUnique, , col, expr, sub, coll, type, vis] of query(conn, Q_INDEXES)) {
    if (!t(tb)) continue;
    const e = (t(tb).indexes[idx] ??= { unique: nonUnique === "0", type, visible: vis, columns: [] });
    e.columns.push(`${col || `(${expr})`}${sub ? `(${sub})` : ""}${coll === "D" ? " DESC" : ""}`);
  }
  for (const [tb, name, col, rt, rc, upd, del] of query(conn, Q_FKS)) {
    if (!t(tb)) continue;
    const key = autoName(tb, name) ? "<auto>" : name;
    const k = key === "<auto>" ? `<auto:${rt}>` : key;
    const e = (t(tb).foreign_keys[k] ??= { name: key, references: rt, on_update: upd, on_delete: del, columns: [], ref_columns: [] });
    e.columns.push(col);
    e.ref_columns.push(rc);
  }
  for (const [tb, name, clause] of query(conn, Q_CHECKS)) {
    if (!t(tb)) continue;
    const key = autoName(tb, name) ? `<auto:${clause}>` : name;
    t(tb).checks[key] = { name: autoName(tb, name) ? "<auto>" : name, clause };
  }
  const sorted = Object.fromEntries(Object.entries(tables).sort(([a], [b]) => (a < b ? -1 : 1)));
  return { format: 1, database: conn.db, tables: sorted };
}

// ----------------------------------------------------------------------- diff
function sortKeys(o) {
  if (Array.isArray(o)) return o.map(sortKeys);
  if (o && typeof o === "object") return Object.fromEntries(Object.keys(o).sort().map((k) => [k, sortKeys(o[k])]));
  return o;
}

export function canonical(dump) {
  // The database name is not part of the schema.
  return JSON.stringify(sortKeys({ format: dump.format, tables: dump.tables }));
}

export const fingerprint = (dump) => createHash("sha256").update(canonical(dump)).digest("hex");

const fmt = (v) => (typeof v === "string" ? v : JSON.stringify(sortKeys(v)));

function diffObjects(kind, table, a, b, out) {
  const keys = new Set([...Object.keys(a ?? {}), ...Object.keys(b ?? {})]);
  for (const k of [...keys].sort()) {
    const x = a?.[k], y = b?.[k];
    if (x === undefined) out.push(`${kind}|${table}|${k}|added ${fmt(y)}`);
    else if (y === undefined) out.push(`${kind}|${table}|${k}|removed ${fmt(x)}`);
    else if (JSON.stringify(sortKeys(x)) !== JSON.stringify(sortKeys(y))) {
      if (typeof x === "object" && typeof y === "object" && !Array.isArray(x)) {
        for (const f of new Set([...Object.keys(x), ...Object.keys(y)])) {
          if (JSON.stringify(x[f]) !== JSON.stringify(y[f])) out.push(`${kind}|${table}|${k}|${f}: ${fmt(x[f])} -> ${fmt(y[f])}`);
        }
      } else out.push(`${kind}|${table}|${k}|${fmt(x)} -> ${fmt(y)}`);
    }
  }
}

export function diffSchemas(a, b) {
  const out = [];
  const names = new Set([...Object.keys(a.tables), ...Object.keys(b.tables)]);
  for (const n of [...names].sort()) {
    const x = a.tables[n], y = b.tables[n];
    if (!x) { out.push(`table|${n}||added`); continue; }
    if (!y) { out.push(`table|${n}||removed`); continue; }
    for (const f of ["engine", "collation", "row_format", "create_options"]) {
      if (x[f] !== y[f]) out.push(`table|${n}|${f}|${x[f]} -> ${y[f]}`);
    }
    diffObjects("column", n, x.columns, y.columns, out);
    diffObjects("index", n, x.indexes, y.indexes, out);
    diffObjects("fk", n, x.foreign_keys, y.foreign_keys, out);
    diffObjects("check", n, x.checks, y.checks, out);
  }
  return out.sort();
}

// ------------------------------------------------------------------------ cli
function load(arg) {
  if (arg.endsWith(".json") && existsSync(arg)) return JSON.parse(readFileSync(arg, "utf8"));
  const dsn = arg.startsWith("env:") ? process.env[arg.slice(4)] : arg;
  if (!dsn) throw new Error(`variabel lingkungan ${arg} kosong`);
  return dumpSchema(parseDSN(dsn));
}

function flag(args, name) {
  const i = args.indexOf(name);
  if (i < 0) return undefined;
  const v = args[i + 1];
  args.splice(i, 2);
  return v;
}

function report(diff, expectedFile) {
  if (expectedFile) {
    const want = readFileSync(expectedFile, "utf8").split(/\r?\n/).map((l) => l.trim()).filter((l) => l && !l.startsWith("#")).sort();
    const missing = want.filter((l) => !diff.includes(l));
    const extra = diff.filter((l) => !want.includes(l));
    if (!missing.length && !extra.length) { console.log(`OK: ${diff.length} perbedaan, sama persis dengan manifest.`); return 0; }
    for (const l of extra) console.log(`TAK TERDAFTAR: ${l}`);
    for (const l of missing) console.log(`TERDAFTAR TAPI TIDAK ADA: ${l}`);
    return 1;
  }
  if (!diff.length) { console.log("OK: schema identik."); return 0; }
  for (const l of diff) console.log(l);
  console.log(`${diff.length} perbedaan.`);
  return 1;
}

function main(argv) {
  const [cmd, ...rest] = argv;
  const expected = flag(rest, "--expected");
  const outFile = flag(rest, "--out");
  switch (cmd) {
    case "dump": {
      if (rest.length !== 1) break;
      const dump = load(rest[0]);
      const text = JSON.stringify(sortKeys(dump), null, 2) + "\n";
      if (outFile) writeFileSync(outFile, text);
      else process.stdout.write(text);
      return 0;
    }
    case "hash":
      if (rest.length !== 1) break;
      console.log(fingerprint(load(rest[0])));
      return 0;
    case "diff":
    case "compare":
      if (rest.length !== 2) break;
      return report(diffSchemas(load(rest[0]), load(rest[1])), expected);
    default:
  }
  console.error("uso: schema-compare.mjs dump <dsn> [--out f] | diff|compare <a> <b> [--expected f] | hash <a>");
  return 2;
}

if (import.meta.url === new URL(`file:///${process.argv[1].replace(/\\/g, "/")}`).href || process.argv[1]?.endsWith("schema-compare.mjs")) {
  try {
    process.exitCode = main(process.argv.slice(2));
  } catch (e) {
    console.error("schema-compare:", e.message);
    process.exitCode = 2;
  }
}
