import { Badge, CardSkeleton, SectionCard, SummaryCard } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { formatDate, formatIDR, formatNumber } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeService } from "@/modules/finance/services/finance.service";
import { dayStatusLabel, dayStatusTone } from "@/modules/finance/lib/daily-overview";
import { docTypeLabel } from "@/modules/finance/types";
import type { OverviewTask } from "@/modules/finance/types";

interface DailyOverviewCardProps {
  date: string;
  /** Berubah setiap antrian dimuat ulang, sehingga ikhtisar ikut disegarkan. */
  refreshKey: unknown;
}

function TaskGroup({ task }: { task: OverviewTask }) {
  return (
    <li className="rounded-lg border border-border p-3">
      <p className="flex flex-wrap items-center gap-2 text-sm font-medium text-foreground">
        {task.label} <Badge tone="warning">{formatNumber(task.count)}</Badge>
      </p>
      <p className="mt-1 text-xs text-muted-foreground">{task.hint}</p>
      <ul className="mt-2 flex flex-col gap-1 text-sm">
        {task.items.slice(0, 5).map((it) => (
          <li key={`${it.docType}:${it.docId}`} className="flex flex-wrap items-baseline justify-between gap-2">
            <span>
              <span className="font-mono text-xs">{it.number}</span>{" "}
              <span className="text-muted-foreground">
                {docTypeLabel(it.docType)}
                {it.partyName ? ` · ${it.partyName}` : ""}
              </span>
            </span>
            <span className="text-xs text-muted-foreground">{it.message}</span>
          </li>
        ))}
        {task.items.length > 5 && <li className="text-xs text-muted-foreground">+{task.items.length - 5} dokumen lain</li>}
      </ul>
    </li>
  );
}

// Ikhtisar tutup hari (FN-02): angka hari itu dari jurnal, status hari, tugas "butuh data pemilik"
// per akar masalah, dan backlog hari sebelumnya (berdiri sendiri, tidak mengubah status hari ini).
export function DailyOverviewCard({ date, refreshKey }: DailyOverviewCardProps) {
  const { data, loading, error, reload } = useAsyncData(
    () => (date ? financeService.dailyOverview(date) : Promise.resolve(null)),
    [date, refreshKey],
  );

  return (
    <SectionCard title="Ikhtisar hari" description={date ? formatDate(date) : "Pilih tanggal"}>
      {error ? (
        <ErrorState message={error} onRetry={reload} title="Gagal memuat ikhtisar" />
      ) : loading && !data ? (
        <CardSkeleton className="h-32 border-0 p-0" />
      ) : data ? (
        <div className="flex flex-col gap-4">
          <p className="flex items-center gap-2 text-sm">
            Status hari: <Badge tone={dayStatusTone(data.status)}>{dayStatusLabel(data.status)}</Badge>
            <span className="text-muted-foreground">{formatNumber(data.activity)} transaksi</span>
          </p>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
            <SummaryCard label="Penjualan" value={formatIDR(data.totals.sales.amount)} hint={`${formatNumber(data.totals.sales.count)} jurnal`} tone="brand" />
            <SummaryCard label="Pembelian" value={formatIDR(data.totals.purchases.amount)} hint={`${formatNumber(data.totals.purchases.count)} jurnal`} tone="brand" />
            <SummaryCard label="Retur penjualan" value={formatIDR(data.totals.returns.amount)} hint={`${formatNumber(data.totals.returns.count)} jurnal`} tone="warning" />
            <SummaryCard label="Kas masuk" value={formatIDR(data.totals.cashIn)} tone="success" />
            <SummaryCard label="Kas keluar" value={formatIDR(data.totals.cashOut)} tone="danger" />
          </div>
          {data.dataWarning.invalidDateCount > 0 && (
            <Notice tone="warning" title={`${data.dataWarning.invalidDateCount} dokumen bertanggal rusak/kosong`}>
              Dokumen ini ikut setiap tutup hari dan perlu diperiksa pemilik.
            </Notice>
          )}
          {data.tasks.length > 0 && (
            <div>
              <h3 className="mb-2 text-sm font-semibold text-foreground">Butuh data pemilik hari ini</h3>
              <ul className="flex flex-col gap-2">
                {data.tasks.map((t) => (
                  <TaskGroup key={t.code} task={t} />
                ))}
              </ul>
            </div>
          )}
          {data.backlog.count > 0 && (
            <div>
              <h3 className="mb-2 text-sm font-semibold text-foreground">
                Backlog hari sebelumnya <Badge tone="warning">{formatNumber(data.backlog.count)}</Badge>
              </h3>
              <p className="mb-2 text-xs text-muted-foreground">
                {formatNumber(data.backlog.ready)} siap diposting, {formatNumber(data.backlog.needsInput)} butuh data pemilik.
                {data.ignored > 0 ? ` ${formatNumber(data.ignored)} dokumen diabaikan.` : ""}
              </p>
              <ul className="flex flex-col gap-2">
                {data.backlog.tasks.map((t) => (
                  <TaskGroup key={t.code} task={t} />
                ))}
              </ul>
            </div>
          )}
          {data.probeCapped && (
            <p className="text-xs text-muted-foreground">Pengelompokan tugas memeriksa {formatNumber(data.probed)} dokumen pertama; hitungan lain tetap penuh.</p>
          )}
          <p className="text-xs text-muted-foreground">
            Angka dihitung dari jurnal bertanggal hari itu: penjualan = pendapatan dari surat jalan, pembelian = persediaan dari penerimaan, kas dari pembayaran.{" "}
            <Money value={data.totals.cashIn - data.totals.cashOut} accounting /> arus kas bersih.
          </p>
        </div>
      ) : null}
    </SectionCard>
  );
}
