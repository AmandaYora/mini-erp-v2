#!/usr/bin/env node
// One-command verification: build, vet, test (Go + web), type-check, lint, arch-check.
//
//   node scripts/verify.mjs            everything (needs MySQL for the integration tests)
//   node scripts/verify.mjs --fast     skips Go integration tests and Vitest
//   node scripts/verify.mjs --api      Go steps only
//   node scripts/verify.mjs --web      web steps + arch-check only
//
// Why this exists instead of one `&&` chain in package.json: it works the same in
// PowerShell, cmd and sh, prints per-step timings, fails with a clear message when
// MySQL is down, and sets GOFLAGS=-buildvcs=false so a repo that git refuses to
// read ("dubious ownership") does not break `go build`/`go vet` on the main package.
import { spawnSync } from "node:child_process";
import { performance } from "node:perf_hooks";

const args = new Set(process.argv.slice(2));
const fast = args.has("--fast");
const onlyApi = args.has("--api");
const onlyWeb = args.has("--web");
const doApi = !onlyWeb;
const doWeb = !onlyApi;

const env = { ...process.env };
if (!/-buildvcs=/.test(env.GOFLAGS ?? "")) {
  env.GOFLAGS = `${env.GOFLAGS ?? ""} -buildvcs=false`.trim();
}

const results = [];

function run(name, cmd, cmdArgs, cwd) {
  const t0 = performance.now();
  process.stdout.write(`\n=== ${name}\n`);
  const r = spawnSync(cmd, cmdArgs, { cwd, env, stdio: "inherit", shell: true });
  const secs = ((performance.now() - t0) / 1000).toFixed(1);
  const ok = r.status === 0;
  results.push({ name, ok, secs });
  if (!ok) {
    summary();
    console.error(`\nverify: GAGAL di langkah "${name}" (kode ${r.status ?? "?"}).`);
    process.exit(r.status || 1);
  }
}

function summary() {
  console.log("\n--- ringkasan");
  for (const r of results) console.log(`${r.ok ? "OK  " : "GAGAL"}  ${r.secs.padStart(7)}s  ${r.name}`);
}

if (doApi) {
  if (!fast) {
    // Fail early and clearly instead of letting TestMain report a connection error.
    const pre = spawnSync("node", ["scripts/test-db.mjs", "status"], { env, stdio: "inherit", shell: true });
    if (pre.status !== 0 && !env.INTEGRATION_OPTIONAL) {
      console.error("\nverify: MySQL test tidak terjangkau. Jalankan `node scripts/test-db.mjs up` (atau --fast).");
      process.exit(1);
    }
  }
  run("go build", "go", ["build", "./..."], "apps/api");
  run("go vet", "go", ["vet", "./..."], "apps/api");
  if (fast) {
    // --fast = tanpa MySQL: paket ETL memakai etltest yang melewati (skip) tesnya bila
    // INTEGRATION_OPTIONAL di-set; tanpa ini "--fast" diam-diam butuh MySQL hidup.
    env.INTEGRATION_OPTIONAL ??= "1";
    const list = spawnSync("go", ["list", "./..."], { cwd: "apps/api", env, encoding: "utf8", shell: true });
    const pkgs = list.stdout.split(/\r?\n/).filter((p) => p && !p.endsWith("/internal/integration"));
    run("go test (tanpa integrasi)", "go", ["test", "-count=1", ...pkgs], "apps/api");
  } else {
    run("go test (termasuk integrasi)", "go", ["test", "-count=1", "-timeout=40m", "./..."], "apps/api");
  }
}

if (doWeb) {
  run("tsc", "npx", ["tsc", "--noEmit"], "apps/web");
  run("eslint", "npm", ["run", "lint"], "apps/web");
  if (!fast) run("vitest", "npm", ["run", "test"], "apps/web");
}

run("ui-ratchet", "node", ["scripts/ui-ratchet.mjs"], ".");
run("arch-check", "node", ["scripts/arch-check.mjs"], ".");
run("arch-check self-test", "node", ["scripts/arch-check.mjs", "--self-test"], ".");
run("sqlc config in sync", "node", ["scripts/sqlc-sync.mjs", "--check"], ".");
run("DB_SCHEMA.md table list in sync", "node", ["scripts/db-schema-doc.mjs", "--check"], ".");
run("contract-check (FE <-> router)", "node", ["scripts/contract-check.mjs"], ".");
summary();
console.log("\nverify: HIJAU");
