import { useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  DataTable,
  FilterBar,
  FormField,
  PageHeader,
  SectionCard,
  SelectInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatIDR, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { financeReportsService } from "@/modules/finance/services/reports.service";
import type { TaxDetailRow } from "@/modules/finance/types";
import { periodLabel } from "@/modules/finance/types";

const MONTHS = [
  "Januari",
  "Februari",
  "Maret",
  "April",
  "Mei",
  "Juni",
  "Juli",
  "Agustus",
  "September",
  "Oktober",
  "November",
  "Desember",
];

function defaultYearMonth(): { year: number; month: number } {
  const today = todayWIB();
  return { year: Number(today.slice(0, 4)), month: Number(today.slice(5, 7)) };
}

function yearOptions(): number[] {
  const y = defaultYearMonth().year;
  return [y, y - 1, y - 2, y - 3, y - 4];
}

export default function TaxDetailPage() {
  const can = useAuthStore((s) => s.can);
  const canExport = can("finance.view");

  const initial = defaultYearMonth();
  const [year, setYear] = useState(initial.year);
  const [month, setMonth] = useState(initial.month);
  const { data, loading, error, reload } = useAsyncData(
    () =>
      financeReportsService.taxDetail(year, month).catch((err) => {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat rincian pajak", apiErr.message);
        throw err;
      }),
    [year, month],
  );
  const rows = data ?? [];
  const [downloading, setDownloading] = useState(false);
  const [downloadingPack, setDownloadingPack] = useState<"actual" | "capped" | null>(null);

  function apply(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    reload();
  }

  async function handleDownload() {
    setDownloading(true);
    try {
      await financeReportsService.downloadTaxDetailCsv(year, month);
      toast.success("Berkas CSV diunduh");
    } catch (err) {
      toast.danger("Gagal mengunduh CSV", toApiError(err).message);
    } finally {
      setDownloading(false);
    }
  }

  // Dua paket TERPISAH, bukan satu berkas dengan saklar: isinya berbeda.
  // "Data riil" membawa pembukuan penuh; "peredaran terbatas" membawa dasar
  // perhitungan plafon PP23 plus daftar transaksi yang dikecualikan, tanpa
  // neraca — begitu transaksi dibuang, posisi keuangan tidak lagi
  // menggambarkan keadaan perusahaan yang sebenarnya.
  async function handleDownloadPackage(variant: "actual" | "capped") {
    setDownloadingPack(variant);
    try {
      await financeReportsService.downloadTaxPackage(year, month, variant);
      toast.success(
        variant === "capped"
          ? "Paket peredaran terbatas diunduh"
          : "Paket data riil diunduh",
      );
    } catch (err) {
      toast.danger("Gagal mengunduh paket pajak", toApiError(err).message);
    } finally {
      setDownloadingPack(null);
    }
  }

  const totalRevenue = rows.reduce((s, r) => s + r.revenue, 0);
  const totalPpn = rows.reduce((s, r) => s + r.ppn, 0);

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Rincian PPN"
        description="Kertas kerja SPT per faktur — hanya baca."
        actions={
          canExport ? (
            <div className="flex gap-3">
              <Button
                variant="secondary"
                onClick={() => void handleDownload()}
                disabled={downloading}
              >
                {downloading ? "Mengunduh…" : "Unduh CSV"}
              </Button>
              <Button
                variant="secondary"
                onClick={() => void handleDownloadPackage("capped")}
                disabled={downloadingPack !== null}
                title="Peredaran bruto dibatasi plafon Rp4,8 M per tahun pajak, seluruh cabang"
              >
                {downloadingPack === "capped"
                  ? "Mengunduh…"
                  : "Paket Peredaran Terbatas"}
              </Button>
              <Button
                onClick={() => void handleDownloadPackage("actual")}
                disabled={downloadingPack !== null}
                title="Pembukuan komersial apa adanya"
              >
                {downloadingPack === "actual"
                  ? "Mengunduh…"
                  : "Paket Data Riil"}
              </Button>
            </div>
          ) : undefined
        }
      />

      <FilterBar>
        <form onSubmit={apply} className="flex flex-wrap items-end gap-4">
          <div className="min-w-36">
            <FormField label="Tahun">
              <SelectInput
                value={year}
                onChange={(e) => setYear(Number(e.target.value))}
              >
                {yearOptions().map((y) => (
                  <option key={y} value={y}>
                    {y}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </div>
          <div className="min-w-44">
            <FormField label="Bulan">
              <SelectInput
                value={month}
                onChange={(e) => setMonth(Number(e.target.value))}
              >
                {MONTHS.map((label, i) => (
                  <option key={label} value={i + 1}>
                    {label}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </div>
          <Button type="submit" variant="secondary">
            Tampilkan
          </Button>
        </form>
      </FilterBar>

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      <SectionCard title={`Rincian PPN — ${periodLabel(year, month)}`}>
        {loading && rows.length === 0 ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : (
          <>
            <DataTable<TaxDetailRow>
              rows={rows}
              rowKey={(r) => `${r.date}-${r.number}`}
              emptyTitle="Belum ada faktur berpajak"
              emptyDescription="Tidak ada penyerahan kena pajak pada periode ini."
              columns={[
                { header: "Tanggal", render: (r) => formatDate(r.date) },
                { header: "Nomor", render: (r) => r.number || "-" },
                { header: "Keterangan", render: (r) => r.memo || "-" },
                {
                  header: "No. Faktur",
                  render: (r) => r.taxInvoiceNumber || "-",
                },
                {
                  header: "Tgl. Faktur",
                  render: (r) => formatDate(r.taxInvoiceDate),
                },
                {
                  header: "Omzet",
                  align: "right",
                  render: (r) => formatIDR(r.revenue),
                },
                {
                  header: "PPN",
                  align: "right",
                  render: (r) => formatIDR(r.ppn),
                },
              ]}
            />
            {rows.length > 0 && (
              <div className="border-t border-hairline px-5 py-3.5">
                <div className="flex flex-wrap items-center justify-end gap-x-8 gap-y-1 text-sm">
                  <span>
                    Total Omzet <strong>{formatIDR(totalRevenue)}</strong>
                  </span>
                  <span>
                    Total PPN <strong>{formatIDR(totalPpn)}</strong>
                  </span>
                </div>
              </div>
            )}
          </>
        )}
      </SectionCard>
    </div>
  );
}
