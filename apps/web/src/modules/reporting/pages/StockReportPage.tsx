import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import {
  Badge,
  DataTable,
  ErrorState,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  SummaryCard,
  ToggleGroup,
  ToggleGroupItem,
} from "@/shared/components/ui";
import { Page, Qty, Toolbar } from "@/shared/components/patterns";
import { formatNumber } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { stockItemPath } from "@/app/routes/route-paths";
import { reportingService } from "@/modules/reporting/services/reporting.service";
import type { StockReportRow } from "@/modules/reporting/types";

const LIMIT = 50;

export default function StockReportPage() {
  // `?critical=1` agar tautan "Semua stok kritis" dari dasbor langsung membuka daftar terfilter.
  const [params, setParams] = useSearchParams();
  const criticalOnly = params.get("critical") === "1";
  const [page, setPage] = useState(1);

  function setCriticalOnly(next: boolean) {
    setParams(
      (prev) => {
        const q = new URLSearchParams(prev);
        if (next) q.set("critical", "1");
        else q.delete("critical");
        return q;
      },
      { replace: true },
    );
    setPage(1);
  }

  const { data, loading, error, reload } = useAsyncData(
    (signal) => reportingService.stock({ criticalOnly, page, limit: LIMIT }, signal),
    [criticalOnly, page],
  );

  return (
    <Page>
      <PageHeader
        title="Laporan Stok"
        description="Posisi stok total produk yang dipantau di cabang aktif: fisik, dipesan, tersedia, dan batas minimum."
      />

      <Toolbar
        primary={
          <FormField label="Tampilkan">
            <ToggleGroup
              type="single"
              variant="outline"
              value={criticalOnly ? "critical" : "all"}
              onValueChange={(v) => v && setCriticalOnly(v === "critical")}
              aria-label="Tampilkan"
            >
              <ToggleGroupItem value="all">Semua dipantau</ToggleGroupItem>
              <ToggleGroupItem value="critical">Hanya stok kritis</ToggleGroupItem>
            </ToggleGroup>
          </FormField>
        }
      />

      {error ? (
        <ErrorState title="Gagal memuat laporan stok" message={error} onRetry={reload} />
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SummaryCard
              label={criticalOnly ? "Produk kritis" : "Produk dipantau"}
              value={data ? formatNumber(data.total) : "-"}
              tone={criticalOnly ? "danger" : "brand"}
            />
          </div>
          <SectionCard title="Posisi stok" description="Angka dalam satuan dasar produk." flush>
            <DataTable<StockReportRow>
              rows={data?.rows}
              rowKey={(r) => r.productId}
              loading={loading}
              emptyTitle={criticalOnly ? "Stok aman" : "Belum ada data stok"}
              emptyDescription={criticalOnly ? "Tidak ada produk di bawah minimum." : undefined}
              columns={[
                {
                  header: "Produk",
                  render: (r) => (
                    <Link to={stockItemPath(r.productId)} className="hover:underline">
                      <span className="font-semibold text-primary">{r.productCode}</span>{" "}
                      <span className="text-muted-foreground">{r.productName}</span>
                    </Link>
                  ),
                },
                { header: "Fisik", align: "right", render: (r) => <Qty value={r.onHand} /> },
                { header: "Dipesan", align: "right", render: (r) => <Qty value={r.reserved} /> },
                { header: "Tersedia", align: "right", render: (r) => <Qty value={r.available} /> },
                { header: "Minimum", align: "right", render: (r) => <Qty value={r.minStock} /> },
                {
                  header: "Status",
                  render: (r) =>
                    r.critical ? (
                      <Badge tone={r.available <= 0 ? "danger" : "warning"}>{r.available <= 0 ? "Habis" : "Kritis"}</Badge>
                    ) : (
                      <Badge tone="success">Aman</Badge>
                    ),
                },
              ]}
            />
            {data && <Pagination page={page} limit={LIMIT} total={data.total} onPageChange={setPage} />}
          </SectionCard>
        </>
      )}
    </Page>
  );
}
