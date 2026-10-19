import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";
import path from "node:path";
import { ROOT } from "../helpers/env";
import { call, type Role } from "../helpers/api";
import { collectErrors, loginAs } from "../helpers/ui";

// T-01 (padanan 18-role-access-matrix legacy): matriks role seed x menu/rute x endpoint kunci.
//
// Daftar menu DIBANGKITKAN dari registry.tsx (label, path, izin, seksi). Deklarasi `ACCESS` di bawah
// menyatakan, untuk tiap izin yang dipakai menu/endpoint, role seed e2e mana yang memegangnya
// (a=admin/owner, k=kasir, g=gudang, u=akuntan). Spec GAGAL bila:
//   - sebuah menu di registry memakai izin yang belum dideklarasikan (rute/menu baru tanpa deklarasi),
//   - ada deklarasi yang tak dipakai lagi (usang),
//   - deklarasi berbeda dari izin nyata role (/auth/me),
//   - UI/API berperilaku tak sesuai: diizinkan => halaman terbuka / API bukan 401-403;
//     ditolak => /403 dan API 403.
// Lapisan Go yang melengkapi: apps/api/internal/integration/role_matrix_test.go (role sistem x semua rute).
const ROLES: { role: Exclude<Role, "adminCb2">; letter: string }[] = [
  { role: "admin", letter: "a" },
  { role: "kasir", letter: "k" },
  { role: "gudang", letter: "g" },
  { role: "akuntan", letter: "u" },
];

const ACCESS: Record<string, string> = {
  "dashboard.view": "akgu",
  "sales.create": "ak",
  "sales.view": "akgu",
  "delivery.view": "akg",
  "salesreturn.view": "ak",
  "purchasing.view": "akgu",
  "goodsreceipt.view": "akg",
  "purchasereturn.view": "a",
  "payment.view": "aku",
  "stock.view": "akg",
  "stock.transfer": "ag",
  "stock.adjust": "akg",
  "stock.update": "ag",
  "products.view": "akgu",
  "product_categories.manage": "a",
  "product_import.manage": "a",
  "customers.view": "akgu",
  "suppliers.view": "akgu",
  "member_types.view": "a",
  "finance.posting.view": "au",
  "finance.expense.view": "au",
  "finance.report.view": "au",
  "finance.tax_report.view": "au",
  "finance.tax_adjustment.view": "au",
  "finance.manage": "au",
  "finance.journal.view": "au",
  "finance.view": "au",
  "reporting.view": "au",
  "audit.view": "a",
  "assistant.view": "a",
  "users.view": "a",
  "roles.view": "a",
  "branches.view": "a",
  "company.view": "a",
};

// Endpoint kunci (GET aman) -> izin yang dituntut guard-nya.
const ENDPOINTS: { path: string; perm: string }[] = [
  { path: "/dashboard/summary", perm: "dashboard.view" },
  { path: "/sales-orders?limit=1", perm: "sales.view" },
  { path: "/purchase-orders?limit=1", perm: "purchasing.view" },
  { path: "/deliveries?limit=1", perm: "delivery.view" },
  { path: "/goods-receipts?limit=1", perm: "goodsreceipt.view" },
  { path: "/sales-returns?limit=1", perm: "salesreturn.view" },
  { path: "/purchase-returns?limit=1", perm: "purchasereturn.view" },
  { path: "/payments?limit=1", perm: "payment.view" },
  { path: "/stock/balances?limit=1", perm: "stock.view" },
  { path: "/products?limit=1", perm: "products.view" },
  { path: "/customers?limit=1", perm: "customers.view" },
  { path: "/suppliers?limit=1", perm: "suppliers.view" },
  { path: "/member-types", perm: "member_types.view" },
  { path: "/finance/journals?limit=1", perm: "finance.journal.view" },
  { path: "/finance/expenses?limit=1", perm: "finance.expense.view" },
  { path: "/finance/tax-periods", perm: "finance.tax_report.view" },
  { path: "/finance/reports/trial-balance?asOf=2026-01-31", perm: "finance.report.view" },
  { path: "/reporting/orders", perm: "reporting.view" },
  { path: "/audit-logs?limit=1", perm: "audit.view" },
  { path: "/assistant/config", perm: "assistant.view" },
  { path: "/users?limit=1", perm: "users.view" },
  { path: "/roles", perm: "roles.view" },
  { path: "/branches", perm: "branches.view" },
  { path: "/company/settings", perm: "company.view" },
];

// ---- registry (sumber kebenaran menu) ---------------------------------------
const web = path.join(ROOT, "apps/web/src/app/routes");
const paths = Object.fromEntries(
  [...readFileSync(path.join(web, "route-paths.ts"), "utf8").matchAll(/^\s+(\w+): "([^"]+)",/gm)].map((m) => [m[1], m[2]]),
);
// Hanya blok MENU_GROUPS (QUICK_ACTIONS memakai bentuk yang sama tetapi bukan menu).
const registrySrc = readFileSync(path.join(web, "registry.tsx"), "utf8");
const menuSrc = registrySrc.slice(registrySrc.indexOf("export const MENU_GROUPS"), registrySrc.indexOf("export function groupSections"));
const items = [
  ...menuSrc.matchAll(
    /label: "([^"]+)", path: ROUTE_PATHS\.(\w+), icon: \w+, perm: "([^"]+)"(?:, section: "([^"]+)")?/g,
  ),
].map((m) => ({ label: m[1], path: paths[m[2]], perm: m[3], section: m[4] }));

const allowed = (perm: string, letter: string) => (ACCESS[perm] ?? "").includes(letter);

test.describe("E12 matriks akses role (T-01)", () => {
  test("deklarasi mencakup persis izin menu registry dan endpoint kunci (tanpa menu/izin baru yang lolos)", () => {
    expect(items.length, "registry terbaca").toBeGreaterThan(40);
    const used = new Set([...items.map((i) => i.perm), ...ENDPOINTS.map((e) => e.perm)]);
    const undeclared = [...used].filter((p) => !(p in ACCESS)).sort();
    expect(undeclared, "izin dipakai menu/endpoint tetapi belum dideklarasikan di ACCESS").toEqual([]);
    const stale = Object.keys(ACCESS).filter((p) => !used.has(p)).sort();
    expect(stale, "deklarasi ACCESS usang (tak dipakai menu/endpoint)").toEqual([]);
    for (const i of items) expect(i.path, i.label).toBeTruthy();
  });

  test("deklarasi sama dengan izin nyata tiap role (/auth/me); admin = seluruh katalog", async () => {
    const catalog = ((await call("admin", "GET", "/permissions")).data ?? []) as any[];
    const catalogCodes = new Set<string>(catalog.map((p) => (typeof p === "string" ? p : p.code)));
    for (const { role, letter } of ROLES) {
      const me = (await call(role, "GET", "/auth/me")).data;
      const have = new Set<string>(me.permissions);
      for (const perm of Object.keys(ACCESS)) {
        expect(have.has(perm), `${role} memegang ${perm}`).toBe(allowed(perm, letter));
        if (catalogCodes.size) expect(catalogCodes.has(perm), `${perm} ada di katalog`).toBe(true);
      }
      if (role === "admin" && catalogCodes.size) expect(have.size).toBe(catalogCodes.size);
    }
  });

  for (const { role, letter } of ROLES) {
    test(`API ${role}: endpoint kunci diizinkan/ditolak sesuai deklarasi`, async () => {
      const wrong: string[] = [];
      for (const e of ENDPOINTS) {
        const r = await call(role, "GET", e.path);
        if (allowed(e.perm, letter)) {
          if (r.status === 401 || r.status === 403) wrong.push(`${e.path}: seharusnya boleh, dapat ${r.status} ${r.message}`);
        } else if (r.status !== 403) {
          wrong.push(`${e.path}: seharusnya 403, dapat ${r.status}`);
        }
      }
      expect(wrong).toEqual([]);
    });

    if (role === "admin") continue; // admin (owner) sudah dilintasi seluruh menunya oleh E10.

    test(`UI ${role}: tiap path menu terbuka bila diizinkan, /403 bila tidak; sidebar sejalan`, async ({ page }) => {
      await loginAs(page, role);
      const errors = collectErrors(page);
      const failures: string[] = [];

      await page.goto("/dashboard");
      const nav = page.getByRole("navigation", { name: "Menu utama" });
      await expect(nav).toBeVisible();
      for (const item of items.filter((i) => !i.section)) {
        const link = nav.getByText(item.label, { exact: true });
        if (allowed(item.perm, letter)) {
          if ((await link.count()) === 0) failures.push(`sidebar ${item.label}: seharusnya tampil`);
        } else if ((await link.count()) > 0) {
          failures.push(`sidebar ${item.label}: seharusnya tersembunyi`);
        }
      }

      for (const item of items) {
        errors.length = 0;
        await page.goto(item.path);
        if (allowed(item.perm, letter)) {
          await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
          if (new URL(page.url()).pathname !== item.path) failures.push(`${item.label}: dialihkan ke ${new URL(page.url()).pathname}`);
          if (await page.getByText("Anda tidak memiliki akses").count()) failures.push(`${item.label}: 403 padahal diizinkan`);
        } else if (!/\/403/.test(page.url())) {
          await page.waitForURL(/\/403/, { timeout: 5_000 }).catch(() => failures.push(`${item.label}: seharusnya /403, di ${page.url()}`));
        }
        if (errors.length) failures.push(`${item.label}: ${errors.join(" | ")}`);
      }
      expect(failures).toEqual([]);
    });
  }
});
