#!/usr/bin/env node
// FE <-> router contract check.
//
// Every API call written in apps/web/src (apiGet/apiPost/.../httpClient.*) must
// match a route registered by the Go backend on BOTH path and HTTP method. A
// call that matches nothing is a bug that only shows up at runtime (a 404 or
// 405 in the browser), so this fails the build instead.
//
// Backend routes that no frontend code calls are listed for information (they
// may be used by non-web clients) and never fail the check.
//
//   node scripts/contract-check.mjs          exit 1 on any unmatched call
//   node scripts/contract-check.mjs --list   also print every backend route not called by the web app
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative, sep } from "node:path";

const ROOT = process.cwd();
const API = join(ROOT, "apps/api/internal");
const WEB = join(ROOT, "apps/web/src");
const showUnused = process.argv.includes("--list");

function walk(dir, accept, out = []) {
  for (const name of readdirSync(dir)) {
    if (name === "node_modules" || name === "dist") continue;
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, accept, out);
    else if (accept(name)) out.push(p);
  }
  return out;
}
const rel = (p) => relative(ROOT, p).split(sep).join("/");

// --- backend: "METHOD /api/v1/..." string literals in non-test Go files -------------
const routes = new Map(); // "METHOD /path" -> source
for (const f of walk(API, (n) => n.endsWith(".go") && !n.endsWith("_test.go"))) {
  const src = readFileSync(f, "utf8");
  for (const m of src.matchAll(/"(GET|POST|PUT|PATCH|DELETE) (\/api\/v1[^"]*)"/g)) {
    routes.set(`${m[1]} ${m[2]}`, rel(f));
  }
}
const matchers = [...routes.keys()].map((k) => {
  const [method, path] = k.split(" ");
  const re = new RegExp("^" + path.replace(/\{[^}]+\}/g, "[^/]+") + "$");
  return { key: k, method, path, re };
});

// --- frontend: apiXxx("/api/v1/...") and httpClient.verb("/api/v1/...") ------------
const VERB = {
  apiGet: "GET", apiGetFull: "GET", apiPage: "GET", apiPageFull: "GET",
  apiPost: "POST", apiPostFull: "POST", apiPut: "PUT", apiPutFull: "PUT",
  apiPatch: "PATCH", apiPatchFull: "PATCH", apiDelete: "DELETE", apiDeleteFull: "DELETE",
  apiUpload: "POST", apiUploadFull: "POST",
};
const callRe = /\b(api[A-Z][A-Za-z]+|httpClient\.(?:get|post|put|patch|delete))\s*(?:<[\s\S]*?>)?\s*\(\s*([`"'])((?:\\.|(?!\2)[\s\S])*?)\2/g;

const used = new Set();
const bad = [];
let total = 0;
for (const f of walk(WEB, (n) => /\.(ts|tsx)$/.test(n) && !/\.test\./.test(n))) {
  const src = readFileSync(f, "utf8");
  for (const m of src.matchAll(callRe)) {
    const fn = m[1];
    const method = VERB[fn] ?? (fn.startsWith("httpClient.") ? fn.split(".")[1].toUpperCase() : null);
    if (!method) continue; // helper that is not an HTTP call (e.g. apiErrorMessage)
    const url = m[3];
    if (!url.startsWith("/api/v1")) continue;
    total++;
    const line = src.slice(0, m.index).split("\n").length;
    // Template parameters become a placeholder; query strings never take part in routing.
    const path = url.replace(/\$\{[^}]*\}/g, "ID").split("?")[0];
    const candidates = [path, path.replace(/\/ID$/, "")]; // tolerate a trailing optional id
    const hit = matchers.find((r) => r.method === method && candidates.some((c) => r.re.test(c)));
    if (hit) {
      used.add(hit.key);
    } else {
      const sameWrongMethod = matchers.filter((r) => candidates.some((c) => r.re.test(c))).map((r) => r.method);
      bad.push({ where: `${rel(f)}:${line}`, call: `${method} ${url}`, note: sameWrongMethod.length ? `path ada, method yang ada: ${sameWrongMethod.join(",")}` : "tidak ada rute dengan path itu" });
    }
  }
}

console.log(`contract-check: ${routes.size} rute backend, ${total} panggilan frontend, ${total - bad.length} cocok`);
// --- permissions: every code the UI checks must exist in a backend catalog --------
// Backend catalog = `{Code: "x.y", ...}` seeds in */contracts/*.go plus literal
// perm("x.y", ...) route guards. A typo (or a code only the UI knows, e.g.
// member_types.manage vs .view) would silently hide or expose a screen.
const CODE = "([a-z_]+(?:\\.[a-z_]+)+)";
const catalog = new Set();
for (const f of walk(API, (n) => n.endsWith(".go") && !n.endsWith("_test.go"))) {
  const src = readFileSync(f, "utf8");
  if (/[\\/]contracts[\\/]/.test(f)) {
    for (const m of src.matchAll(new RegExp(`Code:\\s*"${CODE}"`, "g"))) catalog.add(m[1]);
  }
  for (const m of src.matchAll(new RegExp(`\\bperm\\(\\s*"${CODE}"`, "g"))) catalog.add(m[1]);
}
const permRes = [
  new RegExp(`\\bperm\\s*[:=]\\s*\\{?\\s*["']${CODE}["']`, "g"),
  new RegExp(`\\bcan(?:Any)?\\(\\s*\\[?\\s*["']${CODE}["']`, "g"),
];
const permBad = [];
let permUses = 0;
for (const f of walk(WEB, (n) => /\.(ts|tsx)$/.test(n) && !/\.test\./.test(n))) {
  const src = readFileSync(f, "utf8");
  for (const re of permRes) {
    for (const m of src.matchAll(re)) {
      permUses++;
      if (!catalog.has(m[1])) {
        permBad.push({ code: m[1], where: `${rel(f)}:${src.slice(0, m.index).split("\n").length}` });
      }
    }
  }
}
console.log(`contract-check: ${catalog.size} kode izin backend, ${permUses} pemakaian izin di frontend`);

// Reported mismatches that are known and tracked: they stay visible as warnings
// but do not fail the build (scripts/contract-allowlist.json).
let allowPerms = {};
try {
  allowPerms = JSON.parse(readFileSync(join(ROOT, "scripts/contract-allowlist.json"), "utf8")).permissions ?? {};
} catch {
  /* no allowlist */
}
for (let i = permBad.length - 1; i >= 0; i--) {
  if (allowPerms[permBad[i].code]) {
    console.warn(`  PERINGATAN izin ${permBad[i].code} (di ${permBad[i].where}): ${allowPerms[permBad[i].code]}`);
    permBad.splice(i, 1);
  }
}

let failed = false;
if (bad.length) {
  failed = true;
  console.error("\nPanggilan frontend TANPA rute backend yang cocok:");
  for (const b of bad) console.error(`  ${b.call}\n    di ${b.where} (${b.note})`);
}
if (permBad.length) {
  failed = true;
  console.error("\nKode izin dipakai frontend tetapi TIDAK ada di katalog izin backend:");
  for (const b of permBad) console.error(`  ${b.code}\n    di ${b.where}`);
}
if (failed) process.exit(1);
const unused = [...routes.keys()].filter((k) => !used.has(k));
console.log(`contract-check: OK (${unused.length} rute backend tidak dipanggil web${showUnused ? ":" : "; --list untuk daftar"})`);
if (showUnused) for (const k of unused.sort()) console.log(`  ${k}   [${routes.get(k)}]`);
