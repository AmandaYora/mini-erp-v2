import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";

/** Akar halaman: tumpukan vertikal 24px. Mengakhiri ratusan `mb-4`/`mt-6` ad-hoc antarseksi. */
export function Page({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("flex flex-col gap-6 [&>.page-header]:mb-0", className)}>{children}</div>;
}

/** Dua kolom halaman dokumen: isi (kiri) + ringkasan/metadata (kanan, menempel saat digulir). */
export function PageColumns({ main, aside }: { main: ReactNode; aside: ReactNode }) {
  return (
    <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
      <div className="flex min-w-0 flex-col gap-6">{main}</div>
      <aside className="flex min-w-0 flex-col gap-6 xl:sticky xl:top-20">{aside}</aside>
    </div>
  );
}
