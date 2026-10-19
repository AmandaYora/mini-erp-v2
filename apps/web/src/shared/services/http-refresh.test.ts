import { afterEach, describe, expect, it, vi } from "vitest";
import {
  RefreshUnreachableError,
  resetRefreshForTest,
  rotateForTest,
  tokenStore,
} from "@/shared/services/http-client";

vi.mock("axios", async (importOriginal) => {
  const actual = await importOriginal<typeof import("axios")>();
  return { ...actual, default: { ...actual.default, post: vi.fn() } };
});

// Tanpa static import (arch-check aturan D): ambil mock lewat dynamic import.
async function mockedPost() {
  const axios = await import("axios");
  return axios.default.post as unknown as ReturnType<typeof vi.fn>;
}

function refreshOk(access: string, refresh: string) {
  return { data: { data: { accessToken: access, refreshToken: refresh } } };
}

function refreshRaced() {
  return {
    isAxiosError: true,
    response: { status: 401, data: { success: false, code: "refresh_raced" } },
  };
}

// Antrean kunci serial seperti Web Locks sungguhan: pemegang kedua baru
// jalan setelah pertama selesai.
function mockWebLocks() {
  let tail: Promise<unknown> = Promise.resolve();
  const request = vi.fn(<R>(_: string, fn: () => Promise<R>): Promise<R> => {
    const run = tail.then(fn, fn);
    tail = run.catch(() => undefined);
    return run as Promise<R>;
  });
  Object.defineProperty(navigator, "locks", { value: { request }, configurable: true });
  return request;
}

afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
  resetRefreshForTest();
  // @ts-expect-error membersihkan mock antar test
  delete navigator.locks;
});

describe("rotasi refresh silang-tab (F1-10)", () => {
  it("satu tab, dua request bersamaan: satu /refresh (single-flight)", async () => {
    mockWebLocks();
    tokenStore.set("A1", "R1");
    (await mockedPost()).mockImplementationOnce(async () => refreshOk("A2", "R2"));

    const [a, b] = await Promise.all([rotateForTest("R1"), rotateForTest("R1")]);

    expect(a).toBe("A2");
    expect(b).toBe("A2");
    expect(await mockedPost()).toHaveBeenCalledTimes(1);
  });

  it("dua tab berurutan: tab basi memakai pasangan baru tanpa /refresh ulang", async () => {
    const request = mockWebLocks();
    tokenStore.set("A1", "R1");
    (await mockedPost()).mockImplementationOnce(async () => refreshOk("A2", "R2"));

    // Tab 1 menang: memutar dan menulis A2/R2.
    const tab1 = await rotateForTest("R1");
    // Tab 2 = single-flight terpisah yang masih memegang R1 basi.
    resetRefreshForTest();
    const tab2 = await rotateForTest("R1");

    expect(tab1).toBe("A2");
    expect(tab2).toBe("A2");
    expect(await mockedPost()).toHaveBeenCalledTimes(1);
    expect(request).toHaveBeenCalledTimes(2);
    expect(tokenStore.access).toBe("A2");
    expect(tokenStore.refresh).toBe("R2");
  });

  it("refresh_raced dari server: pakai pasangan terbaru dari storage", async () => {
    tokenStore.set("A1", "R1");
    // Tab lain menulis pasangan baru di antara kegagalan dan giliran kunci.
    (await mockedPost()).mockImplementationOnce(async () => {
      tokenStore.set("A2", "R2");
      throw refreshRaced();
    });

    // Jalur fallback (tanpa Web Locks): raced tetap diambil dari storage.
    const got = await rotateForTest("R1");
    expect(got).toBe("A2");
  });

  it("tanpa Web Locks tetap memutar normal", async () => {
    tokenStore.set("A1", "R1");
    (await mockedPost()).mockResolvedValueOnce(refreshOk("A2", "R2"));
    const got = await rotateForTest("R1");
    expect(got).toBe("A2");
    expect(await mockedPost()).toHaveBeenCalledTimes(1);
  });

  it("tanpa refresh token: null tanpa memanggil server", async () => {
    const got = await rotateForTest(null);
    expect(got).toBeNull();
    expect(await mockedPost()).not.toHaveBeenCalled();
  });

  it("refresh ditolak server (401) = sesi habis: null", async () => {
    tokenStore.set("A1", "R1");
    (await mockedPost()).mockRejectedValueOnce({
      isAxiosError: true,
      response: { status: 401, data: { success: false } },
    });
    expect(await rotateForTest("R1")).toBeNull();
  });

  it("jaringan putus / 5xx bukan sesi habis: galat tak terjangkau, token tetap", async () => {
    tokenStore.set("A1", "R1");
    const post = await mockedPost();
    post.mockRejectedValueOnce({ isAxiosError: true, code: "ERR_NETWORK" });
    await expect(rotateForTest("R1")).rejects.toBeInstanceOf(RefreshUnreachableError);
    resetRefreshForTest();
    post.mockRejectedValueOnce({ isAxiosError: true, response: { status: 503, data: {} } });
    await expect(rotateForTest("R1")).rejects.toBeInstanceOf(RefreshUnreachableError);
    expect(tokenStore.access).toBe("A1");
    expect(tokenStore.refresh).toBe("R1");
  });
});
