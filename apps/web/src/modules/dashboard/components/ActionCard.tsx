import { Link } from "react-router-dom";
import { ChevronRight, CircleCheck } from "lucide-react";
import { CardSkeleton, ErrorState, SectionCard } from "@/shared/components/ui";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { cn } from "@/shared/lib/cn";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { buildActionItems } from "@/modules/dashboard/lib/dashboard";
import type { CriticalStockItem, PriorityOrder } from "@/modules/dashboard/types";

interface Source<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

const DOT = { danger: "bg-destructive", warning: "bg-warning" } as const;
const TEXT = { danger: "text-destructive", warning: "text-warning" } as const;

/**
 * "Perlu tindakan": priorityOrders + criticalStock yang sudah di-fetch dasbor, digabung jadi satu
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
