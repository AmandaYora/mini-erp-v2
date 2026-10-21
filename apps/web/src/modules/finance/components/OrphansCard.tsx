import { useState } from "react";
import { Badge, Button, ConfirmDialog, DataTable, SectionCard } from "@/shared/components/ui";
import { DocNumber } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { financeService } from "@/modules/finance/services/finance.service";
import { queueKey } from "@/modules/finance/lib/posting";
import { MAX_BATCH, docTypeLabel } from "@/modules/finance/types";
import type { PostingSource } from "@/modules/finance/types";

interface OrphansCardProps {
  canPost: boolean;
  onChanged: () => void;
}

// Jurnal hidup milik dokumen yang sudah dibatalkan (GET /posting/orphans), T7.
// Perbaikan = post-batch dengan action "reverse" (jurnal pembalik otomatis).
export function OrphansCard({ canPost, onChanged }: OrphansCardProps) {
  const { data, loading, error, reload } = useAsyncData(() => financeService.orphans().then((r) => r ?? []), []);
  const orphans = data ?? [];
  const [busy, setBusy] = useState(false);
  const [failures, setFailures] = useState<string[]>([]);
  const [confirmAll, setConfirmAll] = useState(false);

  async function reverse(items: PostingSource[]) {
    if (items.length === 0) return;
    setBusy(true);
    setFailures([]);
    try {
      const res = await financeService.postBatch(
        items.slice(0, MAX_BATCH).map((o) => ({ docType: o.docType, docId: o.docId, action: "reverse" as const })),
      );
      const bad = res.data.filter((r) => r.error);
      setFailures(bad.map((r) => `${docTypeLabel(r.docType)} #${r.docId}: ${r.error}`));
      toast.fromServer(res.message, bad.length === 0 ? "Jurnal dibalik" : "Sebagian gagal dibalik");
      reload();
      onChanged();
    } catch (err) {
      toast.danger("Gagal membalik jurnal", formatApiError(err));
    } finally {
      setBusy(false);
      setConfirmAll(false);
    }
  }

  const batchCount = Math.min(orphans.length, MAX_BATCH);
  const overflow = orphans.length > MAX_BATCH ? ` (maksimal ${MAX_BATCH} per jalan; ${orphans.length - MAX_BATCH} sisanya di jalan berikutnya)` : "";

  return (
    <SectionCard
      flush
      title="Jurnal dokumen batal"
      description="Dokumen sudah dibatalkan tetapi jurnalnya masih hidup. Balik jurnalnya agar buku sesuai; ini juga menghalangi tutup periode."
      actions={
        <Button variant="secondary" size="sm" disabled={!canPost || busy || orphans.length === 0} onClick={() => setConfirmAll(true)}>
          Balik semua ({batchCount})
        </Button>
      }
    >
      {failures.length > 0 && (
        <div className="p-4">
          <Notice tone="danger" title="Sebagian jurnal gagal dibalik">
            <ul className="list-disc pl-5">
              {failures.map((f) => (
                <li key={f}>{f}</li>
              ))}
            </ul>
          </Notice>
        </div>
      )}
      <DataTable<PostingSource>
        caption="Jurnal dokumen batal"
        rows={orphans}
        rowKey={queueKey}
        loading={loading}
        error={error}
        onRetry={reload}
        emptyTitle="Tidak ada jurnal yatim"
        emptyDescription="Semua dokumen batal sudah dibalik jurnalnya."
        columns={[
          {
            header: "Dokumen",
            render: (r) => (
              <span>
                <DocNumber value={r.number} /> <span className="text-muted-foreground">{docTypeLabel(r.docType)}</span>
              </span>
            ),
          },
          { header: "Status", render: () => <Badge tone="warning">Dokumen batal</Badge> },
          {
            header: "",
            align: "right",
            render: (r) => (
              <Button variant="ghost" size="sm" disabled={!canPost || busy} onClick={() => void reverse([r])}>
                Balik jurnal
              </Button>
            ),
          },
        ]}
      />
      <ConfirmDialog
        open={confirmAll}
        title="Balik semua jurnal?"
        message={`${batchCount} jurnal dokumen batal akan dibalik dengan jurnal pembalik otomatis${overflow}. Lanjutkan?`}
        confirmLabel={busy ? "Membalik…" : "Balik jurnal"}
        onConfirm={() => void reverse(orphans)}
        onCancel={() => (busy ? undefined : setConfirmAll(false))}
      />
    </SectionCard>
  );
}
