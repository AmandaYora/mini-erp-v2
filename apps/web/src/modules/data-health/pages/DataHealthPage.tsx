import { useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { BellOff, CircleCheck, RefreshCw } from "lucide-react";
import {
  Button,
  ErrorState,
  PageHeader,
  SectionCard,
  Skeleton,
  Spinner,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/shared/components/ui";
import { Page, useUrlTab } from "@/shared/components/patterns";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import { cn } from "@/shared/lib/cn";
import { formatIDR, formatNumber, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { dataHealthService } from "@/modules/data-health/services/data-health.service";
import { AlertRow } from "@/modules/data-health/components/AlertRow";
import { SeverityIcon } from "@/modules/data-health/components/SeverityIcon";
import { AlertItemsPanel } from "@/modules/data-health/components/AlertItemsPanel";
import { SnoozeAlertDialog } from "@/modules/data-health/components/SnoozeAlertDialog";
import {
  AREA_LABEL,
  AREA_ORDER,
  FAILED_ICON,
  alertHeadingId,
  STALE_MINUTES,
  mergeSeen,
  minutesSince,
  readSeen,
  topSeverity,
  totalAmount,
  wibClock,
  wibDay,
  writeSeen,
} from "@/modules/data-health/lib/alerts";
import type { SnoozeValues } from "@/modules/data-health/schemas/snooze.schema";
import type { AlertArea, CheckStatus, DataHealthReport, FailedCheck, HealthAlert } from "@/modules/data-health/types";

const TABS = ["aktif", "ditunda"] as const;

/** Alert yang aksinya membuka daftar rinci di halaman ini (GET /data-health/{id}/items). */
const ITEM_ALERTS = new Set(["inventory.cost_above_price", "inventory.opening_unverified"]);

/** Selama lapis berat belum selesai, muat ulang sekali-sekali agar hasilnya muncul sendiri. */
const PENDING_POLL_MS = 8000;

interface AreaView {
  area: AlertArea;
  label: string;
  items: HealthAlert[];
  failed: FailedCheck[];
  checks: CheckStatus[];
}

function areaViews(report: DataHealthReport): AreaView[] {
  const failedIds = new Set(report.failedChecks.map((f) => f.id));
  return report.areas.map((a) => {
    const checks = report.checks.filter((c) => c.area === a.area);
    return {
      area: a.area,
      label: a.areaLabel || AREA_LABEL[a.area],
      items: a.items,
      checks,
      failed: report.failedChecks.filter((f) => failedIds.has(f.id) && checks.some((c) => c.id === f.id)),
    };
  });
}

// Area dengan kritis di atas, lalu yang hanya perhatian/info, lalu area beres di paling bawah;
// sama tingkatnya mengikuti urutan tetap Keuangan → … → Keamanan (brief §5d).
function areaRank(v: AreaView): number {
  const top = topSeverity(v.items);
  if (top === "critical") return 0;
  if (top !== null || v.failed.length > 0) return 1;
  return 2;
}

function areaCounts(items: HealthAlert[]): string {
  const c = items.filter((a) => a.severity === "critical").length;
  const w = items.filter((a) => a.severity === "warning").length;
  const i = items.filter((a) => a.severity === "info").length;
  return [c && `${c} kritis`, w && `${w} perhatian`, i && `${i} info`].filter(Boolean).join(" · ");
}

export default function DataHealthPage() {
  const [tab, setTab] = useUrlTab(TABS, "aktif");
  const [params, setParams] = useSearchParams();
  const openAlertId = params.get("alert");
  const report = useAsyncData((signal) => dataHealthService.report(false, signal), []);
  const [refreshing, setRefreshing] = useState(false);
  const [snoozing, setSnoozing] = useState<HealthAlert | null>(null);
  const [unsnoozingId, setUnsnoozingId] = useState<string | null>(null);
  const [prevSeen] = useState(readSeen);
  const focusAfter = useRef<string | null>(null);
  const today = todayWIB();
  const data = report.data;

  const active = useMemo(() => (data ? data.areas.flatMap((a) => a.items) : []), [data]);
  const snoozed = useMemo(() => data?.snoozed ?? [], [data]);
  // Jejak penyelesaian per pengguna (brief §2): pembanding lama digabung dengan alert aktif.
  const seen = useMemo(() => mergeSeen(prevSeen, active, new Date().toISOString()), [prevSeen, active]);
  const views = useMemo(
    () => (data ? areaViews(data).sort((a, b) => areaRank(a) - areaRank(b) || AREA_ORDER.indexOf(a.area) - AREA_ORDER.indexOf(b.area)) : []),
    [data],
  );
  const openAlert = openAlertId ? [...active, ...snoozed].find((a) => a.id === openAlertId) ?? null : null;

  useEffect(() => {
    if (data) writeSeen(seen);
  }, [data, seen]);

  // Setelah Tunda/Aktifkan lagi: fokus ke baris berikutnya (atau judul area) agar pengguna
  // keyboard tidak kehilangan posisi (brief §6).
  useEffect(() => {
    if (!data || !focusAfter.current) return;
    const el = document.getElementById(focusAfter.current);
    focusAfter.current = null;
    el?.focus();
  }, [data]);

  // Lapis berat (jurnal tak seimbang, konsistensi dokumen) dihitung di latar oleh server.
  const pending = data?.pending ?? false;
  const reload = report.reload;
  useEffect(() => {
    if (!pending) return;
    const t = window.setTimeout(reload, PENDING_POLL_MS);
    return () => window.clearTimeout(t);
  }, [pending, data, reload]);

  // ?alert=<id> tanpa daftar rinci: gulir ke baris alert itu.
  useEffect(() => {
    if (!openAlertId || ITEM_ALERTS.has(openAlertId) || !data) return;
    document.getElementById(alertHeadingId(openAlertId))?.scrollIntoView({ block: "center" });
  }, [openAlertId, data]);

  async function handleRefresh() {
    setRefreshing(true);
    try {
      await dataHealthService.report(true);
      reload();
      toast.success("Pemeriksaan selesai");
    } catch (err) {
      toast.danger(formatApiError(err));
    } finally {
      setRefreshing(false);
    }
  }

  function nextFocusId(list: HealthAlert[], id: string, area: AlertArea): string {
    const i = list.findIndex((a) => a.id === id);
    const next = list[i + 1] ?? list[i - 1];
    return next ? alertHeadingId(next.id) : `area-${area}-title`;
  }

  async function submitSnooze(alert: HealthAlert, values: SnoozeValues) {
    const areaItems = active.filter((a) => a.area === alert.area);
    const res = await dataHealthService.snooze(alert.id, values);
    setSnoozing(null);
    focusAfter.current = nextFocusId(areaItems, alert.id, alert.area);
    toast.success(`Alert ditunda sampai ${wibDay(res.data.snooze.snoozedUntil)}`);
    reload();
  }

  async function unsnooze(alert: HealthAlert) {
    setUnsnoozingId(alert.id);
    try {
      const res = await dataHealthService.unsnooze(alert.id);
      focusAfter.current = nextFocusId(snoozed, alert.id, alert.area);
      toast.fromServer(res.message, "Alert diaktifkan lagi");
      reload();
    } catch (err) {
      toast.danger(formatApiError(err));
    } finally {
      setUnsnoozingId(null);
    }
  }

  function closeItems() {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.delete("alert");
        return next;
      },
      { replace: true },
    );
  }

  const stale = data ? minutesSince(data.computedAt) > STALE_MINUTES : false;
  const critical = data?.bySeverity.critical ?? 0;
  const warning = data?.bySeverity.warning ?? 0;
  const info = data?.bySeverity.info ?? 0;
  const amount = totalAmount(active);
  const allClear = data !== null && active.length === 0 && (data.failedChecks.length ?? 0) === 0;

  return (
    <Page>
      <PageHeader
        title="Kesehatan Data"
        description="Pemeriksaan otomatis atas data keuangan, persediaan, master, dokumen, dan keamanan."
        actions={
          <div className="flex items-center gap-3">
            {data && (
              <span className={cn("text-sm", stale ? "text-warning" : "text-muted-foreground")}>
                {stale ? `Diperiksa ${formatNumber(minutesSince(data.computedAt))} menit lalu` : `Diperiksa ${wibClock(data.computedAt)}`}
              </span>
            )}
            <Button variant="secondary" size="sm" onClick={handleRefresh} loading={refreshing} disabled={!data}>
              {!refreshing && <RefreshCw size={16} aria-hidden />}
              Periksa ulang
            </Button>
          </div>
        }
      />

      {report.error && !data ? (
        <ErrorState title="Gagal memuat kesehatan data" message={report.error} onRetry={reload} />
      ) : !data ? (
        <LoadingSkeleton />
      ) : (
        <>
          <section aria-label="Ringkasan" className="flex flex-col gap-3 rounded-xl border border-border bg-card px-5 py-4">
            <p role="status" aria-atomic="true" className="text-sm text-foreground">
              {allClear ? (
                <span className="inline-flex items-center gap-2 font-medium text-success">
                  <CircleCheck size={18} aria-hidden />
                  {data.pending ? "Belum ada masalah ditemukan · pemeriksaan berlanjut" : `Semua pemeriksaan lolos · diperiksa ${wibClock(data.computedAt)}`}
                </span>
              ) : (
                <>
                  <span className="font-semibold">{`${formatNumber(critical)} kritis`}</span>
                  {` · ${formatNumber(warning)} perhatian`}
                  {info > 0 && ` · ${formatNumber(info)} info`}
                  {amount > 0 && (
                    <>
                      {" · "}
                      <span className="tabular font-semibold">{formatIDR(amount)}</span>
                      {" terdampak"}
                    </>
                  )}
                </>
              )}
            </p>
            <nav aria-label="Lompat ke area" className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-0.5">
              {views.map((v) => {
                const top = topSeverity(v.items);
                const rendered = v.items.length > 0 || v.failed.length > 0;
                const body = (
                  <>
                    {top ? (
                      <SeverityIcon severity={top} size={16} />
                    ) : v.failed.length > 0 ? (
                      <FAILED_ICON size={16} aria-hidden className="text-muted-foreground" />
                    ) : (
                      <CircleCheck size={16} aria-hidden className="text-success" />
                    )}
                    {v.label}
                    {v.items.length > 0 && <span className="tabular text-muted-foreground">{formatNumber(v.items.length)}</span>}
                  </>
                );
                const cls = "inline-flex h-8 flex-none items-center gap-1.5 rounded-full border border-border px-3 text-sm text-foreground";
                return rendered ? (
                  <a key={v.area} href={`#area-${v.area}`} className={cn(cls, "hover:bg-background")}>
                    {body}
                  </a>
                ) : (
                  <span key={v.area} className={cn(cls, "text-muted-foreground")}>
                    {body}
                  </span>
                );
              })}
            </nav>
            {data.pending && (
              <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <Spinner aria-hidden className="size-4" />
                Sebagian pemeriksaan masih dihitung…
              </p>
            )}
          </section>

          {openAlert && ITEM_ALERTS.has(openAlert.id) && <AlertItemsPanel alert={openAlert} onClose={closeItems} />}

          <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])}>
            <TabsList>
              <TabsTrigger value="aktif">{`Aktif (${formatNumber(active.length)})`}</TabsTrigger>
              <TabsTrigger value="ditunda">{`Ditunda (${formatNumber(snoozed.length)})`}</TabsTrigger>
            </TabsList>

            <TabsContent value="aktif" className="mt-4 flex flex-col gap-6">
              {allClear ? (
                <SectionCard>
                  <EmptyState
                    icon={CircleCheck}
                    title={data.pending ? "Belum ada masalah ditemukan" : "Semua pemeriksaan lolos"}
                    description={`${formatNumber(data.checks.length)} pemeriksaan data dijalankan pukul ${wibClock(data.computedAt)}. ${
                      data.pending ? "Pemeriksaan berlanjut." : "Tidak ada yang perlu ditindak."
                    }`}
                  />
                  <p className="pb-4 text-center text-sm text-muted-foreground">{views.map((v) => v.label).join(" · ")}</p>
                </SectionCard>
              ) : (
                <>
                  {views
                    .filter((v) => v.items.length > 0 || v.failed.length > 0)
                    .map((v) => (
                      <section
                        key={v.area}
                        id={`area-${v.area}`}
                        aria-labelledby={`area-${v.area}-title`}
                        className="scroll-mt-20 overflow-hidden rounded-xl border border-border bg-card"
                      >
                        <div className="flex flex-wrap items-baseline justify-between gap-2 border-b border-border px-5 py-4">
                          <h2 id={`area-${v.area}-title`} tabIndex={-1} className="text-lg font-semibold text-foreground focus:outline-none">
                            {v.label}
                          </h2>
                          <span className="text-sm text-muted-foreground">{areaCounts(v.items)}</span>
                        </div>
                        <ul className="divide-y divide-border">
                          {v.items.map((a) => (
                            <AlertRow
                              key={a.id}
                              alert={a}
                              mode="active"
                              seen={seen[a.id]}
                              today={today}
                              onSnooze={setSnoozing}
                            />
                          ))}
                          {v.failed.map((f) => (
                            <li key={f.id} className="flex flex-col gap-2 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
                              <div className="flex gap-3">
                                <FAILED_ICON size={18} aria-hidden className="mt-0.5 flex-none text-muted-foreground" />
                                <div>
                                  <p className="text-base font-medium text-foreground">{`${f.label} tidak terperiksa`}</p>
                                  <p className="text-sm text-muted-foreground">{f.message}</p>
                                </div>
                              </div>
                              <Button variant="secondary" size="sm" onClick={handleRefresh} loading={refreshing}>
                                Coba lagi
                              </Button>
                            </li>
                          ))}
                        </ul>
                      </section>
                    ))}
                  {views
                    .filter((v) => v.items.length === 0 && v.failed.length === 0)
                    .map((v) => (
                      <p key={v.area} className="flex items-center gap-2 px-1 text-sm text-muted-foreground">
                        <CircleCheck size={16} aria-hidden className="text-success" />
                        {v.checks.some((c) => c.status === "pending")
                          ? `${v.label}: belum ada masalah ditemukan · pemeriksaan berlanjut`
                          : `${v.label}: semua pemeriksaan lolos (${formatNumber(v.checks.length)} pemeriksaan)`}
                      </p>
                    ))}
                </>
              )}
            </TabsContent>

            <TabsContent value="ditunda" className="mt-4">
              <SectionCard flush>
                {snoozed.length === 0 ? (
                  <EmptyState
                    icon={BellOff}
                    title="Tidak ada alert yang ditunda"
                    description="Alert yang ditunda muncul di sini sampai masa tundanya berakhir."
                  />
                ) : (
                  <ul className="divide-y divide-border">
                    {snoozed.map((a) => (
                      <AlertRow
                        key={a.id}
                        alert={a}
                        mode="snoozed"
                        today={today}
                        busy={unsnoozingId === a.id}
                        onUnsnooze={unsnooze}
                      />
                    ))}
                  </ul>
                )}
              </SectionCard>
            </TabsContent>
          </Tabs>
        </>
      )}

      <SnoozeAlertDialog alert={snoozing} onClose={() => setSnoozing(null)} onSubmit={submitSnooze} />
    </Page>
  );
}

function LoadingSkeleton() {
  return (
    <div aria-busy="true" aria-label="Memuat kesehatan data" className="flex flex-col gap-6">
      <div className="flex flex-col gap-3 rounded-xl border border-border bg-card px-5 py-4">
        <Skeleton className="h-4 w-64" />
        <Skeleton className="h-8 w-96 max-w-full" />
      </div>
      {[0, 1].map((k) => (
        <div key={k} className="overflow-hidden rounded-xl border border-border bg-card">
          <div className="border-b border-border px-5 py-4">
            <Skeleton className="h-5 w-32" />
          </div>
          {[0, 1, 2].map((r) => (
            <div key={r} className="flex gap-3 border-b border-border px-5 py-4 last:border-0">
              <Skeleton className="size-5 rounded-full" />
              <div className="flex flex-1 flex-col gap-2">
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-3 w-5/6" />
              </div>
              <Skeleton className="h-4 w-24" />
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}
