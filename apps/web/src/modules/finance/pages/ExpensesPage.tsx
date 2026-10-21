import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Ban } from "lucide-react";
import {
  Button,
  DataTable,
  DateRangeFields,
  FormField,
  PageHeader,
  Pagination,
  SearchInput,
  SectionCard,
  SelectInput,
  StatusBadge,
} from "@/shared/components/ui";
import { ActionMenu, DocNumber, Money, Page, SavedViews, Toolbar } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { formatDate, statusLabel } from "@/shared/lib/format";
import { formatApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import { EXPENSE_LIST_DEFAULTS, expenseListFilterSchema } from "@/modules/finance/schemas/finance-list-filter.schema";
import type { ExpenseFormValues } from "@/modules/finance/schemas/finance.schema";
import type { Account, Expense } from "@/modules/finance/types";
import { ExpenseFormSheet } from "@/modules/finance/components/ExpenseFormSheet";
import { CancelExpenseModal } from "@/modules/finance/components/CancelExpenseModal";

const LIMIT = 20;

// Biaya: daftar + filter + "Catat biaya" (sheet). Batalkan → POST /:id/cancel {reason} (alasan wajib),
// hanya saat status posted (gate finance.expense.cancel).
export default function ExpensesPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("finance.expense.create");
  const canCancel = can("finance.expense.cancel");

  const { filters, setFilters, reset } = useUrlFilters(expenseListFilterSchema, EXPENSE_LIST_DEFAULTS);

  const { data, loading, error, reload } = useAsyncData(
    () =>
      financeService
        .expenses({
          q: filters.search,
          from: filters.from,
          to: filters.to,
          status: filters.status,
          expenseAccountId: filters.expenseAccountId,
          payAccountId: filters.payAccountId,
          page: filters.page,
          limit: LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [filters],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  const { data: accountData } = useAsyncData(() => financeService.accounts("active").then((a) => a ?? ([] as Account[])), []);
  const accounts = accountData ?? [];
  const accountName = (id: number) => {
    const a = accounts.find((x) => x.id === id);
    return a ? `${a.code} — ${a.name}` : `#${id}`;
  };

  const [createOpen, setCreateOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [cancelTarget, setCancelTarget] = useState<Expense | null>(null);
  const [cancelling, setCancelling] = useState(false);

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
        partyId: values.partyId,
      });
      toast.fromServer(res.message, "Biaya dicatat", res.data.number);
      setCreateOpen(false);
      setFilters({ page: 1 });
      reload();
    } catch (err) {
      setServerError(formatApiError(err));
    } finally {
      setSubmitting(false);
    }
  }

  async function handleCancel(reason: string) {
    if (!cancelTarget) return;
    setCancelling(true);
    try {
      const res = await financeService.cancelExpense(cancelTarget.id, reason);
      toast.fromServer(res.message, "Biaya dibatalkan", res.data.number);
      setCancelTarget(null);
      reload();
    } catch (err) {
      toast.danger("Gagal membatalkan biaya", formatApiError(err));
    } finally {
      setCancelling(false);
    }
  }

  const expenseAccounts = accounts.filter((a) => a.type === "beban");
  const payAccounts = accounts.filter((a) => a.type === "aset" && a.isCash);

  const active: ActiveFilter[] = [];
  if (filters.expenseAccountId) active.push({ key: "expenseAccountId", label: `Beban: ${accountName(filters.expenseAccountId)}`, onRemove: () => setFilters({ expenseAccountId: 0 }) });
  if (filters.payAccountId) active.push({ key: "payAccountId", label: `Dibayar dari: ${accountName(filters.payAccountId)}`, onRemove: () => setFilters({ payAccountId: 0 }) });
  const hasFilters = active.length > 0 || Boolean(filters.search || filters.status || filters.from || filters.to);

  const createButton = canCreate ? (
    <Button
      onClick={() => {
        setServerError(null);
        setCreateOpen(true);
      }}
    >
      Catat biaya
    </Button>
  ) : undefined;

  return (
    <Page>
      <PageHeader title="Biaya" description="Biaya operasional yang otomatis memposting jurnal." actions={createButton} />

      <Toolbar
        right={<SavedViews listKey="expenses" columnsKey="expenses" />}
        search={<SearchInput aria-label="Cari biaya" value={filters.search} onSearch={(v) => setFilters({ search: v })} placeholder="Nomor atau catatan…" />}
        primary={
          <>
            <DateRangeFields from={filters.from} to={filters.to} onChange={({ from, to }) => setFilters({ from, to })} />
            <FormField label="Status">
              <SelectInput aria-label="Status biaya" value={filters.status} onChange={(e) => setFilters({ status: e.target.value as typeof filters.status })}>
                <option value="">Semua</option>
                <option value="posted">{statusLabel("posted")}</option>
                <option value="cancelled">{statusLabel("cancelled")}</option>
              </SelectInput>
            </FormField>
          </>
        }
        more={
          <>
            <FormField label="Akun beban">
              <SelectInput aria-label="Akun beban" value={filters.expenseAccountId} onChange={(e) => setFilters({ expenseAccountId: Number(e.target.value) })}>
                <option value={0}>Semua</option>
                {expenseAccounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.code} — {a.name}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <FormField label="Dibayar dari">
              <SelectInput aria-label="Dibayar dari" value={filters.payAccountId} onChange={(e) => setFilters({ payAccountId: Number(e.target.value) })}>
                <option value={0}>Semua</option>
                {payAccounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.code} — {a.name}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </>
        }
        moreActiveCount={active.length}
        active={active}
        onClearAll={reset}
      />

      <SectionCard flush>
        <DataTable<Expense>
          columnsKey="expenses"
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={(r) => navigate(`${ROUTE_PATHS.financeExpenses}/${r.id}`)}
          emptyTitle={hasFilters ? "Tidak ada biaya yang cocok dengan filter ini" : "Belum ada biaya"}
          emptyDescription={hasFilters ? "Ubah atau hapus filter untuk melihat biaya lain." : "Catat biaya operasional pertama untuk memulai."}
          emptyAction={
            hasFilters ? (
              <Button variant="secondary" size="sm" onClick={reset}>
                Hapus filter
              </Button>
            ) : (
              createButton
            )
          }
          columns={[
            {
              header: "Nomor",
              render: (r) => (
                <Link to={`${ROUTE_PATHS.financeExpenses}/${r.id}`} className="font-medium text-primary hover:underline">
                  <DocNumber value={r.number} />
                </Link>
              ),
            },
            { header: "Tanggal", render: (r) => formatDate(r.date) },
            { header: "Akun beban", render: (r) => accountName(r.expenseAccountId) },
            { header: "Pihak", render: (r) => r.partyName || "-" },
            { header: "Dibayar dari", render: (r) => accountName(r.payAccountId) },
            { header: "Nominal", align: "right", render: (r) => <Money value={r.amount} className="font-medium" /> },
            {
              header: "Status",
              render: (r) => (
                <span className="flex flex-col items-start gap-1">
                  <StatusBadge status={r.status} />
                  {r.status === "cancelled" && r.cancelReason && (
                    <span className="text-xs text-muted-foreground">Alasan: {r.cancelReason}</span>
                  )}
                </span>
              ),
            },
            {
              header: "",
              align: "right",
              render: (r) =>
                canCancel && r.status === "posted" ? (
                  <ActionMenu
                    label={`Aksi untuk ${r.number}`}
                    variant="ghost"
                    items={[{ key: "cancel", label: "Batalkan biaya", icon: <Ban aria-hidden />, destructive: true, onSelect: () => setCancelTarget(r) }]}
                  />
                ) : null,
            },
          ]}
        />
        <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
      </SectionCard>

      {createOpen && (
        <ExpenseFormSheet
          accounts={accounts}
          submitting={submitting}
          serverError={serverError}
          onClose={() => setCreateOpen(false)}
          onSubmit={(v) => void handleCreate(v)}
        />
      )}

      {cancelTarget && (
        <CancelExpenseModal
          number={cancelTarget.number}
          submitting={cancelling}
          onClose={() => setCancelTarget(null)}
          onSubmit={(r) => void handleCancel(r)}
        />
      )}
    </Page>
  );
}
