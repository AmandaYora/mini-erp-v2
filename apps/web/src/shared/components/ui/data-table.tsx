import type { ReactNode } from "react";
import { EmptyState } from "../feedback/empty-state";

export type DataTableAlign = "left" | "center" | "right";

export interface DataTableColumn<T> {
  header: string;
  width?: string | number;
  align?: DataTableAlign;
  render: (row: T) => ReactNode;
}

export interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  rows: T[];
  rowKey: (row: T) => string | number;
  emptyTitle?: string;
  emptyDescription?: string;
}

function alignClass(align?: DataTableAlign): string {
  if (align === "center") return "text-center";
  if (align === "right") return "text-right";
  return "text-left";
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  emptyTitle = "Belum ada data",
  emptyDescription,
}: DataTableProps<T>) {
  if (rows.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse">
        <thead>
          <tr>
            {columns.map((col, i) => (
              <th
                key={i}
                style={col.width !== undefined ? { width: col.width } : undefined}
                className={`bg-surface-subtle text-muted px-5 py-3.5 text-[0.75rem] font-semibold uppercase ${alignClass(col.align)}`}
              >
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={rowKey(row)}>
              {columns.map((col, i) => (
                <td
                  key={i}
                  className={`border-b border-hairline px-5 py-3.5 text-sm text-ink ${alignClass(col.align)}`}
                >
                  {col.render(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
