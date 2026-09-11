import { useState } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  PageHeader,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatIDR, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { ManualJournalModal } from "@/modules/finance/components/ManualJournalModal";
import type { ManualJournalValues } from "@/modules/finance/schemas/finance.schema";
import { entryTotals } from "@/modules/finance/types";
import type { Account, JournalEntry } from "@/modules/finance/types";

// Saldo awal (G-revisi): satu jurnal cutover per cabang (tanpa tabel saldo
// awal). Alur: isi baris → pratinjau (validasi seimbang) → posting.
// Idempoten: cabang yang sudah cutover langsung melihat jurnalnya.
export default function OpeningPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");

  const { data, loading, error, reload } = useAsyncData(
    () =>
      Promise.all([
        financeService.openingStatus(),
        financeService.accounts("active").catch(() => [] as Account[]),
      ]).then(([st, acc]) => ({ status: st, accounts: acc ?? [] })),
    [],
  );
  const status = data?.status ?? null;
  const accounts = data?.accounts ?? [];
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [preview, setPreview] = useState<JournalEntry | null>(null);
  const [pending, setPending] = useState<ManualJournalValues | null>(null);

  async function handlePreview(values: ManualJournalValues) {
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await financeService.previewOpening({
        date: values.date,
        memo: values.memo,
        lines: values.lines,
      });
      setPreview(res.data);
      setPending(values);
      setModalOpen(false);
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  async function handlePost() {
    if (!pending) return;
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await financeService.postOpening({
        date: pending.date,
        memo: pending.memo,
        lines: pending.lines,
      });
      setPreview(null);
      setPending(null);
      toast.fromServer(res.message, "Saldo awal diposting", res.data.number);
      reload();
    } catch (err) {
      const apiErr = toApiError(err);
      setServerError(apiErr.message);
      toast.danger("Gagal memposting saldo awal", apiErr.message);
    } finally {
      setSubmitting(false);
    }
  }

  const totals = entryTotals(preview?.lines);
  const balanced =
    preview !== null && totals.debit > 0 && totals.debit === totals.credit;

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Saldo Awal"
        description="Jurnal cutover satu kali per cabang — tanpa tabel saldo awal, buku langsung membacanya."
        actions={
          status === null && canManage ? (
            <Button onClick={() => setModalOpen(true)}>Buat Saldo Awal</Button>
          ) : undefined
        }
      />

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title="Gagal memuat saldo awal">
            {error}{" "}
            <button
              type="button"
              onClick={() => reload()}
              className="cursor-pointer font-semibold text-brand hover:underline"
            >
              Coba lagi
            </button>
          </Notice>
        </div>
      )}

      {loading ? (
        <p className="text-muted px-1 py-6 text-sm">Memuat saldo awal…</p>
      ) : status ? (
        <SectionCard
          title="Sudah Cutover"
          description="Cabang ini sudah memposting saldo awal — posting bersifat final dan idempoten."
          actions={<Badge tone="success">Diposting</Badge>}
        >
          <dl className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">
                Nomor Jurnal
              </dt>
              <dd className="mt-1 text-sm">
                <Link
                  to={`${ROUTE_PATHS.financeJournals}/${status.id}`}
                  className="font-semibold text-brand hover:underline"
                >
                  {status.number}
                </Link>
              </dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">
                Tanggal
              </dt>
              <dd className="mt-1 text-sm text-ink">
                {formatDate(status.date || todayWIB())}
              </dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">
                Memo
              </dt>
              <dd className="mt-1 text-sm text-ink">{status.memo || "-"}</dd>
            </div>
          </dl>
        </SectionCard>
      ) : (
        <SectionCard
          title="Belum Cutover"
          description="Susun baris saldo awal per akun, pratinjau validasinya, lalu posting satu kali."
        >
          <p className="text-muted text-sm">
            Total debit harus sama dengan kredit dan lebih dari 0 — pratinjau
            menolak yang pincang sebelum apa pun tertulis ke buku.
          </p>
        </SectionCard>
      )}

      {preview && status === null && (
        <div className="mt-4">
          <SectionCard
            title="Pratinjau Saldo Awal"
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
            <div className="flex flex-wrap items-center justify-between gap-3 px-1 pt-4">
              <p className="text-sm text-ink">
                Total debit{" "}
                <span className="font-semibold">{formatIDR(totals.debit)}</span>{" "}
                · total kredit{" "}
                <span className="font-semibold">
                  {formatIDR(totals.credit)}
                </span>
              </p>
              <div className="flex gap-3">
                <Button
                  variant="secondary"
                  disabled={submitting}
                  onClick={() => {
                    setPreview(null);
                    setPending(null);
                  }}
                >
                  Batal
                </Button>
                <Button
                  onClick={() => void handlePost()}
                  disabled={!canManage || submitting || !balanced}
                >
                  {submitting ? "Memposting…" : "Posting Saldo Awal"}
                </Button>
              </div>
            </div>
            {serverError && (
              <div className="mt-4">
                <Notice tone="danger" title={serverError} />
              </div>
            )}
          </SectionCard>
        </div>
      )}

      {modalOpen && (
        <ManualJournalModal
          open={modalOpen}
          accounts={accounts}
          submitting={submitting}
          serverError={serverError}
          onClose={() => {
            if (!submitting) setModalOpen(false);
          }}
          onSubmit={(v) => void handlePreview(v)}
          title="Saldo Awal"
          description="Susun baris cutover per akun — pratinjau dulu sebelum posting."
          submitLabel="Pratinjau"
        />
      )}
    </div>
  );
}
