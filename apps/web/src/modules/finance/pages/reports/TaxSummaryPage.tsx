import { Link } from "react-router-dom";
import { Button, SectionCard, SummaryCard } from "@/shared/components/ui";
import { CardSkeleton } from "@/shared/components/ui";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { formatIDR, formatNumber } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { defaultYearMonth } from "@/modules/finance/lib/report-periods";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ReportShell } from "@/modules/finance/components/ReportShell";
import { ExportCsvButton, ReportToolbar, YearMonthFields } from "@/modules/finance/components/ReportToolbar";
import { periodLabel } from "@/modules/finance/types";

export default function TaxSummaryPage() {
  const ym = defaultYearMonth();
  const { draft, patch, applied, apply, version } = useDraftFilters({ year: ym.year, month: ym.month, branch: "" });
  const { data: summary, loading, error, reload } = useAsyncData(
    () => financeReportsService.taxSummary(applied.year, applied.month, applied.branch),
    [applied, version],
  );
  const payable = summary?.payable ?? 0;

  return (
    <ReportShell title="Pajak" description="PPN keluaran dikurangi PPN masukan pada satu bulan. Hanya baca.">
      <ReportToolbar
        onApply={apply}
        busy={loading}
        right={
          <ExportCsvButton
            filename={`pajak-${applied.year}-${String(applied.month).padStart(2, "0")}.csv`}
            getRows={() =>
              summary
                ? [
                    ["Keterangan", "Jumlah"],
                    ["DPP penjualan", summary.salesDpp ?? 0],
                    ["PPN Keluaran", summary.ppnOut],
                    ["DPP pembelian", summary.purchaseDpp ?? 0],
                    ["PPN Masukan", summary.ppnIn],
                    [payable >= 0 ? "Kurang bayar" : "Lebih bayar", Math.abs(payable)],
                  ]
                : null
            }
          />
        }
      >
        <YearMonthFields year={draft.year} month={draft.month} onYear={(year) => patch({ year })} onMonth={(month) => patch({ month })} />
        <BranchScopeSelect value={draft.branch} onChange={(branch) => patch({ branch })} />
      </ReportToolbar>

      <SectionCard
        title={`Pajak — ${periodLabel(applied.year, applied.month)}`}
        description="PPN keluaran dikurangi PPN masukan."
        actions={
          <Button asChild variant="secondary" size="sm">
            <Link to={ROUTE_PATHS.taxDetail}>Rincian per faktur</Link>
          </Button>
        }
      >
        {error ? (
          <ErrorState message={error} onRetry={reload} />
        ) : loading && !summary ? (
          <div className="grid gap-4 sm:grid-cols-3">
            <CardSkeleton />
            <CardSkeleton />
            <CardSkeleton />
          </div>
        ) : summary ? (
          <div className="flex flex-col gap-4">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <SummaryCard label="DPP penjualan" value={formatIDR(summary.salesDpp ?? 0)} tone="brand" />
              <SummaryCard label="PPN keluaran" value={formatIDR(summary.ppnOut)} tone="brand" />
              <SummaryCard label="DPP pembelian" value={formatIDR(summary.purchaseDpp ?? 0)} tone="brand" />
              <SummaryCard label="PPN masukan" value={formatIDR(summary.ppnIn)} tone="brand" />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <SummaryCard
                label={payable >= 0 ? "Kurang bayar" : "Lebih bayar"}
                value={formatIDR(Math.abs(payable))}
                tone={payable > 0 ? "danger" : "success"}
              />
              <SummaryCard
                label="Kena pajak tanpa faktur"
                value={formatNumber(summary.exceptionsMeta?.total ?? 0)}
                tone={(summary.exceptionsMeta?.total ?? 0) > 0 ? "warning" : "success"}
                hint="Lengkapi nomor faktur di pesanan terkait"
              />
            </div>
          </div>
        ) : null}
      </SectionCard>
    </ReportShell>
  );
}
