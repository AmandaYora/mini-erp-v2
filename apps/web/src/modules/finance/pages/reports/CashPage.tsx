import { SectionCard } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, monthStartWIB, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { cashFlowTotals, cashRowReconciles } from "@/modules/finance/lib/reports";
import { AccountingTable } from "@/modules/finance/components/AccountingTable";
import type { AccountingColumn } from "@/modules/finance/components/AccountingTable";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ReportShell } from "@/modules/finance/components/ReportShell";
import { ExportCsvButton, ReportDateField, ReportToolbar } from "@/modules/finance/components/ReportToolbar";
import type { CashFlowRow } from "@/modules/finance/types";

type Line = CashFlowRow & { total?: boolean };

// Laporan kas punya rute sendiri (/finance/reports/cash): datanya memakai rentang tanggal
// (saldo awal + masuk − keluar = saldo akhir), berbeda dari Neraca yang satu tanggal.
export default function CashPage() {
  const { draft, patch, applied, apply, version } = useDraftFilters({
    from: monthStartWIB(),
    to: todayWIB(),
    branch: "",
  });
  const { data: report, loading, error, reload } = useAsyncData(
    () => financeReportsService.cashFlow(applied.from, applied.to, applied.branch),
    [applied, version],
  );

  const rows = report?.rows ?? [];
  const totals = cashFlowTotals(rows);
  const mismatched = rows.some((r) => !cashRowReconciles(r));
  const display: Line[] = report && rows.length > 0
    ? [
        ...rows,
        { code: "", name: "Total", opening: report.openingTotal, in: totals.in, out: totals.out, closing: report.closingTotal, total: true },
      ]
    : rows;

  const columns: AccountingColumn<Line>[] = [
    {
      id: "account",
      header: "Rekening",
      render: (r) =>
        r.total ? (
          r.name
        ) : (
          <span>
            <span className="font-medium">{r.code}</span> <span className="text-muted-foreground">{r.name}</span>
          </span>
        ),
    },
    { id: "opening", header: "Saldo awal", align: "right", width: "10rem", render: (r) => <Money accounting value={r.opening} /> },
    { id: "in", header: "Masuk", align: "right", width: "10rem", render: (r) => <Money accounting value={r.in} /> },
    { id: "out", header: "Keluar", align: "right", width: "10rem", render: (r) => <Money accounting value={r.out} /> },
    { id: "closing", header: "Saldo akhir", align: "right", width: "10rem", render: (r) => <Money accounting value={r.closing} /> },
  ];

  return (
    <ReportShell title="Kas" description="Saldo awal, kas masuk, kas keluar, dan saldo akhir per rekening kas atau bank. Hanya baca.">
      <ReportToolbar
        onApply={apply}
        busy={loading}
        right={
          <ExportCsvButton
            filename={`kas-${applied.from}_${applied.to}.csv`}
            getRows={() =>
              report
                ? [
                    ["Kode", "Rekening", "Saldo awal", "Masuk", "Keluar", "Saldo akhir"],
                    ...rows.map((r) => [r.code, r.name, r.opening, r.in, r.out, r.closing]),
                    ["", "Total", report.openingTotal, totals.in, totals.out, report.closingTotal],
                  ]
                : null
            }
          />
        }
      >
        <ReportDateField label="Dari" value={draft.from} onChange={(from) => patch({ from })} />
        <ReportDateField label="Sampai" value={draft.to} onChange={(to) => patch({ to })} />
        <BranchScopeSelect value={draft.branch} onChange={(branch) => patch({ branch })} />
      </ReportToolbar>

      {mismatched && (
        <Notice tone="warning" title="Saldo awal + masuk − keluar tidak sama dengan saldo akhir">
          Periksa integritas pembukuan di halaman Integritas.
        </Notice>
      )}

      <SectionCard
        flush
        title={`Kas — ${formatDate(applied.from)} s/d ${formatDate(applied.to)}`}
        description="Saldo awal (sebelum tanggal awal) + masuk (debit) − keluar (kredit) = saldo akhir."
      >
        <AccountingTable<Line>
          caption="Laporan kas per rekening"
          columns={columns}
          rows={display}
          rowKey={(r) => (r.total ? "total" : r.code)}
          rowKind={(r) => (r.total ? "total" : "line")}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Tidak ada rekening kas"
          emptyDescription="Belum ada akun bertanda kas atau bank yang tersentuh."
        />
      </SectionCard>
    </ReportShell>
  );
}
