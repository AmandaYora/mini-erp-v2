import { test, expect } from "@playwright/test";
import { loginAs } from "../helpers/ui";

// OP-01 (pemulihan chunk-load + kartu galat) dan OP-02 (aset statis di public/).
// Server Go menyajikan SPA dengan fallback index.html untuk path yang tak ada, jadi "tidak 404"
// BELUM cukup: aset harus bertipe benar (bukan HTML) — itu yang diperiksa di bawah.
test.describe("E11 ketahanan operasional", () => {
  const assets: { path: string; type: RegExp }[] = [
    { path: "/favicon.svg", type: /image\/svg\+xml/ },
    { path: "/logo.svg", type: /image\/svg\+xml/ },
    { path: "/favicon-32.png", type: /image\/png/ },
    { path: "/apple-touch-icon.png", type: /image\/png/ },
    { path: "/icon-192.png", type: /image\/png/ },
    { path: "/icon-512.png", type: /image\/png/ },
    { path: "/robots.txt", type: /text\/plain/ },
  ];

  for (const a of assets) {
    test(`aset ${a.path} tersaji dengan tipe benar`, async ({ request }) => {
      const res = await request.get(a.path);
      expect(res.status()).toBe(200);
      expect(res.headers()["content-type"]).toMatch(a.type);
      expect((await res.body()).length).toBeGreaterThan(50);
    });
  }

  test("robots.txt melarang semua perayap", async ({ request }) => {
    const body = await (await request.get("/robots.txt")).text();
    expect(body).toMatch(/User-agent: \*/);
    expect(body).toMatch(/Disallow: \//);
  });

  test("index.html memasang ikon + theme-color dan semua tautan ikon tidak 404", async ({ page, request }) => {
    await page.goto("/login");
    await expect(page.locator('meta[name="theme-color"]')).toHaveAttribute("content", /^#[0-9a-f]{6}$/i);
    const hrefs = await page.locator('link[rel="icon"], link[rel="apple-touch-icon"]').evaluateAll((els) =>
      els.map((e) => (e as HTMLLinkElement).getAttribute("href") ?? ""),
    );
    expect(hrefs.length).toBeGreaterThanOrEqual(2);
    for (const h of hrefs) {
      const res = await request.get(h);
      expect(res.status(), h).toBe(200);
      expect(res.headers()["content-type"], h).not.toMatch(/text\/html/);
    }
  });

  test("chunk halaman gagal dimuat → reload sekali (tanpa loop) lalu kartu galat; muat ulang memulihkan", async ({ page }) => {
    await loginAs(page, "admin");
    let chunkRequests = 0;
    let blocked = true;
    await page.route(/\/assets\/CategoriesPage-[^/]*\.js(\?.*)?$/, async (route) => {
      chunkRequests++;
      if (blocked) await route.abort();
      else await route.continue();
    });
    await page.goto("/product-categories");

    // Kartu galat muncul (bukan layar putih) dengan id kesalahan dan tombol muat ulang.
    const card = page.getByRole("alert").filter({ hasText: "ID kesalahan" });
    await expect(card).toBeVisible({ timeout: 30_000 });
    await expect(card.getByText(/ERR-[A-Z0-9]+/)).toBeVisible();
    await expect(card.getByRole("button", { name: "Muat ulang halaman" })).toBeVisible();

    // Tidak loop: setelah menetap, jumlah permintaan chunk tetap (percobaan awal + satu setelah reload).
    await page.waitForTimeout(2_500);
    const settled = chunkRequests;
    expect(settled).toBeGreaterThanOrEqual(2);
    expect(settled).toBeLessThanOrEqual(4);
    await page.waitForTimeout(2_500);
    expect(chunkRequests).toBe(settled);

    // Jaringan pulih → tombol memuat ulang, halaman termuat normal (cooldown penjaga dihapus agar uji deterministik).
    blocked = false;
    await page.evaluate(() => sessionStorage.removeItem("mini-erp:chunk-reload-at"));
    await card.getByRole("button", { name: "Muat ulang halaman" }).click();
    await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible({ timeout: 30_000 });
    await expect(page.getByRole("alert").filter({ hasText: "ID kesalahan" })).toHaveCount(0);
  });
});
