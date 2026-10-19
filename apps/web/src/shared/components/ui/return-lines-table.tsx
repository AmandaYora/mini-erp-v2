import { LineItemsEditor } from "@/shared/components/patterns/line-items-editor";
import type { LineColumn } from "@/shared/components/patterns/line-items-editor";
import { Qty } from "@/shared/components/patterns/values";
import { EmptyState } from "../feedback/empty-state";
import type { DataTableColumn } from "./data-table";
import { INPUT } from "./field-classes";
import { clampQty } from "@/shared/lib/return-lines";

/** Baris tabel retur — domain-agnostik (jual maupun beli). */
export interface ReturnLineRow {
  key: string;
  productCode: string;
  productName: string;
  uom: string;
  orderedQty: number;
  /** Sudah dikirim (jual) / sudah diterima (beli). */
  fulfilled: number;
  returned: number;
  remaining: number;
  /** Qty retur sebagai string input. */
  qty: string;
}

interface Props<T extends ReturnLineRow> {
  rows: T[];
  onQtyChange: (key: string, qty: string) => void;
  disabled?: boolean;
  /** Kata kerja untuk kolom "fulfilled": "terkirim" | "diterima". */
  fulfilledLabel: string;
  emptyDescription?: string;
  /** Kolom tambahan (kondisi, lokasi, ...) setelah Qty Retur. */
  extraColumns?: DataTableColumn<T>[];
}

/**
 * Grid baris retur (mode "pilih qty dari sumber") di atas LineItemsEditor:
 * baris datang dari dokumen sumber, pengguna hanya mengisi qty (dibatasi sisa) + kolom tambahan.
 */
export function ReturnLinesTable<T extends ReturnLineRow>({
  rows,
  onQtyChange,
  disabled,
  fulfilledLabel,
  emptyDescription,
  extraColumns = [],
}: Props<T>) {
  if (rows.length === 0) {
    return (
      <div className="rounded-lg border border-border">
        <EmptyState title="Belum ada item" description={emptyDescription} />
      </div>
    );
  }
  const columns: LineColumn<T>[] = [
    {
      id: "product",
      header: "Produk",
      render: (r) => (
        <div className="py-1">
          <p className="font-medium text-foreground">{r.productName}</p>
          <p className="text-xs text-muted-foreground">
            {r.productCode} · {r.uom}
          </p>
          <p className="text-xs text-muted-foreground">
            Sisa <Qty value={r.remaining} /> dari <Qty value={r.orderedQty} /> dipesan ({fulfilledLabel}{" "}
            <Qty value={r.fulfilled} />, diretur <Qty value={r.returned} />)
          </p>
        </div>
      ),
    },
    {
      id: "qty",
      header: "Qty retur",
      width: "8rem",
      align: "right",
      render: (r) => (
        <input
          type="number"
          min={0}
          max={r.remaining}
          step="any"
          value={r.qty}
          disabled={disabled}
          aria-label={`Qty retur ${r.productName}`}
          onChange={(e) => onQtyChange(r.key, clampQty(e.target.value, r.remaining))}
          className={`${INPUT} tabular text-right`}
        />
      ),
    },
    ...extraColumns.map(
      (c, i): LineColumn<T> => ({
        id: c.id ?? `extra-${i}`,
        header: c.header,
        align: c.align,
        width: typeof c.width === "number" ? `${c.width}px` : c.width,
        render: (r) => c.render(r),
      }),
    ),
  ];
  return <LineItemsEditor<T> lines={rows} getKey={(r) => r.key} columns={columns} readOnly />;
}
