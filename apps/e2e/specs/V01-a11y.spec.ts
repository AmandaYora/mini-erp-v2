import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFileSync } from "node:fs";
import path from "node:path";
import { ROOT } from "../helpers/env";
import { loginAs } from "../helpers/ui";

// U7-1 (docs/plan/redesain-ui/PLAN.md): axe WCAG 2.x A/AA pada setiap rute menu.
// Pelanggaran tingkat serious/critical menggagalkan spec.
const web = path.join(ROOT, "apps/web/src/app/routes");
const paths = Object.fromEntries(
  [...readFileSync(path.join(web, "route-paths.ts"), "utf8").matchAll(/^\s+(\w+): "([^"]+)",/gm)].map((m) => [m[1], m[2]]),
);
const items = [...readFileSync(path.join(web, "registry.tsx"), "utf8").matchAll(/label: "([^"]+)", path: ROUTE_PATHS\.(\w+)/g)].map(
  (m) => ({ label: m[1], path: paths[m[2]] }),
);

test("V01 aksesibilitas: tanpa pelanggaran serious/critical di semua rute menu", async ({ page }) => {
  test.setTimeout(400_000);
  await page.setViewportSize({ width: 1366, height: 800 });
  await loginAs(page, "admin");
  const found: string[] = [];
  for (const it of items) {
    await page.goto(it.path);
    await page.waitForLoadState("networkidle");
    const res = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"]).analyze();
    for (const v of res.violations.filter((x) => x.impact === "serious" || x.impact === "critical")) {
      found.push(`${it.path} · ${v.id} (${v.impact}) ×${v.nodes.length}: ${v.nodes[0]?.target?.join(" ")}`);
    }
  }
  expect(found, found.join("\n")).toEqual([]);
});
