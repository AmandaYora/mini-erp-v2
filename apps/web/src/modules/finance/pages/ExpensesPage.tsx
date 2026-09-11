import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  PageHeader,
  Pagination,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
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
import type { ExpenseFormValues } from "@/modules/finance/schemas/finance.schema";
import type { Account, Expense } from "@/modules/finance/types";
import { ExpenseFormModal } from "@/modules/finance/components/ExpenseFormModal";

const LIMIT = 20;

// Biaya: tabel + Catat Modal → POST /expenses {expenseAccountId,
// payAccountId, amount, date?, notes?}; Batalkan → POST /:id/cancel tanpa
// body, hanya saat status posted (gate finance.post).
export default function ExpensesPage() {
  const can = useAuthStore((s) => s.can);
  const canPost = can("finance.post");

  const [page, setPage] = useState(1);
  const { data, loading, error, reload } = useAsyncData(
    () =>
      financeService
        .expenses(page, LIMIT)
        .then((res) => ({ items: res.items, total: res.meta.total }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat biaya", apiErr.message);
          throw err;
        }),
    [page],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const [accounts, setAccounts] = useState<Account[]>([]);

  const [createOpen, setCreateOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [cancelTarget, setCancelTarget] = useState<Expense | null>(null);

  const accountName = useCallback(
    (id: number) => {
      const a = accounts.find((x) => x.id === id);
      return a ? `${a.code} — ${a.name}` : `#${id}`;
    },
    [accounts],
  );

  useEffect(() => {
    let cancelled = false;
    financeService
      .accounts("active")
      .then((res) => {
        if (!cancelled) setAccounts(res);
      })
      .catch((err) => {
        if (!cancelled)
          toast.danger("Gagal memuat daftar akun", toApiError(err).message);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleCreate(values: ExpenseFormValues) {
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await financeService.createExpense({
        expenseAccountId: values.expenseAccountId,
        payAccountId: values.payAccountId,
        amount: values.amount,
        date: values.date,
        notes: values.notes,
      });
      toast.fromServer(res.message, "Biaya dicatat", res.data.number);
      setCreateOpen(false);
      setPage(1);
      reload();
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  async function handleCancel() {
    if (!cancelTarget) return;
    try {
      const res = await financeService.cancelExpense(cancelTarget.id);
      toast.fromServer(res.message, "Biaya dibatalkan", res.data.number);
      setCancelTarget(null);
      reload();
    } catch (err) {
      toast.danger("Gagal membatalkan biaya", toApiError(err).message);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Biaya"
        description="Biaya operasional yang otomatis memposting jurnal."
        actions={
          canPost ? (
            <Button
              onClick={() => {
                setServerError(null);
                setCreateOpen(true);
              }}
            >
              Catat Biaya
            </Button>
          ) : undefined
        }
      />

      {error && (
        <div className="mb-4">
          <Notice tone="danger" title={error}>
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

      <SectionCard>
        {loading && items.length === 0 ? (
          <p className="text-muted py-8 text-center text-sm">Memuat…</p>
        ) : (
          <>
            <DataTable<Expense>
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada biaya"
              emptyDescription="Catat biaya operasional baru."
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.financeExpenses}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {r.number}
                    </Link>
                  ),
                },
                { header: "Tanggal", render: (r) => formatDate(r.date) },
                {
                  header: "Akun Beban",
                  render: (r) => accountName(r.expenseAccountId),
                },
                {
                  header: "Dibayar Dari",
                  render: (r) => accountName(r.payAccountId),
                },
                {
                  header: "Nominal",
                  align: "right",
                  render: (r) => (
                    <span className="font-medium">{formatIDR(r.amount)}</span>
                  ),
                },
                {
                  header: "Status",
                  render: (r) => (
                    <Badge tone={statusTone(r.status)}>
                      {statusLabel(r.status)}
                    </Badge>
                  ),
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <span className="flex justify-end gap-3">
                      <Link
                        to={`${ROUTE_PATHS.financeExpenses}/${r.id}`}
                        className="font-medium text-brand hover:underline"
                      >
                        Detail
                      </Link>
                      {canPost && r.status === "posted" && (
                        <button
                          type="button"
                          onClick={() => setCancelTarget(r)}
                          className="cursor-pointer font-medium text-bad hover:underline"
                        >
                          Batalkan
                        </button>
                      )}
                    </span>
                  ),
                },
              ]}
            />
            <Pagination
              page={page}
              limit={LIMIT}
              total={total}
              onPageChange={(p) => setPage(p)}
            />
          </>
        )}
      </SectionCard>

      {createOpen && (
        <ExpenseFormModal
          open
          accounts={accounts}
          submitting={submitting}
          serverError={serverError}
          onClose={() => setCreateOpen(false)}
          onSubmit={(v) => void handleCreate(v)}
        />
      )}

      <ConfirmDialog
        open={cancelTarget !== null}
        title="Batalkan Biaya?"
        message={`Biaya ${cancelTarget?.number ?? ""} akan dibatalkan dan jurnalnya dibalik. Lanjutkan?`}
        confirmLabel="Batalkan"
        onConfirm={() => void handleCancel()}
        onCancel={() => setCancelTarget(null)}
      />
    </div>
  );
}
