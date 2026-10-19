import { SectionCard, SummaryCard } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { formatDate, formatIDR, monthStartWIB, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { AccountingTable } from "@/modules/finance/components/AccountingTable";
import type { AccountingColumn } from "@/modules/finance/components/AccountingTable";
import { buildProfitLossRows } from "@/modules/finance/lib/profit-loss";
import type { PLRow } from "@/modules/finance/lib/profit-loss";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ReportShell } from "@/modules/finance/components/ReportShell";
import { ExportCsvButton, ReportDateField, ReportToolbar } from "@/modules/finance/components/ReportToolbar";

export default function ProfitLossPage() {
  const { draft, patch, applied, apply, version } = useDraftFilters({
    from: monthStartWIB(),
    to: todayWIB(),
    branch: "",
  });
  const { data: report, loading, error, reload } = useAsyncData(
    () => financeReportsService.profitLoss(applied.from, applied.to, applied.branch),
    [applied, version],
  );

  const rows: PLRow[] = report ? buildProfitLossRows(report) : [];

  const columns: AccountingColumn<PLRow>[] = [
    { id: "label", header: "Keterangan", render: (r) => (r.indent ? <span className="pl-6">{r.label}</span> : r.label) },
    { id: "amount", header: "Jumlah", align: "right", width: "13rem", render: (r) => <Money accounting value={r.amount} /> },
  ];

  return (
    <ReportShell title="Laba Rugi" description="Pendapatan dikurangi harga pokok dan beban pada rentang tanggal. Hanya baca.">
      <ReportToolbar
        onApply={apply}
        busy={loading}
        right={
          <ExportCsvButton
            filename={`laba-rugi-${applied.from}_${applied.to}.csv`}
            getRows={() =>
              report
                ? [["Keterangan", "Jumlah"], ...rows.map((r) => [r.label, r.amount])]
                : null
            }
          />
        }
      >
        <ReportDateField label="Dari" value={draft.from} onChange={(from) => patch({ from })} />
        <ReportDateField label="Sampai" value={draft.to} onChange={(to) => patch({ to })} />
        <BranchScopeSelect value={draft.branch} onChange={(branch) => patch({ branch })} />
      </ReportToolbar>

      {report && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <SummaryCard label="Pendapatan" value={formatIDR(report.revenue)} tone="brand" />
          <SummaryCard label="HPP" value={formatIDR(report.cogs)} tone="warning" />
          <SummaryCard label="Laba kotor" value={formatIDR(report.gross)} tone={report.gross >= 0 ? "success" : "danger"} />
          <SummaryCard label="Laba bersih" value={formatIDR(report.profit)} tone={report.profit >= 0 ? "success" : "danger"} />
        </div>
      )}

      <SectionCard
        flush
        title="Laba Rugi"
        description={report ? `${formatDate(report.from)} – ${formatDate(report.to)}` : undefined}
      >
        <AccountingTable<PLRow>
          caption="Laporan laba rugi"
          columns={columns}
          rows={rows}
          rowKey={(r) => r.key}
          rowKind={(r) => r.kind}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Belum ada data"
          emptyDescription="Tidak ada pendapatan atau beban pada rentang ini."
        />
      </SectionCard>
    </ReportShell>
  );
}
