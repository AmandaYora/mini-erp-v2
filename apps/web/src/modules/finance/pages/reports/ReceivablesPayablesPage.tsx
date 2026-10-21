import { useId } from "react";
import { Badge, DataTable, FormField, SectionCard, SelectInput, SummaryCard } from "@/shared/components/ui";
import { DescriptionList, DocNumber, Money, Qty } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { formatDate, formatIDR, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { agingBucketRows, isOverdue } from "@/modules/finance/lib/reports";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ReportShell } from "@/modules/finance/components/ReportShell";
import { ExportCsvButton, ReportDateField, ReportToolbar } from "@/modules/finance/components/ReportToolbar";
import type { AgingDoc, AgingReport, PartyBalance, PartyBalanceReport } from "@/modules/finance/types";

// Saldo, bukan arus: yang ditanyakan "siapa masih berhutang per tanggal X", jadi hanya ada satu
// tanggal batas — rentang justru menyembunyikan faktur yang lebih tua.

type View = "parties" | "documents";

interface PartyView {
  docs: { ar: AgingReport; ap: AgingReport } | null;
  receivables: PartyBalanceReport | null;
  payables: PartyBalanceReport | null;
}

function PartyTable({ rows, loading, emptyTitle }: { rows: PartyBalance[]; loading: boolean; emptyTitle: string }) {
  return (
    <DataTable<PartyBalance>
      rows={rows}
      rowKey={(r) => String(r.partyId)}
      loading={loading}
      emptyTitle={emptyTitle}
      emptyDescription="Saldo terbuka muncul setelah dokumen diposting ke jurnal."
      columns={[
        { header: "Kode", render: (r) => r.partyCode || "-" },
        { header: "Nama", render: (r) => r.partyName || `#${r.partyId}` },
        { header: "Saldo", align: "right", render: (r) => <Money accounting value={r.balance} className="font-medium" /> },
      ]}
    />
  );
}

function AgingTable({ docs, loading, emptyTitle }: { docs: AgingDoc[]; loading: boolean; emptyTitle: string }) {
  return (
    <DataTable<AgingDoc>
      rows={docs}
      rowKey={(r) => r.orderId}
      loading={loading}
      emptyTitle={emptyTitle}
      columns={[
        { header: "Nomor", render: (r) => <DocNumber value={r.number} /> },
        { header: "Pihak", render: (r) => r.partyName || `#${r.partyId}` },
        { header: "Tanggal", render: (r) => formatDate(r.orderDate.slice(0, 10)) },
        {
          header: "Tempo",
          render: (r) => (
            <span className="inline-flex flex-wrap items-center gap-2">
              {r.dueDate ? formatDate(r.dueDate.slice(0, 10)) : "-"}
              {isOverdue(r) && <Badge tone="danger">{`Lewat ${r.overdueDays} hari`}</Badge>}
            </span>
          ),
        },
        { header: "Umur", align: "right", render: (r) => <Qty value={r.ageDays} unit="hari" /> },
        { header: "Total", align: "right", render: (r) => <Money value={r.grandTotal} /> },
        { header: "Dibayar", align: "right", render: (r) => <Money value={r.paid} /> },
        { header: "Sisa", align: "right", render: (r) => <Money value={r.outstanding} className="font-medium" /> },
      ]}
    />
  );
}

// Rekonsiliasi ke buku (F4): saldo akun kendali per tanggal (semua baris jurnal, dengan atau tanpa
// pihak) vs total dokumen terbuka per tanggal yang sama. Selisihnya ditampilkan, bukan disembunyikan.
function AgingReconciliation({ report }: { report: AgingReport }) {
  if (!report.accountCode) return null;
  return (
    <div className="mb-4 rounded-lg border border-border bg-muted/50 p-4">
      <DescriptionList
        items={[
          {
            label: `Saldo buku ${report.accountCode} ${report.accountName} per ${formatDate(report.asOf)}`,
            value: <Money value={report.glBalance} className="font-medium" />,
          },
          { label: "Total dokumen terbuka", value: <Money value={report.total} className="font-medium" /> },
          {
            label: "Selisih (saldo awal migrasi/penyesuaian tanpa dokumen)",
            value: <Money accounting value={report.difference} className="font-medium" />,
          },
        ]}
      />
      <p className="mt-3 text-xs text-muted-foreground">
        Selisih juga memuat dokumen yang belum diposting ke jurnal. Tampilan Per pihak hanya menjumlah baris jurnal yang
        bertanda pelanggan/pemasok.
      </p>
    </div>
  );
}

function AgingSection({ title, report, loading, emptyTitle }: { title: string; report: AgingReport | null; loading: boolean; emptyTitle: string }) {
  return (
    <SectionCard
      title={title}
      description="Dokumen bertanggal sampai tanggal ini, dikurangi pembayaran sampai tanggal yang sama; dikelompokkan menurut umur sejak tanggal dokumen."
    >
      {report && (
        <div className="mb-4 grid grid-cols-2 gap-4 lg:grid-cols-5">
          {agingBucketRows(report.buckets).map((b) => (
            <SummaryCard key={b.key} label={b.label} value={formatIDR(b.amount)} tone={b.key === "over90" && b.amount > 0 ? "danger" : "brand"} />
          ))}
          <SummaryCard label="Total" value={formatIDR(report.total)} tone="success" />
        </div>
      )}
      {report && <AgingReconciliation report={report} />}
      <div className="-mx-5 -mb-5">
        <AgingTable docs={report?.documents ?? []} loading={loading} emptyTitle={emptyTitle} />
      </div>
    </SectionCard>
  );
}

export default function ReceivablesPayablesPage() {
  const viewId = useId();
  const { draft, patch, applied, apply, version } = useDraftFilters({
    asOf: todayWIB(),
    view: "parties" as View,
    branch: "",
  });
  const { data, loading, error, reload } = useAsyncData<PartyView>(
    () =>
      applied.view === "documents"
        ? Promise.all([
            financeReportsService.receivableDocuments(applied.asOf),
            financeReportsService.payableDocuments(applied.asOf),
          ]).then(([ar, ap]) => ({ docs: { ar, ap }, receivables: null, payables: null }))
        : Promise.all([
            financeReportsService.receivables(applied.asOf, applied.branch),
            financeReportsService.payables(applied.asOf, applied.branch),
          ]).then(([ar, ap]) => ({ docs: null, receivables: ar, payables: ap })),
    [applied, version],
  );
  const receivables = data?.receivables ?? null;
  const payables = data?.payables ?? null;
  const docs = data?.docs ?? null;

  return (
    <ReportShell title="Piutang & Hutang" description="Siapa yang masih berutang dan kepada siapa kita berutang pada satu tanggal. Hanya baca.">
      <ReportToolbar
        onApply={apply}
        busy={loading}
        right={
          <ExportCsvButton
            filename={`piutang-hutang-${applied.asOf}.csv`}
            getRows={() => {
              if (receivables && payables) {
                return [
                  ["Jenis", "Kode", "Nama", "Saldo"],
                  ...receivables.parties?.map((p) => ["Piutang", p.partyCode, p.partyName, p.balance]) ?? [],
                  ...payables.parties?.map((p) => ["Hutang", p.partyCode, p.partyName, p.balance]) ?? [],
                ];
              }
              if (docs) {
                const mk = (kind: string, r: AgingReport) =>
                  (r.documents ?? []).map((d) => [kind, d.number, d.partyName, d.orderDate.slice(0, 10), d.dueDate?.slice(0, 10) ?? "", d.ageDays, d.grandTotal, d.paid, d.outstanding]);
                return [
                  ["Jenis", "Nomor", "Pihak", "Tanggal", "Tempo", "Umur (hari)", "Total", "Dibayar", "Sisa"],
                  ...mk("Piutang", docs.ar),
                  ...mk("Hutang", docs.ap),
                ];
              }
              return null;
            }}
          />
        }
      >
        <FormField label="Tampilan" htmlFor={viewId}>
          <SelectInput id={viewId} value={draft.view} onChange={(e) => patch({ view: e.target.value as View })}>
            <option value="parties">Per pihak</option>
            <option value="documents">Per dokumen (umur)</option>
          </SelectInput>
        </FormField>
        <ReportDateField label="Per tanggal" value={draft.asOf} onChange={(asOf) => patch({ asOf })} />
        {draft.view === "parties" && <BranchScopeSelect value={draft.branch} onChange={(branch) => patch({ branch })} />}
      </ReportToolbar>

      {error ? (
        <ErrorState message={error} onRetry={reload} />
      ) : applied.view === "documents" ? (
        <>
          <AgingSection title="Umur piutang (per dokumen)" report={docs?.ar ?? null} loading={loading} emptyTitle="Tidak ada piutang terbuka" />
          <AgingSection title="Umur hutang (per dokumen)" report={docs?.ap ?? null} loading={loading} emptyTitle="Tidak ada hutang terbuka" />
        </>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            <SummaryCard label="Total piutang" value={formatIDR(receivables?.total ?? 0)} tone="brand" />
            <SummaryCard label="Total hutang" value={formatIDR(payables?.total ?? 0)} tone="warning" />
          </div>
          <SectionCard flush title="Piutang usaha" description={`Per ${formatDate(applied.asOf)} — pelanggan yang masih punya saldo terbuka.`}>
            <PartyTable rows={receivables?.parties ?? []} loading={loading} emptyTitle="Tidak ada piutang terbuka" />
          </SectionCard>
          <SectionCard flush title="Hutang usaha" description={`Per ${formatDate(applied.asOf)} — pemasok yang masih harus dibayar.`}>
            <PartyTable rows={payables?.parties ?? []} loading={loading} emptyTitle="Tidak ada hutang terbuka" />
          </SectionCard>
        </>
      )}
    </ReportShell>
  );
}
