import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";
import path from "node:path";
import { ROOT } from "../helpers/env";
import { collectErrors, loginAs } from "../helpers/ui";

// Menu & rute dibangkitkan dari registry.tsx: setiap path menu harus termuat
// tanpa error runtime dan tanpa layar 403/404 untuk admin (owner).
const web = path.join(ROOT, "apps/web/src/app/routes");
const paths = Object.fromEntries(
  [...readFileSync(path.join(web, "route-paths.ts"), "utf8").matchAll(/^\s+(\w+): "([^"]+)",/gm)].map((m) => [m[1], m[2]]),
);
const items = [...readFileSync(path.join(web, "registry.tsx"), "utf8").matchAll(/label: "([^"]+)", path: ROUTE_PATHS\.(\w+)/g)].map(
  (m) => ({ label: m[1], path: paths[m[2]] }),
);

test.describe("E10 menu & rute smoke", () => {
  test("registry terbaca", () => {
    expect(items.length).toBeGreaterThan(40);
    for (const i of items) expect(i.path, i.label).toBeTruthy();
  });

  test("setiap path menu termuat tanpa error runtime", async ({ page }) => {
    await loginAs(page, "admin");
    const errors = collectErrors(page);
    const failures: string[] = [];
    for (const item of items) {
      errors.length = 0;
      await page.goto(item.path);
      await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
      await page.waitForLoadState("networkidle");
      const url = new URL(page.url());
      if (url.pathname !== item.path) failures.push(`${item.label}: dialihkan ke ${url.pathname}`);
      if (await page.getByText("Anda tidak memiliki akses").count()) failures.push(`${item.label}: 403`);
      if (await page.getByText("Terjadi kesalahan").count()) failures.push(`${item.label}: layar galat`);
      if (errors.length) failures.push(`${item.label}: ${errors.join(" | ")}`);
    }
    expect(failures).toEqual([]);
  });
});
