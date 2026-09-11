import { useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  Badge,
  DataTable,
  PageHeader,
  SectionCard,
  Segmented,
  SummaryCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  formatIDR,
  formatNumber,
  statusLabel,
  todayWIB,
} from "@/shared/lib/format";
import { colors } from "@/theme/colors";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { dashboardService } from "@/modules/dashboard/services/dashboard.service";
import type {
  CriticalStockItem,
  PriorityOrder,
  TopMarginProduct,
  TopSeller,
  TrendRangeKey,
} from "@/modules/dashboard/types";
import { TREND_RANGES } from "@/modules/dashboard/types";

/** YYYY-MM-DD digeser N hari (kalender UTC agar stabil di sekitar tengah malam). */
function addDays(ymd: string, delta: number): string {
  const d = new Date(`${ymd}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + delta);
  return d.toISOString().slice(0, 10);
}

/** Sumbu Y padat: 1500000 → "2 jt", 250000 → "250 rb". */
function compactIDR(v: number): string {
  const abs = Math.abs(v);
  if (abs >= 1_000_000) return `${Math.round(v / 1_000_000)} jt`;
  if (abs >= 1_000) return `${Math.round(v / 1_000)} rb`;
  return String(Math.round(v));
}

/** Selisih hari WIB antara hari ini dan tanggal tempo (negatif = belum tempo). */
function overdueDays(dueDate: string): number {
  const due = dueDate.slice(0, 10);
  const ms =
    new Date(`${todayWIB()}T00:00:00Z`).getTime() -
    new Date(`${due}T00:00:00Z`).getTime();
  return Math.round(ms / 86_400_000);
}

function WidgetError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <Notice tone="danger" title="Gagal memuat">
      {message}{" "}
      <button
        type="button"
        onClick={onRetry}
        className="cursor-pointer font-semibold text-brand hover:underline"
      >
        Coba lagi
      </button>
    </Notice>
  );
}

export default function DashboardPage() {
  const [trendRange, setTrendRange] = useState<TrendRangeKey>("30d");
  const { data: summary, loading, error, reload: reloadSummary } = useAsyncData(
    () => dashboardService.summary(),
    [],
  );
  const {
    data: trendData,
    loading: trendLoading,
    error: trendError,
    reload: reloadTrend,
  } = useAsyncData(
    () => {
      const days =
        TREND_RANGES.find((r) => r.value === trendRange)?.days ?? 30;
      const to = todayWIB();
      const from = addDays(to, -(days - 1));
      return dashboardService.salesTrend(from, to);
    },
    [trendRange],
  );
  const trend = trendData ?? [];

  const critical = useAsyncData(() => dashboardService.criticalStock(), []);
  const priority = useAsyncData(() => dashboardService.priorityOrders(), []);
  const sellers = useAsyncData(() => dashboardService.topSellers(), []);
  const margin = useAsyncData(() => dashboardService.topMargin(), []);

  const {
    data: orderStatus,
    error: orderStatusError,
    reload: reloadOrderStatus,
  } = useAsyncData(() => dashboardService.orderStatus(), []);

  const trendLabel =
    TREND_RANGES.find((r) => r.value === trendRange)?.label ?? "30 hari";

  return (
    <div>
      <PageHeader
        eyebrow="Utama"
        title="Dashboard"
        description="Ringkasan operasional cabang: omzet, laba, dan saldo buku."
      />

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat dashboard">
            {error}{" "}
            <button
              type="button"
              onClick={() => reloadSummary()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      {loading ? (
        <p className="text-muted px-1 py-6 text-sm">Memuat dashboard…</p>
      ) : summary ? (
        <div className="flex flex-col gap-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <SummaryCard
              label="Omzet Hari Ini"
              value={formatIDR(summary.today.revenue)}
              hint={`Beban ${formatIDR(summary.today.expense)}`}
            />
            <SummaryCard
              label="Laba Hari Ini"
              value={formatIDR(summary.today.profit)}
              tone="success"
              hint={formatDate(summary.today.from)}
            />
            <SummaryCard
              label="Omzet Bulan Ini"
              value={formatIDR(summary.monthToDate.revenue)}
              hint={`Sejak ${formatDate(summary.monthToDate.from)}`}
            />
            <SummaryCard
              label="Laba Bulan Ini"
              value={formatIDR(summary.monthToDate.profit)}
              tone="success"
              hint={`Beban ${formatIDR(summary.monthToDate.expense)}`}
            />
          </div>

          <SectionCard
            title="Saldo Buku"
            description="Posisi kas, piutang, hutang, dan persediaan cabang aktif."
          >
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <SummaryCard
                label="Kas"
                value={formatIDR(summary.balances.cash)}
              />
              <SummaryCard
                label="Piutang"
                value={formatIDR(summary.balances.receivable)}
              />
              <SummaryCard
                label="Hutang"
                value={formatIDR(summary.balances.payable)}
                tone="warning"
              />
              <SummaryCard
                label="Persediaan"
                value={formatIDR(summary.balances.inventory)}
              />
            </div>
          </SectionCard>

          <SectionCard
            title="Status Order"
            description="Hitungan order per status turunan — tanpa tabel konfigurasi status."
          >
            {orderStatusError ? (
              <WidgetError
                message={orderStatusError}
                onRetry={() => reloadOrderStatus()}
              />
            ) : orderStatus ? (
              <div className="flex flex-col gap-4">
                <div>
                  <p className="text-muted mb-2 text-xs font-semibold uppercase">
                    Penjualan
                  </p>
                  <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                    {["draft", "confirmed", "completed", "cancelled"].map(
                      (s) => (
                        <SummaryCard
                          key={s}
                          label={statusLabel(s)}
                          value={formatNumber(orderStatus.sales[s] ?? 0)}
                          tone={
                            s === "cancelled"
                              ? "danger"
                              : s === "confirmed"
                                ? "warning"
                                : "brand"
                          }
                        />
                      ),
                    )}
                  </div>
                </div>
                <div>
                  <p className="text-muted mb-2 text-xs font-semibold uppercase">
                    Pembelian
                  </p>
                  <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                    {["draft", "confirmed", "completed", "cancelled"].map(
                      (s) => (
                        <SummaryCard
                          key={s}
                          label={statusLabel(s)}
                          value={formatNumber(orderStatus.purchasing[s] ?? 0)}
                          tone={
                            s === "cancelled"
                              ? "danger"
                              : s === "confirmed"
                                ? "warning"
                                : "brand"
                          }
                        />
                      ),
                    )}
                  </div>
                </div>
              </div>
            ) : (
              <p className="text-muted px-1 py-6 text-sm">Memuat status…</p>
            )}
          </SectionCard>

          <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
            <SectionCard
              title="Stok Kritis"
              description="Produk di/lewat batas minimum, paling tipis dulu."
            >
              {critical.error ? (
                <WidgetError
                  message={critical.error}
                  onRetry={() => critical.reload()}
                />
              ) : critical.loading ? (
                <p className="text-muted px-1 py-6 text-sm">Memuat stok…</p>
              ) : (
                <DataTable<CriticalStockItem>
                  rows={critical.data ?? []}
                  rowKey={(r) => r.productId}
                  emptyTitle="Stok aman"
                  emptyDescription="Tidak ada produk di bawah minimum."
                  columns={[
                    {
                      header: "Produk",
                      render: (r) => (
                        <span>
                          <span className="font-semibold">{r.productCode}</span>{" "}
                          <span className="text-muted">{r.productName}</span>
                        </span>
                      ),
                    },
                    {
                      header: "Tersedia",
                      align: "right",
                      render: (r) => formatNumber(r.available),
                    },
                    {
                      header: "Minimum",
                      align: "right",
                      render: (r) => formatNumber(r.minStock),
                    },
                    {
                      header: "Status",
                      render: (r) => (
                        <Badge tone={r.available <= 0 ? "danger" : "warning"}>
                          {r.available <= 0 ? "Habis" : "Kritis"}
                        </Badge>
                      ),
                    },
                  ]}
                />
              )}
            </SectionCard>

            <SectionCard
              title="Pesanan Prioritas"
              description="SO terkonfirmasi yang lewat/jelang tempo (7 hari)."
            >
              {priority.error ? (
                <WidgetError
                  message={priority.error}
                  onRetry={() => priority.reload()}
                />
              ) : priority.loading ? (
                <p className="text-muted px-1 py-6 text-sm">Memuat pesanan…</p>
              ) : (
                <DataTable<PriorityOrder>
                  rows={priority.data ?? []}
                  rowKey={(r) => r.id}
                  emptyTitle="Tidak ada yang mendesak"
                  emptyDescription="Semua SO terkonfirmasi masih jauh dari tempo."
                  columns={[
                    {
                      header: "Nomor",
                      render: (r) => (
                        <span>
                          <span className="font-semibold">{r.number}</span>{" "}
                          <span className="text-muted">
                            {r.partyName || "Tunai"}
                          </span>
                        </span>
                      ),
                    },
                    {
                      header: "Tempo",
                      render: (r) => {
                        const late = overdueDays(r.dueDate);
                        return (
                          <span className="inline-flex items-center gap-2">
                            {formatDate(r.dueDate.slice(0, 10))}
                            {late > 0 ? (
                              <Badge tone="danger">
                                {`Terlambat ${late} hari`}
                              </Badge>
                            ) : (
                              <Badge tone="warning">Jatuh tempo</Badge>
                            )}
                          </span>
                        );
                      },
                    },
                    {
                      header: "Total",
                      align: "right",
                      render: (r) => formatIDR(r.grandTotal),
                    },
                  ]}
                />
              )}
            </SectionCard>
          </div>

          <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
            <SectionCard
              title="Terlaris Bulan Ini"
              description="5 produk dengan qty basis terbesar, order batal dikecualikan."
            >
              {sellers.error ? (
                <WidgetError
                  message={sellers.error}
                  onRetry={() => sellers.reload()}
                />
              ) : sellers.loading ? (
                <p className="text-muted px-1 py-6 text-sm">Memuat…</p>
              ) : (
                <DataTable<TopSeller>
                  rows={sellers.data ?? []}
                  rowKey={(r) => `${r.productId}/${r.variantId}`}
                  emptyTitle="Belum ada penjualan"
                  emptyDescription="Belum ada order bulan berjalan."
                  columns={[
                    {
                      header: "Produk",
                      render: (r) => (
                        <span>
                          <span className="font-semibold">{r.productCode}</span>{" "}
                          <span className="text-muted">{r.productName}</span>
                        </span>
                      ),
                    },
                    {
                      header: "Terjual",
                      align: "right",
                      render: (r) => formatNumber(r.qtyBase),
                    },
                    {
                      header: "Omzet",
                      align: "right",
                      render: (r) => formatIDR(r.revenue),
                    },
                  ]}
                />
              )}
            </SectionCard>

            <SectionCard
              title="Margin Tertinggi"
              description="5 produk dengan laba kotor terbesar menurut buku."
            >
              {margin.error ? (
                <WidgetError
                  message={margin.error}
                  onRetry={() => margin.reload()}
                />
              ) : margin.loading ? (
                <p className="text-muted px-1 py-6 text-sm">Memuat…</p>
              ) : (
                <DataTable<TopMarginProduct>
                  rows={margin.data ?? []}
                  rowKey={(r) => `${r.productId}/${r.variantId}`}
                  emptyTitle="Belum ada margin"
                  emptyDescription="Belum ada penjualan terposting bulan berjalan."
                  columns={[
                    {
                      header: "Produk",
                      render: (r) => (
                        <span>
                          <span className="font-semibold">{r.productCode}</span>{" "}
                          <span className="text-muted">{r.productName}</span>
                        </span>
                      ),
                    },
                    {
                      header: "Laba",
                      align: "right",
                      render: (r) => (
                        <span className="inline-flex items-center gap-2">
                          {formatIDR(r.gross)}
                          <Badge tone={r.gross > 0 ? "success" : "neutral"}>
                            {`${formatNumber(Math.round(r.percent * 10) / 10)}%`}
                          </Badge>
                        </span>
                      ),
                    },
                    {
                      header: "Omzet",
                      align: "right",
                      render: (r) => formatIDR(r.revenue),
                    },
                  ]}
                />
              )}
            </SectionCard>
          </div>

          <SectionCard
            title={`Tren Penjualan ${trendLabel}`}
            description="Omzet harian cabang aktif."
            actions={
              <Segmented
                options={TREND_RANGES.map((r) => ({
                  value: r.value,
                  label: r.label,
                }))}
                value={trendRange}
                onChange={(v) => setTrendRange(v as TrendRangeKey)}
              />
            }
          >
            {trendError ? (
              <WidgetError
                message={trendError}
                onRetry={() => reloadTrend()}
              />
            ) : trendLoading ? (
              <p className="text-muted px-1 py-6 text-sm">Memuat tren…</p>
            ) : (
              <>
                <ResponsiveContainer width="100%" height={280}>
                  <BarChart data={trend}>
                    <CartesianGrid stroke={colors.hairline} />
                    <XAxis
                      dataKey="date"
                      tickFormatter={(v: string) =>
                        trendRange === "12m"
                          ? `${v.slice(5, 7)}/${v.slice(2, 4)}`
                          : `${v.slice(8, 10)}/${v.slice(5, 7)}`
                      }
                      minTickGap={trendRange === "12m" ? 60 : 24}
                      tick={{ fontSize: 12 }}
                    />
                    <YAxis
                      tickFormatter={compactIDR}
                      width={64}
                      tick={{ fontSize: 12 }}
                    />
                    <Tooltip
                      formatter={(value) => formatIDR(Number(value))}
                      labelFormatter={(label) => formatDate(String(label))}
                    />
                    <Bar dataKey="revenue" name="Omzet" fill={colors.brand} />
                  </BarChart>
                </ResponsiveContainer>
                <div className="mt-4">
                  <Notice tone="info" title="Nilai persediaan">
                    {`Total ${formatIDR(summary.inventoryValue)} dinilai dengan moving average.`}
                  </Notice>
                </div>
              </>
            )}
          </SectionCard>
        </div>
      ) : null}
    </div>
  );
}
