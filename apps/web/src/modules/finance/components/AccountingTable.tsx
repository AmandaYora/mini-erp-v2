import type { ReactNode } from "react";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { TableSkeleton } from "@/shared/components/feedback/skeleton";
import { cn } from "@/shared/lib/cn";

export type AccountingRowKind = "line" | "group" | "subtotal" | "total";

export interface AccountingColumn<T> {
  id: string;
  header: string;
  align?: "left" | "right";
  /** Lebar trek grid CSS, mis. "9rem". Default: membagi sisa ruang. */
  width?: string;
  render: (row: T) => ReactNode;
}

export interface AccountingTableProps<T> {
  columns: AccountingColumn<T>[];
  rows: T[] | null | undefined;
  rowKey: (row: T) => string | number;
  /** line (default) · group (judul golongan, melebar penuh) · subtotal (garis atas) · total (garis atas tebal). */
  rowKind?: (row: T) => AccountingRowKind;
  loading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  emptyTitle?: string;
  emptyDescription?: string;
  /** Label aksesibel tabel. */
  caption: string;
  /** Tinggi maksimum sebelum digulir di dalam; header tetap menempel. */
  maxHeight?: string;
  /** Lebar minimum isi sebelum digulir mendatar (layar sempit). */
  minWidth?: string;
}

const ROW_CLASS: Record<AccountingRowKind, string> = {
  line: "border-b border-border",
  group: "border-b border-border bg-background font-semibold text-foreground",
  subtotal: "border-t border-input border-b border-b-border font-semibold text-foreground",
  total: "border-t-2 border-input bg-background font-semibold text-foreground",
};

/**
 * Tabel laporan gaya akuntansi (PLAN §7 T5): header menempel, baris golongan tebal, subtotal bergaris
 * atas, kolom angka rata kanan. Dibangun dengan grid ber-ARIA (bukan elemen tabel mentah) supaya baris
 * golongan bisa melebar penuh dan baris total sejajar dengan kolomnya. Angka diberikan lewat
 * `<Money accounting />` oleh pemanggil.
 */
export function AccountingTable<T>({
  columns,
  rows: rowsProp,
  rowKey,
  rowKind,
  loading = false,
  error,
  onRetry,
  emptyTitle = "Belum ada data",
  emptyDescription,
  caption,
  maxHeight = "70vh",
  minWidth = "36rem",
}: AccountingTableProps<T>) {
  const rows = rowsProp ?? [];
  if (error) return <ErrorState message={error} onRetry={onRetry} className="m-4" />;
  if (loading && rows.length === 0) return <TableSkeleton />;
  if (rows.length === 0) return <EmptyState title={emptyTitle} description={emptyDescription} />;

  const template = columns.map((c) => c.width ?? "minmax(0,1fr)").join(" ");
  const cellBase = "min-w-0 px-4 py-[var(--row-py)] text-sm first:pl-5 last:pr-5";

  return (
    <div className={cn("overflow-auto", loading && "opacity-60 transition-opacity")} style={{ maxHeight }} aria-busy={loading || undefined} role="region" aria-label={`Gulir ${caption}`} tabIndex={0}>
      <div role="table" aria-label={caption} style={{ minWidth }}>
        <div role="rowgroup" className="sticky top-0 z-10">
          <div role="row" className="grid border-b border-border bg-background" style={{ gridTemplateColumns: template }}>
            {columns.map((c) => (
              <div
                key={c.id}
                role="columnheader"
                className={cn(
                  "px-4 py-2.5 text-xs font-semibold whitespace-nowrap text-muted-foreground first:pl-5 last:pr-5",
                  c.align === "right" ? "text-right" : "text-left",
                )}
              >
                {c.header}
              </div>
            ))}
          </div>
        </div>
        <div role="rowgroup">
          {rows.map((row) => {
            const kind = rowKind?.(row) ?? "line";
            const cols = kind === "group" ? columns.slice(0, 1) : columns;
            return (
              <div
                key={rowKey(row)}
                role="row"
                className={cn("grid", ROW_CLASS[kind])}
                style={{ gridTemplateColumns: kind === "group" ? "minmax(0,1fr)" : template }}
              >
                {cols.map((c) => (
                  <div
                    key={c.id}
                    role="cell"
                    className={cn(cellBase, "text-foreground", c.align === "right" ? "text-right" : "text-left")}
                  >
                    {c.render(row)}
                  </div>
                ))}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
