import { describe, expect, it, vi } from "vitest";
import fs from "node:fs";
import path from "node:path";

// public/sw.js menggantikan service worker legacy yang masih terdaftar di perangkat (lihat komentar
// berkasnya). Tes ini menjalankan skripnya di atas `self` tiruan; tes e2e E18 membuktikannya di Chromium.
const webRoot = path.resolve(__dirname, "../..");
const source = fs.readFileSync(path.join(webRoot, "public/sw.js"), "utf8");

type Listener = (event: { waitUntil: (p: Promise<unknown>) => void }) => void;

function loadWorker() {
  const listeners = new Map<string, Listener>();
  const store = new Set(["mini-erp-pwa-v1-app", "mini-erp-pwa-v1-runtime", "lain"]);
  const navigated: string[] = [];
  const self = {
    addEventListener: (type: string, fn: Listener) => listeners.set(type, fn),
    skipWaiting: vi.fn(async () => undefined),
    registration: { unregister: vi.fn(async () => true) },
    clients: {
      matchAll: vi.fn(async () => [
        { url: "https://minierp.my.id/pos", navigate: async (u: string) => navigated.push(u) },
        { url: "https://minierp.my.id/login", navigate: async () => Promise.reject(new TypeError("not controlled")) },
      ]),
    },
  };
  const caches = {
    keys: async () => [...store],
    delete: async (k: string) => store.delete(k),
  };
  new Function("self", "caches", source)(self, caches);
  return { listeners, self, store, navigated };
}

async function fire(listener: Listener | undefined) {
  const pending: Promise<unknown>[] = [];
  listener?.({ waitUntil: (p) => pending.push(p) });
  await Promise.all(pending);
}

describe("public/sw.js (pembersih service worker legacy)", () => {
  it("langsung aktif, tanpa handler fetch", async () => {
    const { listeners, self } = loadWorker();
    expect([...listeners.keys()].sort()).toEqual(["activate", "install"]);
    await fire(listeners.get("install"));
    expect(self.skipWaiting).toHaveBeenCalledOnce();
  });

  it("aktivasi menghapus semua cache, mencabut registrasi, lalu memuat ulang tab", async () => {
    const { listeners, self, store, navigated } = loadWorker();
    await fire(listeners.get("activate"));
    expect(store.size).toBe(0);
    expect(self.registration.unregister).toHaveBeenCalledOnce();
    expect(self.clients.matchAll).toHaveBeenCalledWith({ type: "window" });
    expect(navigated).toEqual(["https://minierp.my.id/pos"]);
  });

  it("v2 sendiri tidak mendaftarkan service worker", () => {
    const offenders: string[] = [];
    const walk = (dir: string) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) walk(p);
        else if (/\.(ts|tsx)$/.test(e.name) && !e.name.endsWith(".test.ts") && /serviceWorker\s*\.\s*register\(/.test(fs.readFileSync(p, "utf8"))) {
          offenders.push(path.relative(webRoot, p));
        }
      }
    };
    walk(path.join(webRoot, "src"));
    expect(offenders).toEqual([]);
  });
});
