#!/usr/bin/env node
// UI ratchet (docs/plan/redesain-ui/PLAN.md, U13): mengukur sisa pola UI lama per modul
// dan menolak kenaikan. Pola yang sama dengan scripts/sqlc-migration-status.json.
//
//   node scripts/ui-ratchet.mjs            periksa: gagal bila ada angka naik
//   node scripts/ui-ratchet.mjs --update   tulis ulang status (hanya boleh MENURUN)
//   node scripts/ui-ratchet.mjs --report   cetak tabel tanpa memeriksa
//
// Kategori (semua dihitung di apps/web/src, tanpa *.test.*, tanpa modules/print, theme/, shared/components, dan shared/layouts):
//   arbitrary    ukuran/radius arbitrer: text-[ rounded-[ max-w-[ min-w-[ w-[ h-[
//   loadingText  teks "Memuat" tulis-tangan
//   rawTable     <table mentah
//   rawInput     input checkbox/radio/file mentah
//   linkButton   <Link> membungkus <Button>
//   segmented    <Segmented di halaman/komponen modul
//   hex          warna hex mentah di className/style
//
// Audit token (U6-4, nol toleransi, terpisah dari ratchet): di SELURUH apps/web/src kecuali theme/,
// modules/print (kertas), dan *.test.*, tidak boleh ada warna mentah — hex, rgb()/hsl()/oklch(), atau
// kelas palet Tailwind (bg-red-500, text-white, …). Warna hanya lewat token di theme/theme.css, supaya
// tema gelap kelak cukup menambah blok `.dark`. Baris yang memang butuh warna tetap (mis. latar putih
// QR) diberi `ui-ratchet-ignore` beserta alasannya.
import { readdirSync, readFileSync, writeFileSync, existsSync, statSync } from "node:fs";
import { join, relative, sep } from "node:path";

const ROOT = new URL("..", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1");
const SRC = join(ROOT, "apps", "web", "src");
const STATUS = join(ROOT, "scripts", "ui-migration-status.json");

const CATEGORIES = {
  arbitrary: /\b(?:text|rounded|max-w|min-w|w|h|min-h|max-h)-\[/g,
  loadingText: /Memuat(?:\.\.\.|…)/g,
  rawTable: /<table\b/g,
  rawInput: /type=["'](?:checkbox|radio|file)["']/g,
  linkButton: /<Link\b[^>]*>\s*<Button\b/g,
  segmented: /<Segmented\b/g,
  hex: /["'`(\s[]#[0-9a-fA-F]{6}\b|["'`(\s[]#[0-9a-fA-F]{3}\b(?![0-9a-fA-F])/g,
};

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, out);
    else if (/\.(tsx|ts)$/.test(name) && !/\.test\.(tsx|ts)$/.test(name)) out.push(p);
  }
  return out;
}

function scopeOf(file) {
  const rel = relative(SRC, file).split(sep);
  if (rel[0] === "theme" || rel[0] === "test") return null;
  if (rel.includes("primitives")) return null; // kode shadcn hasil generate (PLAN U3)
  // Kit DS itu sendiri (ui/, patterns/, feedback/, layouts/) = implementasi, bukan kode fitur.
  if (rel[0] === "shared" && (rel[1] === "components" || rel[1] === "layouts")) return null;
  if (rel[0] === "modules") return rel[1] === "print" ? null : rel[1];
  return rel[0] === "shared" || rel[0] === "app" ? rel[0] : null;
}

function measure() {
  const res = {};
  for (const file of walk(SRC)) {
    const scope = scopeOf(file);
    if (!scope) continue;
    // Baris bertanda `ui-ratchet-ignore` = implementasi primitif yang sah (mis. FileDropzone).
    const text = readFileSync(file, "utf8")
      .split("\n")
      .filter((l) => !l.includes("ui-ratchet-ignore"))
      .join("\n");
    const row = (res[scope] ??= Object.fromEntries(Object.keys(CATEGORIES).map((k) => [k, 0])));
    for (const [cat, re] of Object.entries(CATEGORIES)) row[cat] += (text.match(re) ?? []).length;
  }
  return Object.fromEntries(Object.entries(res).sort(([a], [b]) => a.localeCompare(b)));
}

const PALETTE =
  "red|blue|green|yellow|gray|grey|slate|zinc|neutral|stone|orange|amber|lime|emerald|teal|cyan|sky|indigo|violet|purple|fuchsia|pink|rose";
const RAW_COLOR = [
  ["hex", CATEGORIES.hex],
  ["fungsi warna", /\b(?:rgba?|hsla?|oklch|oklab|lab|lch)\(/g],
  [
    "kelas palet",
    new RegExp(
      `\\b(?:bg|text|border(?:-[trblxy])?|ring|ring-offset|fill|stroke|from|to|via|outline|divide|shadow|decoration|accent|caret|placeholder)-(?:(?:${PALETTE})-\\d{2,3}|white|black)\\b`,
      "g",
    ),
  ],
];

function tokenAudit() {
  const found = [];
  for (const file of walk(SRC)) {
    const rel = relative(SRC, file).split(sep);
    if (rel[0] === "theme" || rel[0] === "test" || (rel[0] === "modules" && rel[1] === "print")) continue;
    readFileSync(file, "utf8")
      .split("\n")
      .forEach((line, i) => {
        if (line.includes("ui-ratchet-ignore")) return;
        for (const [kind, re] of RAW_COLOR) {
          for (const m of line.matchAll(re)) found.push(`${rel.join("/")}:${i + 1} ${kind} ${m[0].replace(/^["'`(\s[]/, "")}`);
        }
      });
  }
  return found;
}

const args = new Set(process.argv.slice(2));
const rawColors = tokenAudit();
const now = measure();
const cats = Object.keys(CATEGORIES);
const total = Object.fromEntries(cats.map((c) => [c, Object.values(now).reduce((n, r) => n + r[c], 0)]));

if (args.has("--report")) {
  console.log(["scope".padEnd(16), ...cats.map((c) => c.padStart(12))].join(" "));
  for (const [s, r] of Object.entries(now)) console.log([s.padEnd(16), ...cats.map((c) => String(r[c]).padStart(12))].join(" "));
  console.log(["TOTAL".padEnd(16), ...cats.map((c) => String(total[c]).padStart(12))].join(" "));
  console.log(`audit token: ${rawColors.length} warna mentah`);
  for (const f of rawColors) console.log(`  ${f}`);
  process.exit(0);
}

const prev = existsSync(STATUS) ? JSON.parse(readFileSync(STATUS, "utf8")) : null;

if (args.has("--update") || !prev) {
  const base = prev?.scopes ?? {};
  const regress = [];
  for (const [s, r] of Object.entries(now)) for (const c of cats) if (base[s] && r[c] > base[s][c]) regress.push(`${s}.${c}: ${base[s][c]} -> ${r[c]}`);
  if (regress.length > 0) {
    console.error("ui-ratchet: --update menolak angka naik:\n  " + regress.join("\n  "));
    process.exit(1);
  }
  writeFileSync(STATUS, JSON.stringify({ note: "Dihasilkan scripts/ui-ratchet.mjs. Angka hanya boleh turun.", total, scopes: now }, null, 2) + "\n");
  console.log(`ui-ratchet: status ditulis (${Object.keys(now).length} scope). total=${JSON.stringify(total)}`);
  process.exit(0);
}

if (rawColors.length > 0) {
  console.error(
    "ui-ratchet: GAGAL — warna mentah di luar theme/ (audit token U6-4; pakai kelas token, atau beri `ui-ratchet-ignore` + alasan):\n  " +
      rawColors.join("\n  "),
  );
  process.exit(1);
}

const bad = [];
for (const [s, r] of Object.entries(now)) {
  const base = prev.scopes[s] ?? Object.fromEntries(cats.map((c) => [c, 0]));
  for (const c of cats) if (r[c] > base[c]) bad.push(`${s}.${c}: batas ${base[c]}, sekarang ${r[c]}`);
}
if (bad.length > 0) {
  console.error("ui-ratchet: GAGAL — pola UI lama bertambah (lihat docs/plan/redesain-ui/PLAN.md §10):\n  " + bad.join("\n  "));
  process.exit(1);
}
const improved = cats.filter((c) => total[c] < prev.total[c]);
console.log(`ui-ratchet: OK (audit token: 0 warna mentah). total=${JSON.stringify(total)}${improved.length ? `  (turun: ${improved.join(", ")} — jalankan --update untuk mengunci)` : ""}`);
