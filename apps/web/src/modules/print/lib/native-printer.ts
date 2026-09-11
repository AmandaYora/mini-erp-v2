// Jembatan printer native (Android WebView shell) — Tahap E6.
//
// Port dari apps/web/src/lib/native-printer.ts sistem lama. POS web tetap
// berjalan di browser biasa; di sana `window.AndroidPrinter` TIDAK ada, jadi
// seluruh fungsi aman (feature-detected) dan pemanggil wajib fallback ke
// `window.print()`.
//
// Kontrak payload WAJIB sinkron dengan PrinterBridge.kt di android-pos-shell:
// printReceipt(payloadJson) -> window.onPrintResult({ok, code?, message?}).

export interface ThermalPrintPayload {
  store: { name?: string; address?: string; phone?: string; footer?: string };
  meta: {
    orderNumber?: string;
    paymentNumber?: string;
    datetime?: string;
    cashier?: string;
    customer?: string;
    status?: string;
  };
  items: Array<{
    name: string;
    qty: number;
    uom?: string;
    unitPrice: number;
    lineTotal: number;
    discount?: number;
    note?: string;
  }>;
  summary: {
    subtotal?: number;
    discount?: number;
    tax?: number;
    total: number;
    paid?: number;
    tendered?: number;
    change?: number;
    outstanding?: number;
    method?: string;
  };
}

export interface PrintResult {
  ok: boolean;
  code?: string;
  message?: string;
}

export interface ScannedPrinter {
  ip: string;
  port: number;
}

export interface NativePrinterConfig {
  ip: string;
  port: number;
  mac: string;
  autoCut: boolean;
  cashDrawer: boolean;
}

interface AndroidPrinterBridge {
  isAvailable?: () => boolean;
  getConfig?: () => string;
  saveConfig?: (json: string) => void;
  printReceipt?: (payloadJson: string) => void;
  testPrint?: () => void;
  openDrawer?: () => void;
  scan?: () => void;
}

declare global {
  interface Window {
    AndroidPrinter?: AndroidPrinterBridge;
    onPrintResult?: (result: PrintResult) => void;
    onPrinterScanResult?: (list: ScannedPrinter[]) => void;
  }
}

const DEFAULT_TIMEOUT_MS = 20_000;

function bridge(): AndroidPrinterBridge | undefined {
  return typeof window !== "undefined" ? window.AndroidPrinter : undefined;
}

/** True hanya bila halaman berjalan di dalam APK cangkang penyuntik jembatan. */
export function isNativePrinterAvailable(): boolean {
  try {
    return bridge()?.isAvailable?.() === true;
  } catch {
    return false;
  }
}

function callWithResult(
  invoke: (b: AndroidPrinterBridge) => void,
  timeoutMs = DEFAULT_TIMEOUT_MS,
): Promise<PrintResult> {
  const b = bridge();
  if (!b) {
    return Promise.resolve({
      ok: false,
      code: "NO_BRIDGE",
      message: "Jembatan printer tidak tersedia",
    });
  }
  return new Promise((resolve) => {
    let settled = false;
    const finish = (result: PrintResult) => {
      if (settled) return;
      settled = true;
      window.onPrintResult = undefined;
      clearTimeout(timer);
      resolve(result);
    };
    const timer = setTimeout(
      () => finish({ ok: false, code: "TIMEOUT", message: "Printer tidak merespons" }),
      timeoutMs,
    );
    window.onPrintResult = (result) =>
      finish(result ?? { ok: false, code: "UNKNOWN" });
    try {
      invoke(b);
    } catch (error) {
      finish({
        ok: false,
        code: "UNKNOWN",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  });
}

export function printReceipt(payload: ThermalPrintPayload): Promise<PrintResult> {
  return callWithResult((b) => b.printReceipt?.(JSON.stringify(payload)));
}

export function testPrint(): Promise<PrintResult> {
  return callWithResult((b) => b.testPrint?.());
}

export function openDrawer(): Promise<PrintResult> {
  return callWithResult((b) => b.openDrawer?.());
}

export function scanPrinters(
  timeoutMs = DEFAULT_TIMEOUT_MS,
): Promise<ScannedPrinter[]> {
  const b = bridge();
  if (!b?.scan) return Promise.resolve([]);
  const runScan = b.scan.bind(b);
  return new Promise((resolve) => {
    let settled = false;
    const finish = (list: ScannedPrinter[]) => {
      if (settled) return;
      settled = true;
      window.onPrinterScanResult = undefined;
      clearTimeout(timer);
      resolve(list);
    };
    const timer = setTimeout(() => finish([]), timeoutMs);
    window.onPrinterScanResult = (list) =>
      finish(Array.isArray(list) ? list : []);
    try {
      runScan();
    } catch {
      finish([]);
    }
  });
}

export function getPrinterConfig(): NativePrinterConfig | null {
  const raw = bridge()?.getConfig?.();
  if (!raw) return null;
  try {
    return JSON.parse(raw) as NativePrinterConfig;
  } catch {
    return null;
  }
}

export function savePrinterConfig(config: Partial<NativePrinterConfig>): void {
  bridge()?.saveConfig?.(JSON.stringify(config));
}
