import { test, expect } from "@playwright/test";
import { PASSWORD } from "../helpers/env";
import { call } from "../helpers/api";
import { L } from "../helpers/labels";
import { loginAs, uiLogin } from "../helpers/ui";

test.describe("E01 auth + akses", () => {
  test("login gagal menampilkan pesan, login sukses masuk dashboard", async ({ page }) => {
    await uiLogin(page, "kasir", "salah-password-1");
    await expect(page.getByRole("alert")).toBeVisible();
    await expect(page).toHaveURL(/\/login/);

    await page.getByLabel(L.login.pass, { exact: true }).fill(PASSWORD);
    await page.getByRole("button", { name: L.login.submit }).click();
    await expect(page).toHaveURL(/\/dashboard/);
    await expect(page.getByRole("heading", { name: L.titles.dashboard })).toBeVisible();
  });

  test("sesi bertahan setelah reload lalu logout membersihkan sesi", async ({ page }) => {
    await uiLogin(page, "kasir", PASSWORD);
    await expect(page).toHaveURL(/\/dashboard/);
    await page.reload();
    await expect(page).toHaveURL(/\/dashboard/);
    await page.getByRole("button", { name: L.account.menu }).click();
    await page.getByRole("button", { name: L.account.logout }).click();
    await expect(page).toHaveURL(/\/login/);
    await page.goto("/dashboard");
    await expect(page).toHaveURL(/\/login/);
  });

  test("kasir tidak melihat menu Keuangan; akuntan melihatnya", async ({ page }) => {
    await uiLogin(page, "kasir", PASSWORD);
    await expect(page).toHaveURL(/\/dashboard/);
    const nav = page.getByRole("navigation", { name: "Menu utama" });
    await expect(nav.getByText(L.menu.pos, { exact: true })).toBeVisible();
    await expect(nav.getByText(L.menu.journals, { exact: true })).toHaveCount(0);
    await page.goto("/finance/journals");
    await expect(page).toHaveURL(/\/403/);
    await expect(page.getByText(L.titles.forbidden).first()).toBeVisible();
  });

  test("akuntan melihat menu Keuangan", async ({ page }) => {
    await uiLogin(page, "akuntan", PASSWORD);
    await expect(page).toHaveURL(/\/dashboard/);
    const nav = page.getByRole("navigation", { name: "Menu utama" });
    // Jurnal ada di seksi "Detail" (collapsed secara default, seperti aplikasi lama).
    await nav.getByRole("button", { name: L.menu.detail }).click();
    await expect(nav.getByText(L.menu.journals, { exact: true })).toBeVisible();
  });

  test("API menolak tanpa izin: kasir 403 pada jurnal, tanpa token 401", async () => {
    const r = await call("kasir", "GET", "/finance/journals");
    expect(r.status).toBe(403);
    expect(r.success).toBe(false);
    const res = await fetch(`${process.env.E2E_BASE ?? "http://127.0.0.1:18080"}/api/v1/finance/journals`);
    expect(res.status).toBe(401);
  });

  test("matriks hak akses membaca izin nyata role (F-02): API dan UI", async ({ page }) => {
    const roles = (await call("admin", "GET", "/roles")).data as { id: number; code: string }[];
    const kasirRole = roles.find((r) => r.code === "kasir")!;
    const detail = (await call("admin", "GET", `/roles/${kasirRole.id}`)).data;
    const mine = ((await call("kasir", "GET", "/auth/me")).data.permissions as string[]).sort();
    expect([...detail.permissions].sort()).toEqual(mine);
    expect(detail.locked).toBe(false);

    const owner = roles.find((r) => r.code === "owner");
    if (owner) expect((await call("admin", "GET", `/roles/${owner.id}`)).data.locked).toBe(true);
    // kasir tidak punya roles.view
    expect((await call("kasir", "GET", `/roles/${kasirRole.id}`)).status).toBe(403);

    await loginAs(page, "admin");
    await page.goto("/roles");
    const row = page.locator("tr", { hasText: /kasir/i }).first();
    await row.getByRole("button", { name: /Atur izin/ }).click();
    await expect(page.getByText(/belum menyediakan endpoint/i)).toHaveCount(0);
    // izin kasir tercentang dari data server, bukan kosong
    await expect(page.getByLabel(/sales\.create/).first()).toBeChecked();
  });

  test("tabel hak akses: izin per peran sesuai /auth/me", async () => {
    const perms = async (role: "kasir" | "gudang" | "akuntan" | "admin") =>
      ((await call(role, "GET", "/auth/me")).data.permissions as string[]).sort();
    const kasir = await perms("kasir");
    const gudang = await perms("gudang");
    const akuntan = await perms("akuntan");
    const admin = await perms("admin");
    expect(kasir).toContain("sales.create");
    expect(kasir).not.toContain("finance.view");
    expect(gudang).toContain("stock.transfer");
    expect(gudang).not.toContain("finance.view");
    expect(akuntan).toContain("finance.posting.post");
    expect(akuntan).not.toContain("stock.adjust");
    expect(admin.length).toBeGreaterThan(akuntan.length);
  });
});
