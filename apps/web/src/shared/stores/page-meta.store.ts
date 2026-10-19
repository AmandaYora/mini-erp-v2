import { useEffect } from "react";
import { create } from "zustand";

interface PageMetaState {
  /** Segmen terakhir breadcrumb halaman detail (mis. nomor dokumen). */
  crumb: string | null;
  setCrumb: (crumb: string | null) => void;
}

export const usePageMetaStore = create<PageMetaState>((set) => ({
  crumb: null,
  setCrumb: (crumb) => set({ crumb }),
}));

/** Halaman detail memanggil ini dengan nomor dokumen; dibersihkan saat halaman ditinggalkan. */
export function usePageCrumb(crumb: string | null | undefined): void {
  const setCrumb = usePageMetaStore((s) => s.setCrumb);
  useEffect(() => {
    setCrumb(crumb ?? null);
    return () => setCrumb(null);
  }, [crumb, setCrumb]);
}
