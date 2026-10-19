import { useState } from "react";
import { Link } from "react-router-dom";
import { Download } from "lucide-react";
import {
  Badge,
  Button,
  DataTable,
  PageHeader,
  SectionCard,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/shared/components/ui";
import { FileDropzone, Page, Stepper, StickyFooter } from "@/shared/components/patterns";
import type { StepperStep } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { toApiError } from "@/shared/services/http-client";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { toast } from "@/shared/stores/toast.store";
import { productsService } from "@/modules/products/services/products.service";
import {
  IMPORT_ACCEPT,
  canCommit,
  groupIssuesBySheet,
  issueLocation,
  splitPlannedRows,
  validateImportFile,
} from "@/modules/products/lib/import-view";
import type { PlannedRowView } from "@/modules/products/lib/import-view";
import type { ImportIssue, ImportPreview, ImportResult } from "@/modules/products/types";

const STEPS: StepperStep[] = [
  { key: "template", label: "Unduh templat" },
  { key: "upload", label: "Unggah" },
  { key: "review", label: "Tinjau" },
  { key: "done", label: "Selesai" },
];

function SummaryCell({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-md border border-border bg-background px-3 py-2">
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <p className="tabular font-display text-xl font-semibold text-foreground">{value}</p>
    </div>
  );
}

function IssueTable({ issues }: { issues: ImportIssue[] }) {
  return (
    <DataTable<ImportIssue>
      columns={[
        {
          header: "Jenis",
          render: (r) => (
            <Tooltip>
              <TooltipTrigger asChild>
                <button type="button" className="cursor-help rounded-full" aria-label={`${r.message} (${issueLocation(r)})`}>
                  {r.severity === "error" ? <Badge tone="danger">Error</Badge> : <Badge tone="warning">Peringatan</Badge>}
                </button>
              </TooltipTrigger>
              <TooltipContent>
                {r.message} — {issueLocation(r)}
              </TooltipContent>
            </Tooltip>
          ),
        },
        { header: "Baris", align: "right", render: (r) => (r.row > 0 ? <span className="tabular">{r.row}</span> : "–") },
        { header: "Kolom", render: (r) => r.column || "–" },
        { header: "Pesan", render: (r) => r.message },
      ]}
      rows={issues}
      rowKey={(r) => `${r.severity}-${r.sheet}-${r.row}-${r.column}-${r.message}`}
      emptyTitle="Tidak ada temuan"
    />
  );
}

function PlannedTable({ rows, emptyTitle }: { rows: PlannedRowView[]; emptyTitle: string }) {
  return (
    <DataTable<PlannedRowView>
      columns={[
        { header: "Jenis", render: (r) => r.kind },
        { header: "Baris", align: "right", render: (r) => <span className="tabular">{r.row}</span> },
        { header: "Kode", render: (r) => <span className="font-mono">{r.code}</span> },
        { header: "Nama", render: (r) => r.name },
        {
          header: "Tindakan",
          render: (r) =>
            r.action === "create" ? <Badge tone="success">Baru</Badge> : <Badge tone="info">Diperbarui</Badge>,
        },
      ]}
      rows={rows}
      rowKey={(r) => `${r.kind}-${r.row}-${r.code}`}
      emptyTitle={emptyTitle}
    />
  );
}

export default function ImportPage() {
  const can = useAuthStore((s) => s.can);
  const manageable = can("product_import.manage");

  const [started, setStarted] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [downloading, setDownloading] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [commitError, setCommitError] = useState<string[] | null>(null);

  const current = result ? "done" : preview ? "review" : started ? "upload" : "template";

  async function handleDownload() {
    setDownloading(true);
    try {
      await productsService.downloadTemplate();
      toast.success("Templat diunduh");
    } catch (err) {
      toast.danger("Gagal mengunduh templat", toApiError(err).message);
    } finally {
      setDownloading(false);
    }
  }

  function pickFile(f: File) {
    const rejected = validateImportFile(f.name);
    setPreview(null);
    setResult(null);
    setCommitError(null);
    if (rejected) {
      setFile(null);
      setFileError(rejected);
      return;
    }
    setFileError(null);
    setFile(f);
  }

  async function handlePreview() {
    if (!file) {
      toast.warning("Pilih berkas Excel terlebih dahulu");
      return;
    }
    setPreviewing(true);
    setCommitError(null);
    try {
      const res = await productsService.previewImport(file);
      const prev = res.data;
      setPreview(prev);
      setResult(null);
      if (canCommit(prev)) {
        toast.fromServer(
          res.message,
          `Siap impor: ${prev.summary.productsTotal} produk, ${prev.summary.variantsTotal} varian`,
        );
      } else {
        toast.warning(
          `${prev.summary.errors} error ditemukan`,
          "Seluruh berkas ditolak selama masih ada error — perbaiki lalu pratinjau ulang.",
        );
      }
    } catch (err) {
      const apiErr = toApiError(err);
      setPreview(null);
      setCommitError([apiErr.message, ...(apiErr.errors ?? []).map((e) => e.message)]);
      toast.danger("Gagal mempratinjau", apiErr.message);
    } finally {
      setPreviewing(false);
    }
  }

  async function handleCommit() {
    if (!file || !canCommit(preview)) return;
    setCommitting(true);
    setCommitError(null);
    try {
      const res = await productsService.commitImport(file);
      setResult(res.data);
      const i = res.data.imported;
      toast.fromServer(
        res.message,
        `Impor selesai: ${i.productsCreated} produk baru, ${i.productsUpdated} diperbarui`,
      );
    } catch (err) {
      const apiErr = toApiError(err);
      setCommitError([apiErr.message, ...(apiErr.errors ?? []).map((e) => e.message)]);
      toast.danger("Gagal mengimpor", apiErr.message);
    } finally {
      setCommitting(false);
    }
  }

  function restart() {
    setStarted(false);
    setFile(null);
    setFileError(null);
    setPreview(null);
    setResult(null);
    setCommitError(null);
  }

  const groups = preview ? groupIssuesBySheet(preview.issues) : [];
  const ready = canCommit(preview);
  const split = preview ? splitPlannedRows(preview) : { valid: [], problem: [] };
  const problemCount = preview ? preview.issues.length : 0;

  return (
    <Page>
      <PageHeader
        title="Impor produk"
        description="Unduh templat, pratinjau validasi, lalu impor. Kode produk yang sudah ada diperbarui; satu error saja menolak seluruh berkas."
      />

      {!manageable && (
        <Notice tone="warning" title="Akses terbatas">
          Anda membutuhkan izin product_import.manage untuk mengimpor produk.
        </Notice>
      )}

      <Stepper steps={STEPS} current={current} />

      {commitError && (
        <Notice tone="danger" title={commitError[0]}>
          {commitError.length > 1 && (
            <ul className="list-disc pl-5">
              {commitError.slice(1).map((m) => (
                <li key={m}>{m}</li>
              ))}
            </ul>
          )}
        </Notice>
      )}

      {current === "template" && (
        <SectionCard
          title="Unduh templat"
          description="Workbook berisi 5 sheet: Panduan, Kategori Produk, Daftar Produk, Varian Produk, dan Info Tambahan Produk."
        >
          <Button variant="secondary" loading={downloading} onClick={() => void handleDownload()}>
            <Download aria-hidden /> Unduh templat Excel
          </Button>
        </SectionCard>
      )}

      {current === "upload" && (
        <SectionCard title="Unggah berkas" description="Hanya berkas .xlsx. Pratinjau tidak menulis apa pun.">
          <FileDropzone
            accept={IMPORT_ACCEPT}
            disabled={!manageable}
            busy={previewing}
            fileName={file?.name ?? null}
            label="Seret berkas Excel ke sini atau pilih"
            hint="Format .xlsx"
            onFile={pickFile}
          />
          {fileError && (
            <p role="alert" className="mt-2 text-sm font-medium text-destructive">
              {fileError}
            </p>
          )}
        </SectionCard>
      )}

      {current === "review" && preview && (
        <>
          <Notice
            tone={ready ? "success" : "danger"}
            title={ready ? "Berkas siap diimpor" : `${preview.summary.errors} error — berkas ditolak seluruhnya`}
          >
            {ready
              ? preview.summary.warnings > 0
                ? `${preview.summary.warnings} peringatan — tidak menghalangi impor.`
                : "Tidak ada temuan."
              : "Perbaiki semua error di bawah, simpan berkas, lalu pratinjau ulang."}
          </Notice>

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <SummaryCell label="Kategori baru" value={preview.summary.categoriesToCreate} />
            <SummaryCell label="Kategori diperbarui" value={preview.summary.categoriesToUpdate} />
            <SummaryCell label="Produk baru" value={preview.summary.productsToCreate} />
            <SummaryCell label="Produk diperbarui" value={preview.summary.productsToUpdate} />
            <SummaryCell label="Varian" value={preview.summary.variantsTotal} />
            <SummaryCell label="Info tambahan" value={preview.summary.attributesTotal} />
            <SummaryCell label="Error" value={preview.summary.errors} />
            <SummaryCell label="Peringatan" value={preview.summary.warnings} />
          </div>

          <TooltipProvider>
            <Tabs defaultValue={ready || problemCount === 0 ? "valid" : "problem"} className="gap-4">
              <TabsList variant="line" className="border-b border-border">
                <TabsTrigger value="valid" className="flex-none px-3">
                  Valid ({split.valid.length})
                </TabsTrigger>
                <TabsTrigger value="problem" className="flex-none px-3">
                  Bermasalah ({problemCount})
                </TabsTrigger>
              </TabsList>
              <TabsContent value="valid">
                <SectionCard flush>
                  <PlannedTable rows={split.valid} emptyTitle="Belum ada baris yang valid" />
                </SectionCard>
              </TabsContent>
              <TabsContent value="problem" className="flex flex-col gap-4">
                {groups.length === 0 ? (
                  <SectionCard flush>
                    <IssueTable issues={[]} />
                  </SectionCard>
                ) : (
                  groups.map((g) => (
                    <SectionCard
                      key={g.sheet}
                      flush
                      title={`Sheet: ${g.sheet}`}
                      description={`${g.errors} error, ${g.warnings} peringatan`}
                    >
                      <IssueTable issues={g.issues} />
                    </SectionCard>
                  ))
                )}
              </TabsContent>
            </Tabs>
          </TooltipProvider>
        </>
      )}

      {current === "done" && result && (
        <SectionCard title="Hasil impor">
          <Notice tone={result.failures.length === 0 ? "success" : "warning"} title="Ringkasan">
            Kategori: {result.imported.categoriesCreated} baru, {result.imported.categoriesUpdated} diperbarui •
            Produk: {result.imported.productsCreated} baru, {result.imported.productsUpdated} diperbarui •{" "}
            {result.failures.length} gagal.
          </Notice>
          {(result.notes ?? []).length > 0 && (
            <ul className="mt-3 list-disc pl-5 text-sm text-muted-foreground">
              {result.notes.map((n) => (
                <li key={n}>{n}</li>
              ))}
            </ul>
          )}
          {result.failures.length > 0 && (
            <div className="mt-4">
              <DataTable
                columns={[
                  { header: "Sheet", render: (r) => r.sheet },
                  { header: "Baris", align: "right", render: (r) => <span className="tabular">{r.row}</span> },
                  { header: "Kolom", render: (r) => r.column || "–" },
                  { header: "Nilai", render: (r) => r.value || "–" },
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

      <StickyFooter
        summary={
          current === "review" && !ready ? (
            <span>Impor dinonaktifkan sampai seluruh error diperbaiki dan pratinjau ulang bersih.</span>
          ) : undefined
        }
      >
        {current === "template" && (
          <Button onClick={() => setStarted(true)} disabled={!manageable}>
            Lanjut ke unggah
          </Button>
        )}
        {current === "upload" && (
          <>
            <Button variant="secondary" onClick={() => setStarted(false)}>
              Kembali
            </Button>
            <Button loading={previewing} disabled={!file || !manageable} onClick={() => void handlePreview()}>
              Pratinjau
            </Button>
          </>
        )}
        {current === "review" && (
          <>
            <Button
              variant="secondary"
              onClick={() => {
                setPreview(null);
                setCommitError(null);
              }}
            >
              Ganti berkas
            </Button>
            <Button loading={committing} disabled={!ready || !manageable} onClick={() => void handleCommit()}>
              Impor sekarang
            </Button>
          </>
        )}
        {current === "done" && (
          <>
            <Button variant="secondary" onClick={restart}>
              Impor lagi
            </Button>
            <Button asChild>
              <Link to={ROUTE_PATHS.products}>Lihat produk</Link>
            </Button>
          </>
        )}
      </StickyFooter>
    </Page>
  );
}
