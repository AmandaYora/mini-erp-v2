import { test, expect } from "@playwright/test";
import { mkdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { ROOT } from "../helpers/env";
import { collectErrors, loginAs } from "../helpers/ui";

// Baseline/after visual untuk redesain UI (docs/plan/redesain-ui/PLAN.md, U0-1 & U7-6).
// Bukan assertion piksel: hanya memotret setiap rute menu agar sebelum/sesudah bisa dibandingkan.
//   VISUAL_LABEL=before|after   -> dist/visual/<label>/<slug>.png
const label = process.env.VISUAL_LABEL ?? "after";
const outDir = path.join(ROOT, "dist", "visual", label);

const web = path.join(ROOT, "apps/web/src/app/routes");
const paths = Object.fromEntries(
  [...readFileSync(path.join(web, "route-paths.ts"), "utf8").matchAll(/^\s+(\w+): "([^"]+)",/gm)].map((m) => [m[1], m[2]]),
);
const items = [...readFileSync(path.join(web, "registry.tsx"), "utf8").matchAll(/label: "([^"]+)", path: ROUTE_PATHS\.(\w+)/g)].map(
  (m) => ({ label: m[1], path: paths[m[2]] }),
);
const slug = (p: string) => p.replace(/^\//, "").replace(/[^a-z0-9]+/gi, "_") || "root";

test.describe("V00 visual baseline", () => {
  test.beforeAll(() => mkdirSync(outDir, { recursive: true }));

  test("desktop 1366x800: semua rute menu", async ({ page }) => {
    test.setTimeout(300_000);
    await page.setViewportSize({ width: 1366, height: 800 });
    await loginAs(page, "admin");
    const errors = collectErrors(page);
    for (const item of items) {
      await page.goto(item.path);
      await page.waitForLoadState("networkidle");
      await page.screenshot({ path: path.join(outDir, `d_${slug(item.path)}.png`) });
    }
    expect(errors).toEqual([]);
  });

  test("mobile 390x844: rute utama", async ({ page }) => {
    test.setTimeout(120_000);
    await page.setViewportSize({ width: 390, height: 844 });
    await loginAs(page, "admin");
    for (const p of ["/dashboard", "/sales-orders", "/products", "/finance/posting", "/stock"]) {
      await page.goto(p);
      await page.waitForLoadState("networkidle");
      await page.screenshot({ path: path.join(outDir, `m_${slug(p)}.png`) });
    }
  });

  test("tablet POS 1280x800 touch", async ({ browser }) => {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 }, hasTouch: true });
    const page = await ctx.newPage();
    await loginAs(page, "admin");
    await page.goto("/pos");
    await page.waitForLoadState("networkidle");
    await page.screenshot({ path: path.join(outDir, `t_pos.png`) });
    await ctx.close();
  });
});
