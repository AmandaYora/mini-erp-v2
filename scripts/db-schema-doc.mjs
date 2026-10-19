#!/usr/bin/env node
// Keeps the authoritative table list in docs/DB_SCHEMA.md in sync with the
// per-module baseline migrations (apps/api/migrations/NNNNNN_<module>*.up.sql).
//
//   node scripts/db-schema-doc.mjs           rewrite the generated block
//   node scripts/db-schema-doc.mjs --check   exit 1 when the block is stale
//
// The block sits between the BEGIN/END markers in DB_SCHEMA.md; everything
// outside it is hand-written narrative.
import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = dirname(dirname(fileURLToPath(import.meta.url)));
const MIG = join(ROOT, "apps/api/migrations");
const DOC = join(ROOT, "docs/DB_SCHEMA.md");
const BEGIN = "<!-- BEGIN GENERATED: tables (scripts/db-schema-doc.mjs) -->";
const END = "<!-- END GENERATED: tables -->";

export function collect() {
  const mods = new Map();
  for (const f of readdirSync(MIG).filter((f) => f.endsWith(".up.sql")).sort()) {
    const mod = /^\d+_([a-z0-9]+)/.exec(f)?.[1];
    if (!mod) continue;
    const sql = readFileSync(join(MIG, f), "utf8").replace(/--[^\n]*/g, "");
    if (!mods.has(mod)) mods.set(mod, { files: [], tables: [] });
    const e = mods.get(mod);
    e.files.push(f);
    for (const m of sql.matchAll(/CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?`?([A-Za-z0-9_]+)`?[^;]*?;/gis)) {
      const body = m[0];
      const cols = [...body.matchAll(/^\s+`([A-Za-z0-9_]+)`\s/gm)].length;
      const fks = [...body.matchAll(/FOREIGN KEY/gi)].length;
      e.tables.push({ name: m[1], cols, fks });
    }
  }
  return mods;
}

export function render(mods = collect()) {
  let total = 0;
  const rows = [];
  for (const [mod, e] of mods) {
    total += e.tables.length;
    rows.push(`| \`${mod}\` | ${e.files.map((f) => `\`${f}\``).join(", ")} | ${e.tables.map((t) => `\`${t.name}\``).join(", ")} | ${e.tables.length} |`);
  }
  return `${BEGIN}
Daftar di bawah DIBANGKITKAN dari berkas baseline (\`node scripts/db-schema-doc.mjs\`); jangan diedit manual.
Kepemilikan tabel = nama berkas migrasi (\`NNNNNN_<modul>.up.sql\`) dan ditegakkan arch-check Rule T/F.
Modul \`dashboard\` dan \`reporting\` tidak memiliki tabel.

| Modul | Berkas migrasi | Tabel | Jumlah |
|---|---|---|---|
${rows.join("\n")}
| **Total** | ${mods.size} berkas modul | | **${total}** |
${END}`;
}

if (process.argv[1] && process.argv[1].endsWith("db-schema-doc.mjs")) {
  const doc = readFileSync(DOC, "utf8").replace(/\r\n/g, "\n");
  const a = doc.indexOf(BEGIN);
  const b = doc.indexOf(END);
  if (a < 0 || b < 0) {
    console.error("db-schema-doc: markers not found in docs/DB_SCHEMA.md");
    process.exit(1);
  }
  const next = doc.slice(0, a) + render() + doc.slice(b + END.length);
  if (process.argv.includes("--check")) {
    if (next !== doc) {
      console.error("db-schema-doc: docs/DB_SCHEMA.md table list is stale — run node scripts/db-schema-doc.mjs");
      process.exit(1);
    }
    console.log("db-schema-doc: up to date.");
  } else {
    writeFileSync(DOC, next);
    console.log("db-schema-doc: wrote docs/DB_SCHEMA.md");
  }
}
