import { useSyncExternalStore } from "react";
import { loadingBus } from "@/shared/services/loading-bus";

/**
 * True selama ada minimal satu request http-client yang belum selesai.
 *
 * useSyncExternalStore (bukan useState+useEffect) agar pembacaan bus luar
 * React masuk render cycle dengan benar — tanpa stale closure, tanpa tearing
 * di concurrent mode, dan tanpa peringatan set-state-in-effect.
 */
export function useGlobalLoading(): boolean {
  return useSyncExternalStore(
    loadingBus.subscribe,
    loadingBus.isLoading,
    () => false,
  );
}
