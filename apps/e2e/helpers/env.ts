import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
export const ROOT = path.resolve(here, "../../..");
export const API_DIR = path.join(ROOT, "apps/api");
export const WEB_DIST = path.join(ROOT, "apps/web/dist");
export const RUN_DIR = path.join(ROOT, "dist/e2e");
export const PORT = Number(process.env.E2E_PORT ?? 18080);
export const BASE_URL = `http://127.0.0.1:${PORT}`;
// Hanya database *_test yang dipakai bersama test integrasi Go (restricted user).
export const DB_DSN =
  process.env.E2E_DB_DSN ?? "mini_erp_test:mini_erp_test@tcp(127.0.0.1:3306)/mini_erp_test";
export const LOCK_NAME = "mini_erp_test_db";
export const PASSWORD = "E2e-Rahasia-2026";

/** Env server/seed. Tidak mencetak rahasia; JWT_SECRET hanya untuk DB uji. */
export function serverEnv(): NodeJS.ProcessEnv {
  return {
    ...process.env,
    APP_ENV: "test",
    APP_PORT: String(PORT),
    DB_DSN,
    JWT_SECRET: "e2e-jwt-secret-hanya-untuk-uji-minimal-32",
    PUBLIC_DIR: WEB_DIST,
    STORAGE_DRIVER: "local",
    STORAGE_DIR: path.join(RUN_DIR, "storage"),
    CORS_ALLOWED_ORIGINS: BASE_URL,
  };
}
