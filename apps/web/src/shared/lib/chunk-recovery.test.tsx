import { Suspense } from "react";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { ErrorBoundary } from "@/shared/components/feedback/error-boundary";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CHUNK_RELOAD_COOLDOWN_MS,
  CHUNK_RELOAD_KEY,
  installPreloadErrorHandler,
  isChunkLoadError,
  lazyWithRetry,
  reloadOnceForChunkError,
} from "@/shared/lib/chunk-recovery";

function memStorage(initial: Record<string, string> = {}) {
  const m = new Map(Object.entries(initial));
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => void m.set(k, v),
    raw: m,
  };
}

afterEach(() => cleanup());

describe("isChunkLoadError", () => {
  it("mengenali pesan chunk dari Chrome, Firefox, Safari, dan CSS preload", () => {
    expect(isChunkLoadError(new TypeError("Failed to fetch dynamically imported module: /a.js"))).toBe(true);
    expect(isChunkLoadError(new Error("error loading dynamically imported module"))).toBe(true);
    expect(isChunkLoadError(new Error("Importing a module script failed."))).toBe(true);
    expect(isChunkLoadError(new Error("Unable to preload CSS for /assets/x.css"))).toBe(true);
    expect(isChunkLoadError({ reason: "Failed to fetch dynamically imported module" })).toBe(true);
  });
  it("galat biasa bukan chunk", () => {
    expect(isChunkLoadError(new Error("Cannot read properties of undefined"))).toBe(false);
    expect(isChunkLoadError(null)).toBe(false);
  });
});

describe("reloadOnceForChunkError", () => {
  it("memuat ulang sekali, mencatat cap waktu, lalu menolak dalam cooldown (tanpa loop)", () => {
    const storage = memStorage();
    const reload = vi.fn();
    expect(reloadOnceForChunkError({ storage, reload, now: 1_000_000 })).toBe(true);
    expect(reload).toHaveBeenCalledTimes(1);
    expect(storage.raw.get(CHUNK_RELOAD_KEY)).toBe("1000000");
    // gagal lagi tepat setelah reload → ditolak
    expect(reloadOnceForChunkError({ storage, reload, now: 1_005_000 })).toBe(false);
    expect(reload).toHaveBeenCalledTimes(1);
  });
  it("setelah cooldown lewat (deploy berikutnya) boleh memuat ulang lagi", () => {
    const storage = memStorage({ [CHUNK_RELOAD_KEY]: "1000000" });
    const reload = vi.fn();
    expect(reloadOnceForChunkError({ storage, reload, now: 1_000_000 + CHUNK_RELOAD_COOLDOWN_MS + 1 })).toBe(true);
    expect(reload).toHaveBeenCalledTimes(1);
  });
  it("tanpa sessionStorage tidak memuat ulang (hindari loop)", () => {
    const reload = vi.fn();
    expect(reloadOnceForChunkError({ storage: null, reload })).toBe(false);
    expect(reload).not.toHaveBeenCalled();
  });
});

describe("lazyWithRetry", () => {
  const chunkErr = () => new TypeError("Failed to fetch dynamically imported module: /assets/Page-abc.js");

  it("impor sukses merender halaman", async () => {
    const Page = lazyWithRetry(async () => ({ default: () => <p>halaman ok</p> }), { storage: memStorage(), reload: vi.fn() });
    render(<Suspense fallback="muat">{<Page />}</Suspense>);
    await screen.findByText("halaman ok");
  });

  it("chunk gagal → reload sekali dan tidak melempar (promise menggantung)", async () => {
    const reload = vi.fn();
    const Page = lazyWithRetry(() => Promise.reject(chunkErr()), { storage: memStorage(), reload });
    render(<Suspense fallback="muat">{<Page />}</Suspense>);
    await waitFor(() => expect(reload).toHaveBeenCalledTimes(1));
    expect(screen.getByText("muat")).toBeTruthy();
  });

  it("chunk gagal lagi setelah reload → galat dilempar ke error boundary", async () => {
    const reload = vi.fn();
    const storage = memStorage({ [CHUNK_RELOAD_KEY]: String(Date.now()) });
    const Page = lazyWithRetry(() => Promise.reject(chunkErr()), { storage, reload });
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <ErrorBoundary>
        <Suspense fallback="muat">
          <Page />
        </Suspense>
      </ErrorBoundary>,
    );
    await screen.findByRole("alert");
    expect(reload).not.toHaveBeenCalled();
    err.mockRestore();
  });

  it("galat non-chunk dilempar tanpa reload", async () => {
    const reload = vi.fn();
    const Page = lazyWithRetry(() => Promise.reject(new Error("modul rusak")), { storage: memStorage(), reload });
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <ErrorBoundary>
        <Suspense fallback="muat">
          <Page />
        </Suspense>
      </ErrorBoundary>,
    );
    await screen.findByRole("alert");
    expect(reload).not.toHaveBeenCalled();
    err.mockRestore();
  });
});

describe("installPreloadErrorHandler", () => {
  it("vite:preloadError → preventDefault + reload sekali; ulangan dalam cooldown dibiarkan", () => {
    const reload = vi.fn();
    const storage = memStorage();
    const off = installPreloadErrorHandler(window, { storage, reload });
    const first = new Event("vite:preloadError", { cancelable: true });
    window.dispatchEvent(first);
    expect(first.defaultPrevented).toBe(true);
    expect(reload).toHaveBeenCalledTimes(1);
    const second = new Event("vite:preloadError", { cancelable: true });
    window.dispatchEvent(second);
    expect(second.defaultPrevented).toBe(false);
    expect(reload).toHaveBeenCalledTimes(1);
    off();
  });
});
