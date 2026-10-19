import axios, { type InternalAxiosRequestConfig } from "axios";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { httpClient, isAbort } from "@/shared/services/http-client";

// RCA: React StrictMode (dev) menjalankan efek dua kali, sehingga request pertama dibatalkan lewat
// AbortSignal. Interceptor dulu mengubah galat pembatalan menjadi ApiError biasa
// ("Terjadi kesalahan jaringan"), isAbort() gagal mengenalinya, dan halaman yang menampilkan toast
// di blok catch (Order Beli, Penerimaan, Pengiriman, …) menampilkan toast palsu di dev.
describe("pembatalan request lewat interceptor", () => {
  const original = httpClient.defaults.adapter;
  beforeEach(() => {
    httpClient.defaults.adapter = ((config: InternalAxiosRequestConfig) =>
      new Promise((_, reject) => {
        config.signal?.addEventListener?.("abort", () => reject(new axios.CanceledError("canceled", config)));
      })) as typeof original;
  });
  afterEach(() => {
    httpClient.defaults.adapter = original;
  });

  it("request yang dibatalkan tetap dikenali isAbort (bukan galat jaringan)", async () => {
    const controller = new AbortController();
    const pending = httpClient.get("/api/v1/purchase-orders", { signal: controller.signal });
    controller.abort();
    const err = await pending.catch((e) => e);
    expect(isAbort(err)).toBe(true);
  });
});
