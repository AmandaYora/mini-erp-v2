import { useEffect, useRef, type ReactNode } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/shared/components/ui/button";
import { cn } from "@/shared/lib/cn";

export interface LineColumn<T> {
  id: string;
  header: string;
  /** Lebar kolom (CSS), mis. "9rem". Kolom tanpa lebar membagi sisa ruang. */
  width?: string;
  align?: "left" | "right" | "center";
  /** Isi sel — biasanya kontrol input; `index` untuk label aksesibel ("Qty baris 3"). */
  render: (line: T, index: number) => ReactNode;
}

export interface LineItemsEditorProps<T> {
  lines: T[];
  getKey: (line: T, index: number) => string | number;
  columns: LineColumn<T>[];
  /** Tambah baris kosong; dipanggil juga oleh Enter di baris terakhir. */
  onAdd?: () => void;
  onRemove?: (index: number) => void;
  addLabel?: string;
  emptyText?: string;
  /** Hanya tampil (detail/pratinjau): tanpa tombol tambah/hapus. */
  readOnly?: boolean;
  /** Baris di bawah tabel (mis. ringkasan). */
  footer?: ReactNode;
  className?: string;
}

/**
 * Grid baris barang bersama untuk SO/PO/retur/penerimaan/surat jalan/transfer/jurnal.
 * Keyboard: Tab berpindah sel, Enter di baris terakhir menambah baris dan memfokuskan
 * isian pertamanya. Satu pola untuk semua dokumen (PLAN §7 T3) — menggantikan DataTable
 * baris SO vs kartu-per-baris PO yang tidak konsisten.
 */
export function LineItemsEditor<T>({
  lines,
  getKey,
  columns,
  onAdd,
  onRemove,
  addLabel = "Tambah baris",
  emptyText = "Belum ada baris.",
  readOnly = false,
  footer,
  className,
}: LineItemsEditorProps<T>) {
  const bodyRef = useRef<HTMLTableSectionElement>(null);
  const focusLast = useRef(false);

  // Setelah baris ditambah lewat Enter, fokuskan isian pertama baris baru.
  useEffect(() => {
    if (!focusLast.current) return;
    focusLast.current = false;
    const rows = bodyRef.current?.querySelectorAll("tr");
    const last = rows?.[rows.length - 1];
    last?.querySelector<HTMLElement>("input:not([type=hidden]):not(:disabled), select:not(:disabled), button[role=combobox], [aria-haspopup=listbox]")?.focus();
  }, [lines.length]);

  const canEdit = !readOnly;
  const total = columns.length + (canEdit && onRemove ? 2 : 1);

  return (
    <div className={cn("overflow-hidden rounded-lg border border-border", className)}>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[640px] border-collapse">
          <thead>
            <tr className="border-b border-border bg-background">
              <th scope="col" className="w-10 px-3 py-2 text-center text-xs font-semibold text-muted-foreground">
                #
              </th>
              {columns.map((c) => (
                <th
                  key={c.id}
                  scope="col"
                  style={c.width ? { width: c.width } : undefined}
                  className={cn(
                    "px-2 py-2 text-xs font-semibold whitespace-nowrap text-muted-foreground",
                    c.align === "right" ? "text-right" : c.align === "center" ? "text-center" : "text-left",
                  )}
                >
                  {c.header}
                </th>
              ))}
              {canEdit && onRemove && (
                <th scope="col" className="w-10">
                  <span className="sr-only">Hapus</span>
                </th>
              )}
            </tr>
          </thead>
          <tbody ref={bodyRef}>
            {lines.length === 0 ? (
              <tr>
                <td colSpan={total} className="px-4 py-8 text-center text-sm text-muted-foreground">
                  {emptyText}
                </td>
              </tr>
            ) : (
              lines.map((line, index) => (
                <tr
                  key={getKey(line, index)}
                  className="border-b border-border align-top last:border-b-0"
                  onKeyDown={(e) => {
                    if (!canEdit || !onAdd || e.key !== "Enter" || index !== lines.length - 1) return;
                    const el = e.target as HTMLElement;
                    if (el.tagName !== "INPUT" || (el as HTMLInputElement).type === "checkbox") return;
                    e.preventDefault();
                    focusLast.current = true;
                    onAdd();
                  }}
                >
                  <td className="tabular px-3 py-2.5 text-center text-xs text-muted-foreground">{index + 1}</td>
                  {columns.map((c) => (
                    <td
                      key={c.id}
                      className={cn(
                        "px-2 py-1.5 text-sm",
                        c.align === "right" ? "text-right" : c.align === "center" ? "text-center" : "text-left",
                      )}
                    >
                      {c.render(line, index)}
                    </td>
                  ))}
                  {canEdit && onRemove && (
                    <td className="px-1 py-1.5 text-center">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={`Hapus baris ${index + 1}`}
                        onClick={() => onRemove(index)}
                        className="text-muted-foreground hover:text-destructive"
                      >
                        <Trash2 aria-hidden />
                      </Button>
                    </td>
                  )}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
      {(canEdit && onAdd) || footer ? (
        <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border bg-background px-3 py-2">
          {canEdit && onAdd ? (
            <Button variant="ghost" size="sm" onClick={onAdd}>
              <Plus aria-hidden /> {addLabel}
            </Button>
          ) : (
            <span />
          )}
          {canEdit && onAdd && lines.length > 0 && (
            <span className="text-xs text-muted-foreground">Enter di baris terakhir menambah baris · Tab pindah sel</span>
          )}
          {footer}
        </div>
      ) : null}
    </div>
  );
}
