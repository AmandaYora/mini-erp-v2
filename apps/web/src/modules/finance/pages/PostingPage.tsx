import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import { Ban, Eye, RefreshCw, Undo2 } from "lucide-react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  DateInput,
  FormField,
  PageHeader,
  SectionCard,
  SelectInput,
  SummaryCard,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  TextInput,
} from "@/shared/components/ui";
import { DocNumber, Money, Page, useUrlTab } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatNumber, todayWIB } from "@/shared/lib/format";
import { formatApiError, toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { CutoverCard } from "@/modules/finance/components/CutoverCard";
import { DailyOverviewCard } from "@/modules/finance/components/DailyOverviewCard";
import { IgnorePostingModal } from "@/modules/finance/components/IgnorePostingModal";
import { MissingCostNotice } from "@/modules/finance/components/MissingCostNotice";
import { OrphansCard } from "@/modules/finance/components/OrphansCard";
import { PostingPreviewSheet } from "@/modules/finance/components/PostingPreviewSheet";
import type { PostingTarget } from "@/modules/finance/components/PostingPreviewSheet";
import {
  CATCHUP_PERMISSION,
  buildBatch,
  buildCatchup,
  closeDayCandidates,
  closedQueue,
  ignoredQueue,
  isMissingCostError,
  queueKey,
  sanitizeSelection,
  selectableQueue,
} from "@/modules/finance/lib/posting";
import { postingFormSchema } from "@/modules/finance/schemas/finance.schema";
import { MAX_BATCH, POSTING_DOC_TYPES, docTypeLabel } from "@/modules/finance/types";
import type { BatchPostResult, PostingSource } from "@/modules/finance/types";

const TABS = ["antrian", "tutup-hari", "hasil", "manual"] as const;
type PostingTab = (typeof TABS)[number];

// Konsol posting (T7): dokumen terkonfirmasi yang belum dijurnal. Tabs: Antrian · Tutup Hari · Hasil ·
// Posting Manual. Antrian diturunkan live dari dokumen + indeks jurnal (tanpa tabel antrean) sehingga
// tidak bisa basi. Pratinjau memakai builder yang sama dengan Posting.
export default function PostingPage() {
  const can = useAuthStore((s) => s.can);
  const canPost = can("finance.posting.post");
  const canManage = can("finance.manage");
  const canCatchUp = can(CATCHUP_PERMISSION);

  const [tab, setTab] = useUrlTab(TABS, "antrian");
  const { data: queueData, loading: queueLoading, error: queueError, reload: reloadQueue } = useAsyncData(
    () => financeService.postingSources().then((rows) => rows ?? []),
    [],
  );
  const queue = queueData ?? [];
  const ready = selectableQueue(queue);
  const closed = closedQueue(queue);
  const ignored = ignoredQueue(queue);

  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [prevQueueData, setPrevQueueData] = useState(queueData);
  if (queueData !== prevQueueData) {
    setPrevQueueData(queueData);
    setSelected(new Set());
  }

  const [batchConfirm, setBatchConfirm] = useState(false);
  const [catchUpTarget, setCatchUpTarget] = useState<PostingSource | null>(null);
  const [closeDayConfirm, setCloseDayConfirm] = useState(false);
  const [ignoreTarget, setIgnoreTarget] = useState<PostingSource | null>(null);
  const [busy, setBusy] = useState(false);
  const [batchResult, setBatchResult] = useState<BatchPostResult[] | null>(null);
  // Ringkasan tutup harian terakhir; `remaining` > 0 = antrean belum habis (N-52).
  const [closeSummary, setCloseSummary] = useState<{ posted: number; failed: number; remaining: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [closeDate, setCloseDate] = useState(todayWIB());

  const [docType, setDocType] = useState<string>("delivery");
  const [docId, setDocId] = useState("");
  const [fieldError, setFieldError] = useState<string | undefined>(undefined);
  const [target, setTarget] = useState<PostingTarget | null>(null);

  const candidates = closeDayCandidates(queue, closeDate);
  const costError = [error, ...(batchResult ?? []).map((r) => r.error)].find((m) => isMissingCostError(m));
  const okCount = (batchResult ?? []).filter((r) => !r.error).length;
  const failCount = (batchResult ?? []).length - okCount;

  function showResult(res: BatchPostResult[]) {
    setBatchResult(res);
    setTab("hasil");
  }

  async function handleBatch() {
    const items = buildBatch(queue, selected);
    if (items.length === 0) return;
    setBusy(true);
    setBatchResult(null);
    setError(null);
    try {
      const res = await financeService.postBatch(items);
      const ok = res.data.filter((r) => !r.error).length;
      const fail = res.data.length - ok;
      if (fail === 0) toast.success(`Diposting ${ok} dokumen`);
      else toast.danger(`Diposting ${ok}, gagal ${fail} — perbaiki lalu ulangi`);
      showResult(res.data);
      reloadQueue();
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      toast.danger("Gagal posting massal", apiErr.message);
    } finally {
      setBusy(false);
      setBatchConfirm(false);
    }
  }

  async function handleCatchUp() {
    const row = catchUpTarget;
    const item = row ? buildCatchup(row) : null;
    if (!row || !item) return;
    setBusy(true);
    setBatchResult(null);
    setError(null);
    try {
      const res = await financeService.postBatch([item]);
      toast.fromServer(res.message, "Jurnal susulan diproses");
      showResult(res.data);
      reloadQueue();
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      toast.danger("Gagal jurnal susulan", apiErr.message);
    } finally {
      setBusy(false);
      setCatchUpTarget(null);
    }
  }

  async function handleCloseDay() {
    if (closeDate.trim() === "") {
      setError("Tanggal tutup harian wajib diisi.");
      setCloseDayConfirm(false);
      return;
    }
    setBusy(true);
    setBatchResult(null);
    setError(null);
    try {
      const res = await financeService.closeDay(closeDate.trim());
      const { results, posted, failed, remaining } = res.data;
      setCloseSummary({ posted, failed, remaining });
      if (remaining > 0) toast.warning(`Tutup harian: ${posted} diposting, ${remaining} tersisa — jalankan lagi`);
      else toast.success(`Tutup harian: ${posted} dokumen diposting`);
      showResult(results);
      reloadQueue();
    } catch (err) {
      const apiErr = toApiError(err);
      setError(apiErr.message);
      toast.danger("Gagal tutup harian", apiErr.message);
    } finally {
      setBusy(false);
      setCloseDayConfirm(false);
    }
  }

  async function handleIgnore(reason: string) {
    const row = ignoreTarget;
    if (!row) return;
    setBusy(true);
    try {
      const res = await financeService.ignorePosting(row.docType, row.docId, reason);
      toast.fromServer(res.message, "Dokumen diabaikan", row.number);
      setIgnoreTarget(null);
      reloadQueue();
    } catch (err) {
      toast.danger("Gagal mengabaikan dokumen", formatApiError(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleRestore(row: PostingSource) {
    setBusy(true);
    try {
      const res = await financeService.restorePosting(row.docType, row.docId);
      toast.fromServer(res.message, "Dokumen dikembalikan ke antrian", row.number);
      reloadQueue();
    } catch (err) {
      toast.danger("Gagal memulihkan dokumen", formatApiError(err));
    } finally {
      setBusy(false);
    }
  }

  function handleManualPreview(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = postingFormSchema.safeParse({ docType, docId });
    if (!parsed.success) {
      setFieldError(parsed.error.flatten().fieldErrors.docId?.[0] ?? "Pilih tipe dan isi ID dokumen");
      return;
    }
    setFieldError(undefined);
    setTarget({ docType: parsed.data.docType, docId: parsed.data.docId });
  }

  return (
    <Page>
      <PageHeader
        title="Posting"
        description="Dokumen terkonfirmasi yang belum dijurnal — pilih lalu posting massal, atau tutup harian."
        actions={
          <Button variant="secondary" size="sm" loading={queueLoading} onClick={reloadQueue}>
            <RefreshCw aria-hidden /> Muat ulang
          </Button>
        }
      />

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <SummaryCard
          label="Menunggu posting"
          value={formatNumber(ready.length)}
          tone={ready.length > 0 ? "warning" : "success"}
          hint={ready.length > 0 ? "Siap dijurnalkan" : "Antrean bersih"}
          onClick={() => setTab("antrian")}
        />
        <SummaryCard
          label="Di periode tertutup"
          value={formatNumber(closed.length)}
          tone={closed.length > 0 ? "danger" : "success"}
          hint={closed.length > 0 ? "Tidak ikut posting massal" : undefined}
          onClick={() => setTab("antrian")}
        />
        <SummaryCard
          label="Hasil terakhir"
          value={batchResult ? `${formatNumber(okCount)} berhasil` : "—"}
          tone={failCount > 0 ? "danger" : "brand"}
          hint={batchResult ? `${formatNumber(failCount)} gagal` : "Belum ada posting pada sesi ini"}
          onClick={() => setTab("hasil")}
        />
      </div>

      {error && !isMissingCostError(error) && <Notice tone="danger" title={error} />}
      {costError && <MissingCostNotice message={costError} canPost={canPost} />}

      <Tabs value={tab} onValueChange={(v) => setTab(v as PostingTab)} className="gap-6">
        <TabsList variant="line">
          <TabsTrigger value="antrian">Antrian</TabsTrigger>
          <TabsTrigger value="tutup-hari">Tutup hari</TabsTrigger>
          <TabsTrigger value="hasil">
            Hasil {batchResult && <Badge tone={failCount > 0 ? "danger" : "success"}>{batchResult.length}</Badge>}
          </TabsTrigger>
          <TabsTrigger value="manual">Posting manual</TabsTrigger>
        </TabsList>

        <TabsContent value="antrian" className="flex flex-col gap-6">
          <SectionCard flush title="Siap diposting" description="Pilih dokumen (maksimal 50 per posting) lalu posting sekaligus.">
            <DataTable<PostingSource>
              caption="Antrian posting"
              rows={ready}
              rowKey={queueKey}
              loading={queueLoading}
              error={queueError}
              onRetry={reloadQueue}
              selectable={canPost}
              selectedKeys={selected}
              onSelectionChange={(keys) => setSelected(sanitizeSelection(queue, keys))}
              bulkActions={(keys) => (
                <Button size="sm" disabled={!canPost || busy} onClick={() => setBatchConfirm(true)}>
                  Posting {keys.size} dokumen
                </Button>
              )}
              emptyTitle="Antrean kosong"
              emptyDescription="Semua dokumen terkonfirmasi sudah dijurnal."
              columns={[
                {
                  header: "Dokumen",
                  render: (r) => (
                    <span className="inline-flex flex-wrap items-center gap-2">
                      <DocNumber value={r.number} /> <span className="text-muted-foreground">{docTypeLabel(r.docType)}</span>
                      {r.awaitingOrder && <Badge tone="warning">Menunggu order</Badge>}
                    </span>
                  ),
                },
                { header: "Tanggal", render: (r) => (r.date ? formatDate(r.date.slice(0, 10)) : "-") },
                {
                  header: "Pihak",
                  render: (r) => (
                    <span>
                      {r.partyName || "-"}
                      {r.summary && <span className="block text-xs text-muted-foreground">{r.summary}</span>}
                    </span>
                  ),
                },
                {
                  header: "Nominal",
                  align: "right",
                  render: (r) =>
                    r.amount ? (
                      <span>
                        <Money value={r.amount} />
                        {r.amountKind === "order" && <span className="block text-xs text-muted-foreground">total pesanan</span>}
                      </span>
                    ) : (
                      "-"
                    ),
                },
                {
                  header: "",
                  align: "right",
                  render: (r) => (
                    <span className="inline-flex gap-1">
                      <Button variant="ghost" size="sm" disabled={!canPost} onClick={() => setTarget({ docType: r.docType, docId: r.docId, number: r.number })}>
                        <Eye aria-hidden /> Pratinjau
                      </Button>
                      {/* SJ Langsung tanpa order tidak boleh diabaikan (server menolak): ordernya dibuat atau SJ-nya dibatalkan. */}
                      {!r.awaitingOrder && (
                        <Button variant="ghost" size="sm" disabled={!canPost || busy} onClick={() => setIgnoreTarget(r)}>
                          <Ban aria-hidden /> Abaikan
                        </Button>
                      )}
                    </span>
                  ),
                },
              ]}
            />
          </SectionCard>

          {closed.length > 0 && (
            <SectionCard flush title="Di periode tertutup" description="Tetap terlihat sebagai peringatan, tetapi tidak ikut posting massal atau tutup harian.">
              <div className="p-4">
                <Notice tone="warning" title={`${closed.length} dokumen jatuh di periode yang sudah ditutup`}>
                  Buka ulang periodenya, atau
                  {canCatchUp
                    ? " pakai Jurnalkan susulan untuk mencatatnya di periode terbuka (ditandai Susulan)."
                    : " minta pengguna berizin Tutup Periode Tanpa Gerbang menjurnalkannya sebagai Susulan."}
                </Notice>
              </div>
              <DataTable<PostingSource>
                caption="Dokumen di periode tertutup"
                rows={closed}
                rowKey={queueKey}
                emptyTitle="Tidak ada"
                columns={[
                  {
                    header: "Dokumen",
                    render: (r) => (
                      <span className="inline-flex flex-wrap items-center gap-2">
                        <DocNumber value={r.number} />
                        <span className="text-muted-foreground">{docTypeLabel(r.docType)}</span>
                        <Badge tone="warning">Periode tertutup</Badge>
                      </span>
                    ),
                  },
                  { header: "Tanggal", render: (r) => (r.date ? formatDate(r.date.slice(0, 10)) : "-") },
                  {
                    header: "",
                    align: "right",
                    render: (r) =>
                      canCatchUp ? (
                        <Button variant="secondary" size="sm" disabled={busy} onClick={() => setCatchUpTarget(r)}>
                          Jurnalkan susulan
                        </Button>
                      ) : null,
                  },
                ]}
              />
            </SectionCard>
          )}

          {ignored.length > 0 && (
            <SectionCard flush title="Diabaikan" description="Sengaja dikeluarkan dari antrian dengan alasan. Tidak memblokir tutup periode (peringatan); bisa dipulihkan.">
              <DataTable<PostingSource>
                caption="Dokumen diabaikan"
                rows={ignored}
                rowKey={queueKey}
                emptyTitle="Tidak ada"
                columns={[
                  {
                    header: "Dokumen",
                    render: (r) => (
                      <span className="inline-flex flex-wrap items-center gap-2">
                        <DocNumber value={r.number} />
                        <span className="text-muted-foreground">{docTypeLabel(r.docType)}</span>
                        <Badge tone="neutral">Diabaikan</Badge>
                      </span>
                    ),
                  },
                  { header: "Tanggal", render: (r) => (r.date ? formatDate(r.date.slice(0, 10)) : "-") },
                  { header: "Alasan", render: (r) => r.ignoreReason || "-" },
                  {
                    header: "",
                    align: "right",
                    render: (r) => (
                      <Button variant="secondary" size="sm" disabled={!canPost || busy} onClick={() => void handleRestore(r)}>
                        <Undo2 aria-hidden /> Pulihkan
                      </Button>
                    ),
                  },
                ]}
              />
            </SectionCard>
          )}

          <OrphansCard canPost={canPost} onChanged={reloadQueue} />
        </TabsContent>

        <TabsContent value="tutup-hari" className="flex flex-col gap-6">
          <DailyOverviewCard date={closeDate} refreshKey={queueData} />
          <SectionCard title="Tutup hari" description="Posting semua antrean bertanggal sampai tanggal tutup. Berjalan per 50 dokumen sampai habis (maksimal 500 per jalan); sisanya dilaporkan.">
            <div className="flex flex-wrap items-end gap-4">
              <FormField label="Tanggal tutup" htmlFor="close-day" required>
                <DateInput id="close-day" value={closeDate} max={todayWIB()} onChange={(e) => setCloseDate(e.target.value)} disabled={busy || !canPost} />
              </FormField>
              <Button disabled={!canPost || busy || candidates.length === 0} onClick={() => setCloseDayConfirm(true)}>
                Tutup hari ({candidates.length})
              </Button>
            </div>
            {closeSummary && closeSummary.remaining > 0 && (
              <div className="mt-3">
                <Notice tone="warning" title={`${closeSummary.posted} diposting, ${closeSummary.remaining} tersisa — jalankan lagi`}>
                  {closeSummary.failed > 0
                    ? `${closeSummary.failed} dokumen gagal diposting; periksa tab Hasil, perbaiki penyebabnya, lalu jalankan tutup hari lagi.`
                    : "Batas satu jalan tercapai; jalankan tutup hari lagi untuk menuntaskan antrean."}
                </Notice>
              </div>
            )}
            <p className="mt-3 text-sm text-muted-foreground">
              {candidates.length === 0
                ? "Tidak ada dokumen bertanggal sampai tanggal ini yang menunggu posting."
                : `${candidates.length} dokumen akan diposting.`}
            </p>
          </SectionCard>
          <CutoverCard canManage={canManage} onChanged={reloadQueue} />
        </TabsContent>

        <TabsContent value="hasil" className="flex flex-col gap-6">
          <SectionCard
            flush
            title="Hasil posting"
            description={batchResult ? `${okCount} berhasil, ${failCount} gagal.` : "Hasil posting massal, tutup hari, dan jurnal susulan tampil di sini."}
          >
            <DataTable<BatchPostResult>
              caption="Hasil posting"
              rows={batchResult ?? []}
              rowKey={(r) => `${r.docType}:${r.docId}`}
              emptyTitle="Belum ada hasil"
              emptyDescription="Jalankan posting massal atau tutup hari untuk melihat hasilnya."
              columns={[
                {
                  header: "Dokumen",
                  render: (r) => (
                    <span>
                      <span className="font-medium">{docTypeLabel(r.docType)}</span> <span className="text-muted-foreground">#{r.docId}</span>
                    </span>
                  ),
                },
                {
                  header: "Hasil",
                  render: (r) =>
                    r.error ? (
                      <Badge tone="danger">{r.error}</Badge>
                    ) : (
                      <Link to={`${ROUTE_PATHS.financeJournals}/${r.entryId}`} className="font-medium text-primary hover:underline">
                        <DocNumber value={r.entryNumber ?? ""} />
                      </Link>
                    ),
                },
              ]}
            />
          </SectionCard>
        </TabsContent>

        <TabsContent value="manual" className="flex flex-col gap-6">
          <SectionCard title="Posting manual per ID" description="Jalur darurat bila dokumen diketahui ID-nya tanpa lewat antrean. Pratinjau dulu sebelum memposting.">
            <form onSubmit={handleManualPreview} className="flex flex-wrap items-end gap-4" noValidate>
              <div className="min-w-52 flex-1">
                <FormField label="Tipe dokumen" htmlFor="manual-doctype">
                  <SelectInput id="manual-doctype" value={docType} onChange={(e) => setDocType(e.target.value)}>
                    {POSTING_DOC_TYPES.map((t) => (
                      <option key={t} value={t}>
                        {docTypeLabel(t)}
                      </option>
                    ))}
                  </SelectInput>
                </FormField>
              </div>
              <div className="min-w-44">
                <FormField label="ID dokumen" htmlFor="manual-docid" errorText={fieldError}>
                  <TextInput id="manual-docid" value={docId} onChange={(e) => setDocId(e.target.value)} placeholder="cth. 12" inputMode="numeric" />
                </FormField>
              </div>
              <Button type="submit" variant="secondary" disabled={!canPost}>
                Pratinjau
              </Button>
            </form>
          </SectionCard>
        </TabsContent>
      </Tabs>

      {target && (
        <PostingPreviewSheet
          key={`${target.docType}:${target.docId}`}
          target={target}
          canPost={canPost}
          onClose={() => setTarget(null)}
          onPosted={reloadQueue}
        />
      )}

      {ignoreTarget && (
        <IgnorePostingModal
          number={ignoreTarget.number}
          submitting={busy}
          onClose={() => (busy ? undefined : setIgnoreTarget(null))}
          onSubmit={(reason) => void handleIgnore(reason)}
        />
      )}
      <ConfirmDialog
        open={batchConfirm}
        title="Posting dokumen terpilih?"
        message={`${selected.size} dokumen akan diposting ke jurnal (maksimal ${MAX_BATCH} per jalan). Kegagalan satu dokumen tidak menghentikan yang lain.`}
        tone="primary"
        confirmLabel={busy ? "Memposting…" : `Posting ${selected.size} dokumen`}
        onConfirm={() => void handleBatch()}
        onCancel={() => (busy ? undefined : setBatchConfirm(false))}
      />
      <ConfirmDialog
        open={closeDayConfirm}
        title="Tutup hari?"
        message={`${candidates.length} dokumen bertanggal sampai ${formatDate(closeDate)} akan diposting ke jurnal. Lanjutkan?`}
        tone="primary"
        confirmLabel={busy ? "Memposting…" : `Posting ${candidates.length} dokumen`}
        onConfirm={() => void handleCloseDay()}
        onCancel={() => (busy ? undefined : setCloseDayConfirm(false))}
      />
      <ConfirmDialog
        open={catchUpTarget !== null}
        title="Jurnalkan susulan?"
        message={`Jurnalkan ${catchUpTarget?.number ?? ""} ke periode terbuka sebagai Susulan? Periode asalnya yang sudah ditutup tidak diubah.`}
        tone="primary"
        confirmLabel={busy ? "Memproses…" : "Jurnalkan susulan"}
        onConfirm={() => void handleCatchUp()}
        onCancel={() => (busy ? undefined : setCatchUpTarget(null))}
      />
    </Page>
  );
}
