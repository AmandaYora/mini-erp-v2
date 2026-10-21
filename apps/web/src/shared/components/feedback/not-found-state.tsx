import type { ReactNode } from "react";
import { FileQuestion } from "lucide-react";
import { Link } from "react-router-dom";
import { buttonClass } from "@/shared/components/ui/button-class";

interface NotFoundStateProps {
  title?: string;
  message?: string;
  /** Tautan kembali (mis. ke daftar). */
  backTo?: string;
  backLabel?: string;
  children?: ReactNode;
}

/**
 * Tampilan 404 domain-agnostik: dipakai halaman rute tak dikenal dan detail
 * dokumen yang id-nya tidak ada. Tidak bergantung pada modul mana pun.
 */
export function NotFoundState({
  title = "Halaman tidak ditemukan",
  message = "Alamat yang Anda buka tidak ada atau sudah dipindahkan.",
  backTo = "/dashboard",
  backLabel = "Kembali ke Dashboard",
  children,
}: NotFoundStateProps) {
  return (
    <div role="alert" className="mx-auto flex max-w-md flex-col items-center rounded-xl border border-border bg-card p-8 text-center">
      <span className="mb-3 flex size-11 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <FileQuestion size={20} aria-hidden />
      </span>
      <p className="font-display text-3xl font-semibold text-foreground">404</p>
      <h1 className="mt-1 text-lg font-semibold text-foreground">{title}</h1>
      <p className="mt-1 text-sm break-words text-muted-foreground [overflow-wrap:anywhere]">{message}</p>
      {children}
      <Link to={backTo} className={`${buttonClass("primary", "md")} mt-6 hover:no-underline`}>
        {backLabel}
      </Link>
    </div>
  );
}
