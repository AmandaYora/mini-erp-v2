import { useMemo } from "react";
import { DescriptionList } from "@/shared/components/patterns";
import { formatDateTime } from "@/shared/lib/format";
import { cn } from "@/shared/lib/cn";
import { diffRecords, formatDiffValue } from "@/modules/audit/lib/diff";
import type { DiffKind } from "@/modules/audit/lib/diff";
import { actorLabel, branchLabel, entityLabel } from "@/modules/audit/lib/audit";
import type { AuditRecord } from "@/modules/audit/types";

const KIND_LABEL: Record<DiffKind, string> = {
  added: "Ditambah",
  removed: "Dihapus",
  changed: "Diubah",
  same: "Tetap",
};

const KIND_CLASS: Record<DiffKind, string> = {
  added: "text-success",
  removed: "text-destructive",
  changed: "text-warning",
  same: "text-muted-foreground",
};

/** Satu baris diff gaya kode: `-` merah untuk sebelum, `+` hijau untuk sesudah. */
function DiffLine({ sign, value }: { sign: "-" | "+"; value: unknown }) {
  const removed = sign === "-";
  return (
    <div
      className={cn(
        "flex gap-2 px-3 py-1 font-mono text-xs break-all",
        removed ? "bg-destructive-soft text-destructive" : "bg-success-soft text-success",
      )}
    >
      <span aria-hidden className="select-none">
        {sign}
      </span>
      <span className="sr-only">{removed ? "Sebelum: " : "Sesudah: "}</span>
      <span className="min-w-0 flex-1">{formatDiffValue(value)}</span>
    </div>
  );
}

/**
 * AuditDetail — isi panel rincian satu entri audit: konteks (waktu, aktor, cabang, entitas),
 * diff sebelum/sesudah per kunci bergaya kode (hanya kunci yang berbeda), dan metadata
 * sebagai JSON terformat.
 */
export function AuditDetail({ record }: { record: AuditRecord }) {
  const rows = useMemo(() => diffRecords(record.before, record.after), [record.before, record.after]);
  const hasMeta = record.metadata != null;
  const hasSides = record.before != null || record.after != null;

  return (
    <div className="flex flex-col gap-6" data-testid="audit-detail">
      <DescriptionList
        columns={2}
        items={[
          { label: "Waktu", value: formatDateTime(record.createdAt) },
          { label: "Aktor", value: actorLabel(record) },
          { label: "Entitas", value: entityLabel(record) },
          { label: "Cabang", value: branchLabel(record) },
          { label: "Catatan", value: record.note, wide: true },
        ]}
      />

      {rows.length > 0 ? (
        <section aria-label="Perubahan">
          <h3 className="mb-2 text-sm font-semibold text-foreground">Perubahan</h3>
          <ul className="flex flex-col gap-3">
            {rows.map((r) => (
              <li key={r.path} className="overflow-hidden rounded-lg border border-border">
                <div className="flex items-center justify-between gap-3 border-b border-border bg-background px-3 py-1.5">
                  <span className="font-mono text-xs font-medium break-all text-foreground">{r.path}</span>
                  <span className={cn("flex-none text-xs font-medium", KIND_CLASS[r.kind])}>{KIND_LABEL[r.kind]}</span>
                </div>
                {r.kind !== "added" && <DiffLine sign="-" value={r.before} />}
                {r.kind !== "removed" && <DiffLine sign="+" value={r.after} />}
              </li>
            ))}
          </ul>
        </section>
      ) : hasSides ? (
        <p className="text-sm text-muted-foreground">Sebelum dan sesudah identik — tidak ada perbedaan.</p>
      ) : null}

      {hasMeta && (
        <section aria-label="Metadata">
          <h3 className="mb-2 text-sm font-semibold text-foreground">Metadata</h3>
          <pre className="overflow-x-auto rounded-lg border border-border bg-muted p-3 font-mono text-xs">
            {JSON.stringify(record.metadata, null, 2)}
          </pre>
        </section>
      )}
    </div>
  );
}
