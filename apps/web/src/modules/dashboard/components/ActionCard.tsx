import { Link } from "react-router-dom";
import { ChevronRight, CircleCheck } from "lucide-react";
import { CardSkeleton, ErrorState, SectionCard, Spinner } from "@/shared/components/ui";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { cn } from "@/shared/lib/cn";
import { formatNumber } from "@/shared/lib/format";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { buildActionItems } from "@/modules/dashboard/lib/dashboard";
import type { CriticalStockItem, PriorityOrder } from "@/modules/dashboard/types";
import { SeverityIcon } from "@/modules/data-health/components/SeverityIcon";
import { SEVERITY, cardAccessibleName, cardDetail, wibClock } from "@/modules/data-health/lib/alerts";
import type { AlertSummary, HealthAlert } from "@/modules/data-health/types";

interface Source<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

const DOT = { danger: "bg-destructive", warning: "bg-warning" } as const;
const TEXT = { danger: "text-destructive", warning: "text-warning" } as const;

/**
 * Kartu "Perlu tindakan" untuk peran berizin dashboard.alerts.view_all (ADR-0025, brief §5a):
 * maksimal 5 alert kritis/perhatian dari GET /dashboard/alerts — order lewat jatuh tempo dan stok
 * kritis ikut sebagai alert gabungan (A14/A15). Tanpa kalimat dampak dan tanpa Tunda: keputusan
 * menunda dibuat di halaman Kesehatan Data.
 */
export function AlertActionCard({ alerts, today }: { alerts: Source<AlertSummary>; today: string }) {
  const can = useAuthStore((s) => s.can);
  if (alerts.loading && !alerts.data) return <CardSkeleton className="h-full min-h-64" />;
  const data = alerts.data;
  const total = data?.totalActive ?? 0;
  const failed = data?.failedChecks.length ?? 0;
  const target = (a: HealthAlert) =>
    a.action && can(a.action.perm) ? a.action.to : `${ROUTE_PATHS.dataHealth}?alert=${encodeURIComponent(a.id)}`;

  return (
    <SectionCard
      title={total > 0 ? `Perlu tindakan (${formatNumber(total)})` : "Perlu tindakan"}
      description="Masalah data dan operasional paling mendesak."
      flush
      className="h-full"
    >
      {alerts.error && !data ? (
        <div className="p-3">
          <ErrorState title="Gagal memuat alert" message={alerts.error} onRetry={alerts.reload} />
        </div>
      ) : data && data.items.length === 0 ? (
        <EmptyState
          icon={CircleCheck}
          title={failed === 0 && !data.pending ? "Semua pemeriksaan lolos" : "Belum ada masalah ditemukan"}
          description={
            failed === 0 && !data.pending
              ? `Pemeriksaan data dijalankan pukul ${wibClock(data.computedAt)}. Tidak ada yang perlu ditindak.`
              : "Pemeriksaan berlanjut; buka Kesehatan Data untuk rinciannya."
          }
        />
      ) : (
        <ul className="divide-y divide-border">
          {data?.items.map((a) => (
            <li key={a.id}>
              <Link
                to={target(a)}
                aria-label={cardAccessibleName(a, today)}
                className="flex min-h-14 items-center gap-3 px-5 py-3 transition-colors hover:bg-background focus-visible:bg-background"
              >
                <SeverityIcon severity={a.severity} size={16} />
                <span className="min-w-0 flex-1">
                  <span className="line-clamp-2 text-sm font-medium text-foreground">
                    <span className="sr-only">{`${SEVERITY[a.severity].label}: `}</span>
                    {a.title}
                  </span>
                  <span className={cn("block text-xs", SEVERITY[a.severity].text)}>{cardDetail(a, today)}</span>
                </span>
                <ChevronRight size={16} aria-hidden className="flex-none text-muted-foreground" />
              </Link>
            </li>
          ))}
        </ul>
      )}
      {data && (failed > 0 || data.pending) && (
        <p className="flex items-center gap-2 border-t border-border px-5 py-2.5 text-xs text-muted-foreground">
          {data.pending && <Spinner aria-hidden className="size-3.5" />}
          {[failed > 0 && `${formatNumber(failed)} pemeriksaan tidak terperiksa`, data.pending && "Sebagian pemeriksaan masih dihitung"]
            .filter(Boolean)
            .join(" · ")}
        </p>
      )}
      <div className="flex flex-wrap gap-x-4 gap-y-1 border-t border-border px-5 py-3 text-sm">
        <Link to={ROUTE_PATHS.dataHealth} className="font-medium text-primary hover:underline">
          {total > 0 ? `Lihat semua (${formatNumber(total)})` : "Buka Kesehatan Data"}
        </Link>
        {data && data.snoozedCount > 0 && (
          <Link to={`${ROUTE_PATHS.dataHealth}?tab=ditunda`} className="text-muted-foreground hover:underline">
            {`${formatNumber(data.snoozedCount)} ditunda`}
          </Link>
        )}
      </div>
    </SectionCard>
  );
}

/**
 * "Perlu tindakan" (versi peran tanpa alert, mis. kasir): priorityOrders + criticalStock yang sudah di-fetch dasbor, digabung jadi satu
 * daftar. Tiap baris menautkan ke daftar terfilter (SO per nomor, kartu stok per produk).
 */
export function ActionCard({
  orders,
  critical,
  criticalCount,
  today,
  canReporting,
}: {
  orders: Source<PriorityOrder[]>;
  critical: Source<CriticalStockItem[]>;
  /** Total produk kritis (summary.operations.criticalCount) — daftar `critical` dibatasi 20. */
  criticalCount: number | null;
  today: string;
  canReporting: boolean;
}) {
  if (orders.loading && critical.loading) return <CardSkeleton className="h-full min-h-64" />;

  const items = buildActionItems(orders.data, critical.data, today);
  const orderTotal = orders.data?.length ?? 0;
  const stockTotal = criticalCount ?? critical.data?.length ?? 0;
  const total = orderTotal + stockTotal;
  const showAllStock = canReporting && stockTotal > 0;

  return (
    <SectionCard
      title={total > 0 ? `Perlu tindakan (${total})` : "Perlu tindakan"}
      description="Pesanan jatuh tempo dan stok kritis yang perlu ditangani."
      flush
      className="h-full"
    >
      {(orders.error || critical.error) && (
        <div className="flex flex-col gap-3 p-3">
          {orders.error && <ErrorState title="Gagal memuat pesanan prioritas" message={orders.error} onRetry={orders.reload} />}
          {critical.error && <ErrorState title="Gagal memuat stok kritis" message={critical.error} onRetry={critical.reload} />}
        </div>
      )}
      {items.length === 0 ? (
        orders.error || critical.error || orders.loading || critical.loading ? null : (
          <EmptyState
            icon={CircleCheck}
            title="Tidak ada yang mendesak"
            description="Tidak ada pesanan yang jatuh tempo dan semua stok di atas minimum."
          />
        )
      ) : (
        <ul className="divide-y divide-border">
          {items.map((it) => (
            <li key={it.key}>
              <Link to={it.to} className="flex items-center gap-3 px-5 py-3 transition-colors hover:bg-background">
                <span aria-hidden className={cn("size-2 flex-none rounded-full", DOT[it.tone])} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-foreground">{it.title}</span>
                  <span className={cn("block text-xs", TEXT[it.tone])}>{it.detail}</span>
                </span>
                <ChevronRight size={16} aria-hidden className="flex-none text-muted-foreground" />
              </Link>
            </li>
          ))}
        </ul>
      )}
      {(orderTotal > 0 || showAllStock) && (
        <div className="flex flex-wrap gap-x-4 gap-y-1 border-t border-border px-5 py-3 text-sm">
          {orderTotal > 0 && (
            <Link to={`${ROUTE_PATHS.salesOrders}?status=confirmed`} className="font-medium text-primary hover:underline">
              Semua order terkonfirmasi
            </Link>
          )}
          {showAllStock && (
            <Link to={`${ROUTE_PATHS.reportingStock}?critical=1`} className="font-medium text-primary hover:underline">
              {`Semua stok kritis (${stockTotal})`}
            </Link>
          )}
        </div>
      )}
    </SectionCard>
  );
}
