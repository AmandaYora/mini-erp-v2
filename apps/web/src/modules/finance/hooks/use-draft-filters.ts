import { useCallback, useState } from "react";

/**
 * Filter laporan dengan dua lapis: `draft` (yang sedang diisi) dan `applied` (yang dipakai memuat).
 * Isian tanggal tidak boleh memicu permintaan di setiap ketikan (tahun "0002" lolos sebagai tanggal sah),
 * jadi laporan baru dimuat ulang saat pengguna menekan "Tampilkan". `version` naik tiap kali
 * diterapkan — dipakai sebagai dependensi pemuat agar menekan "Tampilkan" pada filter yang sama tetap memuat ulang.
 */
export function useDraftFilters<T extends object>(initial: T) {
  const [draft, setDraft] = useState<T>(initial);
  const [applied, setApplied] = useState<T>(initial);
  const [version, setVersion] = useState(0);

  const patch = useCallback((p: Partial<T>) => setDraft((d) => ({ ...d, ...p })), []);
  const apply = useCallback(() => {
    setApplied(draft);
    setVersion((v) => v + 1);
  }, [draft]);

  return { draft, patch, applied, apply, version };
}
