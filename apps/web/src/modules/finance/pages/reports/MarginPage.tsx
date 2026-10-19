import { Badge, DataTable, SectionCard, SummaryCard } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { formatDate, formatIDR, monthStartWIB, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { AccountingTable } from "@/modules/finance/components/AccountingTable";
import type { AccountingColumn } from "@/modules/finance/components/AccountingTable";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ReportShell } from "@/modules/finance/components/ReportShell";
import { ExportCsvButton, ReportDateField, ReportToolbar } from "@/modules/finance/components/ReportToolbar";
import { formatPercent } from "@/modules/finance/types";
import type { BranchMargin, ProductMarginRow } from "@/modules/finance/types";

export default function MarginPage() {
  const { draft, patch, applied, apply, version } = useDraftFilters({
    from: monthStartWIB(),
    to: todayWIB(),
    branch: "",
  });
  const { data, loading, error, reload } = useAsyncData(
    () =>
      Promise.all([
        financeReportsService.margin(applied.from, applied.to, applied.branch),
        financeReportsService.marginByProduct(applied.from, applied.to, undefined, applied.branch),
      ]).then(([total, breakdown]) => ({ total, breakdown: breakdown ?? [] })),
    [applied, version],
  );
  const report = data?.total ?? null;
  const rows = data?.breakdown ?? [];

  const columns: AccountingColumn<ProductMarginRow>[] = [
    { id: "code", header: "Kode", width: "9rem", render: (r) => r.productCode || "-" },
    { id: "name", header: "Produk", render: (r) => r.productName || "-" },
    { id: "revenue", header: "Pendapatan", align: "right", width: "11rem", render: (r) => <Money accounting value={r.revenue} /> },
    { id: "cogs", header: "HPP", align: "right", width: "11rem", render: (r) => <Money accounting value={r.cogs} /> },
    { id: "gross", header: "Laba kotor", align: "right", width: "11rem", render: (r) => <Money accounting value={r.gross} /> },
    {
      id: "percent",
      header: "Marjin",
      align: "right",
      width: "7rem",
      render: (r) => <Badge tone={r.gross >= 0 ? "success" : "danger"}>{formatPercent(r.percent)}</Badge>,
    },
  ];

  return (
    <ReportShell title="Margin Produk" description="Laba kotor dari buku, total dan per produk. Hanya baca.">
      <ReportToolbar
        onApply={apply}
        busy={loading}
        right={
          <ExportCsvButton
            filename={`marjin-${applied.from}_${applied.to}.csv`}
            getRows={() =>
              report
                ? [
                    ["Kode", "Produk", "Pendapatan", "HPP", "Laba kotor", "Marjin (%)"],
                    ...rows.map((r) => [r.productCode, r.productName, r.revenue, r.cogs, r.gross, r.percent]),
                  ]
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
          <SummaryCard label="Marjin" value={formatPercent(report.percent)} tone={report.gross >= 0 ? "success" : "danger"} />
        </div>
      )}

      {(report?.byBranch?.length ?? 0) > 1 && (
        <SectionCard flush title="Marjin per cabang" description="Pecahan per cabang; jumlahnya sama dengan total di atas.">
          <DataTable<BranchMargin>
            caption="Marjin per cabang"
            rows={report?.byBranch ?? []}
            rowKey={(b) => b.branchId}
            emptyTitle="Tidak ada"
            columns={[
              { header: "Cabang", render: (b) => b.branchName || `#${b.branchId}` },
              { header: "Pendapatan", align: "right", render: (b) => <Money accounting value={b.revenue} /> },
              { header: "HPP", align: "right", render: (b) => <Money accounting value={b.cogs} /> },
              { header: "Laba kotor", align: "right", render: (b) => <Money accounting value={b.gross} /> },
              { header: "Marjin", align: "right", render: (b) => <Badge tone={b.gross >= 0 ? "success" : "danger"}>{formatPercent(b.percent)}</Badge> },
            ]}
          />
        </SectionCard>
      )}

      <SectionCard
        flush
        title="Marjin per produk"
        description={`${formatDate(applied.from)} – ${formatDate(applied.to)}. Dari buku: pendapatan dikurangi retur, HPP dikurangi barang masuk kembali. Terurut laba kotor terbesar.`}
      >
        <AccountingTable<ProductMarginRow>
          caption="Marjin per produk"
          columns={columns}
          rows={rows}
          rowKey={(r) => `${r.productId}-${r.variantId}`}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Belum ada penjualan terbukukan"
          emptyDescription="Marjin per produk muncul setelah surat jalan diposting ke jurnal."
        />
      </SectionCard>
    </ReportShell>
  );
}
