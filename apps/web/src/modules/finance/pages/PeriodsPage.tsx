import { useState } from "react";
import { Lock, LockOpen } from "lucide-react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  FormField,
  PageHeader,
  SectionCard,
  SelectInput,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/shared/components/ui";
import { Page, useUrlTab } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { TableSkeleton } from "@/shared/components/ui";
import { todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { cn } from "@/shared/lib/cn";
import { financeService } from "@/modules/finance/services/finance.service";
import { ReopenPeriodModal } from "@/modules/finance/components/ReopenPeriodModal";
import { buildYearTimeline, closedCount, timelineYears } from "@/modules/finance/lib/period-timeline";
import type { TimelineMonth } from "@/modules/finance/lib/period-timeline";
import { MONTH_NAMES } from "@/modules/finance/lib/report-periods";
import { checkLabel, checkTone, periodLabel } from "@/modules/finance/types";
import type { FiscalPeriod, PeriodReadiness, ReadinessCheck } from "@/modules/finance/types";

type Scope = "fiscal" | "tax";

interface PendingAction {
  kind: "close" | "reopen" | "safeclose";
  scope: Scope;
  year: number;
  month: number;
}

const TABS = ["fiskal", "pajak"] as const;

function MonthTile({
  m,
  scope,
  canManage,
  canForceClose,
  onAct,
}: {
  m: TimelineMonth;
  scope: Scope;
  canManage: boolean;
  canForceClose: boolean;
  onAct: (kind: "close" | "reopen") => void;
}) {
  const closed = m.state === "closed";
  // Tutup fiskal langsung butuh finance.close.force; selain itu lewat "Cek & tutup aman".
  const closeBlocked = !closed && scope === "fiscal" && !canForceClose;
  const actionLabel = closed ? "Buka ulang" : closeBlocked ? "Gunakan tutup aman" : "Tutup";
  const disabled = !canManage || closeBlocked;
  const Icon = closed ? Lock : LockOpen;
  return (
    <li>
      <button
        type="button"
        disabled={disabled}
        onClick={() => onAct(closed ? "reopen" : "close")}
        aria-label={`${MONTH_NAMES[m.month - 1]} ${m.year}: ${closed ? "terkunci" : "terbuka"}. ${canManage && !closeBlocked ? actionLabel : ""}`}
        className={cn(
          "flex w-full flex-col items-start gap-1 rounded-lg border p-3 text-left transition-colors",
          closed ? "border-border bg-muted" : "border-success/25 bg-success-soft",
          !disabled && "hover:border-input",
          disabled && "cursor-default",
        )}
      >
        <span className="flex w-full items-center justify-between gap-2">
          <span className="text-sm font-semibold text-foreground">{MONTH_NAMES[m.month - 1]}</span>
          <Icon size={16} aria-hidden className={closed ? "text-muted-foreground" : "text-success"} />
        </span>
        <span className={cn("text-xs font-medium", closed ? "text-muted-foreground" : "text-success")}>{closed ? "Terkunci" : "Terbuka"}</span>
        <span className={cn("text-xs", closeBlocked ? "text-muted-foreground" : closed ? "text-primary" : "text-destructive")}>
          {canManage ? actionLabel : ""}
        </span>
      </button>
    </li>
  );
}

function Timeline({
  periods,
  scope,
  canManage,
  canForceClose,
  onAct,
}: {
  periods: FiscalPeriod[];
  scope: Scope;
  canManage: boolean;
  canForceClose: boolean;
  onAct: (a: PendingAction) => void;
}) {
  const currentYear = Number(todayWIB().slice(0, 4));
  const years = timelineYears(periods, currentYear);
  const [year, setYear] = useState(currentYear);
  const months = buildYearTimeline(year, periods);
  const yearId = `timeline-year-${scope}`;
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="w-36">
          <FormField label="Tahun" htmlFor={yearId}>
            <SelectInput id={yearId} value={year} onChange={(e) => setYear(Number(e.target.value))}>
              {years.map((y) => (
                <option key={y} value={y}>
                  {y}
                </option>
              ))}
            </SelectInput>
          </FormField>
        </div>
        <p className="text-sm text-muted-foreground">
          {closedCount(months)} dari 12 bulan terkunci pada {year}.
        </p>
      </div>
      <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6">
        {months.map((m) => (
          <MonthTile
            key={m.month}
            m={m}
            scope={scope}
            canManage={canManage}
            canForceClose={canForceClose}
            onAct={(kind) => onAct({ kind, scope, year: m.year, month: m.month })}
          />
        ))}
      </ul>
    </div>
  );
}

// Periode fiskal + pajak (T8). Kunci/buka bulan lewat ConfirmDialog → POST /periods/close|/reopen dan
// /tax-periods/close|/reopen {year, month} (gate finance.manage). Daftar server hanya memuat bulan yang
// pernah tersentuh; bulan lain tampil sebagai terbuka.
export default function PeriodsPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");
  // Tutup periode fiskal langsung (tanpa gerbang) = finance.close.force; tutup aman cukup finance.manage.
  const canForceClose = can("finance.close.force");

  const [tab, setTab] = useUrlTab(TABS, "fiskal");
  const { data, loading, error, reload } = useAsyncData(
    () =>
      Promise.all([financeService.periods(), financeService.taxPeriods()]).then(([p, t]) => ({
        periods: p ?? [],
        taxPeriods: t ?? [],
      })),
    [],
  );
  const periods = data?.periods ?? [];
  const taxPeriods = data?.taxPeriods ?? [];

  const now = todayWIB();
  const [safeYear, setSafeYear] = useState(Number(now.slice(0, 4)));
  const [safeMonth, setSafeMonth] = useState(Number(now.slice(5, 7)));
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [readiness, setReadiness] = useState<PeriodReadiness | null>(null);
  const [readinessLoading, setReadinessLoading] = useState(false);

  async function handleConfirm(reason = "") {
    if (!pending) return;
    const body = { year: pending.year, month: pending.month };
    const label = periodLabel(pending.year, pending.month);
    setConfirming(true);
    try {
      if (pending.kind === "safeclose") {
        const res = await financeService.safeClose(body);
        toast.fromServer(res.message, "Periode ditutup aman", label);
      } else if (pending.scope === "fiscal") {
        const res = pending.kind === "close" ? await financeService.closePeriod(body) : await financeService.reopenPeriod({ ...body, reason });
        toast.fromServer(res.message, pending.kind === "close" ? "Periode ditutup" : "Periode dibuka", label);
      } else {
        const res = pending.kind === "close" ? await financeService.closeTaxPeriod(body) : await financeService.reopenTaxPeriod({ ...body, reason });
        toast.fromServer(res.message, pending.kind === "close" ? "Periode ditutup" : "Periode dibuka", label);
      }
      setPending(null);
      setReadiness(null);
      reload();
    } catch (err) {
      toast.danger("Gagal memproses periode", toApiError(err).message);
    } finally {
      setConfirming(false);
    }
  }

  async function checkReadiness() {
    setReadinessLoading(true);
    try {
      const res = await financeService.readiness(safeYear, safeMonth);
      setReadiness(res);
      if (res.ready) setPending({ kind: "safeclose", scope: "fiscal", year: safeYear, month: safeMonth });
    } catch (err) {
      toast.danger("Gagal memeriksa kesiapan", toApiError(err).message);
    } finally {
      setReadinessLoading(false);
    }
  }

  const pendingLabel = pending ? periodLabel(pending.year, pending.month) : "";
  const pendingScope = pending?.scope === "tax" ? "pajak" : "fiskal";
  const currentYear = Number(now.slice(0, 4));

  return (
    <Page>
      <PageHeader title="Periode" description="Kunci bulan fiskal dan pajak agar jurnal tak bisa ditambah ke bulan tertutup." />

      {error ? (
        <ErrorState message={error} onRetry={reload} />
      ) : loading && !data ? (
        <TableSkeleton rows={4} cols={4} />
      ) : (
        <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])} className="gap-6">
          <TabsList variant="line">
            <TabsTrigger value="fiskal">Periode fiskal</TabsTrigger>
            <TabsTrigger value="pajak">Periode pajak</TabsTrigger>
          </TabsList>

          <TabsContent value="fiskal" className="flex flex-col gap-6">
            {canManage && (
              <SectionCard
                title="Cek dan tutup aman"
                description="Memeriksa gerbang wajib (antrean, jurnal seimbang, pemetaan, persediaan, jurnal dokumen batal) sebelum mengunci bulan."
              >
                <div className="flex flex-wrap items-end gap-4">
                  <FormField label="Tahun" htmlFor="safe-year">
                    <SelectInput id="safe-year" value={safeYear} onChange={(e) => setSafeYear(Number(e.target.value))}>
                      {[currentYear, currentYear - 1, currentYear - 2, currentYear - 3].map((y) => (
                        <option key={y} value={y}>
                          {y}
                        </option>
                      ))}
                    </SelectInput>
                  </FormField>
                  <FormField label="Bulan" htmlFor="safe-month">
                    <SelectInput id="safe-month" value={safeMonth} onChange={(e) => setSafeMonth(Number(e.target.value))}>
                      {MONTH_NAMES.map((n, i) => (
                        <option key={n} value={i + 1}>
                          {n}
                        </option>
                      ))}
                    </SelectInput>
                  </FormField>
                  <Button loading={readinessLoading} onClick={() => void checkReadiness()}>
                    Cek dan tutup aman
                  </Button>
                </div>
                {readiness && (
                  <div className="mt-5 flex flex-col gap-3">
                    <Notice
                      tone={readiness.ready ? "success" : "warning"}
                      title={
                        readiness.ready
                          ? readiness.warnings.length > 0
                            ? "Siap ditutup, dengan peringatan — konfirmasi di dialog"
                            : "Siap ditutup — konfirmasi di dialog"
                          : "Belum siap ditutup"
                      }
                    >
                      {!readiness.ready && (
                        <ul className="list-disc pl-5">
                          {(readiness.blockers ?? []).map((b) => (
                            <li key={b}>{b}</li>
                          ))}
                        </ul>
                      )}
                      {readiness.ready && (readiness.warnings ?? []).length === 0 && <span>Semua gerbang lolos tanpa peringatan.</span>}
                    </Notice>
                    <div className="overflow-hidden rounded-lg border border-border">
                      <DataTable<ReadinessCheck>
                        rows={readiness.checks ?? []}
                        rowKey={(c) => c.key}
                        emptyTitle="Tidak ada gerbang"
                        columns={[
                          { header: "Gerbang", render: (c) => c.label },
                          { header: "Status", render: (c) => <Badge tone={checkTone(c.status)}>{checkLabel(c.status)}</Badge> },
                          { header: "Keterangan", render: (c) => (c.status === "ok" ? <span className="text-muted-foreground">-</span> : c.detail) },
                        ]}
                      />
                    </div>
                    {(readiness.warnings ?? []).length > 0 && (
                      <Notice tone="warning" title="Peringatan (tidak menghalangi)">
                        <ul className="list-disc pl-5">
                          {readiness.warnings.map((w) => (
                            <li key={w}>{w}</li>
                          ))}
                        </ul>
                      </Notice>
                    )}
                  </div>
                )}
              </SectionCard>
            )}
            <SectionCard title="Periode fiskal" description="Bulan terkunci menolak jurnal baru (termasuk posting, manual, biaya). Pilih bulan untuk menutup atau membuka ulang.">
              <Timeline periods={periods} scope="fiscal" canManage={canManage} canForceClose={canForceClose} onAct={setPending} />
            </SectionCard>
          </TabsContent>

          <TabsContent value="pajak" className="flex flex-col gap-6">
            <SectionCard title="Periode pajak" description="Mengunci snapshot ringkasan pajak bulan tersebut. Pilih bulan untuk menutup atau membuka ulang.">
              <Timeline periods={taxPeriods} scope="tax" canManage={canManage} canForceClose={canForceClose} onAct={setPending} />
            </SectionCard>
          </TabsContent>
        </Tabs>
      )}

      {pending?.kind === "reopen" && (
        <ReopenPeriodModal
          periodLabel={pendingLabel}
          scopeLabel={pendingScope}
          submitting={confirming}
          onClose={() => (confirming ? undefined : setPending(null))}
          onSubmit={(reason) => void handleConfirm(reason)}
        />
      )}
      <ConfirmDialog
        open={pending !== null && pending.kind !== "reopen"}
        title={pending?.kind === "safeclose" ? "Tutup aman periode?" : "Tutup periode?"}
        message={
          pending?.kind === "safeclose"
            ? `Periode ${pendingLabel} lolos semua gerbang wajib dan akan dikunci. Lanjutkan?`
            : `Periode ${pendingLabel} (${pendingScope}) akan dikunci. Jurnal baru ke bulan ini akan ditolak. Lanjutkan?`
        }
        confirmLabel={confirming ? "Memproses…" : "Tutup periode"}
        tone="danger"
        onConfirm={() => void handleConfirm()}
        onCancel={() => (confirming ? undefined : setPending(null))}
      />
    </Page>
  );
}
