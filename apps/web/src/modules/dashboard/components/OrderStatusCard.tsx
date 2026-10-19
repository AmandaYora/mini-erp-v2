import { CardSkeleton, ErrorState, SectionCard } from "@/shared/components/ui";
import { formatNumber, statusLabel } from "@/shared/lib/format";
import { cn } from "@/shared/lib/cn";
import { statusSegments, type OrderStatusKey } from "@/modules/dashboard/lib/dashboard";
import type { OrderStatusCounts } from "@/modules/dashboard/types";

// Warna segmen mengikuti arti status DS (PLAN §4.2): abu = draf, navy = berjalan,
// hijau = selesai, abu pucat = batal.
const SEGMENT: Record<OrderStatusKey, string> = {
  draft: "bg-muted-foreground/50",
  confirmed: "bg-primary",
  completed: "bg-success",
  cancelled: "bg-input/60",
};

function StatusBar({ label, counts }: { label: string; counts: Record<string, number> }) {
  const { total, segments } = statusSegments(counts);
  const summary = `${label}: ${segments.map((s) => `${formatNumber(s.count)} ${statusLabel(s.status).toLowerCase()}`).join(", ")}`;
  return (
    <div>
      <div className="mb-2 flex items-baseline justify-between gap-3">
        <p className="text-sm font-medium text-foreground">{label}</p>
        <p className="tabular text-xs text-muted-foreground">{formatNumber(total)} order</p>
      </div>
      <div role="img" aria-label={summary} className="flex h-2.5 w-full overflow-hidden rounded-full bg-muted">
        {segments
          .filter((s) => s.count > 0)
          .map((s) => (
            <span key={s.status} className={cn("h-full", SEGMENT[s.status])} style={{ width: `${s.percent}%` }} />
          ))}
      </div>
      <ul className="mt-2.5 grid grid-cols-2 gap-x-4 gap-y-1">
        {segments.map((s) => (
          <li key={s.status} className="flex items-center gap-2 text-xs text-muted-foreground">
            <span aria-hidden className={cn("size-2 flex-none rounded-full", SEGMENT[s.status])} />
            <span className="flex-1">{statusLabel(s.status)}</span>
            <span className="tabular font-medium text-foreground">{formatNumber(s.count)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/** Status order: satu bar 100% per arus (penjualan, pembelian) + legenda hitungan. */
export function OrderStatusCard({
  data,
  loading,
  error,
  onRetry,
}: {
  data: OrderStatusCounts | null;
  loading: boolean;
  error: string | null;
  onRetry: () => void;
}) {
  if (loading && !data) return <CardSkeleton className="h-full min-h-64" />;
  return (
    <SectionCard title="Status order" description="Hitungan order per status turunan." className="h-full">
      {error ? (
        <ErrorState message={error} onRetry={onRetry} />
      ) : data ? (
        <div className="flex flex-col gap-6">
          <StatusBar label="Penjualan" counts={data.sales} />
          <StatusBar label="Pembelian" counts={data.purchasing} />
        </div>
      ) : null}
    </SectionCard>
  );
}
