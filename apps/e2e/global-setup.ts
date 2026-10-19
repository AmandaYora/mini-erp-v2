import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import fs, { mkdirSync, writeFileSync, rmSync, existsSync } from "node:fs";
import path from "node:path";
import mysql from "mysql2/promise";
import { API_DIR, BASE_URL, DB_DSN, LOCK_NAME, ROOT, RUN_DIR, serverEnv } from "./helpers/env";

function parseDsn(dsn: string) {
  const m = dsn.match(/^([^:]+):([^@]*)@tcp\(([^:]+):(\d+)\)\/([^?]+)/);
  if (!m) throw new Error("DSN e2e tidak dikenali");
  return { user: m[1], password: m[2], host: m[3], port: Number(m[4]), database: m[5] };
}

function run(cmd: string, args: string[], cwd: string, env = process.env) {
  const r = spawnSync(cmd, args, { cwd, env, stdio: "inherit", shell: process.platform === "win32" });
  if (r.status !== 0) throw new Error(`${cmd} ${args.join(" ")} gagal (kode ${r.status})`);
}

export default async function globalSetup() {
  const cfg = parseDsn(DB_DSN);
  if (!cfg.database.endsWith("_test")) throw new Error("e2e hanya boleh memakai database *_test");

  // 1. Kunci advisory pada koneksi khusus selama seluruh run (server + seed + spec),
  //    sama dengan harness integrasi Go, supaya tak pernah bertumpuk dengan `go test`.
  const lockConn = await mysql.createConnection(cfg);
  const [rows] = (await lockConn.query(`SELECT GET_LOCK('${LOCK_NAME}', 3000) AS got`)) as any;
  if (rows[0].got !== 1) throw new Error("kunci database test tidak didapat");
  (globalThis as any).__e2eLock = lockConn;

  // 2. Build: web (disajikan server Go seperti produksi) dan dua binari Go.
  mkdirSync(RUN_DIR, { recursive: true });
  const env = { ...process.env, GOFLAGS: "-buildvcs=false" };
  if (process.env.E2E_SKIP_WEB_BUILD !== "1") run("npm", ["run", "build:web"], ROOT);
  run("go", ["build", "-o", path.join(RUN_DIR, "server.exe"), "./cmd/server"], API_DIR, env);
  run("go", ["build", "-o", path.join(RUN_DIR, "seed.exe"), "./cmd/seed"], API_DIR, env);

  // 3. Seed deterministik (migrasi bila perlu + TRUNCATE + data tetap).
  run(path.join(RUN_DIR, "seed.exe"), ["-profile=e2e"], API_DIR, serverEnv());

  // 4. Server Go dengan PUBLIC_DIR=apps/web/dist.
  const logPath = path.join(RUN_DIR, "server.log");
  rmSync(logPath, { force: true });
  const out = fs.openSync(logPath, "a");
  const child: ChildProcess = spawn(path.join(RUN_DIR, "server.exe"), [], {
    cwd: API_DIR,
    env: serverEnv(),
    stdio: ["ignore", out, out],
    windowsHide: true,
  });
  writeFileSync(path.join(RUN_DIR, "server.pid"), String(child.pid));
  (globalThis as any).__e2eServer = child;

  const deadline = Date.now() + 60_000;
  for (;;) {
    try {
      const res = await fetch(`${BASE_URL}/api/v1/auth/me`);
      if (res.status > 0) break;
    } catch {
      /* belum siap */
    }
    if (child.exitCode !== null) throw new Error(`server berhenti (lihat ${logPath})`);
    if (Date.now() > deadline) throw new Error(`server tidak siap (lihat ${logPath})`);
    await new Promise((r) => setTimeout(r, 300));
  }
  if (!existsSync(path.join(ROOT, "apps/web/dist/index.html"))) throw new Error("apps/web/dist belum dibangun");
}
