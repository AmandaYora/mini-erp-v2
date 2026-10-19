import { useState } from "react";
import { Badge, Button, DataTable, PageHeader, SectionCard, SummaryCard } from "@/shared/components/ui";
import { FileDropzone, Money, Page, Qty, Stepper, StickyFooter } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { openingPreviewSummary } from "@/modules/stock/lib/opening";
import { stockService } from "@/modules/stock/services/stock.service";
import type { OpeningPreview, OpeningPreviewRow, OpeningResult, OpeningRowError } from "@/modules/stock/types";

const STEPS = [
  { key: "template", label: "Unduh templat" },
  { key: "upload", label: "Unggah" },
  { key: "review", label: "Tinjau" },
  { key: "done", label: "Selesai" },
];

const ERROR_COLUMNS = [
  { header: "Baris", align: "right" as const, render: (r: OpeningRowError) => formatNumber(r.row) },
  { header: "Kolom", render: (r: OpeningRowError) => r.column },
  { header: "Nilai", render: (r: OpeningRowError) => r.value || "–" },
  { header: "Alasan", render: (r: OpeningRowError) => r.reason },
];

const PREVIEW_COLUMNS = [
  { header: "Baris", align: "right" as const, render: (r: OpeningPreviewRow) => formatNumber(r.row) },
  {
    header: "Barang",
    render: (r: OpeningPreviewRow) => (
      <div>
        <p className="font-medium">{r.productName || r.productCode}</p>
        <p className="text-xs text-muted-foreground">{[r.productCode, r.variantCode].filter(Boolean).join(" / ")}</p>
      </div>
    ),
  },
  { header: "Lokasi", render: (r: OpeningPreviewRow) => r.locationCode },
  { header: "Jumlah", align: "right" as const, render: (r: OpeningPreviewRow) => <Qty value={r.qty} /> },
  { header: "Nilai", align: "right" as const, render: (r: OpeningPreviewRow) => <Money value={r.value} /> },
  {
    header: "Status",
    render: (r: OpeningPreviewRow) =>
      r.status === "post" ? <Badge tone="success">Dibukukan</Badge> : <Badge tone="neutral">{"Dilewati: " + r.note}</Badge>,
  },
];

export default function OpeningPage() {
  const can = useAuthStore((s) => s.can);
  // Route backend opening/template|preview|commit dijaga stock.adjust.
  const canOpen = can("stock.adjust");

  const [file, setFile] = useState<File | null>(null);
  const [downloading, setDownloading] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [preview, setPreview] = useState<OpeningPreview | null>(null);
  const [result, setResult] = useState<OpeningResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const busy = previewing || committing;
  const current = result ? "done" : preview ? "review" : file ? "upload" : "template";

  async function handleDownload() {
    setDownloading(true);
    try {
      await stockService.downloadOpeningTemplate();
      toast.success("Templat diunduh");
    } catch (err) {
      toast.danger("Gagal mengunduh templat", toApiError(err).message);
    } finally {
      setDownloading(false);
    }
  }

  async function handlePreview() {
    if (!file) {
      setError("Pilih berkas Excel dulu.");
      return;
    }
    setError(null);
    setResult(null);
    setPreviewing(true);
    try {
      // Multipart field "file" — pratinjau tanpa tulis.
      const res = await stockService.previewOpening(file);
      setPreview(res.data);
      if (res.data.errors.length === 0) {
        toast.fromServer(res.message, `${formatNumber(res.data.validRows)} baris valid`);
      } else {
        toast.warning(openingPreviewSummary(res.data));
      }
    } catch (err) {
      setError(toApiError(err).message);
    } finally {
      setPreviewing(false);
    }
  }

  async function handleCommit() {
    if (!file) {
      setError("Pilih berkas Excel dulu.");
      return;
    }
    setError(null);
    setCommitting(true);
    try {
      // Posisi yang sudah punya mutasi dilewati (resumable), tidak ganda.
      const res = await stockService.commitOpening(file);
      setResult(res.data);
      toast.fromServer(
        res.message,
        `Dibukukan ${formatNumber(res.data.posted)}, dilewati ${formatNumber(res.data.skipped)}`,
      );
    } catch (err) {
      setError(toApiError(err).message);
    } finally {
      setCommitting(false);
    }
  }

  function reset() {
    setFile(null);
    setPreview(null);
    setResult(null);
    setError(null);
  }

  return (
    <Page>
      <PageHeader
        title="Saldo Awal"
        description="Impor saldo awal dari Excel: unduh templat, unggah, tinjau, lalu bukukan."
      />

      <Stepper steps={STEPS} current={current} />

      {!canOpen && (
        <Notice tone="warning" title="Izin terbatas">
          Akun Anda tidak memiliki izin stock.adjust. Aksi dikunci.
        </Notice>
      )}
      {error && (
        <Notice tone="danger" title="Gagal memproses berkas">
          {error}
        </Notice>
      )}

      <SectionCard
        title="Berkas Excel"
        description="Kolom: Kode Barang, Kode Varian (opsional), Kode Lokasi, Jumlah Stok Awal. Templat sudah berisi daftar barang dan lokasi saran; isi jumlahnya saja. Judul kolom berbahasa Indonesia maupun Inggris dikenali."
        actions={
          <Button variant="secondary" size="sm" loading={downloading} onClick={() => void handleDownload()}>
            Unduh templat
          </Button>
        }
      >
        <FileDropzone
          accept=".xlsx"
          disabled={!canOpen || busy}
          fileName={file?.name ?? null}
          hint="Hanya format .xlsx (Excel 2007 ke atas). Simpan ulang file lama sebagai .xlsx."
          label={file ? "Ganti berkas" : "Seret berkas Excel ke sini atau pilih"}
          onFile={(f) => {
            setFile(f);
            setPreview(null);
            setResult(null);
            setError(null);
          }}
        />
      </SectionCard>

      {preview && !result && (
        <SectionCard
          title="Hasil pratinjau"
          description={`${openingPreviewSummary(preview)}. Belum ada yang dibukukan.`}
          flush
        >
          {(preview.warnings ?? []).length > 0 && (
            <div className="px-5 pt-5">
              <Notice tone="warning" title="Perhatian">
                <ul className="list-disc pl-5">
                  {(preview.warnings ?? []).map((w) => (
                    <li key={w}>{w}</li>
                  ))}
                </ul>
              </Notice>
            </div>
          )}
          <div className="grid gap-4 p-5 sm:grid-cols-3 lg:grid-cols-5">
            <SummaryCard label="Baris valid" value={formatNumber(preview.validRows)} tone="success" />
            <SummaryCard label="Total jumlah" value={formatNumber(preview.totalQty ?? 0)} hint="Satuan dasar, baris yang dibukukan" />
            <SummaryCard label="Total nilai" value={`Rp ${formatNumber(preview.totalValue ?? 0)}`} hint="Jumlah x harga beli" />
            <SummaryCard label="Dilewati" value={formatNumber(preview.skipped ?? 0)} hint="Posisi sudah punya mutasi" />
            <SummaryCard
              label="Bermasalah"
              value={formatNumber(preview.errors.length)}
              tone={preview.errors.length > 0 ? "danger" : "brand"}
            />
          </div>
          {(preview.rows ?? []).length > 0 && (
            <DataTable<OpeningPreviewRow>
              columns={PREVIEW_COLUMNS}
              rows={preview.rows ?? []}
              rowKey={(r) => String(r.row)}
              emptyTitle="Tidak ada baris"
            />
          )}
          <DataTable<OpeningRowError>
            columns={ERROR_COLUMNS}
            rows={preview.errors}
            rowKey={(r) => `${r.row}-${r.column}`}
            emptyTitle="Semua baris valid"
            emptyDescription="Silakan lanjutkan ke pembukuan."
          />
        </SectionCard>
      )}

      {result && (
        <SectionCard
          title="Hasil pembukuan"
          description={`Dibukukan ${formatNumber(result.posted)}, dilewati ${formatNumber(result.skipped)} (sudah punya mutasi), gagal ${formatNumber(result.failures.length)}.`}
          flush
        >
          <DataTable<OpeningRowError>
            columns={ERROR_COLUMNS}
            rows={result.failures}
            rowKey={(r) => `${r.row}-${r.column}`}
            emptyTitle="Tanpa kegagalan"
            emptyDescription="Semua baris valid berhasil dibukukan."
          />
        </SectionCard>
      )}

      <StickyFooter
        summary={
          result ? (
            <span>
              Selesai: <Qty value={result.posted} /> baris dibukukan
            </span>
          ) : file ? (
            <span className="truncate">{file.name}</span>
          ) : (
            <span>Pilih berkas untuk memulai.</span>
          )
        }
      >
        {result ? (
          <Button onClick={reset}>Impor berkas lain</Button>
        ) : (
          <>
            <Button
              variant="secondary"
              disabled={!canOpen || !file || committing}
              loading={previewing}
              onClick={() => void handlePreview()}
            >
              Pratinjau
            </Button>
            <Button disabled={!canOpen || !file || previewing} loading={committing} onClick={() => void handleCommit()}>
              Bukukan
            </Button>
          </>
        )}
      </StickyFooter>
    </Page>
  );
}
