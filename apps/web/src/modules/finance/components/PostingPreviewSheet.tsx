import { useState } from "react";
import { Link } from "react-router-dom";
import { Button, DetailSkeleton, FormSheet } from "@/shared/components/ui";
import { DescriptionList } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { isMissingCostError } from "@/modules/finance/lib/posting";
import { isEntryBalanced } from "@/modules/finance/lib/journal-lines";
import { BalanceBadge, JournalLinesTable } from "@/modules/finance/components/JournalLinesTable";
import { MissingCostNotice } from "@/modules/finance/components/MissingCostNotice";
import { docTypeLabel } from "@/modules/finance/types";
import type { JournalEntry } from "@/modules/finance/types";

export interface PostingTarget {
  docType: string;
  docId: number;
  /** Nomor dokumen bila diketahui (dari antrean). */
  number?: string;
}

interface PostingPreviewSheetProps {
  target: PostingTarget;
  canPost: boolean;
  onClose: () => void;
  /** Dipanggil setelah jurnal berhasil diposting (muat ulang antrean). */
  onPosted: () => void;
}

/**
 * Pratinjau jurnal satu dokumen dalam panel samping. Pratinjau memakai builder yang sama dengan Posting,
 * jadi tidak pernah meleset. Posting hanya aktif bila jurnal seimbang.
 */
export function PostingPreviewSheet({ target, canPost, onClose, onPosted }: PostingPreviewSheetProps) {
  const { data: preview, loading, error, reload } = useAsyncData(() => financeService.preview(target.docType, target.docId), [target.docType, target.docId]);
  const [posted, setPosted] = useState<JournalEntry | null>(null);
  const [posting, setPosting] = useState(false);
  const [postError, setPostError] = useState<string | null>(null);

  const entry = posted ?? preview;
  const balanced = isEntryBalanced(entry?.lines);
  const title = target.number ? `Pratinjau ${target.number}` : `Pratinjau ${docTypeLabel(target.docType)} #${target.docId}`;

  async function handlePost() {
    setPosting(true);
    setPostError(null);
    try {
      const res = await financeService.post(target.docType, target.docId);
      setPosted(res.data);
      toast.fromServer(res.message, "Jurnal diposting", res.data.number);
      onPosted();
    } catch (err) {
      const apiErr = toApiError(err);
      setPostError(apiErr.message);
      toast.danger("Gagal memposting jurnal", apiErr.message);
    } finally {
      setPosting(false);
    }
  }

  return (
    <FormSheet
      open
      size="lg"
      title={title}
      description={`${docTypeLabel(target.docType)} #${target.docId} — jurnal yang akan dibukukan.`}
      badge={entry?.lines && entry.lines.length > 0 ? <BalanceBadge balanced={balanced} /> : undefined}
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" onClick={onClose}>
            {posted ? "Tutup" : "Batal"}
          </Button>
          {!posted && (
            <Button loading={posting} disabled={!canPost || !balanced || loading || Boolean(error)} onClick={() => void handlePost()}>
              Posting jurnal
            </Button>
          )}
        </>
      }
    >
      {loading && !preview ? (
        <DetailSkeleton />
      ) : error ? (
        isMissingCostError(error) ? (
          <MissingCostNotice message={error} canPost={canPost} />
        ) : (
          <ErrorState message={error} onRetry={reload} title="Gagal memuat pratinjau" />
        )
      ) : entry ? (
        <div className="flex flex-col gap-5">
          {posted && (
            <Notice tone="success" title={`Diposting sebagai ${posted.number}`}>
              <Link to={`${ROUTE_PATHS.financeJournals}/${posted.id}`} className="font-medium text-primary hover:underline">
                Buka jurnal {posted.number}
              </Link>
            </Notice>
          )}
          {postError &&
            (isMissingCostError(postError) ? (
              <MissingCostNotice message={postError} canPost={canPost} />
            ) : (
              <Notice tone="danger" title="Jurnal gagal diposting">
                {postError}
              </Notice>
            ))}
          <DescriptionList
            columns={2}
            items={[
              { label: "Tanggal", value: formatDate(entry.date || todayWIB()) },
              { label: "Memo", value: entry.memo },
            ]}
          />
          <div className="overflow-hidden rounded-lg border border-border">
            <JournalLinesTable lines={entry.lines} caption="Pratinjau baris jurnal" />
          </div>
        </div>
      ) : null}
    </FormSheet>
  );
}
