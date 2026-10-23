import { useSyncExternalStore } from "react";

/** Ambang tata letak Mode Kasir: ≥ 1024px = dua panel (tablet), di bawahnya = ringkas (HP, tablet tegak). */
export const POS_WIDE_QUERY = "(min-width: 1024px)";
/** HP mendatar: tinggi sempit, pencarian naik ke topbar agar katalog tetap terlihat. */
export const POS_LANDSCAPE_QUERY = "(orientation: landscape)";

function mql(query: string): MediaQueryList | null {
  return typeof window !== "undefined" && typeof window.matchMedia === "function" ? window.matchMedia(query) : null;
}

/**
 * Nilai media query, dibaca sinkron dari matchMedia sejak render pertama (tanpa kedip tata letak).
 * Tanpa matchMedia (mis. jsdom) memakai `fallback`.
 */
export function useMediaQuery(query: string, fallback: boolean): boolean {
  return useSyncExternalStore(
    (onChange) => {
      const m = mql(query);
      if (!m) return () => {};
      m.addEventListener("change", onChange);
      return () => m.removeEventListener("change", onChange);
    },
    () => mql(query)?.matches ?? fallback,
    () => fallback,
  );
}

/** true = tata letak tablet (dua panel). Tanpa matchMedia dianggap lebar: perilaku lama tidak berubah. */
export function useIsWide(): boolean {
  return useMediaQuery(POS_WIDE_QUERY, true);
}
