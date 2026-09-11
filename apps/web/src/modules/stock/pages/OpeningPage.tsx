import { useRef, useState } from "react";
import {
  Button,
  DataTable,
  PageHeader,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { stockService } from "@/modules/stock/services/stock.service";
import type {
  OpeningPreview,
  OpeningResult,
  OpeningRowError,
} from "@/modules/stock/types";

export default function OpeningPage() {
  const can = useAuthStore((s) => s.can);
  // Menu memakai stock.adjust, tetapi route backend opening/template|preview|commit
  // dijaga stock.manage — butuh salah satunya, backend yang memutuskan akhir.
  const canOpen = can("stock.adjust") || can("stock.manage");

  const fileRef = useRef<HTMLInputElement>(null);
  const [fileName, setFileName] = useState("");
  const [downloading, setDownloading] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [preview, setPreview] = useState<OpeningPreview | null>(null);
  const [result, setResult] = useState<OpeningResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  function selectedFile(): File | null {
    return fileRef.current?.files?.[0] ?? null;
  }

  async function handleDownload() {
    setDownloading(true);
    try {
      await stockService.downloadOpeningTemplate();
      toast.success("Template diunduh");
    } catch (err) {
      toast.danger("Gagal mengunduh template", toApiError(err).message);
    } finally {
      setDownloading(false);
    }
  }

  async function handlePreview() {
    const file = selectedFile();
    if (!file) {
      setError("Pilih file Excel dulu.");
      return;
    }
    setError(null);
    setResult(null);
    setPreviewing(true);
    try {
      // Multipart field "file" (httpx.FormFile("file")) — pratinjau tanpa tulis.
      const res = await stockService.previewOpening(file);
      setPreview(res.data);
      if (res.data.errors.length === 0) {
        toast.fromServer(res.message, `${formatNumber(res.data.validRows)} baris valid`);
      } else {
        toast.warning(
          `${formatNumber(res.data.validRows)} baris valid, ${formatNumber(res.data.errors.length)} baris bermasalah`,
        );
      }
    } catch (err) {
      setError(toApiError(err).message);
    } finally {
      setPreviewing(false);
    }
  }

  async function handleCommit() {
    const file = selectedFile();
    if (!file) {
      setError("Pilih file Excel dulu.");
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

  return (
    <div>
      <PageHeader
        eyebrow="Stok & Gudang"
        title="Saldo Awal"
        description="Impor saldo awal dari Excel: unduh template, pratinjau, lalu bukukan."
      />

      {!canOpen && (
        <div className="mb-4">
          <Notice tone="warning" title="Izin terbatas">
            Akun Anda tidak memiliki izin stock.adjust/stock.manage. Aksi
            dikunci.
          </Notice>
        </div>
      )}
      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memproses file">
            {error}
          </Notice>
        </div>
      )}

      <SectionCard
        title="Berkas Excel"
        description="Kolom template: product_code, variant_code, location_code, qty."
        actions={
          <Button
            variant="secondary"
            size="sm"
            disabled={downloading}
            onClick={() => void handleDownload()}
          >
            {downloading ? "Mengunduh…" : "Unduh Template"}
          </Button>
        }
      >
        <fieldset
          disabled={!canOpen || previewing || committing}
          className="flex flex-wrap items-end gap-3"
        >
          <div>
            <label
              htmlFor="opening-file"
              className="text-heading mb-1.5 block text-[0.85rem] font-medium"
            >
              File Excel
            </label>
            <input
              id="opening-file"
              ref={fileRef}
              type="file"
              accept=".xlsx,.xls"
              onChange={(e) => {
                setFileName(e.target.files?.[0]?.name ?? "");
                setPreview(null);
                setResult(null);
              }}
              className="text-sm text-ink file:mr-3 file:cursor-pointer file:rounded-md file:border file:border-hairline file:bg-surface-subtle file:px-3 file:py-1.5 file:text-sm file:font-medium"
            />
            {fileName && (
              <p className="text-muted mt-1 text-[0.8rem]">{fileName}</p>
            )}
          </div>
          <Button
            variant="secondary"
            disabled={previewing || committing}
            onClick={() => void handlePreview()}
          >
            {previewing ? "Memeriksa…" : "Pratinjau"}
          </Button>
          <Button disabled={previewing || committing} onClick={() => void handleCommit()}>
            {committing ? "Membukukan…" : "Bukukan"}
          </Button>
        </fieldset>
      </SectionCard>

      {preview && (
        <div className="mt-6">
          <SectionCard
            title="Hasil Pratinjau"
            description={`${formatNumber(preview.validRows)} baris valid, ${formatNumber(preview.errors.length)} baris bermasalah. Belum ada yang dibukukan.`}
          >
            <DataTable<OpeningRowError>
              columns={[
                {
                  header: "Baris",
                  align: "right",
                  render: (r) => formatNumber(r.row),
                },
                { header: "Kolom", render: (r) => r.column },
                { header: "Nilai", render: (r) => r.value || "-" },
                { header: "Alasan", render: (r) => r.reason },
              ]}
              rows={preview.errors}
              rowKey={(r) => `${r.row}-${r.column}`}
              emptyTitle="Semua baris valid"
              emptyDescription="Silakan lanjutkan ke pembukuan."
            />
          </SectionCard>
        </div>
      )}

      {result && (
        <div className="mt-6">
          <SectionCard
            title="Hasil Pembukuan"
            description={`Dibukukan ${formatNumber(result.posted)}, dilewati ${formatNumber(result.skipped)} (sudah punya mutasi), gagal ${formatNumber(result.failures.length)}.`}
          >
            <DataTable<OpeningRowError>
              columns={[
                {
                  header: "Baris",
                  align: "right",
                  render: (r) => formatNumber(r.row),
                },
                { header: "Kolom", render: (r) => r.column },
                { header: "Nilai", render: (r) => r.value || "-" },
                { header: "Alasan", render: (r) => r.reason },
              ]}
              rows={result.failures}
              rowKey={(r) => `${r.row}-${r.column}`}
              emptyTitle="Tanpa kegagalan"
              emptyDescription="Semua baris valid berhasil dibukukan."
            />
          </SectionCard>
        </div>
      )}
    </div>
  );
}
