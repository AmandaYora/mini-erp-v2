#!/usr/bin/env node
// Checkpoint kelengkapan PLAN penyempurnaan paritas (docs/plan/penyempurnaan-paritas/PLAN.md).
//
//   node scripts/plan-check.mjs            laporan progres + validasi (gagal bila ada pelanggaran)
//   node scripts/plan-check.mjs --list     tambah daftar tugas yang belum selesai/terblokir
//   node scripts/plan-check.mjs --strict   gagal juga bila ada tugas yang belum [x] (gerbang penutupan Z-01)
//
// Aturan yang ditegakkan (agar tidak ada yang terlewat):
//   1. ID tugas unik.
//   2. Tugas bertanda [x] WAJIB punya `Bukti:` terisi (≥ 3 karakter) di baris miliknya sendiri.
//   3. Tugas induk bertanda [x] hanya bila semua sub-tugasnya [x].
//   4. Tugas [~] (terblokir) WAJIB punya `Blokir:`.
//   5. Matriks keterlacakan (§10): setiap sumber temuan wajib ada, dan setiap ID yang dirujuk wajib ada sebagai tugas.
//   6. Tugas yang tidak muncul di matriks → peringatan (bukan galat), kecuali tugas proses (G-/Z-/N-/DOC-).
//   7. G-01 [x] mensyaratkan ADR-0018 ada.
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

const ROOT = new URL("..", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1");
const PLAN = join(ROOT, "docs", "plan", "penyempurnaan-paritas", "PLAN.md");
const ADR = join(ROOT, "knowledge", "decisions", "ADR-0018-keputusan-paritas.md");
const args = new Set(process.argv.slice(2));

const REQUIRED_SOURCES = [
  "F-01", "F-02", "F-03", "F-04", "F-05", "F-06",
  "§3.1", "§3.2", "§3.3", "§3.4", "§3.5", "§3.6", "§3.7", "§3.8", "§3.9", "§3.10", "§4", "§5",
];
const PROCESS_PREFIX = /^(G|Z|N|DOC)-/;

const lines = readFileSync(PLAN, "utf8").split(/\r?\n/);
const errors = [];
const warnings = [];

// --- tugas ---
const TASK = /^(\s*)- \[( |x|~)\] \*\*([A-Z]{1,3}-\d+[a-z]?)\*\*/;
const tasks = [];
let section = "(awal)";
lines.forEach((line, i) => {
  const h = /^## (.+)$/.exec(line);
  if (h) section = h[1];
  const m = TASK.exec(line);
  if (m) tasks.push({ id: m[3], mark: m[2], indent: m[1].length, line: i, section, children: [], parent: null });
});
for (const [idx, t] of tasks.entries()) {
  // blok = baris sesudah tugas hingga baris non-kosong berindentasi <= indent tugas
  let end = lines.length;
  for (let j = t.line + 1; j < lines.length; j += 1) {
    const l = lines[j];
    if (l.trim() === "") continue;
    const ind = /^(\s*)/.exec(l)[1].length;
    if (ind <= t.indent) { end = j; break; }
  }
  t.end = end;
  const own = new RegExp(`^\\s{${t.indent + 2}}- (Bukti|Blokir):(.*)$`);
  for (let j = t.line + 1; j < end; j += 1) {
    const m = own.exec(lines[j]);
    if (m) t[m[1].toLowerCase()] = m[2].trim();
  }
  for (const c of tasks.slice(idx + 1)) {
    if (c.line >= end) break;
    if (c.indent === t.indent + 2) { t.children.push(c); c.parent = t; }
  }
}

const ids = new Map();
for (const t of tasks) {
  if (ids.has(t.id)) errors.push(`ID ganda: ${t.id} (baris ${ids.get(t.id).line + 1} dan ${t.line + 1})`);
  else ids.set(t.id, t);
}
for (const t of tasks) {
  if (t.mark === "x") {
    if (!t.bukti || t.bukti.length < 3) errors.push(`${t.id}: dicentang [x] tanpa 'Bukti:' terisi (baris ${t.line + 1})`);
    const open = t.children.filter((c) => c.mark !== "x");
    if (open.length) errors.push(`${t.id}: dicentang [x] padahal sub-tugas belum selesai: ${open.map((c) => c.id).join(", ")}`);
  }
  if (t.mark === "~" && (!t.blokir || t.blokir.length < 3)) errors.push(`${t.id}: bertanda [~] tanpa 'Blokir:' terisi (baris ${t.line + 1})`);
}
if (ids.get("G-01")?.mark === "x" && !existsSync(ADR)) errors.push("G-01 [x] tetapi ADR-0018 tidak ada: knowledge/decisions/ADR-0018-keputusan-paritas.md");

// --- matriks keterlacakan ---
const mStart = lines.findIndex((l) => /^## 10\. Matriks keterlacakan/.test(l));
const matrixIds = new Set();
const matrixSources = new Set();
if (mStart < 0) errors.push("Bagian '## 10. Matriks keterlacakan' tidak ditemukan");
else {
  for (let i = mStart; i < lines.length; i += 1) {
    if (/^### Tabel penutupan/.test(lines[i])) break;
    const row = /^\|\s*([^|]+?)\s*\|\s*([^|]*)\|\s*([^|]*)\|\s*$/.exec(lines[i]);
    if (!row || /^-+$/.test(row[1].replace(/\s/g, "")) || row[1] === "Sumber (analisis)") continue;
    for (const s of REQUIRED_SOURCES) if (row[1].replace(/\s/g, "").startsWith(s) || row[1].includes(s)) matrixSources.add(s);
    for (const m of row[3].matchAll(/[A-Z]{1,3}-\d+[a-z]?/g)) matrixIds.add(m[0]);
  }
  for (const s of REQUIRED_SOURCES) if (!matrixSources.has(s)) errors.push(`Matriks: sumber '${s}' tidak punya baris`);
  for (const id of matrixIds) if (!ids.has(id) && !/^D\d/.test(id)) errors.push(`Matriks merujuk ID yang tidak ada sebagai tugas: ${id}`);
  for (const t of tasks) if (!matrixIds.has(t.id) && !PROCESS_PREFIX.test(t.id) && !t.parent) warnings.push(`${t.id} tidak muncul di matriks keterlacakan`);
}

// --- laporan ---
const bySection = new Map();
for (const t of tasks) {
  const s = bySection.get(t.section) ?? { done: 0, blocked: 0, total: 0 };
  s.total += 1;
  if (t.mark === "x") s.done += 1;
  if (t.mark === "~") s.blocked += 1;
  bySection.set(t.section, s);
}
const total = tasks.length;
const done = tasks.filter((t) => t.mark === "x").length;
const blocked = tasks.filter((t) => t.mark === "~").length;
console.log("PLAN penyempurnaan paritas — progres");
for (const [name, s] of bySection) console.log(`  ${String(s.done).padStart(3)}/${String(s.total).padEnd(3)} ${s.blocked ? `(${s.blocked} terblokir) ` : ""}${name}`);
console.log(`  ${"-".repeat(40)}\n  ${done}/${total} tugas selesai (${total ? Math.round((done / total) * 100) : 0}%), ${blocked} terblokir, ${total - done - blocked} terbuka`);

if (args.has("--list") || args.has("--strict")) {
  const open = tasks.filter((t) => t.mark !== "x");
  if (open.length) {
    console.log("\nBelum selesai:");
    for (const t of open) console.log(`  [${t.mark === "~" ? "~" : " "}] ${t.id}${t.mark === "~" ? `  — blokir: ${t.blokir}` : ""}`);
  }
}
if (warnings.length) console.log("\nPeringatan:\n  " + warnings.join("\n  "));
if (args.has("--strict") && done !== total) errors.push(`--strict: ${total - done} tugas belum [x]`);
if (errors.length) {
  console.error("\nGAGAL:\n  " + errors.join("\n  "));
  process.exit(1);
}
console.log("\nplan-check: OK (tidak ada pelanggaran aturan)");
