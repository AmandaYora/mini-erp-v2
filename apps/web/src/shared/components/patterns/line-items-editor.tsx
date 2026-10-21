import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode, type RefObject } from "react";
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

/** Di bawah lebar ini (px, lebar wadah editor) baris tampil sebagai kartu bertumpuk (PLAN U7-3). */
const LINE_CARD_BREAKPOINT = 600;

/**
 * Lebar wadah < `threshold`? Diukur dengan ResizeObserver agar ikut sidebar terbuka/tertutup,
 * bukan hanya lebar layar. Lebar 0 (belum tata letak / jsdom) dianggap lebar: tetap tabel.
 */
function useNarrow(ref: RefObject<HTMLElement | null>, threshold: number): boolean {
  const [narrow, setNarrow] = useState(false);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = (width: number) => setNarrow(width > 0 && width < threshold);
    measure(el.getBoundingClientRect().width);
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver((entries) => measure(entries[0]?.contentRect.width ?? 0));
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref, threshold]);
  return narrow;
}

/**
 * Grid baris barang bersama untuk SO/PO/retur/penerimaan/surat jalan/transfer/jurnal.
 * Keyboard: Tab berpindah sel, Enter di baris terakhir menambah baris dan memfokuskan
 * isian pertamanya. Satu pola untuk semua dokumen (PLAN §7 T3) — menggantikan DataTable
 * baris SO vs kartu-per-baris PO yang tidak konsisten. Di wadah sempit (ponsel, tablet tegak
 * dengan sidebar) tiap baris menjadi kartu: label kolom di atas isiannya, tanpa gulir menyamping.
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
  const rootRef = useRef<HTMLDivElement>(null);
  const focusLast = useRef(false);
  const narrow = useNarrow(rootRef, LINE_CARD_BREAKPOINT);

  // Setelah baris ditambah lewat Enter, fokuskan isian pertama baris baru.
  useEffect(() => {
    if (!focusLast.current) return;
    focusLast.current = false;
    const rows = rootRef.current?.querySelectorAll("[data-line]");
    const last = rows?.[rows.length - 1];
    last?.querySelector<HTMLElement>("input:not([type=hidden]):not(:disabled), select:not(:disabled), button[role=combobox], [aria-haspopup=listbox]")?.focus();
  }, [lines.length]);

  const canEdit = !readOnly;
  const total = columns.length + (canEdit && onRemove ? 2 : 1);

  function handleLineKeyDown(e: KeyboardEvent<HTMLElement>, index: number) {
    if (!canEdit || !onAdd || e.key !== "Enter" || index !== lines.length - 1) return;
    const el = e.target as HTMLElement;
    // Event dari portal (popover pemilih di dalam sel) merambat lewat pohon React: abaikan.
    if (!e.currentTarget.contains(el)) return;
    if (el.tagName !== "INPUT" || (el as HTMLInputElement).type === "checkbox") return;
    e.preventDefault();
    focusLast.current = true;
    onAdd();
  }

  const removeButton = (index: number) => (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={`Hapus baris ${index + 1}`}
      onClick={() => onRemove?.(index)}
      className="text-muted-foreground hover:text-destructive"
    >
      <Trash2 aria-hidden />
    </Button>
  );

  return (
    <div ref={rootRef} className={cn("overflow-hidden rounded-lg border border-border", className)}>
      {narrow ? (
        lines.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-muted-foreground">{emptyText}</p>
        ) : (
          <ol className="divide-y divide-border">
            {lines.map((line, index) => (
              <li key={getKey(line, index)} data-line="" className="px-3 py-3" onKeyDown={(e) => handleLineKeyDown(e, index)}>
                <div className="mb-2 flex items-center justify-between gap-2">
                  <span className="tabular text-xs font-semibold text-muted-foreground">Baris {index + 1}</span>
                  {canEdit && onRemove && removeButton(index)}
                </div>
                <div className="grid grid-cols-2 gap-x-3 gap-y-2">
                  {columns.map((c, ci) => (
                    // Kolom pertama (biasanya produk/akun) dan kolom tanpa lebar tetap memakai lebar penuh.
                    <div key={c.id} className={cn("min-w-0", (ci === 0 || !c.width) && "col-span-2")}>
                      {c.header && <div className="mb-1 text-xs font-medium text-muted-foreground">{c.header}</div>}
                      <div className={cn("text-sm", c.align === "right" && "text-right")}>{c.render(line, index)}</div>
                    </div>
                  ))}
                </div>
              </li>
            ))}
          </ol>
        )
      ) : (
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
            <tbody>
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
                    data-line=""
                    className="border-b border-border align-top last:border-b-0"
                    onKeyDown={(e) => handleLineKeyDown(e, index)}
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
                    {canEdit && onRemove && <td className="px-1 py-1.5 text-center">{removeButton(index)}</td>}
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
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
