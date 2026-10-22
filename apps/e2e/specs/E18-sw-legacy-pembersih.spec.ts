import { test, expect, type Page } from "@playwright/test";
import { L } from "../helpers/labels";

// Cutover domain: perangkat yang pernah membuka aplikasi legacy masih membawa service worker `/sw.js`
// (lingkup `/`, cache `mini-erp-pwa-v1-*`, navigasi network-first). v2 menyajikan `public/sw.js` pembersih:
// browser mengambilnya saat memeriksa pembaruan, lalu registrasi dan cache legacy hilang.
// Fase 1 meniru legacy dengan mencegat `/sw.js` (context.route juga mencegat skrip service worker);
// fase 2 melepas cegatan sehingga pemeriksaan pembaruan mendapat berkas v2 yang asli.

const LEGACY_SW = `
const APP_CACHE = "mini-erp-pwa-v1-app";
const RUNTIME_CACHE = "mini-erp-pwa-v1-runtime";
self.addEventListener("install", (event) => {
  event.waitUntil((async () => {
    await (await caches.open(APP_CACHE)).put("/offline.html", new Response("legacy offline", { headers: { "Content-Type": "text/html" } }));
    await (await caches.open(RUNTIME_CACHE)).put("/assets/legacy.js", new Response("legacy"));
    await self.skipWaiting();
  })());
});
self.addEventListener("activate", (event) => event.waitUntil(self.clients.claim()));
self.addEventListener("fetch", (event) => {
  if (event.request.mode !== "navigate") return;
  event.respondWith(fetch(event.request).catch(() => caches.match("/offline.html")));
});
`;

async function swState(page: Page) {
  return page.evaluate(async () => ({
    registrations: (await navigator.serviceWorker.getRegistrations()).length,
    caches: await caches.keys(),
    controlled: navigator.serviceWorker.controller !== null,
  }));
}

test.describe("E18 pembersih service worker legacy", () => {
  test("GET /sw.js: JavaScript, tidak disimpan cache", async ({ request }) => {
    const res = await request.get("/sw.js");
    expect(res.status()).toBe(200);
    expect(res.headers()["content-type"]).toMatch(/^text\/javascript/);
    expect(res.headers()["cache-control"]).toBe("no-store");
    const body = await res.text();
    expect(body).toContain("unregister");
    expect(body).not.toMatch(/addEventListener\(\s*["']fetch/);
  });

  test("registrasi + cache legacy hilang setelah browser mengambil /sw.js v2", async ({ page, context }) => {
    // Fase 1: perangkat dengan service worker legacy yang mengendalikan halaman.
    await context.route("**/sw.js", (route) =>
      route.fulfill({ status: 200, contentType: "text/javascript", body: LEGACY_SW }),
    );
    await page.goto("/login");
    await page.evaluate(async () => {
      await navigator.serviceWorker.register("/sw.js", { scope: "/" });
      await navigator.serviceWorker.ready;
    });
    await page.reload();
    await expect.poll(() => swState(page)).toEqual({
      registrations: 1,
      caches: expect.arrayContaining(["mini-erp-pwa-v1-app", "mini-erp-pwa-v1-runtime"]),
      controlled: true,
    });

    // Fase 2: server sudah v2. Navigasi memicu pemeriksaan pembaruan; berkas pembersih memuat ulang tab.
    await context.unroute("**/sw.js");
    await page.goto("/login");
    await expect
      .poll(() => swState(page).catch(() => null), { timeout: 30_000 })
      .toEqual({ registrations: 0, caches: [], controlled: false });

    // Halaman v2 tetap berjalan normal tanpa service worker.
    await page.reload();
    await expect(page.getByRole("button", { name: L.login.submit, exact: true })).toBeVisible();
    expect((await swState(page)).registrations).toBe(0);
  });
});
