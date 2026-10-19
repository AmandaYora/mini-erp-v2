import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";

/**
 * Bar aksi form yang menempel di dasar kanvas: ringkasan/total di kiri, Batal · Simpan di kanan.
 * Total tetap terlihat saat menggulir (PLAN §7 T3).
 */
export function StickyFooter({
  summary,
  children,
  className,
}: {
  summary?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "sticky bottom-0 z-20 -mx-6 mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-border bg-card/95 px-6 py-3 max-md:-mx-4 max-md:px-4",
        className,
      )}
    >
      <div className="min-w-0 text-sm text-muted-foreground">{summary}</div>
      <div className="flex flex-wrap items-center justify-end gap-2">{children}</div>
    </div>
  );
}
