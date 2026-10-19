import { useEffect, useState } from "react";
import { clearDraft, loadDraft, saveDraft } from "@/shared/lib/draft-storage";

/**
 * Draf formulir otomatis, dua langkah agar state formulir bisa diinisialisasi
 * dari draf:
 *  1. `useDraftInitial(key)` — baca SEKALI saat mount (null = tak ada draf).
 *  2. `useDraftAutosave(key, snapshot, isEmpty)` — simpan snapshot dengan
 *     debounce; snapshot kosong menghapus draf.
 * Panggil `clearDraft(key)` setelah simpan sukses. key null = nonaktif
 * (mis. mode ubah, atau pengguna belum diketahui).
 */
export function useDraftInitial<T>(key: string | null): T | null {
  const [initial] = useState<T | null>(() => (key === null ? null : loadDraft<T>(key)));
  return initial;
}

export function useDraftAutosave<T>(
  key: string | null,
  snapshot: T,
  isEmpty: (v: T) => boolean,
  debounceMs = 500,
): void {
  useEffect(() => {
    if (key === null) return undefined;
    const t = window.setTimeout(() => {
      if (isEmpty(snapshot)) clearDraft(key);
      else saveDraft(key, snapshot);
    }, debounceMs);
    return () => window.clearTimeout(t);
    // isEmpty dianggap stabil (fungsi modul); snapshot memicu simpan ulang.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, snapshot, debounceMs]);
}
