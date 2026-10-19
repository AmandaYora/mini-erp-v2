import { DataTable, SectionCard } from "@/shared/components/ui";
import { Money, Qty } from "@/shared/components/patterns";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ReportToolbar } from "@/modules/finance/components/ReportToolbar";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { AccountingTable } from "@/modules/finance/components/AccountingTable";
import type { AccountingColumn } from "@/modules/finance/components/AccountingTable";
import { ReportShell } from "@/modules/finance/components/ReportShell";
import { ExportCsvButton } from "@/modules/finance/components/ReportToolbar";
import type { BranchValue, InventoryPosition } from "@/modules/finance/types";

type Line = InventoryPosition & { total?: boolean };

export default function InventoryValuePage() {
  const { draft, patch, applied, apply, version } = useDraftFilters({ branch: "" });
  const { data: report, loading, error, reload } = useAsyncData(
    () => financeReportsService.inventoryValue(applied.branch),
    [applied, version],
  );
  const byBranch = report?.byBranch ?? [];
  const positions = report?.positions ?? [];
  const display: Line[] = report && positions.length > 0
    ? [
        ...positions,
        { productId: 0, variantId: 0, productCode: "", productName: "Total nilai", qty: 0, avgCost: 0, value: report.total, total: true },
      ]
    : positions;

  const columns: AccountingColumn<Line>[] = [
    { id: "code", header: "Kode", width: "9rem", render: (r) => (r.total ? "" : r.productCode || "-") },
    { id: "name", header: "Produk", render: (r) => r.productName || "-" },
    { id: "qty", header: "Qty", align: "right", width: "8rem", render: (r) => (r.total ? "" : <Qty value={r.qty} />) },
    { id: "avg", header: "Rata-rata", align: "right", width: "10rem", render: (r) => (r.total ? "" : <Money accounting value={r.avgCost} />) },
    { id: "value", header: "Nilai", align: "right", width: "11rem", render: (r) => <Money accounting value={r.value} /> },
  ];

  return (
    <ReportShell title="Nilai Persediaan" description="Setiap posisi stok dinilai pada rata-rata bergerak. Hanya baca.">
      <ReportToolbar
        onApply={apply}
        busy={loading}
        right={
          <ExportCsvButton
            filename="nilai-persediaan.csv"
            getRows={() =>
              report
                ? [
                    ["Kode", "Produk", "Qty", "Rata-rata", "Nilai"],
                    ...positions.map((r) => [r.productCode, r.productName, r.qty, r.avgCost, r.value]),
                    ["", "Total nilai", "", "", report.total],
                    ...(byBranch.length > 1 ? byBranch.map((b) => ["", `Cabang ${b.branchName}`, "", "", b.total]) : []),
                  ]
                : null
            }
          />
        }
      >
        <BranchScopeSelect value={draft.branch} onChange={(branch) => patch({ branch })} />
      </ReportToolbar>
      {byBranch.length > 1 && (
        <SectionCard flush title="Per cabang" description="Pecahan nilai persediaan; jumlahnya sama dengan total.">
          <DataTable<BranchValue>
            caption="Nilai persediaan per cabang"
            rows={byBranch}
            rowKey={(b) => b.branchId}
            emptyTitle="Tidak ada"
            columns={[
              { header: "Cabang", render: (b) => b.branchName || `#${b.branchId}` },
              { header: "Posisi", align: "right", render: (b) => b.positions },
              { header: "Nilai", align: "right", render: (b) => <Money accounting value={b.total} /> },
            ]}
          />
        </SectionCard>
      )}
      <SectionCard flush title="Nilai Persediaan" description="Posisi stok saat ini.">
        <AccountingTable<Line>
          caption="Nilai persediaan per produk"
          columns={columns}
          rows={display}
          rowKey={(r) => (r.total ? "total" : `${r.productId}-${r.variantId}-${r.productCode}`)}
          rowKind={(r) => (r.total ? "total" : "line")}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Persediaan kosong"
          emptyDescription="Tidak ada posisi berstok saat ini."
        />
      </SectionCard>
    </ReportShell>
  );
}
