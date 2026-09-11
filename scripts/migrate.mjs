#!/usr/bin/env node
// golang-migrate wrapper that works the same in PowerShell, cmd, and sh.
// npm runs scripts through cmd.exe on Windows, which does not expand $VAR — so the DB
// connection is read here from .env and passed as a literal argument instead.
import { readFileSync, existsSync } from "node:fs";
import { spawnSync } from "node:child_process";

if (existsSync(".env")) {
  for (const line of readFileSync(".env", "utf8").split(/\r?\n/)) {
    const m = line.match(/^\s*([A-Z0-9_]+)\s*=\s*(.*)\s*$/);
    if (m && !process.env[m[1]]) process.env[m[1]] = m[2];
  }
}

const dsn = process.env.DB_DSN;
if (!dsn) {
  console.error("DB_DSN not found. Copy .env.example to .env and fill in DB_DSN.");
  process.exit(1);
}

const r = spawnSync(
  "migrate",
  ["-path", "migrations", "-database", `mysql://${dsn}`, ...process.argv.slice(2)],
  { cwd: "apps/api", stdio: "inherit", shell: true }
);
process.exit(r.status ?? 1);
