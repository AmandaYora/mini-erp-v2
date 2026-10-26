import { LineItemsEditor } from "@/shared/components/patterns/line-items-editor";
import type { LineColumn } from "@/shared/components/patterns/line-items-editor";
import { Qty } from "@/shared/components/patterns/values";
import { EmptyState } from "../feedback/empty-state";
import { Button } from "./button";
import type { DataTableColumn } from "./data-table";
import { INPUT } from "./field-classes";
import { SelectInput } from "./input";
import { clampQty, remainingInUnit } from "@/shared/lib/return-lines";
import type { ReturnUom } from "@/shared/lib/return-lines";

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
  /** Satuan baris dokumen sumber (label "dipesan"); default = uom. */
  orderUom?: string;
  /** Faktor satuan terpilih ke satuan dasar + sisa dalam satuan dasar: bila ada, sisa dan batas
   * mengikuti satuan terpilih (retur per PCS atas baris ber-DUS). */
  uomFactor?: number;
  remainingBase?: number;
  uoms?: ReturnUom[];
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
  /** Ganti satuan retur; kolom "Satuan" muncul bila ada baris dengan > 1 satuan. */
  onUomChange?: (key: string, uom: string) => void;
}

/**
 * Grid baris retur (mode "pilih qty dari sumber") di atas LineItemsEditor:
 * baris datang dari dokumen sumber, pengguna hanya mengisi qty (dibatasi sisa) + kolom tambahan.
 * Qty awal kosong (pilih-dulu, seperti sistem lama): "Retur semua sisa" mengisi setiap baris dengan
 * sisanya, "Maks" mengisi satu baris — retur penuh selalu aksi yang disengaja.
 */
export function ReturnLinesTable<T extends ReturnLineRow>({
  rows,
  onQtyChange,
  disabled,
  fulfilledLabel,
  emptyDescription,
  extraColumns = [],
  onUomChange,
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
            {r.productCode} · dipesan <Qty value={r.orderedQty} unit={r.orderUom ?? r.uom} />
          </p>
          <p className="text-xs text-muted-foreground">
            Sisa <Qty value={remainingInUnit(r)} unit={r.uom} /> ({fulfilledLabel} <Qty value={r.fulfilled} />, diretur{" "}
            <Qty value={r.returned} /> {r.orderUom ?? r.uom})
          </p>
        </div>
      ),
    },
    ...(onUomChange && rows.some((r) => (r.uoms?.length ?? 0) > 1)
      ? [
          {
            id: "uom",
            header: "Satuan",
            width: "7rem",
            render: (r: T) =>
              (r.uoms?.length ?? 0) > 1 ? (
                <SelectInput
                  aria-label={`Satuan retur ${r.productName}`}
                  value={r.uom}
                  disabled={disabled}
                  onChange={(e) => onUomChange(r.key, e.target.value)}
                >
                  {r.uoms?.map((u) => (
                    <option key={u.uom} value={u.uom}>
                      {u.factor === 1 ? u.uom : `${u.uom} (isi ${u.factor})`}
                    </option>
                  ))}
                </SelectInput>
              ) : (
                <span className="inline-block py-2">{r.uom}</span>
              ),
          } satisfies LineColumn<T>,
        ]
      : []),
    {
      id: "qty",
      header: "Qty retur",
      width: "11rem",
      align: "right",
      render: (r) => (
        <div className="flex items-center justify-end gap-1">
          <input
            type="number"
            min={0}
            max={remainingInUnit(r)}
            step="any"
            value={r.qty}
            placeholder="0"
            disabled={disabled}
            aria-label={`Qty retur ${r.productName}`}
            onChange={(e) => onQtyChange(r.key, clampQty(e.target.value, remainingInUnit(r)))}
            className={`${INPUT} tabular text-right`}
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={disabled || remainingInUnit(r) <= 0}
            aria-label={`Isi maksimal ${r.productName}`}
            onClick={() => onQtyChange(r.key, String(remainingInUnit(r)))}
          >
            Maks
          </Button>
        </div>
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
  const returnable = rows.filter((r) => remainingInUnit(r) > 0);
  const anyFilled = rows.some((r) => r.qty.trim() !== "");
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-muted-foreground">
          Isi qty hanya pada barang yang dikembalikan; baris kosong tidak ikut diretur.
        </p>
        <div className="flex gap-2">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={disabled || returnable.length === 0}
            onClick={() => returnable.forEach((r) => onQtyChange(r.key, String(remainingInUnit(r))))}
          >
            Retur semua sisa
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={disabled || !anyFilled}
            onClick={() => rows.forEach((r) => onQtyChange(r.key, ""))}
          >
            Kosongkan
          </Button>
        </div>
      </div>
      <LineItemsEditor<T> lines={rows} getKey={(r) => r.key} columns={columns} readOnly />
    </div>
  );
}
