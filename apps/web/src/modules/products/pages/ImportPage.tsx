import { useRef, useState } from "react";
import {
  Button,
  DataTable,
  PageHeader,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { toast } from "@/shared/stores/toast.store";
import { productsService } from "@/modules/products/services/products.service";
import type { ImportPreview, ImportResult } from "@/modules/products/types";

export default function ImportPage() {
  const can = useAuthStore((s) => s.can);
  const manageable = can("product_import.manage");
  const fileRef = useRef<HTMLInputElement>(null);

  const [file, setFile] = useState<File | null>(null);
  const [downloading, setDownloading] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);

  async function handleDownload() {
    setDownloading(true);
    try {
      await productsService.downloadTemplate();
      toast.success("Template diunduh");
    } catch (err) {
      toast.danger("Gagal mengunduh template", toApiError(err).message);
    } finally {
      setDownloading(false);
    }
  }

  function pickFile(f: File | undefined) {
    if (!f) return;
    setFile(f);
    setPreview(null);
    setResult(null);
  }

  async function handlePreview() {
    if (!file) {
      toast.warning("Pilih berkas Excel terlebih dahulu");
      return;
    }
    setPreviewing(true);
    try {
      const res = await productsService.previewImport(file);
      const prev = res.data;
      setPreview(prev);
      setResult(null);
      if (prev.errors.length === 0) {
        toast.fromServer(res.message, `Siap impor: ${prev.validProducts} produk, ${prev.validVariants} varian`);
      } else {
        toast.warning(`${prev.errors.length} baris bermasalah`, "Perbaiki sebelum commit.");
      }
    } catch (err) {
      toast.danger("Gagal mempratinjau", toApiError(err).message);
    } finally {
      setPreviewing(false);
    }
  }

  async function handleCommit() {
    if (!file) {
      toast.warning("Pilih berkas Excel terlebih dahulu");
      return;
    }
    setCommitting(true);
    try {
      const res = await productsService.commitImport(file);
      setResult(res.data);
      toast.fromServer(res.message, `Impor selesai: ${res.data.inserted} disisipkan, ${res.data.skipped} dilewati`);
    } catch (err) {
      toast.danger("Gagal mengimpor", toApiError(err).message);
    } finally {
      setCommitting(false);
    }
  }

  return (
    <div className="space-y-4">
      <PageHeader
        eyebrow="Master"
        title="Impor Produk"
        description="Unduh template, pratinjau validasi, lalu commit. File yang sama bisa diunggah ulang — kode yang sudah ada dilewati."
      />

      {!manageable && (
        <Notice tone="warning" title="Akses terbatas">
          Anda membutuhkan izin product_import.manage untuk mengimpor produk.
        </Notice>
      )}

      <SectionCard title="Langkah 1 — Unduh template" description="Workbook berisi sheet products dan variants.">
        <Button variant="secondary" disabled={downloading} onClick={handleDownload}>
          {downloading ? "Mengunduh…" : "Unduh template Excel"}
        </Button>
      </SectionCard>

      <SectionCard title="Langkah 2 — Pilih berkas & pratinjau" description="Pratinjau tidak menulis apa pun.">
        <div className="flex flex-wrap items-center gap-3">
          <input
            ref={fileRef}
            type="file"
            accept=".xlsx,.xls"
            disabled={!manageable}
            onChange={(e) => pickFile(e.target.files?.[0])}
            className="text-sm text-ink"
          />
          {file && <p className="text-sm text-ink">{file.name}</p>}
        </div>
        <div className="mt-4 flex flex-wrap gap-3">
          <Button variant="secondary" disabled={!file || previewing || !manageable} onClick={handlePreview}>
            {previewing ? "Mempratinjau…" : "Pratinjau"}
          </Button>
          <Button disabled={!file || committing || !manageable} onClick={handleCommit}>
            {committing ? "Mengimpor…" : "Commit"}
          </Button>
        </div>

        {preview && (
          <div className="mt-4 space-y-3">
            <Notice
              tone={preview.errors.length === 0 ? "success" : "warning"}
              title={`Valid: ${preview.validProducts} produk, ${preview.validVariants} varian`}
            >
              {preview.errors.length === 0
                ? "Semua baris valid — aman untuk commit."
                : `${preview.errors.length} baris bermasalah — lihat tabel di bawah.`}
            </Notice>
            {preview.errors.length > 0 && (
              <DataTable
                columns={[
                  { header: "Sheet", render: (r) => r.sheet },
                  { header: "Baris", align: "right", render: (r) => r.row },
                  { header: "Kolom", render: (r) => r.column || "-" },
                  { header: "Nilai", render: (r) => r.value || "-" },
                  { header: "Alasan", render: (r) => r.reason },
                ]}
                rows={preview.errors}
                rowKey={(r) => `${r.sheet}-${r.row}-${r.column}-${r.value}-${r.reason}`}
                emptyTitle="Tidak ada galat"
              />
            )}
          </div>
        )}
      </SectionCard>

      {result && (
        <SectionCard title="Langkah 3 — Hasil impor">
          <Notice tone={result.failures.length === 0 ? "success" : "warning"} title="Ringkasan">
            {result.inserted} disisipkan • {result.skipped} dilewati (sudah ada) • {result.failures.length} gagal.
          </Notice>
          {result.failures.length > 0 && (
            <div className="mt-4">
              <DataTable
                columns={[
                  { header: "Sheet", render: (r) => r.sheet },
                  { header: "Baris", align: "right", render: (r) => r.row },
                  { header: "Kolom", render: (r) => r.column || "-" },
                  { header: "Nilai", render: (r) => r.value || "-" },
                  { header: "Alasan", render: (r) => r.reason },
                ]}
                rows={result.failures}
                rowKey={(r) => `${r.sheet}-${r.row}-${r.column}-${r.value}-${r.reason}`}
                emptyTitle="Tidak ada kegagalan"
              />
            </div>
          )}
        </SectionCard>
      )}
    </div>
  );
}
