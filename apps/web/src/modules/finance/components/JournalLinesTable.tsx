import { Badge } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { entryTotals } from "@/modules/finance/types";
import type { JournalLine } from "@/modules/finance/types";
import { AccountingTable } from "@/modules/finance/components/AccountingTable";
import type { AccountingColumn } from "@/modules/finance/components/AccountingTable";

type Row = JournalLine & { total?: boolean; idx: number };

export function BalanceBadge({ balanced }: { balanced: boolean }) {
  return <Badge tone={balanced ? "success" : "danger"}>{balanced ? "Seimbang ✓" : "Tidak seimbang"}</Badge>;
}

/** Tabel Debit/Kredit baris jurnal dengan baris total di bawah (bergaris atas). */
export function JournalLinesTable({
  lines,
  caption = "Baris jurnal",
  minWidth = "30rem",
}: {
  lines: JournalLine[] | null | undefined;
  caption?: string;
  minWidth?: string;
}) {
  const list = lines ?? [];
  const totals = entryTotals(list);
  const rows: Row[] = list.map((l, idx) => ({ ...l, idx }));
  if (list.length > 0) {
    rows.push({ accountId: 0, accountCode: "", accountName: "Total", debit: totals.debit, credit: totals.credit, total: true, idx: -1 });
  }
  const columns: AccountingColumn<Row>[] = [
    {
      id: "account",
      header: "Akun",
      render: (l) =>
        l.total ? (
          l.accountName
        ) : (
          <span>
            <span className="font-mono text-xs">{l.accountCode}</span> {l.accountName}
            {l.description ? <span className="block text-xs text-muted-foreground">{l.description}</span> : null}
          </span>
        ),
    },
    { id: "debit", header: "Debit", align: "right", width: "9.5rem", render: (l) => <Money accounting value={l.debit} /> },
    { id: "credit", header: "Kredit", align: "right", width: "9.5rem", render: (l) => <Money accounting value={l.credit} /> },
  ];
  return (
    <AccountingTable<Row>
      caption={caption}
      columns={columns}
      rows={rows}
      rowKey={(r) => (r.total ? "total" : `${r.idx}-${r.accountId}`)}
      rowKind={(r) => (r.total ? "total" : "line")}
      emptyTitle="Tidak ada baris jurnal"
      minWidth={minWidth}
    />
  );
}
