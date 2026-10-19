import { useState } from "react";
import { Info } from "lucide-react";
import {
  CardSkeleton,
  ErrorState,
  PageHeader,
  SectionCard,
  SummaryCard,
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
  ToggleGroup,
  ToggleGroupItem,
} from "@/shared/components/ui";
import { Page } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { formatIDR, formatNumber, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { dashboardService } from "@/modules/dashboard/services/dashboard.service";
import { ActionCard } from "@/modules/dashboard/components/ActionCard";
import { OrderStatusCard } from "@/modules/dashboard/components/OrderStatusCard";
import { TopProductsCard } from "@/modules/dashboard/components/TopProductsCard";
import { TrendBarChart } from "@/modules/dashboard/components/TrendBarChart";
import { addDays, greetingTitle, longDateWIB, overdueDays } from "@/modules/dashboard/lib/dashboard";
import { WHATSAPP_LABEL, whatsappState } from "@/modules/dashboard/lib/whatsapp";
import type { DashboardSummary, TrendRangeKey } from "@/modules/dashboard/types";
import { TREND_RANGES } from "@/modules/dashboard/types";

type Source = "ledger" | "order";

// Perbedaan semantik pembukuan vs order adalah keputusan domain — tetap dijelaskan di UI.
const SOURCE_NOTE: Record<Source, string> = {
  ledger: "Pembukuan: dari jurnal yang sudah diposting (omzet, laba, kas, dan piutang buku).",
  order:
    "Order: dihitung dari dokumen order, bukan jurnal. Penjualan = order selesai bersih retur; piutang dan hutang dihitung per order, tidak pernah di bawah nol.",
};

function KpiRow({ summary, source, overdueOrders }: { summary: DashboardSummary; source: Source; overdueOrders: number | null }) {
  const overdueHint =
    overdueOrders === null ? undefined : overdueOrders > 0 ? `${formatNumber(overdueOrders)} order lewat tempo` : "Tidak ada yang lewat tempo";
  const ops = summary.operations;
  if (source === "ledger") {
    return (
      <>
        <SummaryCard
          label="Omzet hari ini"
          value={formatIDR(summary.today.revenue)}
          hint={`Bulan ini ${formatIDR(summary.monthToDate.revenue)}`}
        />
        <SummaryCard
          label="Laba hari ini"
          value={formatIDR(summary.today.profit)}
          tone={summary.today.profit < 0 ? "danger" : "success"}
          hint={`Beban ${formatIDR(summary.today.expense)} · Bulan ini ${formatIDR(summary.monthToDate.profit)}`}
        />
        <SummaryCard label="Kas" value={formatIDR(summary.balances.cash)} hint={`Hutang ${formatIDR(summary.balances.payable)}`} />
        <SummaryCard
          label="Piutang"
          value={formatIDR(summary.balances.receivable)}
          tone={overdueOrders ? "warning" : "brand"}
          hint={overdueHint}
        />
      </>
    );
  }
  return (
    <>
      <SummaryCard
        label="Penjualan hari ini"
        value={formatIDR(ops.salesToday)}
        hint={`${formatNumber(ops.salesOrdersToday)} order selesai · Bulan ini ${formatIDR(ops.salesMonth)}`}
      />
      <SummaryCard
        label="Pembelian hari ini"
        value={formatIDR(ops.purchasesToday)}
        hint={`${formatNumber(ops.ordersToday)} order · Bulan ini ${formatIDR(ops.purchasesMonth)}`}
      />
      <SummaryCard label="Hutang berjalan" value={formatIDR(ops.outstandingPayable)} tone="warning" />
      <SummaryCard
        label="Piutang berjalan"
        value={formatIDR(ops.outstandingReceivable)}
        tone={overdueOrders ? "warning" : "brand"}
        hint={overdueHint}
      />
    </>
  );
}

export default function DashboardPage() {
  const [source, setSource] = useState<Source>("ledger");
  const [trendRange, setTrendRange] = useState<TrendRangeKey>("30d");
  const user = useAuthStore((s) => s.user);
  const branch = useAuthStore((s) => s.branch);
  // Tren memakai /reporting/sales-trend (izin reporting.view), bukan dashboard.view:
  // peran tanpa izin itu (mis. kasir) tidak boleh melihat widget rusak.
  const canTrend = useAuthStore((s) => s.permissions.includes("reporting.view"));
  const today = todayWIB();

  const summary = useAsyncData(() => dashboardService.summary(), []);
  const trend = useAsyncData(
    () => {
      if (!canTrend) return Promise.resolve([]);
      const days = TREND_RANGES.find((r) => r.value === trendRange)?.days ?? 30;
      const to = todayWIB();
      return dashboardService.salesTrend(addDays(to, -(days - 1)), to);
    },
    [trendRange, canTrend],
  );
  // Kartu status WhatsApp hanya untuk peran yang boleh melihat asisten (DB-01).
  const canAssistant = useAuthStore((s) => s.permissions.includes("assistant.view"));
  const whatsapp = useAsyncData(() => (canAssistant ? dashboardService.whatsappChannel() : Promise.resolve(null)), [canAssistant]);
  const critical = useAsyncData(() => dashboardService.criticalStock(), []);
  const priority = useAsyncData(() => dashboardService.priorityOrders(), []);
  const sellers = useAsyncData(() => dashboardService.topSellers(), []);
  const margin = useAsyncData(() => dashboardService.topMargin(), []);
  const orderStatus = useAsyncData(() => dashboardService.orderStatus(), []);

  const trendPoints = trend.data ?? [];
  const trendTotal = trendPoints.reduce((n, p) => n + p.revenue, 0);
  const trendLabel = TREND_RANGES.find((r) => r.value === trendRange)?.label ?? "30 hari";
  const overdueOrders =
    priority.data && !priority.error ? priority.data.filter((o) => overdueDays(o.dueDate, today) > 0).length : null;
  const data = summary.data;

  return (
    <Page>
      <PageHeader
        title={greetingTitle(user?.fullName)}
        subtitle={[branch?.name, longDateWIB(today)].filter(Boolean).join(" · ")}
      />

      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <div className="flex items-center gap-2">
            <span id="source-label" className="text-sm font-medium text-foreground">
              Sumber angka
            </span>
            <ToggleGroup
              type="single"
              variant="outline"
              size="sm"
              value={source}
              onValueChange={(v) => v && setSource(v as Source)}
              aria-labelledby="source-label"
            >
              <ToggleGroupItem value="ledger">Pembukuan</ToggleGroupItem>
              <ToggleGroupItem value="order">Order</ToggleGroupItem>
            </ToggleGroup>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    aria-label="Penjelasan sumber angka"
                    className="rounded-full p-1 text-muted-foreground hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
                  >
                    <Info size={16} aria-hidden />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="bottom" className="max-w-xs">
                  <p>{SOURCE_NOTE.ledger}</p>
                  <p className="mt-1.5">{SOURCE_NOTE.order}</p>
                  <p className="mt-1.5">Angka keduanya dapat berbeda karena dasar hitungnya berbeda.</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          {data && (
            <p className="tabular text-sm text-muted-foreground sm:ml-auto">
              {`Nilai persediaan ${formatIDR(data.inventoryValue)} (moving average)`}
            </p>
          )}
        </div>
        <p className="text-xs text-muted-foreground">{SOURCE_NOTE[source]}</p>
      </div>

      {data && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4" data-testid="dashboard-status-cards">
          <SummaryCard
            label="Produk dipantau"
            value={formatNumber(data.operations.trackedCount)}
            tone="brand"
            hint={`${formatNumber(data.operations.criticalCount)} di bawah stok minimum`}
          />
          <SummaryCard
            label="Basis biaya margin"
            value={data.operations.costBasis === "estimated" ? "Estimasi" : "Lengkap"}
            tone={data.operations.costBasis === "estimated" ? "warning" : "success"}
            hint={
              (data.operations.missingCostCount ?? 0) > 0
                ? `${formatNumber(data.operations.missingCostCount ?? 0)} produk memakai HPP estimasi — margin indikatif`
                : "Semua HPP berdasar biaya riil"
            }
          />
          {canAssistant && (
            <SummaryCard
              label="WhatsApp asisten"
              value={whatsapp.error ? "Tidak diketahui" : WHATSAPP_LABEL[whatsappState(whatsapp.data)]}
              tone={whatsappState(whatsapp.data) === "connected" ? "success" : whatsappState(whatsapp.data) === "connecting" ? "warning" : "danger"}
              hint={whatsapp.data?.phone ? whatsapp.data.phone : whatsapp.error ?? "Hubungkan di halaman Asisten"}
            />
          )}
        </div>
      )}

      {summary.error ? (
        <ErrorState title="Gagal memuat dashboard" message={summary.error} onRetry={summary.reload} />
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {data ? (
            <KpiRow summary={data} source={source} overdueOrders={overdueOrders} />
          ) : (
            Array.from({ length: 4 }, (_, i) => <CardSkeleton key={i} />)
          )}
        </div>
      )}

      <div className="grid items-stretch gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <SectionCard
          title="Tren penjualan"
          description={`Omzet harian cabang aktif, ${trendLabel} terakhir.`}
          actions={
            canTrend ? (
              <ToggleGroup
                type="single"
                variant="outline"
                size="sm"
                value={trendRange}
                onValueChange={(v) => v && setTrendRange(v as TrendRangeKey)}
                aria-label="Rentang tren"
              >
                {TREND_RANGES.map((r) => (
                  <ToggleGroupItem key={r.value} value={r.value}>
                    {r.label}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            ) : undefined
          }
        >
          {!canTrend ? (
            <p className="py-6 text-center text-sm text-muted-foreground">
              Grafik tren penjualan hanya untuk peran dengan izin Laporan.
            </p>
          ) : trend.error ? (
            <ErrorState message={trend.error} onRetry={trend.reload} />
          ) : trend.loading && !trend.data ? (
            <CardSkeleton className="h-64 border-0 p-0" />
          ) : trendPoints.length === 0 || trendTotal === 0 ? (
            <EmptyState title="Belum ada penjualan" description={`Tidak ada omzet dalam ${trendLabel} terakhir.`} />
          ) : (
            <div className={trend.loading ? "opacity-60 transition-opacity" : undefined}>
              <p className="tabular mb-2 text-sm text-muted-foreground">
                {`Total ${trendLabel}: `}
                <span className="font-semibold text-foreground">{formatIDR(trendTotal)}</span>
              </p>
              <TrendBarChart
                data={trendPoints}
                range={trendRange}
                summary={`Grafik area omzet harian ${trendLabel} terakhir, total ${formatIDR(trendTotal)}.`}
              />
            </div>
          )}
        </SectionCard>

        <ActionCard
          orders={priority}
          critical={critical}
          criticalCount={data ? data.operations.criticalCount : null}
          today={today}
          canReporting={canTrend}
        />
      </div>

      <div className="grid items-stretch gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <TopProductsCard sellers={sellers} margin={margin} />
        <OrderStatusCard
          data={orderStatus.data}
          loading={orderStatus.loading}
          error={orderStatus.error}
          onRetry={orderStatus.reload}
        />
      </div>
    </Page>
  );
}
