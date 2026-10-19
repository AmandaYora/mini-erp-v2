import { spawnSync } from "node:child_process";
import { readFileSync, rmSync } from "node:fs";
import path from "node:path";
import { LOCK_NAME, RUN_DIR } from "./helpers/env";

export default async function globalTeardown() {
  const pidFile = path.join(RUN_DIR, "server.pid");
  try {
    const pid = Number(readFileSync(pidFile, "utf8"));
    if (pid) spawnSync("taskkill", ["/PID", String(pid), "/T", "/F"], { stdio: "ignore" });
    rmSync(pidFile, { force: true });
  } catch {
    /* sudah berhenti */
  }
  const conn = (globalThis as any).__e2eLock;
  if (conn) {
    try {
      await conn.query(`SELECT RELEASE_LOCK('${LOCK_NAME}')`);
    } finally {
      await conn.end();
    }
  }
}
