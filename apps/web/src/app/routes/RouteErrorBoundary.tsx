import { useEffect, useMemo } from "react";
import { useRouteError } from "react-router-dom";
import { CrashCard } from "@/shared/components/feedback/crash-card";
import { isChunkLoadError } from "@/shared/lib/chunk-recovery";
import { createErrorId } from "@/shared/lib/error-id";

/**
 * `errorElement` rute (OP-01): menangkap galat render/loader/lazy-import di bawah rute,
 * sehingga halaman rusak tidak mengosongkan layar. Dipasang pada akar router DAN pada
 * tiap layout (AppShell/FocusLayout) agar sidebar tetap tampil saat satu halaman gagal.
 */
export function RouteErrorBoundary({ fullScreen = false }: { fullScreen?: boolean }) {
  const error = useRouteError();
  const errorId = useMemo(() => createErrorId(), []);
  useEffect(() => {
    console.error(`[${errorId}] galat rute`, error);
  }, [error, errorId]);
  return <CrashCard errorId={errorId} stale={isChunkLoadError(error)} fullScreen={fullScreen} />;
}
