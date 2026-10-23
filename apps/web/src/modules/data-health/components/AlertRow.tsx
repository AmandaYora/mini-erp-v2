import { Link } from "react-router-dom";
import { BellOff, ChevronDown } from "lucide-react";
import {
  Badge,
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { formatNumber } from "@/shared/lib/format";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { SEVERITY, alertHeadingId, countLabel, oldestDays, progressOf, wibClock, wibDay } from "@/modules/data-health/lib/alerts";
import { SeverityIcon } from "@/modules/data-health/components/SeverityIcon";
import type { SeenMark } from "@/modules/data-health/lib/alerts";
import type { HealthAlert } from "@/modules/data-health/types";

export interface AlertRowProps {
  alert: HealthAlert;
  /** "snoozed" = tampilan tab Ditunda (tanggal akhir, alasan, Aktifkan lagi). */
  mode: "active" | "snoozed";
  seen?: SeenMark;
  today: string;
  busy?: boolean;
  onSnooze?: (alert: HealthAlert) => void;
  onUnsnooze?: (alert: HealthAlert) => void;
}

/**
 * Satu alert di halaman Kesehatan Data (brief §4a): judul + tingkat + Rp, jumlah · dampak,
 * lalu jejak penyelesaian/umur, contoh, dan dua tombol (aksi penyelesaian, Tunda).
 */
export function AlertRow({ alert, mode, seen, today, busy = false, onSnooze, onUnsnooze }: AlertRowProps) {
  const can = useAuthStore((s) => s.can);
  const sev = SEVERITY[alert.severity];
  const snoozed = mode === "snoozed";
  const progress = progressOf(alert, snoozed ? undefined : seen);
  const age = oldestDays(alert.oldestAt, today);
  const headingId = alertHeadingId(alert.id);
  const action = alert.action && can(alert.action.perm) ? alert.action : null;
  const expired =
    !snoozed && alert.snooze.snoozedUntil !== null && !alert.snooze.reactivated && Date.parse(alert.snooze.snoozedUntil) < Date.parse(alert.computedAt);
  const extra = alert.count - alert.sample.length;

  return (
    <li className="px-5 py-4" aria-labelledby={headingId}>
      <div className="flex gap-3">
        {snoozed ? (
          <BellOff size={18} aria-hidden className="mt-0.5 flex-none text-muted-foreground" />
        ) : (
          <SeverityIcon severity={alert.severity} className="mt-0.5" />
        )}
        <div className="min-w-0 flex-1">
          <div className="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
            <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
              <h3 id={headingId} tabIndex={-1} className="text-base font-medium text-foreground focus:outline-none">
                {alert.title}
              </h3>
              {snoozed ? (
                <Badge tone="muted">{`Ditunda s.d. ${wibDay(alert.snooze.snoozedUntil)}`}</Badge>
              ) : (
                <Badge tone={sev.tone}>{sev.label}</Badge>
              )}
            </div>
            {alert.amountRp !== null && (
              <span className="flex flex-none flex-col sm:items-end">
                <Money value={alert.amountRp} className="text-sm font-semibold text-foreground" />
                {alert.amountLabel && <span className="text-xs text-muted-foreground">{alert.amountLabel}</span>}
              </span>
            )}
          </div>

          <p className="mt-1 text-sm text-muted-foreground">
            <span className="font-medium text-foreground">{countLabel(alert)}</span>
            {` · ${alert.impact}`}
          </p>

          {snoozed && alert.snooze.reason && (
            <p className="mt-1 text-sm text-muted-foreground">
              {`Ditunda oleh ${alert.snooze.byName ?? "pengguna"}, ${wibDay(alert.snooze.snoozedAt)} ${wibClock(alert.snooze.snoozedAt)}: `}
              <q className="text-foreground">{alert.snooze.reason}</q>
            </p>
          )}
          {alert.snooze.reactivated && (
            <p className="mt-1 text-xs text-warning">
              {`Muncul lagi: masalah bertambah sejak ditunda ${wibDay(alert.snooze.snoozedAt)}`}
            </p>
          )}
          {expired && (
            <p className="mt-1 text-xs text-muted-foreground">{`Masa tunda berakhir ${wibDay(alert.snooze.snoozedUntil)}`}</p>
          )}
          {alert.hint && <p className="mt-1 text-sm text-foreground">{alert.hint}</p>}

          <div className="mt-2.5 flex flex-col gap-2.5 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
            <div className="flex min-w-0 flex-col gap-1.5">
              {progress ? (
                <ProgressTrail
                  remaining={progress.remaining}
                  from={progress.from}
                  text={
                    progress.basis === "snooze"
                      ? `Berkurang ${formatNumber(progress.from - progress.remaining)} sejak ditunda ${wibDay(progress.since)}`
                      : `${formatNumber(progress.remaining)} dari ${formatNumber(progress.from)} tersisa sejak ${wibDay(progress.since)}`
                  }
                />
              ) : (
                age !== null &&
                age > 0 && <p className="text-xs text-muted-foreground">{`Tertua ${formatNumber(age)} hari`}</p>
              )}
              {alert.sample.length > 0 && (
                <Collapsible>
                  <CollapsibleTrigger className="group inline-flex items-center gap-1 rounded text-sm font-medium text-primary hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none">
                    {`Lihat contoh (${alert.sample.length})`}
                    <ChevronDown size={14} aria-hidden className="transition-transform group-data-[state=open]:rotate-180" />
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <ul className="mt-2 flex flex-col gap-1 rounded-lg bg-muted px-3 py-2 text-sm">
                      {alert.sample.map((s, i) => (
                        <li key={`${s.label}-${i}`} className="flex flex-wrap justify-between gap-x-4">
                          <span className="min-w-0 text-foreground">{s.label}</span>
                          <span className="tabular text-muted-foreground">{s.value}</span>
                        </li>
                      ))}
                      {extra > 0 && <li className="text-muted-foreground">{`dan ${formatNumber(extra)} lainnya`}</li>}
                    </ul>
                  </CollapsibleContent>
                </Collapsible>
              )}
            </div>

            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              {action && (
                <Button asChild variant="secondary" className="w-full sm:h-8 sm:w-auto sm:px-3">
                  <Link to={action.to} aria-label={`${action.label}: ${alert.title}`}>
                    {action.label}
                  </Link>
                </Button>
              )}
              {snoozed ? (
                <Button
                  variant="ghost"
                  className="w-full sm:h-8 sm:w-auto sm:px-3"
                  loading={busy}
                  onClick={() => onUnsnooze?.(alert)}
                  aria-label={`Aktifkan lagi: ${alert.title}`}
                >
                  Aktifkan lagi
                </Button>
              ) : alert.snooze.snoozable ? (
                onSnooze && (
                  <Button
                    variant="ghost"
                    className="w-full sm:h-8 sm:w-auto sm:px-3"
                    onClick={() => onSnooze(alert)}
                    aria-label={`Tunda: ${alert.title}`}
                  >
                    Tunda
                  </Button>
                )
              ) : (
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span tabIndex={0} className="rounded text-xs text-muted-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none">
                        Tidak bisa ditunda
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>Alert keamanan harus diselesaikan, tidak bisa ditunda.</TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              )}
            </div>
          </div>
        </div>
      </div>
    </li>
  );
}

/** Bilah tipis "sudah selesai" — satu-satunya elemen visual baru (brief §2). */
function ProgressTrail({ remaining, from, text }: { remaining: number; from: number; text: string }) {
  const done = from > 0 ? Math.round(((from - remaining) / from) * 100) : 0;
  return (
    <div className="flex items-center gap-2.5">
      <div role="img" aria-label={text} className="h-1 w-24 flex-none overflow-hidden rounded-full bg-border">
        <div className="h-full rounded-full bg-success transition-[width] duration-200 ease-out" style={{ width: `${done}%` }} />
      </div>
      <p aria-hidden className="text-xs text-muted-foreground">
        {text}
      </p>
    </div>
  );
}
