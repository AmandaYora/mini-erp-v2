import { useMemo, useState } from "react";
import {
  CardSkeleton,
  DataTable,
  DateInput,
  ErrorState,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  SummaryCard,
} from "@/shared/components/ui";
import { Money, Page, Qty, Toolbar } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatIDR, monthStartWIB, todayWIB } from "@/shared/lib/format";
import { reportingService } from "@/modules/reporting/services/reporting.service";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { TrendChart } from "@/modules/reporting/components/TrendChart";
import { paginate, sumTrend, trendRangeError } from "@/modules/reporting/lib/trend";
import type { InventoryPosition, TrendPoint } from "@/modules/reporting/types";

const TREND_LIMIT = 31;
const INVENTORY_LIMIT = 25;

export default function ReportingPage() {
  const [from, setFrom] = useState(monthStartWIB());
  const [to, setTo] = useState(todayWIB());
  const [trendPage, setTrendPage] = useState(1);
  const [invPage, setInvPage] = useState(1);
  const rangeError = trendRangeError(from, to);

  const trend = useAsyncData(
    () => (rangeError ? Promise.resolve([] as TrendPoint[]) : reportingService.salesTrend(from, to)),
    [from, to],
  );
  const inventory = useAsyncData(() => reportingService.inventory(), []);

  const points = useMemo(() => trend.data ?? [], [trend.data]);
  const totals = useMemo(() => sumTrend(points), [points]);
  const positions = useMemo(() => inventory.data?.positions ?? [], [inventory.data]);
  const today = todayWIB();
  const hasSales = points.some((p) => p.revenue !== 0 || p.expense !== 0 || p.profit !== 0);

  return (
    <Page>
      <PageHeader title="Tren & Stok" description="Tren penjualan harian dan nilai persediaan cabang aktif." />

      <Toolbar
        primary={
          <>
            <FormField label="Dari tanggal">
              <DateInput
                aria-label="Dari tanggal"
                value={from}
                max={today}
                onChange={(e) => {
                  setFrom(e.target.value);
                  setTrendPage(1);
                }}
              />
            </FormField>
            <FormField label="Sampai tanggal" helperText="Rentang maksimal 366 hari.">
              <DateInput
                aria-label="Sampai tanggal"
                value={to}
                max={today}
                onChange={(e) => {
                  setTo(e.target.value);
                  setTrendPage(1);
                }}
              />
            </FormField>
          </>
        }
      />

      {rangeError ? (
        <Notice tone="warning" title="Rentang tanggal tidak valid">
          {rangeError}
        </Notice>
      ) : trend.error ? (
        <ErrorState title="Gagal memuat tren penjualan" message={trend.error} onRetry={trend.reload} />
      ) : trend.loading && !trend.data ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {Array.from({ length: 3 }, (_, i) => (
            <CardSkeleton key={i} />
          ))}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <SummaryCard label="Total omzet" value={formatIDR(totals.revenue)} />
            <SummaryCard label="Total beban" value={formatIDR(totals.expense)} tone="warning" />
            <SummaryCard label="Total laba" value={formatIDR(totals.profit)} tone={totals.profit < 0 ? "danger" : "success"} />
          </div>

          <SectionCard title="Tren penjualan" description={`Periode ${formatDate(from)} – ${formatDate(to)}.`}>
            {hasSales ? (
              <TrendChart
                data={points}
                summary={`Grafik tren omzet dan laba harian ${formatDate(from)} sampai ${formatDate(to)}; total omzet ${formatIDR(totals.revenue)}, total laba ${formatIDR(totals.profit)}.`}
              />
            ) : (
              <EmptyState title="Belum ada penjualan" description="Tidak ada transaksi pada rentang tanggal ini." />
            )}
          </SectionCard>

          <SectionCard title="Rincian harian" description="Angka per hari dari pembukuan." flush>
            <DataTable<TrendPoint>
              caption="Rincian tren penjualan harian"
              rows={paginate(points, trendPage, TREND_LIMIT)}
              rowKey={(r) => r.date}
              emptyTitle="Belum ada penjualan"
              emptyDescription="Tidak ada transaksi pada rentang tanggal ini."
              columns={[
                { header: "Tanggal", render: (r) => formatDate(r.date) },
                { header: "Omzet", align: "right", render: (r) => <Money value={r.revenue} /> },
                { header: "Beban", align: "right", render: (r) => <Money value={r.expense} /> },
                { header: "Laba", align: "right", render: (r) => <Money value={r.profit} accounting /> },
              ]}
            />
            {points.length > TREND_LIMIT && (
              <Pagination page={trendPage} limit={TREND_LIMIT} total={points.length} onPageChange={setTrendPage} />
            )}
          </SectionCard>
        </>
      )}

      <SectionCard title="Persediaan" description="Posisi stok dinilai dengan moving average." flush>
        {inventory.error ? (
          <ErrorState title="Gagal memuat persediaan" message={inventory.error} onRetry={inventory.reload} className="m-4" />
        ) : inventory.loading && !inventory.data ? (
          <CardSkeleton className="m-4" />
        ) : inventory.data ? (
          <>
            <div className="border-b border-border p-4">
              <div className="max-w-sm">
                <SummaryCard label="Total nilai persediaan" value={formatIDR(inventory.data.total)} />
              </div>
            </div>
            <DataTable<InventoryPosition>
              caption="Posisi stok bernilai"
              rows={paginate(positions, invPage, INVENTORY_LIMIT)}
              rowKey={(r) => `${r.productId}-${r.variantId}`}
              emptyTitle="Belum ada persediaan"
              emptyDescription="Belum ada posisi stok bernilai di cabang ini."
              columns={[
                { header: "Kode", render: (r) => <span className="font-mono">{r.productCode}</span> },
                { header: "Produk", render: (r) => r.productName },
                { header: "Qty", align: "right", render: (r) => <Qty value={r.qty} /> },
                { header: "HPP rata-rata", align: "right", render: (r) => <Money value={r.avgCost} /> },
                { header: "Nilai", align: "right", render: (r) => <Money value={r.value} /> },
              ]}
            />
            {positions.length > INVENTORY_LIMIT && (
              <Pagination page={invPage} limit={INVENTORY_LIMIT} total={positions.length} onPageChange={setInvPage} />
            )}
          </>
        ) : null}
      </SectionCard>
    </Page>
  );
}
