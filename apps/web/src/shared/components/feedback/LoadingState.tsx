import { Spinner } from "@/shared/components/primitives/spinner";

/** Layar penuh saat sesi dipulihkan. Teks Bahasa Indonesia (U7). */
export function LoadingState() {
  return (
    <div className="flex min-h-screen items-center justify-center gap-2 text-muted-foreground" role="status">
      <Spinner aria-hidden className="size-4" />
      Memuat… {/* ui-ratchet-ignore: satu-satunya layar pemuatan penuh */}
    </div>
  );
}
