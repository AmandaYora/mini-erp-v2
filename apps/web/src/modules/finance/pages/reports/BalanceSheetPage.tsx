import { Badge, SectionCard } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { AccountingTable } from "@/modules/finance/components/AccountingTable";
import type { AccountingColumn, AccountingRowKind } from "@/modules/finance/components/AccountingTable";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ReportShell } from "@/modules/finance/components/ReportShell";
import { ExportCsvButton, ReportDateField, ReportToolbar } from "@/modules/finance/components/ReportToolbar";
import type { BalanceSheetReport, TrialRow } from "@/modules/finance/types";

interface BSRow {
  key: string;
  code: string;
  label: string;
  amount: number | null;
  kind: AccountingRowKind;
}

function sum(rows: TrialRow[]): number {
  return rows.reduce((s, r) => s + r.debit, 0);
}

/** Susun baris neraca: golongan → akun → subtotal, lalu total aset dan total kewajiban + modal. */
function buildRows(report: BalanceSheetReport): BSRow[] {
  // Backend mengirim null (bukan []) untuk golongan tanpa akun saat pembukuan kosong.
  const assets = report.assets ?? [];
  const liabilities = report.liabilities ?? [];
  const equity = report.equity ?? [];
  const out: BSRow[] = [];
  const group = (key: string, title: string, rows: TrialRow[], total: number) => {
    out.push({ key: `g-${key}`, code: "", label: title, amount: null, kind: "group" });
    for (const r of rows) out.push({ key: `${key}-${r.code}-${r.name}`, code: r.code, label: r.name, amount: r.debit, kind: "line" });
    out.push({ key: `s-${key}`, code: "", label: `Total ${title.toLowerCase()}`, amount: total, kind: "subtotal" });
  };
  group("aset", "Aset", assets, report.totalAssets);
  group("kewajiban", "Kewajiban", liabilities, sum(liabilities));
  group("modal", "Modal", equity, sum(equity));
  out.push({ key: "t-aset", code: "", label: "Total aset", amount: report.totalAssets, kind: "total" });
  out.push({ key: "t-lm", code: "", label: "Total kewajiban + modal", amount: report.totalLiabilitiesEquity, kind: "total" });
  return out;
}

export default function BalanceSheetPage() {
  const { draft, patch, applied, apply, version } = useDraftFilters({ date: todayWIB(), branch: "" });
  const { data: report, loading, error, reload } = useAsyncData(
    () => financeReportsService.balanceSheet(applied.date, applied.branch),
    [applied, version],
  );
  const rows = report ? buildRows(report) : [];

  const columns: AccountingColumn<BSRow>[] = [
    { id: "code", header: "Kode", width: "7rem", render: (r) => (r.kind === "group" ? <span>{r.label}</span> : r.code || "") },
    { id: "label", header: "Akun", render: (r) => r.label },
    {
      id: "amount",
      header: "Saldo",
      align: "right",
      width: "13rem",
      render: (r) => (r.amount === null ? "" : <Money accounting value={r.amount} />),
    },
  ];

  return (
    <ReportShell title="Neraca" description="Posisi aset, kewajiban, dan modal pada satu tanggal. Hanya baca.">
      <ReportToolbar
        onApply={apply}
        busy={loading}
        right={
          <ExportCsvButton
            filename={`neraca-${applied.date}.csv`}
            getRows={() =>
              report
                ? [["Kode", "Akun", "Saldo"], ...rows.filter((r) => r.kind !== "group").map((r) => [r.code, r.label, r.amount])]
                : null
            }
          />
        }
      >
        <ReportDateField label="Per tanggal" value={draft.date} onChange={(date) => patch({ date })} />
        <BranchScopeSelect value={draft.branch} onChange={(branch) => patch({ branch })} />
      </ReportToolbar>

      {report && !report.balanced && (
        <Notice tone="danger" title="Neraca tidak seimbang">
          Total aset tidak sama dengan total kewajiban + modal. Periksa halaman Integritas.
        </Notice>
      )}

      <SectionCard
        flush
        title="Neraca"
        description={report ? `Posisi per ${formatDate(report.date)}` : undefined}
        actions={
          report ? (
            <Badge tone={report.balanced ? "success" : "danger"}>{report.balanced ? "Seimbang ✓" : "Tidak seimbang"}</Badge>
          ) : undefined
        }
      >
        <AccountingTable<BSRow>
          caption="Neraca"
          columns={columns}
          rows={rows}
          rowKey={(r) => r.key}
          rowKind={(r) => r.kind}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Belum ada data"
        />
      </SectionCard>
    </ReportShell>
  );
}
