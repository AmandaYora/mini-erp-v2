import { useState } from "react";

// Tampilan kisi katalog Mode Kasir (analysis/003): "photo" = foto 4:3 selebar kartu (bawaan),
// "compact" = kartu ringkas berfoto kecil, lebih banyak barang per layar (pola majoo Grid/SKU,
// Square tile gambar/teks). Preferensi per perangkat — gagal baca/tulis storage = bawaan.

export type CatalogView = "photo" | "compact";

export const CATALOG_VIEW_KEY = "pos.catalogView";

export function readCatalogView(storage: Pick<Storage, "getItem"> | null = safeStorage()): CatalogView {
  try {
    return storage?.getItem(CATALOG_VIEW_KEY) === "compact" ? "compact" : "photo";
  } catch {
    return "photo";
  }
}

function safeStorage(): Storage | null {
  try {
    return typeof window !== "undefined" ? window.localStorage : null;
  } catch {
    return null;
  }
}

export function useCatalogView(): [CatalogView, (next: CatalogView) => void] {
  const [view, setView] = useState<CatalogView>(() => readCatalogView());
  const update = (next: CatalogView) => {
    setView(next);
    try {
      safeStorage()?.setItem(CATALOG_VIEW_KEY, next);
    } catch {
      /* mode privat / storage penuh: pilihan tetap berlaku sampai halaman ditutup */
    }
  };
  return [view, update];
}
