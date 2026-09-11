import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import {
  ActionRow,
  Badge,
  Button,
  DataTable,
  FormField,
  Modal,
  PageHeader,
  SectionCard,
  TextArea,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { EmptyState } from "@/shared/components/feedback/empty-state";
import {
  formatDate,
  formatIDR,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { reverseSchema } from "@/modules/finance/schemas/finance.schema";
import {
  entryTotals,
  journalSourceLabel,
} from "@/modules/finance/types";

// Detail jurnal: header + baris + Balikkan (Modal alasan wajib →
// POST /:id/reverse {reason}, gate finance.manage, sembunyi saat reversed).
// Jurnal pembalik (sourceType reversal) juga tak bisa dibalik (aturan backend).
export default function JournalDetailPage() {
  const { id } = useParams();
  const entryId = Number(id);
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");

  const { data: entry, loading, error, reload } = useAsyncData(
    () => {
      if (!Number.isFinite(entryId) || entryId <= 0) {
        return Promise.reject(new Error("ID jurnal tidak valid"));
      }
      return financeService.journal(entryId).catch((err) => {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat jurnal", apiErr.message);
        throw err;
      });
    },
    [entryId],
  );

  const [reverseOpen, setReverseOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState<string | undefined>(undefined);
  const [reversing, setReversing] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  async function handleReverse(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = reverseSchema.safeParse({ reason: reason.trim() });
    if (!parsed.success) {
      setReasonError(
        parsed.error.flatten().fieldErrors.reason?.[0] ?? "Alasan wajib diisi",
      );
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
      setServerError(toApiError(err).message);
    } finally {
      setReversing(false);
    }
  }

  if (loading && !entry) {
    return (
      <div>
        <PageHeader eyebrow="Keuangan" title="Detail Jurnal" />
        <p className="text-muted py-8 text-center text-sm">Memuat…</p>
      </div>
    );
  }

  if (error || !entry) {
    return (
      <div>
        <PageHeader eyebrow="Keuangan" title="Detail Jurnal" />
        <EmptyState
          title={error ?? "Jurnal tidak ditemukan"}
          description="Kembali ke daftar jurnal dan coba lagi."
          action={
            <Link
              to={ROUTE_PATHS.financeJournals}
              className="font-medium text-brand hover:underline"
            >
              Kembali ke Jurnal
            </Link>
          }
        />
      </div>
    );
  }

  const totals = entryTotals(entry.lines);
  const isReversed = entry.status === "reversed";
  const isReversal = entry.sourceType === "reversal";
  const canReverse = canManage && !isReversed && !isReversal;

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title={entry.number}
        description={`${entry.memo || "-"} · ${formatDate(entry.date)}`}
        actions={
          canReverse ? (
            <Button
              variant="danger"
              onClick={() => {
                setServerError(null);
                setReasonError(undefined);
                setReverseOpen(true);
              }}
            >
              Balikkan Jurnal
            </Button>
          ) : undefined
        }
      />

      {isReversed && (
        <div className="mb-4">
          <Notice tone="danger" title="Jurnal ini sudah dibalik">
            {entry.reversedBy > 0 ? (
              <Link
                to={`${ROUTE_PATHS.financeJournals}/${entry.reversedBy}`}
                className="font-semibold text-brand hover:underline"
              >
                Buka jurnal pembalik
              </Link>
            ) : (
              "Tidak dapat dibalik lagi — tulis jurnal manual bila perlu koreksi."
            )}
          </Notice>
        </div>
      )}

      <SectionCard
        title="Informasi"
        actions={
          <Badge tone={statusTone(entry.status)}>
            {statusLabel(entry.status)}
          </Badge>
        }
      >
        <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-3">
          <div>
            <dt className="text-muted">Tanggal</dt>
            <dd className="font-medium text-ink">{formatDate(entry.date)}</dd>
          </div>
          <div>
            <dt className="text-muted">Sumber</dt>
            <dd className="font-medium text-ink">
              {journalSourceLabel(entry)}
            </dd>
          </div>
          <div>
            <dt className="text-muted">Memo</dt>
            <dd className="font-medium text-ink">{entry.memo || "-"}</dd>
          </div>
        </dl>
      </SectionCard>

      <div className="mt-4">
        <SectionCard title="Baris Jurnal">
          <DataTable
            rows={entry.lines ?? []}
            rowKey={(l) => `${l.accountId}-${l.debit}-${l.credit}`}
            emptyTitle="Tidak ada baris"
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
          <p className="px-1 pt-4 text-sm text-ink">
            Total debit{" "}
            <span className="font-semibold">{formatIDR(totals.debit)}</span> ·
            total kredit{" "}
            <span className="font-semibold">{formatIDR(totals.credit)}</span>
          </p>
        </SectionCard>
      </div>

      {reverseOpen && (
        <Modal
          open
          title={`Balikkan ${entry.number}`}
          description="Membukukan jurnal cermin dan menandai jurnal ini dibalik."
          onClose={() => setReverseOpen(false)}
        >
          <form onSubmit={(e) => void handleReverse(e)} className="flex flex-col gap-4">
            {serverError && <Notice tone="danger" title={serverError} />}
            <FormField label="Alasan" required errorText={reasonError}>
              <TextArea
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="cth. Salah tanggal posting"
                rows={3}
              />
            </FormField>
            <ActionRow>
              <Button
                type="button"
                variant="secondary"
                onClick={() => setReverseOpen(false)}
                disabled={reversing}
              >
                Batal
              </Button>
              <Button type="submit" variant="danger" disabled={reversing}>
                {reversing ? "Memproses…" : "Balikkan"}
              </Button>
            </ActionRow>
          </form>
        </Modal>
      )}
    </div>
  );
}
