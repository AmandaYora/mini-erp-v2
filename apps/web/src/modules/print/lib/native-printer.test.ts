// Port contract test native-printer legacy: feature-detect + fallback.
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  getPrinterConfig,
  isNativePrinterAvailable,
  printReceipt,
  scanPrinters,
} from "@/modules/print/lib/native-printer";

afterEach(() => {
  window.AndroidPrinter = undefined;
  window.onPrintResult = undefined;
  window.onPrinterScanResult = undefined;
  vi.unstubAllGlobals();
});

describe("tanpa jembatan Android (browser biasa)", () => {
  it("feature-detect false dan cetak mengembalikan NO_BRIDGE", async () => {
    expect(isNativePrinterAvailable()).toBe(false);
    const result = await printReceipt({
      store: {},
      meta: {},
      items: [],
      summary: { total: 10000 },
    });
    expect(result).toEqual({
      ok: false,
      code: "NO_BRIDGE",
      message: "Jembatan printer tidak tersedia",
    });
    expect(await scanPrinters(10)).toEqual([]);
    expect(getPrinterConfig()).toBeNull();
  });
});

describe("di dalam APK cangkang", () => {
  it("feature-detect true dan printReceipt meneruskan payload JSON", async () => {
    const seen: string[] = [];
    window.AndroidPrinter = {
      isAvailable: () => true,
      printReceipt: (json: string) => {
        seen.push(json);
        window.setTimeout(
          () => window.onPrintResult?.({ ok: true }),
          0,
        );
      },
    };
    expect(isNativePrinterAvailable()).toBe(true);
    const result = await printReceipt({
      store: { name: "TOKO X" },
      meta: { orderNumber: "SO-1" },
      items: [{ name: "Gula", qty: 2, unitPrice: 15000, lineTotal: 30000 }],
      summary: { total: 30000 },
    });
    expect(result).toEqual({ ok: true });
    expect(JSON.parse(seen[0]).summary.total).toBe(30000);
  });

  it("kegagalan native dikembalikan sebagai PrintResult, bukan lemparan", async () => {
    window.AndroidPrinter = {
      isAvailable: () => true,
      printReceipt: () => {
        window.setTimeout(
          () => window.onPrintResult?.({ ok: false, code: "OFFLINE", message: "mati" }),
          0,
        );
      },
    };
    const result = await printReceipt({
      store: {},
      meta: {},
      items: [],
      summary: { total: 1 },
    });
    expect(result.ok).toBe(false);
    expect(result.code).toBe("OFFLINE");
  });
});
