import { defineConfig } from "@playwright/test";
import { BASE_URL } from "./helpers/env";

// Satu worker: semua spec berbagi satu database (mini_erp_test) dan satu server.
export default defineConfig({
  testDir: "./specs",
  workers: 1,
  fullyParallel: false,
  retries: 0,
  timeout: 90_000,
  expect: { timeout: 10_000 },
  reporter: [["list"], ["html", { open: "never" }]],
  globalSetup: "./global-setup.ts",
  globalTeardown: "./global-teardown.ts",
  use: {
    baseURL: BASE_URL,
    locale: "id-ID",
    timezoneId: "Asia/Jakarta",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    viewport: { width: 1366, height: 800 },
  },
  // Mode Kasir (PLAN U5-4): `POS_TABLET=1 npx playwright test E02` menjalankan E02 pada viewport
  // tablet sentuh (1280x800, hasTouch). Tanpa variabel itu, konfigurasi bawaan tidak berubah.
  ...(process.env.POS_TABLET
    ? { projects: [{ name: "pos-tablet", use: { viewport: { width: 1280, height: 800 }, hasTouch: true } }] }
    : {}),
});
