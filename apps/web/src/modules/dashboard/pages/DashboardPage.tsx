import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { ChartCandlestick, ChartColumn, ChartLine, Info } from "lucide-react";
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
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { dashboardService } from "@/modules/dashboard/services/dashboard.service";
import { ActionCard } from "@/modules/dashboard/components/ActionCard";
import { OrderStatusCard } from "@/modules/dashboard/components/OrderStatusCard";
import { TopProductsCard } from "@/modules/dashboard/components/TopProductsCard";
import { SalesTrendChart } from "@/modules/dashboard/components/SalesTrendChart";
import { greetingTitle, longDateWIB, overdueDays } from "@/modules/dashboard/lib/dashboard";
import {
  TREND_CHART_TYPES,
  TREND_INTERVALS,
  TREND_PREFS_BASE,
  bucketTrend,
  isChartAllowed,
  isIntervalAllowed,
  normalizeTrendView,
  readTrendPrefs,
  trendWindow,
  writeTrendPrefs,
} from "@/modules/dashboard/lib/trend";
import type { TrendChartType, TrendInterval, TrendPrefs } from "@/modules/dashboard/lib/trend";
import { userKey } from "@/shared/stores/ui-prefs.store";
import { WHATSAPP_LABEL, whatsappState } from "@/modules/dashboard/lib/whatsapp";
import type { DashboardSummary, TrendRangeKey } from "@/modules/dashboard/types";
import { TREND_RANGES } from "@/modules/dashboard/types";

type Source = "ledger" | "order";

// Perbedaan semantik pembukuan vs order adalah keputusan domain — tetap dijelaskan di UI.
const SOURCE_NOTE: Record<Source, string> = {
  ledger: "Pembukuan: dari jurnal yang sudah diposting (omzet, laba, kas, dan piutang buku).",
  order:
    "Order: dihitung dari dokumen order, bukan jurnal. Penjualan = order selesai bersih retur; pembelian = nilai order beli bertanggal periode itu (selain yang dibatalkan); piutang dan hutang dihitung per order, tidak pernah di bawah nol.",
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
        hint={`${formatNumber(ops.purchaseOrdersToday)} order beli · Bulan ini ${formatIDR(ops.purchasesMonth)}`}
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
  const navigate = useNavigate();
  const [source, setSource] = useState<Source>("ledger");
  // Rentang, periode (harian/mingguan/bulanan), dan bentuk grafik tren diingat per pengguna.
  const [trendPrefs, setTrendPrefsState] = useState<TrendPrefs>(() => readTrendPrefs(userKey(TREND_PREFS_BASE)));
  const { range: trendRange, interval: trendInterval, chart: trendChart } = trendPrefs;
  const setTrendPrefs = (next: Partial<TrendPrefs>) => {
    const range = next.range ?? trendRange;
    const prefs = { range, ...normalizeTrendView(range, next.interval ?? trendInterval, next.chart ?? trendChart) };
    writeTrendPrefs(userKey(TREND_PREFS_BASE), prefs);
    setTrendPrefsState(prefs);
  };
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
      const { from, to } = trendWindow(trendRange, trendInterval, todayWIB());
      return dashboardService.salesTrend(from, to);
    },
    [trendRange, trendInterval, canTrend],
  );
  // Kartu status WhatsApp hanya untuk peran yang boleh melihat asisten (DB-01).
  const canAssistant = useAuthStore((s) => s.permissions.includes("assistant.view"));
  const whatsapp = useAsyncData(() => (canAssistant ? dashboardService.whatsappChannel() : Promise.resolve(null)), [canAssistant]);
  // SJ Langsung yang belum diorder (ADR-0022): hanya untuk peran yang boleh melihat surat jalan;
  // tampil hanya bila ada (N > 0), tautan ke daftar order (daftar tunggunya ada di sana).
  const canDelivery = useAuthStore((s) => s.can("delivery.view"));
  const openDirect = useAsyncData(() => (canDelivery ? dashboardService.openDirect() : Promise.resolve(null)), [canDelivery]);
  const directCount = openDirect.data && !openDirect.error ? openDirect.data.count : 0;
  const directAgeDays = directCount > 0 && openDirect.data?.oldestDate ? Math.max(0, overdueDays(openDirect.data.oldestDate, today)) : 0;
  const critical = useAsyncData(() => dashboardService.criticalStock(), []);
  const priority = useAsyncData(() => dashboardService.priorityOrders(), []);
  const sellers = useAsyncData(() => dashboardService.topSellers(), []);
  const margin = useAsyncData(() => dashboardService.topMargin(), []);
  const orderStatus = useAsyncData(() => dashboardService.orderStatus(), []);

  const trendPoints = trend.data ?? [];
  const trendBuckets = useMemo(() => bucketTrend(trend.data ?? [], trendInterval), [trend.data, trendInterval]);
  const trendTotal = trendPoints.reduce((n, p) => n + p.revenue, 0);
  const trendLabel = TREND_RANGES.find((r) => r.value === trendRange)?.label ?? "30 hari";
  const intervalLabel = (TREND_INTERVALS.find((i) => i.value === trendInterval)?.label ?? "Harian").toLowerCase();
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

      {(data || directCount > 0) && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4" data-testid="dashboard-status-cards">
          {data && (
            <>
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
            </>
          )}
          {directCount > 0 && (
            <SummaryCard
              label="SJ Langsung belum diorder"
              value={formatNumber(directCount)}
              tone="warning"
              hint={directAgeDays > 0 ? `Tertua ${formatNumber(directAgeDays)} hari · buat ordernya di Order Jual` : "Buat ordernya di Order Jual"}
              onClick={() => navigate(ROUTE_PATHS.salesOrders)}
            />
          )}
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

      <div className="grid grid-cols-1 items-stretch gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <SectionCard
          title="Tren penjualan"
          description={`Omzet ${intervalLabel} cabang aktif, ${trendLabel} terakhir.`}
          actions={
            canTrend ? (
              <ToggleGroup
                type="single"
                variant="outline"
                size="sm"
                value={trendRange}
                onValueChange={(v) => v && setTrendPrefs({ range: v as TrendRangeKey })}
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
              <div className="mb-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
                <p className="tabular text-sm text-muted-foreground">
                  {`Total ${trendLabel}: `}
                  <span className="font-semibold text-foreground">{formatIDR(trendTotal)}</span>
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <ToggleGroup
                    type="single"
                    variant="outline"
                    size="sm"
                    value={trendInterval}
                    onValueChange={(v) => v && setTrendPrefs({ interval: v as TrendInterval })}
                    aria-label="Periode tren"
                  >
                    {TREND_INTERVALS.map((i) => (
                      <ToggleGroupItem key={i.value} value={i.value} disabled={!isIntervalAllowed(trendRange, i.value)}>
                        {i.label}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                  <ToggleGroup
                    type="single"
                    variant="outline"
                    size="sm"
                    value={trendChart}
                    onValueChange={(v) => v && setTrendPrefs({ chart: v as TrendChartType })}
                    aria-label="Bentuk grafik"
                  >
                    {TREND_CHART_TYPES.map((c) => {
                      const Icon = c.value === "line" ? ChartLine : c.value === "candle" ? ChartCandlestick : ChartColumn;
                      const allowed = isChartAllowed(trendInterval, c.value);
                      return (
                        <ToggleGroupItem
                          key={c.value}
                          value={c.value}
                          disabled={!allowed}
                          aria-label={c.label}
                          title={allowed ? c.label : `${c.label}: pilih Mingguan atau Bulanan`}
                        >
                          <Icon aria-hidden />
                        </ToggleGroupItem>
                      );
                    })}
                  </ToggleGroup>
                </div>
              </div>
              <SalesTrendChart
                data={trendBuckets}
                interval={trendInterval}
                chart={trendChart}
                summary={`Grafik ${TREND_CHART_TYPES.find((c) => c.value === trendChart)?.label.toLowerCase()} omzet ${intervalLabel} ${trendLabel} terakhir, total ${formatIDR(trendTotal)}.`}
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

      <div className="grid grid-cols-1 items-stretch gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
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
