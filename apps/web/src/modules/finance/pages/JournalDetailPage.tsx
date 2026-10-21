import { useState } from "react";
import type { FormEvent } from "react";
import { useParams } from "react-router-dom";
import { RotateCcw } from "lucide-react";
import { Button, DetailSkeleton, FormField, Modal, PageHeader, SectionCard, StatusBadge, TextArea } from "@/shared/components/ui";
import { ActionMenu, DescriptionList, DocNumber, DocumentRail, Page } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { formatDate } from "@/shared/lib/format";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { reverseSchema } from "@/modules/finance/schemas/finance.schema";
import { JOURNAL_RAIL_STEPS, journalRailCurrent, journalRelated } from "@/modules/finance/lib/document-rails";
import { isEntryBalanced } from "@/modules/finance/lib/journal-lines";
import { BalanceBadge, JournalLinesTable } from "@/modules/finance/components/JournalLinesTable";
import { journalSourceLabel } from "@/modules/finance/types";

// Detail jurnal (T2): tabel Debit/Kredit seimbang + total. Balikkan = dialog alasan wajib →
// POST /:id/reverse {reason} (gate finance.journal.reverse; disembunyikan bila sudah dibalik).
// Jurnal pembalik (sourceType reversal) juga tak bisa dibalik (aturan backend).
export default function JournalDetailPage() {
  const { id } = useParams();
  const entryId = Number(id);
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.journal.reverse");

  const { data: entry, loading, error, errorStatus, reload } = useAsyncData(() => {
    if (!Number.isFinite(entryId) || entryId <= 0) return Promise.reject(new Error("ID jurnal tidak valid"));
    return financeService.journal(entryId);
  }, [entryId]);
  usePageCrumb(entry?.number);

  const [reverseOpen, setReverseOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState<string | undefined>(undefined);
  const [reversing, setReversing] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  async function handleReverse(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = reverseSchema.safeParse({ reason: reason.trim() });
    if (!parsed.success) {
      setReasonError(parsed.error.flatten().fieldErrors.reason?.[0] ?? "Alasan wajib diisi");
      return;
    }
    setReasonError(undefined);
    setReversing(true);
    setServerError(null);
    try {
      const res = await financeService.reverse(entryId, parsed.data.reason);
      toast.fromServer(res.message, "Jurnal dibalik", res.data.number);
      setReverseOpen(false);
      setReason("");
      reload();
    } catch (err) {
      setServerError(formatApiError(err));
    } finally {
      setReversing(false);
    }
  }

  if (loading && !entry) return <DetailSkeleton />;
  if (errorStatus === 404 || (!error && !entry)) {
    return <NotFoundState title="Jurnal tidak ditemukan" message="Jurnal ini tidak ada atau sudah dihapus." backTo={ROUTE_PATHS.financeJournals} backLabel="Ke daftar jurnal" />;
  }
  if (error || !entry) {
    return (
      <Page>
        <PageHeader title="Detail jurnal" />
        <ErrorState message={error ?? "Jurnal tidak dapat dimuat"} onRetry={reload} />
      </Page>
    );
  }

  const isReversed = entry.status === "reversed";
  const isReversal = entry.sourceType === "reversal";
  const canReverse = canManage && !isReversed && !isReversal;
  const balanced = isEntryBalanced(entry.lines);

  return (
    <Page>
      <PageHeader
        title={entry.number}
        meta={<StatusBadge status={entry.status} />}
        subtitle={`${entry.memo || "Tanpa memo"} · ${formatDate(entry.date)}`}
        actions={
          canReverse ? (
            <ActionMenu
              items={[
                {
                  key: "reverse",
                  label: "Balikkan jurnal",
                  icon: <RotateCcw aria-hidden />,
                  destructive: true,
                  onSelect: () => {
                    setServerError(null);
                    setReasonError(undefined);
                    setReverseOpen(true);
                  },
                },
              ]}
            />
          ) : undefined
        }
      >
        <DocumentRail steps={JOURNAL_RAIL_STEPS} current={journalRailCurrent(entry)} stopped={isReversed} related={journalRelated(entry)} />
      </PageHeader>

      {isReversed && (
        <Notice tone="info" title="Jurnal ini sudah dibalik">
          {entry.reversedBy > 0 ? "Buka jurnal pembalik lewat Rel Dokumen di atas." : "Tidak dapat dibalik lagi — tulis jurnal manual bila perlu koreksi."}
        </Notice>
      )}

      <SectionCard
        flush
        title="Baris jurnal"
        actions={entry.lines && entry.lines.length > 0 ? <BalanceBadge balanced={balanced} /> : undefined}
      >
        <JournalLinesTable lines={entry.lines} />
      </SectionCard>

      <SectionCard title="Informasi">
        <DescriptionList
          columns={3}
          items={[
            { label: "Nomor", value: <DocNumber value={entry.number} copyable /> },
            { label: "Tanggal", value: formatDate(entry.date) },
            { label: "Sumber", value: journalSourceLabel(entry) },
            { label: "Memo", value: entry.memo, wide: true },
          ]}
        />
      </SectionCard>

      {reverseOpen && (
        <Modal
          open
          size="sm"
          title={`Balikkan ${entry.number}`}
          description="Membukukan jurnal cermin dan menandai jurnal ini dibalik."
          onClose={() => setReverseOpen(false)}
          actions={
            <>
              <Button variant="secondary" disabled={reversing} onClick={() => setReverseOpen(false)}>
                Batal
              </Button>
              <Button type="submit" form="reverse-form" variant="danger" loading={reversing}>
                Balikkan
              </Button>
            </>
          }
        >
          <form id="reverse-form" onSubmit={(e) => void handleReverse(e)} className="flex flex-col gap-4">
            {serverError && <Notice tone="danger" title={serverError} />}
            <FormField label="Alasan" htmlFor="reverse-reason" required errorText={reasonError}>
              <TextArea
                id="reverse-reason"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="cth. Salah tanggal posting"
                rows={3}
              />
            </FormField>
          </form>
        </Modal>
      )}
    </Page>
  );
}
