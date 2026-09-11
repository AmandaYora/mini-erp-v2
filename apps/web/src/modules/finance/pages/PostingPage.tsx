import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  DateInput,
  FormField,
  PageHeader,
  SectionCard,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatIDR, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { postingFormSchema } from "@/modules/finance/schemas/finance.schema";
import {
  POSTING_DOC_TYPES,
  docTypeLabel,
  entryTotals,
} from "@/modules/finance/types";
import type {
  BatchPostResult,
  JournalEntry,
  PostingSource,
} from "@/modules/finance/types";

// Posting dokumen operasional → jurnal (G-revisi): daftar turunan
// (terkonfirmasi tapi belum dijurnal) + pilihan massal + tutup harian.
// Pratinjau memakai builder yang sama dengan Post, jadi tidak pernah meleset.
export default function PostingPage() {
  const can = useAuthStore((s) => s.can);
  const canPost = can("finance.post");

  const {
    data: queueData,
    loading: queueLoading,
    error: queueError,
    reload: reloadQueue,
  } = useAsyncData(
    () =>
      financeService.postingSources().then((rows) => rows ?? []),
    [],
  );
  const queue = queueData ?? [];
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [prevQueueData, setPrevQueueData] = useState(queueData);
  if (queueData !== prevQueueData) {
    setPrevQueueData(queueData);
    setSelected(new Set());
  }
  const [batching, setBatching] = useState(false);
  const [batchResult, setBatchResult] = useState<BatchPostResult[] | null>(
    null,
  );
  const [closeDate, setCloseDate] = useState(todayWIB());
  const [closing, setClosing] = useState(false);

  const [docType, setDocType] = useState<string>("delivery");
  const [docId, setDocId] = useState("");
  const [preview, setPreview] = useState<JournalEntry | null>(null);
  const [posted, setPosted] = useState<JournalEntry | null>(null);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [posting, setPosting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | undefined>(undefined);

  const totals = entryTotals(preview?.lines);
  const balanced =
    preview !== null && totals.debit > 0 && totals.debit === totals.credit;

  function toggleRow(key: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function toggleAll() {
    setSelected((prev) =>
      prev.size === queue.length
        ? new Set()
        : new Set(queue.map((q) => `${q.docType}:${q.docId}`)),
    );
  }

  async function handleBatch() {
    const items = queue
      .filter((q) => selected.has(`${q.docType}:${q.docId}`))
      .map((q) => ({ docType: q.docType, docId: q.docId }));
    if (items.length === 0) return;
    setBatching(true);
    setBatchResult(null);
    setError(null);
    try {
      const res = await financeService.postBatch(items);
      setBatchResult(res.data);
      const ok = res.data.filter((r) => !r.error).length;
      const fail = res.data.length - ok;
      if (fail === 0) toast.success(`Diposting ${ok} dokumen`);
      else toast.danger(`Diposting ${ok}, gagal ${fail} — perbaiki lalu ulangi`);
      reloadQueue();
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      toast.danger("Gagal posting massal", apiErr.message);
    } finally {
      setBatching(false);
    }
  }

  async function handleCloseDay(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (closeDate.trim() === "") {
      setError("Tanggal tutup harian wajib diisi.");
      return;
    }
    setClosing(true);
    setBatchResult(null);
    setError(null);
    try {
      const res = await financeService.closeDay(closeDate.trim());
      setBatchResult(res.data);
      toast.success(`Tutup harian: ${res.data.length} dokumen diposting`);
      reloadQueue();
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      toast.danger("Gagal tutup harian", apiErr.message);
    } finally {
      setClosing(false);
    }
  }

  function parseForm(): { docType: string; docId: number } | null {
    const parsed = postingFormSchema.safeParse({ docType, docId });
    if (!parsed.success) {
      setFieldError(
        parsed.error.flatten().fieldErrors.docId?.[0] ??
          "Pilih tipe dan isi ID dokumen",
      );
      return null;
    }
    setFieldError(undefined);
    return { docType: parsed.data.docType, docId: parsed.data.docId };
  }

  async function handlePreview(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = parseForm();
    if (!form) return;
    setLoadingPreview(true);
    setError(null);
    setPosted(null);
    try {
      const res = await financeService.preview(form.docType, form.docId);
      setPreview(res);
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      setPreview(null);
      toast.danger("Gagal memuat pratinjau", apiErr.message);
    } finally {
      setLoadingPreview(false);
    }
  }

  async function handlePost() {
    const form = parseForm();
    if (!form) return;
    setPosting(true);
    setError(null);
    try {
      const res = await financeService.post(form.docType, form.docId);
      setPosted(res.data);
      setPreview(res.data);
      toast.fromServer(res.message, "Jurnal diposting", res.data.number);
      reloadQueue();
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      toast.danger("Gagal memposting jurnal", apiErr.message);
    } finally {
      setPosting(false);
    }
  }

  function previewRow(q: PostingSource) {
    setDocType(q.docType);
    setDocId(String(q.docId));
    setPosted(null);
    setLoadingPreview(true);
    setError(null);
    financeService
      .preview(q.docType, q.docId)
      .then((res) => setPreview(res))
      .catch((err) => {
        const apiErr = toApiError(err);
        setError(apiErr.message);
        setPreview(null);
        toast.danger("Gagal memuat pratinjau", apiErr.message);
      })
      .finally(() => setLoadingPreview(false));
  }

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Posting"
        description="Dokumen terkonfirmasi yang belum dijurnal — pilih lalu posting massal, atau tutup harian."
      />

      <SectionCard
        title="Antrean Posting"
        description="Diturunkan live dari dokumen + indeks jurnal (tanpa tabel antrean) — tidak bisa basi."
        actions={
          <Button
            variant="secondary"
            size="sm"
            onClick={() => reloadQueue()}
            disabled={queueLoading}
          >
            Muat Ulang
          </Button>
        }
      >
        {queueError ? (
          <Notice tone="danger" title="Gagal memuat antrean">
            {queueError}{" "}
            <button
              type="button"
              onClick={() => reloadQueue()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        ) : queueLoading ? (
          <p className="text-muted px-1 py-6 text-sm">Memuat antrean…</p>
        ) : (
          <>
            <DataTable<PostingSource>
              rows={queue}
              rowKey={(r) => `${r.docType}:${r.docId}`}
              emptyTitle="Antrean kosong"
              emptyDescription="Semua dokumen terkonfirmasi sudah dijurnal."
              columns={[
                {
                  header: "",
                  render: (r) => (
                    <input
                      type="checkbox"
                      checked={selected.has(`${r.docType}:${r.docId}`)}
                      onChange={() => toggleRow(`${r.docType}:${r.docId}`)}
                      disabled={!canPost}
                      aria-label={`Pilih ${r.number}`}
                    />
                  ),
                },
                {
                  header: "Dokumen",
                  render: (r) => (
                    <span>
                      <span className="font-semibold">{r.number}</span>{" "}
                      <span className="text-muted">
                        {docTypeLabel(r.docType)}
                      </span>
                    </span>
                  ),
                },
                {
                  header: "Tanggal",
                  render: (r) =>
                    r.date ? formatDate(r.date.slice(0, 10)) : "-",
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={!canPost || loadingPreview}
                      onClick={() => previewRow(r)}
                    >
                      Pratinjau
                    </Button>
                  ),
                },
              ]}
            />
            <div className="flex flex-wrap items-center justify-between gap-3 px-1 pt-4">
              <label className="flex cursor-pointer items-center gap-2 text-sm text-ink">
                <input
                  type="checkbox"
                  checked={queue.length > 0 && selected.size === queue.length}
                  onChange={toggleAll}
                  disabled={!canPost || queue.length === 0}
                />
                Pilih semua ({selected.size}/{queue.length})
              </label>
              <Button
                onClick={() => void handleBatch()}
                disabled={!canPost || batching || selected.size === 0}
              >
                {batching
                  ? "Memposting…"
                  : `Posting ${selected.size} Terpilih`}
              </Button>
            </div>
          </>
        )}
      </SectionCard>

      <div className="mt-4">
        <SectionCard
          title="Tutup Harian"
          description="Posting semua antrean tertanggal ≤ tanggal tutup (maks 50 per jalan)."
        >
          <form
            onSubmit={(e) => void handleCloseDay(e)}
            className="flex flex-wrap items-end gap-4"
          >
            <div className="min-w-44">
              <FormField label="Tanggal" required htmlFor="close-day">
                <DateInput
                  id="close-day"
                  value={closeDate}
                  max={todayWIB()}
                  onChange={(e) => setCloseDate(e.target.value)}
                  disabled={closing || !canPost}
                />
              </FormField>
            </div>
            <Button type="submit" disabled={closing || !canPost}>
              {closing ? "Menutup…" : "Tutup Harian"}
            </Button>
          </form>
        </SectionCard>
      </div>

      {batchResult && (
        <div className="mt-4">
          <SectionCard
            title="Hasil Posting"
            description={`${batchResult.filter((r) => !r.error).length} berhasil, ${batchResult.filter((r) => r.error).length} gagal.`}
          >
            <DataTable<BatchPostResult>
              rows={batchResult}
              rowKey={(r) => `${r.docType}:${r.docId}`}
              emptyTitle="Tidak ada hasil"
              columns={[
                {
                  header: "Dokumen",
                  render: (r) => (
                    <span>
                      <span className="font-semibold">
                        {docTypeLabel(r.docType)}
                      </span>{" "}
                      <span className="text-muted">#{r.docId}</span>
                    </span>
                  ),
                },
                {
                  header: "Hasil",
                  render: (r) =>
                    r.error ? (
                      <Badge tone="danger">{r.error}</Badge>
                    ) : (
                      <Link
                        to={`${ROUTE_PATHS.financeJournals}/${r.entryId}`}
                        className="font-semibold text-brand hover:underline"
                      >
                        {r.entryNumber}
                      </Link>
                    ),
                },
              ]}
            />
          </SectionCard>
        </div>
      )}

      <div className="mt-4">
        <SectionCard
          title="Posting Manual per ID"
          description="Jalur darurat bila dokumen diketahui ID-nya tanpa lewat antrean."
        >
          <form
            onSubmit={(e) => void handlePreview(e)}
            className="flex flex-wrap items-end gap-4"
          >
            <div className="min-w-52 flex-1">
              <FormField label="Tipe Dokumen">
                <SelectInput
                  value={docType}
                  onChange={(e) => setDocType(e.target.value)}
                >
                  {POSTING_DOC_TYPES.map((t) => (
                    <option key={t} value={t}>
                      {docTypeLabel(t)}
                    </option>
                  ))}
                </SelectInput>
              </FormField>
            </div>
            <div className="min-w-44">
              <FormField label="ID Dokumen" errorText={fieldError}>
                <TextInput
                  value={docId}
                  onChange={(e) => setDocId(e.target.value)}
                  placeholder="cth. 12"
                  inputMode="numeric"
                />
              </FormField>
            </div>
            <Button
              type="submit"
              variant="secondary"
              disabled={!canPost || loadingPreview}
            >
              {loadingPreview ? "Memuat…" : "Pratinjau"}
            </Button>
          </form>
        </SectionCard>
      </div>

      {error && (
        <div className="mt-4">
          <Notice tone="danger" title={error} />
        </div>
      )}

      {posted && (
        <div className="mt-4">
          <Notice tone="success" title={`Diposting sebagai ${posted.number}`}>
            <Link
              to={`${ROUTE_PATHS.financeJournals}/${posted.id}`}
              className="font-semibold text-brand hover:underline"
            >
              Buka jurnal {posted.number}
            </Link>
          </Notice>
        </div>
      )}

      {preview && (
        <div className="mt-4">
          <SectionCard
            title="Pratinjau Jurnal"
            description={`${preview.memo || "-"} · ${formatDate(preview.date || todayWIB())}`}
            actions={
              <Badge tone={balanced ? "success" : "danger"}>
                {balanced ? "Seimbang" : "Tidak seimbang"}
              </Badge>
            }
          >
            <DataTable
              rows={preview.lines ?? []}
              rowKey={(l) => `${l.accountId}-${l.debit}-${l.credit}`}
              emptyTitle="Tidak ada baris jurnal"
              columns={[
                {
                  header: "Akun",
                  render: (l) => `${l.accountCode} — ${l.accountName}`,
                },
                {
                  header: "Debit",
                  align: "right",
                  render: (l) => formatIDR(l.debit),
                },
                {
                  header: "Kredit",
                  align: "right",
                  render: (l) => formatIDR(l.credit),
                },
              ]}
            />
            <div className="flex flex-wrap items-center justify-between gap-3 px-1 pt-4">
              <p className="text-sm text-ink">
                Total debit{" "}
                <span className="font-semibold">{formatIDR(totals.debit)}</span>{" "}
                · total kredit{" "}
                <span className="font-semibold">
                  {formatIDR(totals.credit)}
                </span>
              </p>
              <Button
                onClick={() => void handlePost()}
                disabled={!canPost || posting || !balanced}
              >
                {posting ? "Memposting…" : "Posting Jurnal"}
              </Button>
            </div>
          </SectionCard>
        </div>
      )}
    </div>
  );
}
