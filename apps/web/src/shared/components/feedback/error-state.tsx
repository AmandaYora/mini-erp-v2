import { Button } from "@/shared/components/ui/button";
import { Notice } from "./notice";

/**
 * Galat pemuatan: tampilkan `message` ASLI dari API (kontrak FRONTEND_GUIDE "Umpan Balik ke
 * Pengguna") + tombol "Coba lagi". Menggantikan ±40 Notice + tombol manual.
 */
export function ErrorState({
  message,
  onRetry,
  title = "Gagal memuat",
  className,
}: {
  message: string;
  onRetry?: () => void;
  title?: string;
  className?: string;
}) {
  return (
    <Notice tone="danger" title={title} className={className}>
      {message}
      {onRetry && (
        <>
          {" "}
          <Button variant="ghost" size="sm" onClick={onRetry} className="h-auto px-1.5 py-0.5 align-baseline">
            Coba lagi
          </Button>
        </>
      )}
    </Notice>
  );
}
