import { useId } from "react";
import { Badge, FormField, SectionCard, SelectInput } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { formatDate, todayWIB } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { defaultYearMonth } from "@/modules/finance/lib/report-periods";
import { AccountingTable } from "@/modules/finance/components/AccountingTable";
import type { AccountingColumn } from "@/modules/finance/components/AccountingTable";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ReportShell } from "@/modules/finance/components/ReportShell";
import {
  ExportCsvButton,
  ReportDateField,
  ReportToolbar,
  YearMonthFields,
} from "@/modules/finance/components/ReportToolbar";
import { accountTypeLabel, periodLabel } from "@/modules/finance/types";
import type { TrialRow } from "@/modules/finance/types";

type Mode = "month" | "asof";
type Line = TrialRow & { total?: boolean };

export default function TrialBalancePage() {
  const ym = defaultYearMonth();
  const { draft, patch, applied, apply, version } = useDraftFilters({
    mode: "month" as Mode,
    year: ym.year,
    month: ym.month,
    asOf: todayWIB(),
    branch: "",
  });
  const modeId = useId();

  const { data: report, loading, error, reload } = useAsyncData(
    () =>
      applied.mode === "asof"
        ? financeReportsService.trialBalanceAsOf(applied.asOf, applied.branch)
        : financeReportsService.trialBalance(applied.year, applied.month, applied.branch),
    [applied, version],
  );

  const rows: Line[] = (report?.rows ?? []).map((r) => ({ ...r }));
  const display: Line[] = report && rows.length > 0
    ? [...rows, { code: "", name: "Total", type: "", debit: report.totalDebit, credit: report.totalCredit, total: true }]
    : rows;

  const columns: AccountingColumn<Line>[] = [
    { id: "code", header: "Kode", width: "7rem", render: (r) => (r.total ? "" : r.code) },
    { id: "name", header: "Akun", render: (r) => r.name },
    { id: "type", header: "Tipe", width: "9rem", render: (r) => (r.total ? "" : <Badge tone="neutral">{accountTypeLabel(r.type)}</Badge>) },
    { id: "debit", header: "Debit", align: "right", width: "11rem", render: (r) => <Money accounting value={r.debit} /> },
    { id: "credit", header: "Kredit", align: "right", width: "11rem", render: (r) => <Money accounting value={r.credit} /> },
  ];

  const scope =
    applied.mode === "asof"
      ? `kumulatif s/d ${formatDate(applied.asOf)}`
      : periodLabel(applied.year, applied.month);

  return (
    <ReportShell title="Neraca Saldo" description="Saldo debit dan kredit seluruh akun yang bergerak. Hanya baca.">
      <ReportToolbar
        onApply={apply}
        busy={loading}
        right={
          <ExportCsvButton
            filename={`neraca-saldo-${applied.mode === "asof" ? applied.asOf : `${applied.year}-${String(applied.month).padStart(2, "0")}`}.csv`}
            getRows={() =>
              report
                ? [
                    ["Kode", "Akun", "Tipe", "Debit", "Kredit"],
                    ...rows.map((r) => [r.code, r.name, accountTypeLabel(r.type), r.debit, r.credit]),
                    ["", "Total", "", report.totalDebit, report.totalCredit],
                  ]
                : null
            }
          />
        }
      >
        <FormField label="Mode" htmlFor={modeId}>
          <SelectInput id={modeId} value={draft.mode} onChange={(e) => patch({ mode: e.target.value as Mode })}>
            <option value="month">Bulanan</option>
            <option value="asof">Kumulatif s/d tanggal</option>
          </SelectInput>
        </FormField>
        {draft.mode === "month" ? (
          <YearMonthFields
            year={draft.year}
            month={draft.month}
            onYear={(year) => patch({ year })}
            onMonth={(month) => patch({ month })}
          />
        ) : (
          <ReportDateField label="Per tanggal" value={draft.asOf} onChange={(asOf) => patch({ asOf })} />
        )}
        <BranchScopeSelect value={draft.branch} onChange={(branch) => patch({ branch })} />
      </ReportToolbar>

      <SectionCard
        flush
        title={`Neraca Saldo — ${scope}`}
        actions={
          report ? (
            <Badge tone={report.balanced ? "success" : "danger"}>{report.balanced ? "Seimbang ✓" : "Tidak seimbang"}</Badge>
          ) : undefined
        }
      >
        <AccountingTable<Line>
          caption="Neraca saldo"
          columns={columns}
          rows={display}
          rowKey={(r) => (r.total ? "total" : r.code)}
          rowKind={(r) => (r.total ? "total" : "line")}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Belum ada transaksi"
          emptyDescription="Tidak ada akun yang tersentuh pada periode ini."
        />
      </SectionCard>
    </ReportShell>
  );
}
