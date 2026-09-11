import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Badge,
  Button,
  ConfirmDialog,
  PageHeader,
  SectionCard,
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

// Detail biaya: GET /expenses/:id + Batalkan (POST /:id/cancel tanpa body,
// hanya saat posted, gate finance.post). Menautkan jurnal otomatisnya.
export default function ExpenseDetailPage() {
  const { id } = useParams();
  const expenseId = Number(id);
  const can = useAuthStore((s) => s.can);
  const canPost = can("finance.post");

  const { data, loading, error, reload } = useAsyncData(
    () => {
      if (!Number.isFinite(expenseId) || expenseId <= 0) {
        return Promise.reject(new Error("ID biaya tidak valid"));
      }
      return Promise.all([
        financeService.expense(expenseId),
        financeService.accounts("active"),
      ])
        .then(([res, accs]) => ({ expense: res, accounts: accs }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat biaya", apiErr.message);
          throw err;
        });
    },
    [expenseId],
  );
  const expense = data?.expense ?? null;
  const accounts = data?.accounts ?? [];
  const [confirmOpen, setConfirmOpen] = useState(false);

  async function handleCancel() {
    if (!expense) return;
    try {
      const res = await financeService.cancelExpense(expense.id);
      toast.fromServer(res.message, "Biaya dibatalkan", res.data.number);
      setConfirmOpen(false);
      reload();
    } catch (err) {
      toast.danger("Gagal membatalkan biaya", toApiError(err).message);
    }
  }

  if (loading && !expense) {
    return (
      <div>
        <PageHeader eyebrow="Keuangan" title="Detail Biaya" />
        <p className="text-muted py-8 text-center text-sm">Memuat…</p>
      </div>
    );
  }

  if (error || !expense) {
    return (
      <div>
        <PageHeader eyebrow="Keuangan" title="Detail Biaya" />
        <EmptyState
          title={error ?? "Biaya tidak ditemukan"}
          action={
            <Link
              to={ROUTE_PATHS.financeExpenses}
              className="font-medium text-brand hover:underline"
            >
              Kembali ke Biaya
            </Link>
          }
        />
      </div>
    );
  }

  const accountName = (accId: number) => {
    const a = accounts.find((x) => x.id === accId);
    return a ? `${a.code} — ${a.name}` : `#${accId}`;
  };
  const cancellable = canPost && expense.status === "posted";

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title={expense.number}
        description={`${accountName(expense.expenseAccountId)} · ${formatDate(expense.date)}`}
        actions={
          cancellable ? (
            <Button variant="danger" onClick={() => setConfirmOpen(true)}>
              Batalkan Biaya
            </Button>
          ) : undefined
        }
      />

      {expense.status === "cancelled" && (
        <div className="mb-4">
          <Notice tone="danger" title="Biaya ini sudah dibatalkan" />
        </div>
      )}

      <SectionCard
        title="Informasi"
        actions={
          <Badge tone={statusTone(expense.status)}>
            {statusLabel(expense.status)}
          </Badge>
        }
      >
        <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-3">
          <div>
            <dt className="text-muted">Tanggal</dt>
            <dd className="font-medium text-ink">{formatDate(expense.date)}</dd>
          </div>
          <div>
            <dt className="text-muted">Akun Beban</dt>
            <dd className="font-medium text-ink">
              {accountName(expense.expenseAccountId)}
            </dd>
          </div>
          <div>
            <dt className="text-muted">Dibayar Dari</dt>
            <dd className="font-medium text-ink">
              {accountName(expense.payAccountId)}
            </dd>
          </div>
          <div>
            <dt className="text-muted">Nominal</dt>
            <dd className="font-medium text-ink">
              {formatIDR(expense.amount)}
            </dd>
          </div>
          <div>
            <dt className="text-muted">Catatan</dt>
            <dd className="font-medium text-ink">{expense.notes || "-"}</dd>
          </div>
          <div>
            <dt className="text-muted">Jurnal</dt>
            <dd className="font-medium text-ink">
              {expense.journalEntryId > 0 ? (
                <Link
                  to={`${ROUTE_PATHS.financeJournals}/${expense.journalEntryId}`}
                  className="text-brand hover:underline"
                >
                  Buka jurnal
                </Link>
              ) : (
                "-"
              )}
            </dd>
          </div>
        </dl>
      </SectionCard>

      <ConfirmDialog
        open={confirmOpen}
        title="Batalkan Biaya?"
        message={`Biaya ${expense.number} akan dibatalkan dan jurnalnya dibalik. Lanjutkan?`}
        confirmLabel="Batalkan"
        onConfirm={() => void handleCancel()}
        onCancel={() => setConfirmOpen(false)}
      />
    </div>
  );
}
