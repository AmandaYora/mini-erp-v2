import { Link } from "react-router-dom";
import { Badge, SectionCard, Skeleton } from "@/shared/components/ui";
import { DocNumber, Money } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, statusLabel, statusTone } from "@/shared/lib/format";
import { methodLabel } from "@/modules/payment/types";
import { HISTORY_KIND_LABEL } from "@/modules/payment/lib/order-history";
import type { HistoryEvent } from "@/modules/payment/lib/order-history";

interface Props {
  events: HistoryEvent[];
  loading: boolean;
  warnings: string[];
  /** Bagian yang disembunyikan karena izin — dijelaskan, bukan dibiarkan kosong diam-diam. */
  hiddenNotes: string[];
  /** Order walk-in tanpa pihak: buku pembayaran tidak tersedia. */
  paymentsUnavailable?: boolean;
}

function eventNote(e: HistoryEvent): string {
  if (e.kind === "payment") return methodLabel(e.note);
  return e.note && e.note.trim() !== "" ? e.note : "";
}

/**
 * Kartu "Aktivitas": garis waktu dokumen terkait order (pengiriman/penerimaan,
 * pembayaran, retur), terbaru dulu. Setiap butir menaut ke dokumennya.
 */
export function OrderHistoryCard({ events, loading, warnings, hiddenNotes, paymentsUnavailable }: Props) {
  return (
    <SectionCard title="Aktivitas" description="Pengiriman/penerimaan, pembayaran, dan retur untuk order ini.">
      <div className="flex flex-col gap-3">
        {warnings.map((w) => (
          <Notice key={w} tone="warning" title="Sebagian riwayat gagal dimuat">
            {w}
          </Notice>
        ))}
        {hiddenNotes.length > 0 && <p className="text-xs text-muted-foreground">{hiddenNotes.join(" · ")}</p>}
        {paymentsUnavailable && (
          <p className="text-xs text-muted-foreground">
            Order tanpa pelanggan/supplier tercatat: rincian pembayaran tidak tersedia di sini, lihat ringkasan tagihan.
          </p>
        )}
        {loading && events.length === 0 ? (
          <div role="status" aria-busy="true" aria-label="Memuat aktivitas" className="flex flex-col gap-4">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-3/4" />
          </div>
        ) : events.length === 0 ? (
          <EmptyState title="Belum ada dokumen terkait" />
        ) : (
          <ol className="relative flex flex-col gap-5 border-l border-border pl-5">
            {events.map((e) => {
              const note = eventNote(e);
              return (
                <li key={e.key} className="relative">
                  <span
                    aria-hidden
                    className="absolute top-1.5 -left-[1.6rem] size-2.5 rounded-full border-2 border-card bg-primary ring-1 ring-border"
                  />
                  <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                    <p className="text-sm text-foreground">
                      <span className="text-muted-foreground">{HISTORY_KIND_LABEL[e.kind]}</span>{" "}
                      <Link to={e.href} className="font-medium text-primary hover:underline">
                        <DocNumber value={e.number || `#${e.id}`} />
                      </Link>
                    </p>
                    <Badge tone={statusTone(e.status)}>{statusLabel(e.status)}</Badge>
                  </div>
                  <p className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
                    <span>{formatDate(e.date)}</span>
                    {e.amount !== undefined && <Money value={e.amount} className="text-foreground" />}
                    {note && <span>{note}</span>}
                  </p>
                </li>
              );
            })}
          </ol>
        )}
      </div>
    </SectionCard>
  );
}
