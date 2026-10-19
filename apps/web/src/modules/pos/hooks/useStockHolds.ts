import { useCallback, useEffect, useRef, useState } from "react";
import { readApiError } from "@/modules/pos/lib/api-error";
import type { HeldSnapshot } from "@/modules/pos/lib/cart-draft";
import {
  holdKey,
  lineKey,
  neededDelta,
  planHolds,
  type HeldMap,
} from "@/modules/pos/lib/holds";
import { posService } from "@/modules/pos/services/pos.service";

// Tahanan stok keranjang POS (F2a-12): qty berubah → debounce 400 ms →
// suggest (hanya untuk selisih) → hold per lokasi. Hapus baris / kosongkan →
// release. beforeunload → release best-effort; sisanya mengandalkan TTL.
// Tahanan diperbarui tiap 5 menit (TTL 20 menit). Semua operasi diserialkan
// dalam satu antrean supaya tak saling balap.

const DEBOUNCE_MS = 400;
const REFRESH_MS = 5 * 60 * 1000;
const TTL_MINUTES = 20;

export interface HoldLine {
  productId: number;
  variantId: number;
  qty: number;
}

export interface LineStockState {
  /** Qty yang tak tertutup stok tersedia. */
  shortfall: number;
  /** Pesan server bila suggest/hold gagal (mis. stok direbut kasir lain). */
  error: string | null;
}

export interface StockHoldsApi {
  /** Status per baris, kunci `${productId}:${variantId}`. */
  lineState: Record<string, LineStockState>;
  /** true selama sinkronisasi berjalan / terjadwal. */
  syncing: boolean;
  /** Tuntaskan sinkronisasi tertunda (tanpa menunggu debounce) dan kembalikan
   * status terbaru per baris — dibaca langsung, bukan dari state React. */
  flush: () => Promise<Record<string, LineStockState>>;
  /** Tahanan saat ini untuk baris (untuk reservationKey SJ). */
  heldFor: (productId: number, variantId: number) => HeldMap;
  /** Lepas semua tahanan di server (batal/kosongkan keranjang). */
  releaseAll: () => Promise<void>;
  /** Lupakan tahanan tanpa memanggil server (sudah dikonsumsi SJ). */
  forget: () => void;
}

/** sync = keranjang bebas diubah (sinkron + rilis saat pergi); hold =
 * checkout berjalan (hanya perpanjang TTL, tahanan dipertahankan untuk
 * resume); off = selesai/dibatalkan. */
export type HoldMode = "sync" | "hold" | "off";

export function useStockHolds(opts: {
  cartRef: string;
  lines: HoldLine[];
  mode: HoldMode;
  /** Tahanan yang diketahui dari draf tersimpan (dipakai sekali saat mount). */
  initialHeld?: HeldSnapshot;
  /** Dipanggil tiap peta tahanan berubah (untuk disimpan di draf). */
  onHeldChange?: (snapshot: HeldSnapshot) => void;
}): StockHoldsApi {
  const { cartRef, lines, mode, initialHeld, onHeldChange } = opts;
  const enabled = mode === "sync";
  const keepAlive = mode !== "off";
  const onHeldChangeRef = useRef(onHeldChange);
  const [lineState, setLineState] = useState<Record<string, LineStockState>>({});
  const [syncing, setSyncing] = useState(false);

  const [heldMap] = useState(
    () => new Map<string, { productId: number; variantId: number; held: HeldMap }>(
      Object.entries(initialHeld ?? {}),
    ),
  );
  const heldRef = useRef(heldMap);
  const linesRef = useRef<HoldLine[]>([]);
  const refRef = useRef(cartRef);
  const enabledRef = useRef(enabled);
  const chainRef = useRef<Promise<void>>(Promise.resolve());
  const timerRef = useRef<number | null>(null);
  const noHoldRef = useRef(false);
  const aliveRef = useRef(true);
  const latestRef = useRef<Record<string, LineStockState>>({});

  const linesKey = JSON.stringify(lines.map((l) => [l.productId, l.variantId, l.qty]));

  useEffect(() => {
    linesRef.current = lines;
    refRef.current = cartRef;
    enabledRef.current = enabled;
    onHeldChangeRef.current = onHeldChange;
  });

  const publishHeld = useCallback(() => {
    onHeldChangeRef.current?.(Object.fromEntries(heldRef.current.entries()));
  }, []);

  const releaseKey = useCallback(async (key: string) => {
    try {
      await posService.releaseStock(key);
    } catch {
      /* tahanan kedaluwarsa / sudah dilepas — TTL membereskan sisanya */
    }
  }, []);

  const releaseEntry = useCallback(
    async (entry: { productId: number; variantId: number; held: HeldMap }) => {
      for (const loc of Object.keys(entry.held).map(Number)) {
        await releaseKey(holdKey(refRef.current, entry.productId, entry.variantId, loc));
      }
    },
    [releaseKey],
  );

  const runSync = useCallback(async () => {
    if (!enabledRef.current) return;
    const desired = linesRef.current.filter((l) => l.qty > 0);
    const next: Record<string, LineStockState> = {};

    // Baris yang hilang dari keranjang → lepas.
    const wanted = new Set(desired.map((l) => lineKey(l.productId, l.variantId)));
    for (const [lk, entry] of [...heldRef.current.entries()]) {
      if (!wanted.has(lk)) {
        await releaseEntry(entry);
        heldRef.current.delete(lk);
      }
    }

    for (const line of desired) {
      const lk = lineKey(line.productId, line.variantId);
      const entry = heldRef.current.get(lk) ?? {
        productId: line.productId,
        variantId: line.variantId,
        held: {} as HeldMap,
      };
      let error: string | null = null;
      let suggestion = null;
      const delta = neededDelta(entry.held, line.qty);
      if (delta > 1e-9) {
        try {
          suggestion = await posService.suggestAllocation({
            productId: line.productId,
            variantId: line.variantId,
            qty: delta,
          });
        } catch (err) {
          const e = readApiError(err);
          // Tanpa izin stock.view: tak bisa memeriksa — server tetap otoritas.
          if (e.status !== 403) error = e.message;
          suggestion = { lines: [], shortfall: 0 };
        }
      }
      const plan = planHolds(entry.held, line.qty, suggestion);

      if (!noHoldRef.current) {
        const held: HeldMap = { ...entry.held };
        for (const op of plan.holds) {
          try {
            await posService.holdStock({
              productId: line.productId,
              variantId: line.variantId,
              locationId: op.locationId,
              qty: op.qty,
              key: holdKey(refRef.current, line.productId, line.variantId, op.locationId),
              ttlMinutes: TTL_MINUTES,
            });
            held[op.locationId] = op.qty;
          } catch (err) {
            const e = readApiError(err);
            if (e.status === 403) {
              noHoldRef.current = true; // kasir tanpa stock.reserve: lanjut tanpa tahanan
            } else {
              error = e.message;
            }
            break;
          }
        }
        if (!noHoldRef.current) {
          for (const loc of plan.releases) {
            await releaseKey(holdKey(refRef.current, line.productId, line.variantId, loc));
            delete held[loc];
          }
          entry.held = held;
          if (Object.keys(held).length > 0) heldRef.current.set(lk, entry);
          else heldRef.current.delete(lk);
        }
      }
      next[lk] = { shortfall: plan.shortfall, error };
    }
    latestRef.current = next;
    publishHeld();
    if (aliveRef.current) setLineState(next);
  }, [publishHeld, releaseEntry, releaseKey]);

  const enqueue = useCallback(
    (task: () => Promise<void>): Promise<void> => {
      if (aliveRef.current) setSyncing(true);
      const run = chainRef.current.then(task).catch(() => undefined);
      chainRef.current = run;
      void run.then(() => {
        if (chainRef.current === run && aliveRef.current) setSyncing(false);
      });
      return run;
    },
    [],
  );

  // Sinkron saat isi keranjang berubah (debounce 400 ms).
  useEffect(() => {
    if (!enabled) return;
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      void enqueue(runSync);
    }, DEBOUNCE_MS);
    return () => {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [linesKey, cartRef, enabled, enqueue, runSync]);

  // Perpanjang TTL tiap 5 menit (hold = upsert; qty sama).
  useEffect(() => {
    if (!keepAlive) return;
    const id = window.setInterval(() => {
      void enqueue(async () => {
        for (const [lk, entry] of [...heldRef.current.entries()]) {
          for (const [loc, qty] of Object.entries(entry.held)) {
            try {
              await posService.holdStock({
                productId: entry.productId,
                variantId: entry.variantId,
                locationId: Number(loc),
                qty,
                key: holdKey(refRef.current, entry.productId, entry.variantId, Number(loc)),
                ttlMinutes: TTL_MINUTES,
              });
            } catch {
              // Kedaluwarsa & direbut — buang lalu hitung ulang dari nol.
              heldRef.current.delete(lk);
              break;
            }
          }
        }
        await runSync(); // no-op saat mode hold (enabledRef false)
        publishHeld();
      });
    }, REFRESH_MS);
    return () => window.clearInterval(id);
  }, [keepAlive, enqueue, publishHeld, runSync]);

  // Tutup tab: lepas best-effort (request tak dijamin selesai; TTL cadangan).
  useEffect(() => {
    if (!enabled) return;
    const onUnload = () => {
      for (const entry of heldRef.current.values()) {
        for (const loc of Object.keys(entry.held).map(Number)) {
          void releaseKey(holdKey(refRef.current, entry.productId, entry.variantId, loc));
        }
      }
    };
    window.addEventListener("beforeunload", onUnload);
    return () => window.removeEventListener("beforeunload", onUnload);
  }, [enabled, releaseKey]);

  // Pindah halaman / unmount: lepas supaya stok tak tertahan sia-sia. Draf
  // keranjang tetap tersimpan dan menahan ulang saat halaman dibuka lagi.
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
      if (!enabledRef.current) return;
      const entries = [...heldMap.values()];
      heldMap.clear();
      chainRef.current = chainRef.current.then(async () => {
        for (const entry of entries) await releaseEntry(entry);
      });
    };
  }, [heldMap, releaseEntry]);

  const flush = useCallback(async () => {
    if (!enabledRef.current) return latestRef.current;
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    await enqueue(runSync);
    return latestRef.current;
  }, [enqueue, runSync]);

  const heldFor = useCallback(
    (productId: number, variantId: number): HeldMap =>
      heldRef.current.get(lineKey(productId, variantId))?.held ?? {},
    [],
  );

  const releaseAll = useCallback(async () => {
    const entries = [...heldRef.current.values()];
    heldRef.current.clear();
    await enqueue(async () => {
      for (const entry of entries) await releaseEntry(entry);
    });
    publishHeld();
    latestRef.current = {};
    if (aliveRef.current) setLineState({});
  }, [enqueue, publishHeld, releaseEntry]);

  const forget = useCallback(() => {
    heldRef.current.clear();
    latestRef.current = {};
    publishHeld();
    setLineState({});
  }, [publishHeld]);

  return { lineState, syncing, flush, heldFor, releaseAll, forget };
}
