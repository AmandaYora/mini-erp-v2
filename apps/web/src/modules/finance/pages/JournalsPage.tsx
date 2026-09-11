import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  DateInput,
  FilterBar,
  FormField,
  PageHeader,
  Pagination,
  SearchSelect,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  monthStartWIB,
  statusLabel,
  statusTone,
  todayWIB,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import type { ManualJournalValues } from "@/modules/finance/schemas/finance.schema";
import { journalSourceLabel } from "@/modules/finance/types";
import type { Account, JournalEntry } from "@/modules/finance/types";
import { ManualJournalModal } from "@/modules/finance/components/ManualJournalModal";

const LIMIT = 20;

// List backend (entryView withLines=false) TIDAK membawa lines/total, jadi
// kolom Debit/Kredit menampilkan "-" (lihat komentar kolom di bawah). Filter
// terverifikasi dari handler.go: from, to (YYYY-MM-DD), accountId.
export default function JournalsPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");

  const [fromInput, setFromInput] = useState(monthStartWIB());
  const [toInput, setToInput] = useState(todayWIB());
  const [accountInput, setAccountInput] = useState<number | null>(null);
  const [from, setFrom] = useState(monthStartWIB());
  const [to, setTo] = useState(todayWIB());
  const [accountId, setAccountId] = useState<number | null>(null);
  const [page, setPage] = useState(1);
  const { data, loading, error, reload } = useAsyncData(
    () =>
      financeService
        .journals({
          from: from ? from : undefined,
          to: to ? to : undefined,
          accountId: accountId ?? undefined,
          page,
          limit: LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat jurnal", apiErr.message);
          throw err;
        }),
    [from, to, accountId, page],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const [accounts, setAccounts] = useState<Account[]>([]);

  const [manualOpen, setManualOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

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

  function applyFilter(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setFrom(fromInput);
    setTo(toInput);
    setAccountId(accountInput);
    setPage(1);
  }

  async function handleManual(values: ManualJournalValues) {
    setSubmitting(true);
    setServerError(null);
    try {
      const res = await financeService.manual({
        date: values.date,
        memo: values.memo,
        lines: values.lines.map((l) => ({
          accountCode: l.accountCode,
          debit: l.debit,
          credit: l.credit,
        })),
      });
      toast.fromServer(res.message, "Jurnal manual dicatat", res.data.number);
      setManualOpen(false);
      setPage(1);
      reload();
    } catch (err) {
      setServerError(toApiError(err).message);
    } finally {
      setSubmitting(false);
    }
  }

  const accountOptions = accounts.map((a) => ({
    value: a.id,
    label: `${a.code} — ${a.name}`,
  }));

  return (
    <div>
      <PageHeader
        eyebrow="Keuangan"
        title="Jurnal"
        description="Daftar entri jurnal terposting dan pembaliknya."
        actions={
          canManage ? (
            <Button
              onClick={() => {
                setServerError(null);
                setManualOpen(true);
              }}
            >
              Jurnal Manual
            </Button>
          ) : undefined
        }
      />

      <FilterBar>
        <form onSubmit={applyFilter} className="flex flex-wrap items-end gap-4">
          <div className="min-w-40">
            <FormField label="Dari">
              <DateInput
                value={fromInput}
                onChange={(e) => setFromInput(e.target.value)}
              />
            </FormField>
          </div>
          <div className="min-w-40">
            <FormField label="Sampai">
              <DateInput
                value={toInput}
                onChange={(e) => setToInput(e.target.value)}
              />
            </FormField>
          </div>
          <div className="min-w-52 flex-1">
            <FormField label="Akun">
              <SearchSelect
                options={accountOptions}
                value={accountInput}
                onChange={(v) =>
                  setAccountInput(
                    typeof v === "number" ? v : v === null ? null : Number(v),
                  )
                }
                placeholder="Semua akun…"
                allowClear
              />
            </FormField>
          </div>
          <Button type="submit" variant="secondary">
            Terapkan
          </Button>
        </form>
      </FilterBar>

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
            <DataTable<JournalEntry>
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada jurnal"
              emptyDescription="Posting dokumen dari halaman Posting atau catat jurnal manual."
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.financeJournals}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {r.number}
                    </Link>
                  ),
                },
                { header: "Tanggal", render: (r) => formatDate(r.date) },
                { header: "Memo", render: (r) => r.memo || "-" },
                { header: "Sumber", render: (r) => journalSourceLabel(r) },
                // entryView list (withLines=false) tidak membawa total —
                // nilai per baris hanya ada di detail (GET /journals/:id).
                { header: "Debit", align: "right", render: () => "-" },
                { header: "Kredit", align: "right", render: () => "-" },
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
                    <Link
                      to={`${ROUTE_PATHS.financeJournals}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      Detail
                    </Link>
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

      {manualOpen && (
        <ManualJournalModal
          open
          accounts={accounts}
          submitting={submitting}
          serverError={serverError}
          onClose={() => setManualOpen(false)}
          onSubmit={(v) => void handleManual(v)}
        />
      )}
    </div>
  );
}
