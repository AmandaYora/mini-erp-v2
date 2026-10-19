import { useState } from "react";
import { Download } from "lucide-react";
import { Badge, Button, DataTable, Pagination, SectionCard, SummaryCard } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { formatDate, formatIDR, formatNumber, monthStartWIB, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import { useDraftFilters } from "@/modules/finance/hooks/use-draft-filters";
import { defaultYearMonth } from "@/modules/finance/lib/report-periods";
import { pageTaxTotals, taxKindLabel, taxKindTone } from "@/modules/finance/lib/tax-detail";
import { AccountingTable } from "@/modules/finance/components/AccountingTable";
import type { AccountingColumn } from "@/modules/finance/components/AccountingTable";
import { BranchScopeSelect } from "@/modules/finance/components/BranchScopeSelect";
import { ReportShell } from "@/modules/finance/components/ReportShell";
import { ReportDateField, ReportToolbar, YearMonthFields } from "@/modules/finance/components/ReportToolbar";
import { periodLabel } from "@/modules/finance/types";
import type { TaxDetailRow, TaxException } from "@/modules/finance/types";

const LIMIT = 200;

type Line = TaxDetailRow & { total?: boolean };

export default function TaxDetailPage() {
  const can = useAuthStore((s) => s.can);
  const canExport = can("finance.report.view");

  const ym = defaultYearMonth();
  const { draft, patch, applied, apply, version } = useDraftFilters({ year: ym.year, month: ym.month, branch: "" });
  const [page, setPage] = useState(1);
  const { data, loading, error, reload } = useAsyncData(
    () => financeReportsService.taxDetail(applied.year, applied.month, page, LIMIT, applied.branch),
    [applied, version, page],
  );
  const rows = data?.items ?? [];
  const total = data?.meta.total ?? 0;
  // Ringkasan bulan yang sama (FN-06): DPP jual/beli dan transaksi kena pajak tanpa faktur.
  const { data: summary } = useAsyncData(
    () => financeReportsService.taxSummary(applied.year, applied.month, applied.branch, 1, 20),
    [applied, version],
  );
  const exceptionTotal = summary?.exceptionsMeta?.total ?? 0;
  const [pkgFrom, setPkgFrom] = useState(monthStartWIB());
  const [pkgTo, setPkgTo] = useState(todayWIB());
  const [downloading, setDownloading] = useState(false);
  const [downloadingPack, setDownloadingPack] = useState<"actual" | "capped" | null>(null);

  async function handleDownload() {
    setDownloading(true);
    try {
      await financeReportsService.downloadTaxDetailCsv(applied.year, applied.month);
      toast.success("Berkas CSV diunduh");
    } catch (err) {
      toast.danger("Gagal mengunduh CSV", toApiError(err).message);
    } finally {
      setDownloading(false);
    }
  }

  // Dua paket TERPISAH, bukan satu berkas dengan saklar. Kedua paket memuat sheet laporan yang sama
  // (Untung Rugi, Posisi Harta Hutang, Cek Saldo Akun, Rincian PPN, PPN Masukan, Rekonsiliasi Fiskal,
  // Buku Besar). "Data riil" memakai pembukuan apa adanya; "peredaran terbatas" memakai peredaran bruto
  // dibatasi plafon PP23 dan menambah dua sheet: Peredaran Bruto dan Transaksi Dikecualikan.
  async function handleDownloadPackage(variant: "actual" | "capped") {
    setDownloadingPack(variant);
    try {
      await financeReportsService.downloadTaxPackage(applied.year, applied.month, variant, applied.branch);
      toast.success(variant === "capped" ? "Paket peredaran terbatas diunduh" : "Paket data riil diunduh");
    } catch (err) {
      toast.danger("Gagal mengunduh paket pajak", toApiError(err).message);
    } finally {
      setDownloadingPack(null);
    }
  }

  // Paket untuk rentang tanggal bebas (FN-04).
  async function handleDownloadRange(variant: "actual" | "capped") {
    setDownloadingPack(variant);
    try {
      await financeReportsService.downloadTaxPackageRange(pkgFrom, pkgTo, variant, applied.branch);
      toast.success(variant === "capped" ? "Paket peredaran terbatas diunduh" : "Paket data riil diunduh");
    } catch (err) {
      toast.danger("Gagal mengunduh paket pajak", toApiError(err).message);
    } finally {
      setDownloadingPack(null);
    }
  }

  const totals = pageTaxTotals(rows);
  const display: Line[] = rows.length > 0
    ? [
        ...rows,
        {
          entryId: 0, kind: "", ppnIn: totals.ppnIn, date: "", number: "Total halaman ini", memo: "",
          revenue: totals.revenue, ppn: totals.ppnOut, taxInvoiceNumber: "", taxInvoiceDate: "", dpp: totals.dpp, total: true,
        },
      ]
    : rows;

  const columns: AccountingColumn<Line>[] = [
    { id: "date", header: "Tanggal", width: "8rem", render: (r) => (r.total ? "" : formatDate(r.date)) },
    { id: "number", header: "Nomor", width: "11rem", render: (r) => (r.total ? r.number : <span className="font-mono text-xs">{r.number || "-"}</span>) },
    { id: "kind", header: "Jenis", width: "8rem", render: (r) => (r.total ? "" : <Badge tone={taxKindTone(r.kind)}>{taxKindLabel(r.kind)}</Badge>) },
    { id: "memo", header: "Keterangan", render: (r) => (r.total ? "" : r.memo || "-") },
    {
      id: "inv",
      header: "No. faktur",
      width: "10rem",
      render: (r) =>
        r.total ? (
          ""
        ) : r.missingInvoice ? (
          <Badge tone="warning">Tanpa faktur</Badge>
        ) : (
          r.taxInvoiceNumber || "-"
        ),
    },
    { id: "invDate", header: "Tgl. faktur", width: "8rem", render: (r) => (r.total ? "" : formatDate(r.taxInvoiceDate)) },
    { id: "revenue", header: "Omzet", align: "right", width: "10rem", render: (r) => <Money accounting value={r.revenue} /> },
    { id: "dpp", header: "DPP", align: "right", width: "10rem", render: (r) => <Money accounting value={r.dpp ?? 0} /> },
    { id: "ppn", header: "PPN keluaran", align: "right", width: "10rem", render: (r) => <Money accounting value={r.ppn} /> },
    { id: "ppnIn", header: "PPN masukan", align: "right", width: "10rem", render: (r) => <Money accounting value={r.ppnIn} /> },
  ];

  return (
    <ReportShell
      title="Rincian PPN"
      description="Kertas kerja SPT per faktur. Hanya baca."
      actions={
        canExport ? (
          <>
            <Button variant="secondary" size="sm" loading={downloading} onClick={() => void handleDownload()}>
              <Download aria-hidden /> Unduh CSV
            </Button>
            <Button
              variant="secondary"
              size="sm"
              loading={downloadingPack === "capped"}
              disabled={downloadingPack !== null}
              onClick={() => void handleDownloadPackage("capped")}
              title="Sheet sama dengan paket data riil, peredaran bruto dibatasi plafon Rp4,8 M per tahun pajak (seluruh cabang), plus sheet Peredaran Bruto dan Transaksi Dikecualikan"
            >
              Paket peredaran terbatas
            </Button>
            <Button
              size="sm"
              loading={downloadingPack === "actual"}
              disabled={downloadingPack !== null}
              onClick={() => void handleDownloadPackage("actual")}
              title="Pembukuan komersial apa adanya"
            >
              Paket data riil
            </Button>
          </>
        ) : undefined
      }
    >
      <ReportToolbar
        busy={loading}
        onApply={() => {
          setPage(1);
          apply();
        }}
      >
        <YearMonthFields year={draft.year} month={draft.month} onYear={(year) => patch({ year })} onMonth={(month) => patch({ month })} />
        <BranchScopeSelect value={draft.branch} onChange={(branch) => patch({ branch })} />
      </ReportToolbar>

      {summary && (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
          <SummaryCard label="DPP penjualan" value={formatIDR(summary.salesDpp ?? 0)} tone="brand" />
          <SummaryCard label="PPN keluaran" value={formatIDR(summary.ppnOut)} tone="brand" />
          <SummaryCard label="DPP pembelian" value={formatIDR(summary.purchaseDpp ?? 0)} tone="brand" />
          <SummaryCard label="PPN masukan" value={formatIDR(summary.ppnIn)} tone="brand" />
          <SummaryCard
            label="Tanpa faktur"
            value={formatNumber(exceptionTotal)}
            tone={exceptionTotal > 0 ? "warning" : "success"}
            hint={exceptionTotal > 0 ? "Transaksi kena pajak tanpa nomor faktur" : "Semua kena pajak berfaktur"}
          />
        </div>
      )}

      {canExport && (
        <SectionCard title="Paket pajak rentang bebas" description="Unduh paket untuk rentang tanggal apa pun (kuartal, tahun, atau rentang sendiri). Varian terbatas harus berada dalam satu tahun pajak. Cakupan cabang mengikuti pilihan Cabang di atas (semua cabang bila izin ada).">
          <div className="flex flex-wrap items-end gap-4">
            <ReportDateField label="Dari" value={pkgFrom} onChange={setPkgFrom} />
            <ReportDateField label="Sampai" value={pkgTo} onChange={setPkgTo} />
            <Button variant="secondary" size="sm" loading={downloadingPack === "capped"} disabled={downloadingPack !== null} onClick={() => void handleDownloadRange("capped")}>
              Paket peredaran terbatas
            </Button>
            <Button size="sm" loading={downloadingPack === "actual"} disabled={downloadingPack !== null} onClick={() => void handleDownloadRange("actual")}>
              Paket data riil
            </Button>
          </div>
        </SectionCard>
      )}

      {exceptionTotal > 0 && (
        <SectionCard flush title="Kena pajak tanpa faktur" description="Pesanan berpajak yang nomor fakturnya belum diisi.">
          <DataTable<TaxException>
            caption="Transaksi kena pajak tanpa faktur"
            rows={summary?.exceptions ?? []}
            rowKey={(r) => `${r.kind}-${r.orderId}-${r.date}`}
            emptyTitle="Tidak ada"
            columns={[
              { header: "Tanggal", render: (r) => formatDate(r.date) },
              { header: "Jenis", render: (r) => (r.kind === "purchase" ? "Pembelian" : "Penjualan") },
              { header: "Pesanan", render: (r) => <span className="font-mono text-xs">{r.orderNumber || "-"}</span> },
              { header: "Pihak", render: (r) => r.partyName || "-" },
              { header: "DPP", align: "right", render: (r) => <Money value={r.dpp} /> },
              { header: "PPN", align: "right", render: (r) => <Money value={r.ppn} /> },
            ]}
          />
        </SectionCard>
      )}

      <SectionCard flush title={`Rincian PPN — ${periodLabel(applied.year, applied.month)}`}>
        <AccountingTable<Line>
          caption="Rincian PPN per faktur"
          columns={columns}
          rows={display}
          rowKey={(r) => (r.total ? "total" : `${r.entryId}-${r.kind}-${r.number}`)}
          rowKind={(r) => (r.total ? "total" : "line")}
          loading={loading}
          error={error}
          onRetry={reload}
          minWidth="74rem"
          emptyTitle="Belum ada faktur berpajak"
          emptyDescription="Tidak ada penyerahan kena pajak pada periode ini."
        />
        <Pagination page={page} limit={LIMIT} total={total} onPageChange={setPage} />
      </SectionCard>
    </ReportShell>
  );
}
