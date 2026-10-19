import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Ban } from "lucide-react";
import { Button, DetailSkeleton, PageHeader, SectionCard, StatusBadge } from "@/shared/components/ui";
import { ActionMenu, DescriptionList, DocNumber, DocumentRail, Money, Page } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { formatDate } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { EXPENSE_RAIL_STEPS, expenseRailCurrent, expenseRelated } from "@/modules/finance/lib/document-rails";
import { CancelExpenseModal } from "@/modules/finance/components/CancelExpenseModal";

// Detail biaya (T2): GET /expenses/:id. Batalkan (POST /:id/cancel {reason}) hanya saat posted
// dan berizin finance.expense.cancel; jurnal otomatisnya tertaut di Rel Dokumen.
export default function ExpenseDetailPage() {
  const { id } = useParams();
  const expenseId = Number(id);
  const can = useAuthStore((s) => s.can);
  const canCancel = can("finance.expense.cancel");

  const { data, loading, error, errorStatus, reload } = useAsyncData(() => {
    if (!Number.isFinite(expenseId) || expenseId <= 0) return Promise.reject(new Error("ID biaya tidak valid"));
    return Promise.all([financeService.expense(expenseId), financeService.accounts("active")]).then(([res, accs]) => ({
      expense: res,
      accounts: accs ?? [],
    }));
  }, [expenseId]);
  const expense = data?.expense ?? null;
  const accounts = data?.accounts ?? [];
  usePageCrumb(expense?.number);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [cancelling, setCancelling] = useState(false);

  async function handleCancel(reason: string) {
    if (!expense) return;
    setCancelling(true);
    try {
      const res = await financeService.cancelExpense(expense.id, reason);
      toast.fromServer(res.message, "Biaya dibatalkan", res.data.number);
      setConfirmOpen(false);
      reload();
    } catch (err) {
      toast.danger("Gagal membatalkan biaya", toApiError(err).message);
    } finally {
      setCancelling(false);
    }
  }

  if (loading && !expense) return <DetailSkeleton />;
  if (errorStatus === 404 || (!error && !expense)) {
    return <NotFoundState title="Biaya tidak ditemukan" message="Biaya ini tidak ada atau sudah dihapus." backTo={ROUTE_PATHS.financeExpenses} backLabel="Ke daftar biaya" />;
  }
  if (error || !expense) {
    return (
      <Page>
        <PageHeader title="Detail biaya" />
        <ErrorState message={error ?? "Biaya tidak dapat dimuat"} onRetry={reload} />
      </Page>
    );
  }

  const accountName = (accId: number) => {
    const a = accounts.find((x) => x.id === accId);
    return a ? `${a.code} — ${a.name}` : `#${accId}`;
  };
  const cancelled = expense.status === "cancelled";
  const cancellable = canCancel && expense.status === "posted";

  return (
    <Page>
      <PageHeader
        title={expense.number}
        meta={<StatusBadge status={expense.status} />}
        subtitle={`${accountName(expense.expenseAccountId)} · ${formatDate(expense.date)}`}
        actions={
          <>
            {expense.journalEntryId > 0 && (
              <Button asChild variant="secondary" size="sm">
                <Link to={`${ROUTE_PATHS.financeJournals}/${expense.journalEntryId}`}>Buka jurnal</Link>
              </Button>
            )}
            {cancellable && (
              <ActionMenu
                items={[{ key: "cancel", label: "Batalkan biaya", icon: <Ban aria-hidden />, destructive: true, onSelect: () => setConfirmOpen(true) }]}
              />
            )}
          </>
        }
      >
        <DocumentRail steps={EXPENSE_RAIL_STEPS} current={expenseRailCurrent(expense)} stopped={cancelled} related={expenseRelated(expense)} />
      </PageHeader>

      {cancelled && (
        <Notice tone="info" title="Biaya ini sudah dibatalkan">
          {expense.cancelReason ? `Alasan: ${expense.cancelReason}` : "Jurnalnya sudah dibalik."}
        </Notice>
      )}

      <SectionCard title="Informasi">
        <DescriptionList
          columns={3}
          items={[
            { label: "Nomor", value: <DocNumber value={expense.number} copyable /> },
            { label: "Tanggal", value: formatDate(expense.date) },
            { label: "Nominal", value: <Money value={expense.amount} className="font-medium" /> },
            { label: "Akun beban", value: accountName(expense.expenseAccountId) },
            { label: "Pihak lawan", value: expense.partyName || "-" },
            { label: "Dibayar dari", value: accountName(expense.payAccountId) },
            { label: "Catatan", value: expense.notes, wide: true },
          ]}
        />
      </SectionCard>

      {confirmOpen && (
        <CancelExpenseModal number={expense.number} submitting={cancelling} onClose={() => setConfirmOpen(false)} onSubmit={(r) => void handleCancel(r)} />
      )}
    </Page>
  );
}
