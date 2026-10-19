import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";

export interface DescriptionItem {
  label: string;
  value: ReactNode;
  /** Lebarkan item ke seluruh baris (alamat, catatan). */
  wide?: boolean;
}

const COLS = {
  1: "grid-cols-1",
  2: "grid-cols-1 sm:grid-cols-2",
  3: "grid-cols-1 sm:grid-cols-2 lg:grid-cols-3",
  4: "grid-cols-1 sm:grid-cols-2 lg:grid-cols-4",
} as const;

/** Daftar label-nilai baku; menggantikan 21 blok `<dl>` dengan 6 varian grid. Nilai kosong tampil "–". */
export function DescriptionList({
  items,
  columns = 3,
  className,
}: {
  items: DescriptionItem[];
  columns?: 1 | 2 | 3 | 4;
  className?: string;
}) {
  return (
    <dl className={cn("grid gap-x-6 gap-y-4", COLS[columns], className)}>
      {items.map((it) => (
        <div key={it.label} className={cn("min-w-0", it.wide && "col-span-full")}>
          <dt className="text-xs font-medium text-muted-foreground">{it.label}</dt>
          <dd className="mt-0.5 text-base break-words text-foreground">
            {it.value === null || it.value === undefined || it.value === "" ? (
              <span className="text-muted-foreground">–</span>
            ) : (
              it.value
            )}
          </dd>
        </div>
      ))}
    </dl>
  );
}
