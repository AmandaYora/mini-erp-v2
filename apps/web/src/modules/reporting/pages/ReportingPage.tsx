import { useMemo, useState } from "react";
import {
  DataTable,
  DateInput,
  FilterBar,
  FormField,
  PageHeader,
  SectionCard,
  SummaryCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  formatIDR,
  formatNumber,
  monthStartWIB,
  todayWIB,
} from "@/shared/lib/format";
import { reportingService } from "@/modules/reporting/services/reporting.service";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import type {
  InventoryPosition,
  TrendPoint,
} from "@/modules/reporting/types";

/** Jumlah hari inklusif antara dua tanggal YYYY-MM-DD. */
function diffDays(from: string, to: string): number {
  const a = new Date(`${from}T00:00:00Z`).getTime();
  const b = new Date(`${to}T00:00:00Z`).getTime();
  return Math.round((b - a) / 86400000) + 1;
}

export default function ReportingPage() {
  const [from, setFrom] = useState(monthStartWIB());
  const [to, setTo] = useState(todayWIB());
  const {
    data: trendData,
    loading: trendLoading,
    error: trendError,
    reload: reloadTrend,
  } = useAsyncData(
    () => {
      if (!from || !to || from > to || diffDays(from, to) > 366) {
        return Promise.resolve([]);
      }
      return reportingService.salesTrend(from, to);
    },
    [from, to],
  );
  const trend = useMemo(() => trendData ?? [], [trendData]);
  const {
    data: inventory,
    loading: invLoading,
    error: invError,
    reload: reloadInventory,
  } = useAsyncData(() => reportingService.inventory(), []);

  const today = todayWIB();

  const rangeError =
    !from || !to
      ? "Pilih tanggal mulai dan tanggal akhir."
      : from > to
        ? "Tanggal akhir sebelum tanggal awal."
        : diffDays(from, to) > 366
          ? "Rentang maksimal 366 hari."
          : null;

  const totals = useMemo(
    () =>
      trend.reduce(
        (acc, p) => ({
          revenue: acc.revenue + p.revenue,
          expense: acc.expense + p.expense,
          profit: acc.profit + p.profit,
        }),
        { revenue: 0, expense: 0, profit: 0 },
      ),
    [trend],
  );

  return (
    <div>
      <PageHeader
        eyebrow="Laporan"
        title="Tren & Stok"
        description="Tren penjualan harian dan nilai persediaan cabang aktif."
      />

      <FilterBar>
        <FormField label="Dari">
          <DateInput
            value={from}
            max={today}
            onChange={(e) => setFrom(e.target.value)}
          />
        </FormField>
        <FormField label="Sampai" helperText="Rentang maksimal 366 hari.">
          <DateInput
            value={to}
            max={today}
            onChange={(e) => setTo(e.target.value)}
          />
        </FormField>
      </FilterBar>

      <SectionCard
        title="Tren Penjualan"
        description={
          rangeError
            ? "Perbaiki rentang tanggal untuk memuat data."
            : `Periode ${formatDate(from)} – ${formatDate(to)}.`
        }
      >
        {rangeError ? (
          <Notice tone="warning" title="Rentang tanggal tidak valid">
            {rangeError}
          </Notice>
        ) : trendError ? (
          <Notice tone="danger" title="Gagal memuat tren penjualan">
            {trendError}{" "}
            <button
              type="button"
              onClick={() => reloadTrend()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        ) : trendLoading ? (
          <p className="text-muted px-1 py-6 text-sm">Memuat tren…</p>
        ) : (
          <div className="flex flex-col gap-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <SummaryCard
                label="Total Omzet"
                value={formatIDR(totals.revenue)}
              />
              <SummaryCard
                label="Total Beban"
                value={formatIDR(totals.expense)}
              />
              <SummaryCard
                label="Total Laba"
                value={formatIDR(totals.profit)}
                tone="success"
              />
            </div>
            <DataTable<TrendPoint>
              columns={[
                {
                  header: "Tanggal",
                  render: (r) => formatDate(r.date),
                },
                {
                  header: "Omzet",
                  align: "right",
                  render: (r) => formatIDR(r.revenue),
                },
                {
                  header: "Beban",
                  align: "right",
                  render: (r) => formatIDR(r.expense),
                },
                {
                  header: "Laba",
                  align: "right",
                  render: (r) => formatIDR(r.profit),
                },
              ]}
              rows={trend}
              rowKey={(r) => r.date}
              emptyTitle="Belum ada penjualan"
              emptyDescription="Tidak ada transaksi pada rentang tanggal ini."
            />
          </div>
        )}
      </SectionCard>

      <div className="mt-6">
        <SectionCard
          title="Persediaan"
          description="Posisi stok dinilai dengan moving average."
        >
          {invError ? (
            <Notice tone="danger" title="Gagal memuat persediaan">
              {invError}{" "}
              <button
                type="button"
                onClick={() => reloadInventory()}
                className="cursor-pointer font-semibold text-brand hover:underline"
              >
                Coba lagi
              </button>
            </Notice>
          ) : invLoading ? (
            <p className="text-muted px-1 py-6 text-sm">Memuat persediaan…</p>
          ) : inventory ? (
            <div className="flex flex-col gap-4">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                <SummaryCard
                  label="Total Nilai Persediaan"
                  value={formatIDR(inventory.total)}
                />
              </div>
              <DataTable<InventoryPosition>
                columns={[
                  {
                    header: "Kode",
                    render: (r) => (
                      <span className="font-medium">{r.productCode}</span>
                    ),
                  },
                  { header: "Produk", render: (r) => r.productName },
                  {
                    header: "Qty",
                    align: "right",
                    render: (r) => formatNumber(r.qty),
                  },
                  {
                    header: "HPP Rata-rata",
                    align: "right",
                    render: (r) => formatIDR(r.avgCost),
                  },
                  {
                    header: "Nilai",
                    align: "right",
                    render: (r) => formatIDR(r.value),
                  },
                ]}
                rows={inventory.positions}
                rowKey={(r) => `${r.productId}-${r.variantId}`}
                emptyTitle="Belum ada persediaan"
                emptyDescription="Belum ada posisi stok bernilai di cabang ini."
              />
            </div>
          ) : null}
        </SectionCard>
      </div>
    </div>
  );
}
