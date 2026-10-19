#!/usr/bin/env node
// Prepares the MySQL used by the integration tests (apps/api/internal/integration).
//
//   node scripts/test-db.mjs status   is MySQL reachable? does mini_erp_test exist?
//   node scripts/test-db.mjs up       start MySQL if it is down, create the test DB + restricted user
//   node scripts/test-db.mjs reset    drop and recreate ONLY mini_erp_test (name is hardcoded)
//   node scripts/test-db.mjs dsn      print the TEST_DB_DSN value
//
// The MySQL server on this machine is shared with other projects, so this script
// (1) never starts a second instance on a busy port, (2) only ever touches the
// database named below, and (3) creates a user that can reach nothing else.
//
// Overrides: LARAGON_MYSQL_DIR (folder containing bin/mysql.exe), MYSQL_ROOT_PASSWORD.
import { spawn, spawnSync } from "node:child_process";
import { existsSync, readdirSync, writeFileSync, readFileSync, unlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const DB = "mini_erp_test";
const USER = "mini_erp_test";
const PASS = "mini_erp_test";
const HOST = "127.0.0.1";
const PORT = "3306";
const DSN = `${USER}:${PASS}@tcp(${HOST}:${PORT})/${DB}`;
const PID_FILE = join(tmpdir(), "mini-erp-test-mysql.pid");

function findMysqlDir() {
  if (process.env.LARAGON_MYSQL_DIR) return process.env.LARAGON_MYSQL_DIR;
  const base = "C:/laragon/bin/mysql";
  if (existsSync(base)) {
    const dirs = readdirSync(base).filter((d) => existsSync(join(base, d, "bin")));
    if (dirs.length) return join(base, dirs.sort().at(-1));
  }
  return "";
}

const dir = findMysqlDir();
const exe = (name) => (dir ? join(dir, "bin", `${name}.exe`) : name);
const rootPass = process.env.MYSQL_ROOT_PASSWORD ?? "";

function rootArgs() {
  const a = ["--protocol=tcp", `-h${HOST}`, `-P${PORT}`, "-uroot"];
  if (rootPass) a.push(`-p${rootPass}`);
  return a;
}

function sql(statement) {
  return spawnSync(exe("mysql"), [...rootArgs(), "-N", "-B", "-e", statement], { encoding: "utf8" });
}

function ping() {
  const r = spawnSync(exe("mysqladmin"), [...rootArgs(), "ping"], { encoding: "utf8" });
  return r.status === 0;
}

function fail(msg) {
  console.error(`test-db: ${msg}`);
  process.exit(1);
}

function status() {
  if (!ping()) {
    console.log(`MySQL tidak terjangkau di ${HOST}:${PORT}. Jalankan: node scripts/test-db.mjs up`);
    return false;
  }
  const r = sql(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='${DB}'`);
  const exists = r.status === 0 && r.stdout.trim() === "1";
  console.log(`MySQL hidup di ${HOST}:${PORT}; database ${DB}: ${exists ? "ada" : "belum ada"}`);
  return true;
}

async function start() {
  if (ping()) return; // already up (shared server): never start a second instance
  if (!dir) fail("folder MySQL Laragon tidak ditemukan; set LARAGON_MYSQL_DIR atau nyalakan MySQL manual.");
  const ini = join(dir, "my.ini");
  const args = existsSync(ini) ? [`--defaults-file=${ini}`, "--console"] : ["--console"];
  const child = spawn(exe("mysqld"), args, { detached: true, stdio: "ignore", windowsHide: true });
  child.unref();
  writeFileSync(PID_FILE, String(child.pid));
  console.log(`menyalakan mysqld (pid ${child.pid}), menunggu siap...`);
  for (let i = 0; i < 60; i++) {
    await new Promise((r) => setTimeout(r, 1000));
    if (ping()) return;
  }
  fail("MySQL belum siap setelah 60 detik; periksa berkas .err di folder data MySQL.");
}

function provision() {
  const stmts = [
    `CREATE DATABASE IF NOT EXISTS \`${DB}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci`,
    `CREATE USER IF NOT EXISTS '${USER}'@'localhost' IDENTIFIED BY '${PASS}'`,
    `CREATE USER IF NOT EXISTS '${USER}'@'127.0.0.1' IDENTIFIED BY '${PASS}'`,
    `GRANT ALL PRIVILEGES ON \`${DB}\`.* TO '${USER}'@'localhost'`,
    `GRANT ALL PRIVILEGES ON \`${DB}\`.* TO '${USER}'@'127.0.0.1'`,
  ];
  for (const s of stmts) {
    const r = sql(s);
    if (r.status !== 0) fail(`gagal menjalankan: ${s}\n${r.stderr}`);
  }
}

const cmd = process.argv[2] ?? "status";
switch (cmd) {
  case "status":
    process.exit(status() ? 0 : 1);
    break;
  case "up":
    await start();
    provision();
    status();
    console.log(`TEST_DB_DSN=${DSN}`);
    break;
  case "reset": {
    if (!ping()) fail("MySQL tidak terjangkau; jalankan `up` dulu.");
    for (const s of [`DROP DATABASE IF EXISTS \`${DB}\``]) {
      const r = sql(s);
      if (r.status !== 0) fail(r.stderr);
    }
    provision();
    console.log(`${DB} dibuat ulang (kosong).`);
    break;
  }
  case "dsn":
    console.log(DSN);
    break;
  case "down": {
    if (!existsSync(PID_FILE)) {
      console.log("tidak ada proses yang dinyalakan skrip ini; MySQL milik Anda tidak disentuh.");
      break;
    }
    const pid = Number(readFileSync(PID_FILE, "utf8"));
    try {
      process.kill(pid);
      console.log(`mysqld pid ${pid} dihentikan.`);
    } catch {
      console.log(`pid ${pid} sudah tidak ada.`);
    }
    unlinkSync(PID_FILE);
    break;
  }
  default:
    fail("perintah: status | up | reset | down | dsn");
}
