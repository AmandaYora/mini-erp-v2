import { useMemo, useState, type MouseEvent, type ReactNode } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown, Columns3 } from "lucide-react";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@/shared/components/primitives/popover";
import { EmptyState } from "../feedback/empty-state";
import { ErrorState } from "../feedback/error-state";
import { TableSkeleton } from "../feedback/skeleton";
import { cn } from "@/shared/lib/cn";
import { Button } from "./button";

export type DataTableAlign = "left" | "center" | "right";
export type SortDir = "asc" | "desc";
export interface DataTableSort {
  key: string;
  dir: SortDir;
}

export interface DataTableColumn<T> {
  header: string;
  width?: string | number;
  align?: DataTableAlign;
  render: (row: T) => ReactNode;
  /** Kunci sortir server-side; kolom dengan `sortKey` menjadi bisa diklik di header. */
  sortKey?: string;
  /** Id stabil untuk pilihan kolom (default: header). */
  id?: string;
  /** Tersembunyi secara default; pengguna bisa menampilkannya lewat menu "Kolom". */
  defaultHidden?: boolean;
}

export interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  /** null/undefined diperlakukan sebagai kosong (Go mengirim null untuk slice kosong). */
  rows: T[] | null | undefined;
  rowKey: (row: T) => string | number;
  emptyTitle?: string;
  emptyDescription?: string;
  /** Aksi di keadaan kosong (mis. tombol "Buat order pertama"). */
  emptyAction?: ReactNode;
  /** Memuat: skeleton bila belum ada baris, redup bila sedang memuat ulang. */
  loading?: boolean;
  /** Galat pemuatan — tampilkan message API asli + "Coba lagi" bila `onRetry` ada. */
  error?: string | null;
  onRetry?: () => void;
  /** Seluruh baris bisa diklik (klik pada link/tombol/isian di dalam baris diabaikan). */
  onRowClick?: (row: T) => void;
  sort?: DataTableSort | null;
  onSortChange?: (sort: DataTableSort) => void;
  /** Kotak centang per baris + bar aksi massal. */
  selectable?: boolean;
  selectedKeys?: ReadonlySet<string | number>;
  onSelectionChange?: (keys: Set<string | number>) => void;
  /** Isi bar aksi massal; menggantikan toolbar saat ada baris terpilih. */
  bulkActions?: (selectedKeys: ReadonlySet<string | number>) => ReactNode;
  /** Header menempel di atas saat tabel digulir (default aktif). */
  stickyHeader?: boolean;
  /** Tinggi maksimum area tabel sebelum digulir di dalam. */
  maxHeight?: string;
  /** Bila diberikan, pilihan kolom tampil dan disimpan di localStorage dengan kunci ini. */
  columnsKey?: string;
  /** Label aksesibel tabel. */
  caption?: string;
}

function alignClass(align?: DataTableAlign): string {
  if (align === "center") return "text-center";
  if (align === "right") return "text-right";
  return "text-left";
}

// Termasuk konten portal (menu ⋯, dialog): event React merambat lewat pohon komponen ke <tr>.
const INTERACTIVE =
  "a,button,input,select,textarea,label,[role=button],[role=checkbox],[role=menu],[role=menuitem],[role=dialog],[role=alertdialog],[role=listbox]";

function colId<T>(c: DataTableColumn<T>): string {
  return c.id ?? c.header;
}

function readHidden<T>(key: string | undefined, columns: DataTableColumn<T>[]): Set<string> {
  const defaults = new Set(columns.filter((c) => c.defaultHidden).map(colId));
  if (!key) return defaults;
  try {
    const raw = localStorage.getItem(`mini-erp.cols.${key}`);
    if (raw) return new Set(JSON.parse(raw) as string[]);
  } catch {
    /* penyimpanan privat: pakai default */
  }
  return defaults;
}

export function DataTable<T>({
  columns,
  rows: rowsProp,
  rowKey,
  emptyTitle = "Belum ada data",
  emptyDescription,
  emptyAction,
  loading = false,
  error,
  onRetry,
  onRowClick,
  sort,
  onSortChange,
  selectable = false,
  selectedKeys,
  onSelectionChange,
  bulkActions,
  stickyHeader = true,
  maxHeight = "70vh",
  columnsKey,
  caption,
}: DataTableProps<T>) {
  const rows = rowsProp ?? [];
  const [hidden, setHidden] = useState<Set<string>>(() => readHidden(columnsKey, columns));
  const visible = useMemo(() => columns.filter((c) => !hidden.has(colId(c))), [columns, hidden]);

  if (error) return <ErrorState message={error} onRetry={onRetry} className="m-4" />;
  if (loading && rows.length === 0) return <TableSkeleton />;
  if (rows.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} action={emptyAction} />;
  }

  const selected = selectedKeys ?? new Set<string | number>();
  const allSelected = rows.length > 0 && rows.every((r) => selected.has(rowKey(r)));
  const someSelected = !allSelected && rows.some((r) => selected.has(rowKey(r)));

  function toggleAll(checked: boolean) {
    const next = new Set(selected);
    for (const r of rows) {
      if (checked) next.add(rowKey(r));
      else next.delete(rowKey(r));
    }
    onSelectionChange?.(next);
  }
  function toggleOne(key: string | number, checked: boolean) {
    const next = new Set(selected);
    if (checked) next.add(key);
    else next.delete(key);
    onSelectionChange?.(next);
  }
  function toggleColumn(id: string) {
    const next = new Set(hidden);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setHidden(next);
    if (columnsKey) {
      try {
        localStorage.setItem(`mini-erp.cols.${columnsKey}`, JSON.stringify([...next]));
      } catch {
        /* abaikan */
      }
    }
  }
  function handleRowClick(e: MouseEvent<HTMLTableRowElement>, row: T) {
    if (!onRowClick) return;
    if ((e.target as HTMLElement).closest(INTERACTIVE)) return;
    onRowClick(row);
  }

  const hasSelection = selectable && selected.size > 0;
  const showToolbar = Boolean(columnsKey) || hasSelection;
  const hideableColumns = columns.filter((c) => c.header !== "");

  return (
    <div className={cn(loading && "opacity-60 transition-opacity")} aria-busy={loading || undefined}>
      {showToolbar && (
        <div className="flex min-h-12 items-center justify-between gap-3 border-b border-border px-5 py-2">
          {hasSelection ? (
            <div className="flex flex-wrap items-center gap-3">
              <span className="tabular text-sm font-medium text-foreground">{selected.size} dipilih</span>
              {bulkActions?.(selected)}
              <Button variant="ghost" size="sm" onClick={() => onSelectionChange?.(new Set())}>
                Batal pilih
              </Button>
            </div>
          ) : (
            <span />
          )}
          {columnsKey && (
            <Popover>
              <PopoverTrigger asChild>
                <Button variant="secondary" size="sm">
                  <Columns3 aria-hidden /> Kolom
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" className="w-56 p-2">
                <ul className="flex flex-col">
                  {hideableColumns.map((c) => {
                    const id = colId(c);
                    return (
                      <li key={id}>
                        <label className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent">
                          <Checkbox checked={!hidden.has(id)} onCheckedChange={() => toggleColumn(id)} />
                          {c.header}
                        </label>
                      </li>
                    );
                  })}
                </ul>
              </PopoverContent>
            </Popover>
          )}
        </div>
      )}
      <div className="overflow-auto" style={{ maxHeight }} role="region" aria-label={caption ?? "Tabel data"} tabIndex={0}>
        <table className="w-full min-w-[640px] border-collapse">
          {caption && <caption className="sr-only">{caption}</caption>}
          <thead className={cn(stickyHeader && "sticky top-0 z-10")}>
            <tr className="border-b border-border bg-background">
              {selectable && (
                <th scope="col" className="w-10 py-2.5 pl-5">
                  <Checkbox
                    aria-label="Pilih semua baris"
                    checked={allSelected ? true : someSelected ? "indeterminate" : false}
                    onCheckedChange={(v) => toggleAll(v === true)}
                  />
                </th>
              )}
              {visible.map((col, i) => {
                const sorted = col.sortKey && sort?.key === col.sortKey ? sort.dir : null;
                const SortIcon = sorted === "asc" ? ArrowUp : sorted === "desc" ? ArrowDown : ArrowUpDown;
                return (
                  <th
                    key={colId(col) + i}
                    scope="col"
                    aria-sort={sorted ? (sorted === "asc" ? "ascending" : "descending") : undefined}
                    style={col.width !== undefined ? { width: col.width } : undefined}
                    className={cn(
                      "px-4 py-2.5 text-xs font-semibold tracking-[0.02em] whitespace-nowrap text-muted-foreground first:pl-5 last:pr-5",
                      alignClass(col.align),
                    )}
                  >
                    {col.sortKey && onSortChange ? (
                      <button
                        type="button"
                        onClick={() =>
                          onSortChange({ key: col.sortKey!, dir: sorted === "asc" ? "desc" : "asc" })
                        }
                        className={cn(
                          "inline-flex items-center gap-1 rounded-sm hover:text-foreground",
                          col.align === "right" && "flex-row-reverse",
                        )}
                      >
                        {col.header}
                        <SortIcon size={12} aria-hidden className={sorted ? "text-foreground" : "opacity-50"} />
                      </button>
                    ) : (
                      col.header
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const key = rowKey(row);
              const isSelected = selected.has(key);
              return (
                <tr
                  key={key}
                  data-state={isSelected ? "selected" : undefined}
                  onClick={onRowClick ? (e) => handleRowClick(e, row) : undefined}
                  className={cn(
                    "border-b border-border transition-colors last:border-b-0 hover:bg-background",
                    isSelected && "bg-accent/60",
                    onRowClick && "cursor-pointer",
                  )}
                >
                  {selectable && (
                    <td className="w-10 py-[var(--row-py)] pl-5">
                      <Checkbox
                        aria-label="Pilih baris"
                        checked={isSelected}
                        onCheckedChange={(v) => toggleOne(key, v === true)}
                      />
                    </td>
                  )}
                  {visible.map((col, i) => (
                    <td
                      key={colId(col) + i}
                      className={cn(
                        "px-4 py-[var(--row-py)] text-sm text-foreground first:pl-5 last:pr-5",
                        alignClass(col.align),
                      )}
                    >
                      {col.render(row)}
                    </td>
                  ))}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
