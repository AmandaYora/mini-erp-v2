import { useState, type ReactNode } from "react";
import { ListFilter, X } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/shared/components/primitives/popover";
import { Button } from "@/shared/components/ui/button";
import { cn } from "@/shared/lib/cn";

export interface ActiveFilter {
  key: string;
  label: string;
  onRemove: () => void;
}

/**
 * Toolbar tabel: pencarian + maksimal 3 filter utama selalu terlihat; sisanya masuk popover
 * "+ Filter". Filter aktif tampil sebagai chip yang bisa dihapus. `right` = aksi toolbar (kolom, ekspor).
 */
export function Toolbar({
  search,
  primary,
  more,
  moreActiveCount = 0,
  active = [],
  onClearAll,
  right,
  className,
}: {
  search?: ReactNode;
  primary?: ReactNode;
  /** Filter tambahan di popover. */
  more?: ReactNode;
  moreActiveCount?: number;
  active?: ActiveFilter[];
  onClearAll?: () => void;
  right?: ReactNode;
  className?: string;
}) {
  const [moreOpen, setMoreOpen] = useState(false);
  return (
    <div className={cn("rounded-xl border border-border bg-card px-4 py-3", className)}>
      <div className="flex flex-wrap items-end gap-3">
        {search && <div className="min-w-[220px] flex-1 sm:max-w-sm">{search}</div>}
        {primary}
        {more && (
          <Popover open={moreOpen} onOpenChange={setMoreOpen}>
            <PopoverTrigger asChild>
              <Button variant="secondary">
                <ListFilter aria-hidden /> Filter
                {moreActiveCount > 0 && (
                  <span className="tabular rounded-full bg-primary px-1.5 text-2xs font-semibold text-primary-foreground">
                    {moreActiveCount}
                  </span>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="flex w-80 flex-col gap-3 p-4">
              {more}
            </PopoverContent>
          </Popover>
        )}
        {right && <div className="ml-auto flex items-center gap-2">{right}</div>}
      </div>
      {active.length > 0 && (
        <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-border pt-3">
          {active.map((f) => (
            <span
              key={f.key}
              className="inline-flex items-center gap-1 rounded-full bg-accent py-0.5 pr-1 pl-2.5 text-xs font-medium text-accent-foreground"
            >
              {f.label}
              <button
                type="button"
                onClick={f.onRemove}
                aria-label={`Hapus filter ${f.label}`}
                className="rounded-full p-0.5 hover:bg-primary/10"
              >
                <X size={12} aria-hidden />
              </button>
            </span>
          ))}
          {onClearAll && (
            <button type="button" onClick={onClearAll} className="text-xs font-medium text-primary hover:underline">
              Hapus semua
            </button>
          )}
        </div>
      )}
    </div>
  );
}
