#!/usr/bin/env node
// Serialises everything that touches the shared integration database.
//
//   node scripts/db-lock.mjs -- go test ./internal/integration -run X -count=1
//
// The integration tests TRUNCATE mini_erp_test, so two runs at once (two
// terminals, two agents) corrupt each other. The lock is a directory created
// atomically; a holder that died is detected by its pid and by age.
import { spawn } from "node:child_process";
import { mkdirSync, rmSync, readFileSync, writeFileSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const LOCK = join(tmpdir(), "mini-erp-test-db.lock");
const STALE_MS = 45 * 60 * 1000;
const sep = process.argv.indexOf("--");
const cmd = sep >= 0 ? process.argv.slice(sep + 1) : [];
if (cmd.length === 0) {
  console.error("usage: node scripts/db-lock.mjs -- <command...>");
  process.exit(2);
}

function alive(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function tryAcquire() {
  try {
    mkdirSync(LOCK);
    writeFileSync(join(LOCK, "pid"), String(process.pid));
    return true;
  } catch {
    try {
      const pid = Number(readFileSync(join(LOCK, "pid"), "utf8"));
      const age = Date.now() - statSync(LOCK).mtimeMs;
      if ((pid && !alive(pid)) || age > STALE_MS) {
        rmSync(LOCK, { recursive: true, force: true });
      }
    } catch {
      /* the holder is mid-creation; retry */
    }
    return false;
  }
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
let waited = 0;
while (!tryAcquire()) {
  if (waited === 0) console.error("db-lock: menunggu pemakai database test lain…");
  await sleep(2000);
  waited += 2;
}
const release = () => rmSync(LOCK, { recursive: true, force: true });
process.on("SIGINT", () => {
  release();
  process.exit(130);
});
process.on("SIGTERM", () => {
  release();
  process.exit(143);
});

const child = spawn(cmd[0], cmd.slice(1), { stdio: "inherit", shell: process.platform === "win32" });
child.on("exit", (code) => {
  release();
  process.exit(code ?? 1);
});
