import { useCallback, useEffect, useEffectEvent, useState } from "react";
import { isAbort, toApiError } from "@/shared/services/http-client";

export interface AsyncData<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  /** Status HTTP galat terakhir (404 = data tidak ada); null bila tak ada galat/bukan HTTP. */
  errorStatus: number | null;
  reload: () => void;
}

/**
 * useAsyncData — satu-satunya tempat fetch-on-mount di frontend.
 *
 * Menggantikan pola `useCallback(load) + useEffect(() => { void load() })`
 * yang tersebar di ±47 halaman dan memicu 89 peringatan
 * react-hooks/set-state-in-effect + render beruntun (setLoading sinkron di
 * dalam effect). Di sini effect hanya ada SATU, dengan pembatalan saat
 * unmount / loader berganti (flag alive) sehingga setState basi tidak terjadi.
 *
 * `loader` dibaca via useEffectEvent (React 19): identitas fungsi yang tidak
 * stabil tidak memicu fetch ulang, tanpa perlu tulis ref saat render.
 * `deps` diserialkan menjadi kunci stabil — ganti isi deps untuk fetch ulang.
 *
 * Aturan eslint `set-state-in-effect` adalah heuristik performa untuk
 * sinkronisasi turunan (derived state) — bukan untuk I/O. Effect di bawah
 * adalah kasus I/O yang sah (fetch + alive-guard), jadi penekanan di sini
 * disengaja dan terdokumentasi, bukan pengabaian massal per halaman.
 */
export function useAsyncData<T>(
  loader: (signal: AbortSignal) => Promise<T>,
  deps: unknown[],
): AsyncData<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [errorStatus, setErrorStatus] = useState<number | null>(null);
  const [nonce, setNonce] = useState(0);

  const loadNow = useEffectEvent(loader);

  const reload = useCallback(() => {
    setNonce((n) => n + 1);
  }, []);

  // Kunci stabil dari deps — ganti isi deps untuk fetch ulang.
  const depsKey = JSON.stringify(deps, (_, v) =>
    typeof v === "function" ? String(v) : v,
  );

  useEffect(() => {
    let alive = true;
    // Pembatalan nyata, bukan sekadar mengabaikan hasil: ganti filter atau
    // pindah halaman saat request masih jalan dulu hanya membuang responsnya
    // — koneksinya tetap dipakai sampai selesai, dan di daftar yang berat
    // request basi itu mengantre di depan request yang benar-benar dilihat
    // pengguna. `alive` tetap ada untuk loader yang mengabaikan signal.
    const controller = new AbortController();
    // Satu-satunya penekanan set-state-in-effect di seluruh frontend.
    // Baris di bawah adalah I/O (fetch + alive-guard), bukan derived state —
    // kasus sah untuk effect yang tidak bisa dipindah ke event handler
    // (fetch-on-mount + reload + perubahan filter deklaratif).
    /* eslint-disable react-hooks/set-state-in-effect */
    setLoading(true);
    setError(null);
    setErrorStatus(null);
    /* eslint-enable react-hooks/set-state-in-effect */
    loadNow(controller.signal)
      .then((result) => {
        if (alive) setData(result);
      })
      .catch((err) => {
        // Request yang kita batalkan sendiri bukan kegagalan — jangan
        // tampilkan pesan error untuk sesuatu yang memang sudah usang.
        if (alive && !isAbort(err)) {
          const apiErr = toApiError(err);
          setError(apiErr.message);
          setErrorStatus(apiErr.status > 0 ? apiErr.status : null);
        }
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
      controller.abort();
    };
  }, [depsKey, nonce]);

  return { data, loading, error, errorStatus, reload };
}
