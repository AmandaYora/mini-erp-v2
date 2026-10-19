import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  DataTable,
  DateRangeFields,
  FormField,
  PageHeader,
  Pagination,
  SearchInput,
  SearchSelect,
  SectionCard,
  SelectInput,
  StatusBadge,
} from "@/shared/components/ui";
import { DocNumber, Page, Toolbar } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { formatDate, statusLabel } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { financeService } from "@/modules/finance/services/finance.service";
import {
  JOURNAL_LIST_DEFAULTS,
  journalListFilterSchema,
} from "@/modules/finance/schemas/finance-list-filter.schema";
import { JOURNAL_SOURCE_TYPES, JOURNAL_STATUSES, docTypeLabel, journalSourceLabel } from "@/modules/finance/types";
import type { Account, JournalEntry } from "@/modules/finance/types";

const LIMIT = 20;

// List backend (entryView withLines=false) tidak membawa baris/total — nilai per jurnal ada di detail.
export default function JournalsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canManage = can("finance.manage");

  const { filters, setFilters, reset } = useUrlFilters(journalListFilterSchema, JOURNAL_LIST_DEFAULTS);

  const { data, loading, error, reload } = useAsyncData(
    () =>
      financeService
        .journals({
          from: filters.from,
          to: filters.to,
          accountId: filters.accountId || undefined,
          q: filters.search,
          sourceType: filters.sourceType,
          status: filters.status,
          page: filters.page,
          limit: LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [filters],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  const { data: accounts } = useAsyncData(() => financeService.accounts("active").then((a) => a ?? ([] as Account[])), []);
  const accountOptions = (accounts ?? []).map((a) => ({ value: a.id, label: `${a.code} — ${a.name}` }));
  const accountLabel = accountOptions.find((o) => o.value === filters.accountId)?.label ?? `Akun #${filters.accountId}`;

  const active: ActiveFilter[] = [];
  if (filters.accountId) active.push({ key: "accountId", label: `Akun: ${accountLabel}`, onRemove: () => setFilters({ accountId: 0 }) });
  if (filters.sourceType) active.push({ key: "sourceType", label: `Sumber: ${docTypeLabel(filters.sourceType)}`, onRemove: () => setFilters({ sourceType: "" }) });
  if (filters.status) active.push({ key: "status", label: `Status: ${statusLabel(filters.status)}`, onRemove: () => setFilters({ status: "" }) });
  const hasFilters = active.length > 0 || Boolean(filters.search || filters.from || filters.to);

  return (
    <Page>
      <PageHeader
        title="Jurnal"
        description="Daftar entri jurnal terposting dan pembaliknya. Klik baris untuk melihat debit dan kredit."
        actions={
          canManage ? (
            <Button asChild>
              <Link to={ROUTE_PATHS.financeJournalNew}>Jurnal manual</Link>
            </Button>
          ) : undefined
        }
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari jurnal"
            value={filters.search}
            onSearch={(v) => setFilters({ search: v })}
            placeholder="Nomor atau memo…"
          />
        }
        primary={
          <>
            <DateRangeFields from={filters.from} to={filters.to} onChange={({ from, to }) => setFilters({ from, to })} />
            <FormField label="Akun">
              <SearchSelect
                options={accountOptions}
                value={filters.accountId || null}
                onChange={(v) => setFilters({ accountId: v === null ? 0 : Number(v) })}
                placeholder="Semua akun…"
                allowClear
              />
            </FormField>
          </>
        }
        more={
          <>
            <FormField label="Sumber">
              <SelectInput
                aria-label="Sumber jurnal"
                value={filters.sourceType}
                onChange={(e) => setFilters({ sourceType: e.target.value as typeof filters.sourceType })}
              >
                <option value="">Semua</option>
                {JOURNAL_SOURCE_TYPES.map((t) => (
                  <option key={t} value={t}>
                    {docTypeLabel(t)}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <FormField label="Status">
              <SelectInput
                aria-label="Status jurnal"
                value={filters.status}
                onChange={(e) => setFilters({ status: e.target.value as typeof filters.status })}
              >
                <option value="">Semua</option>
                {JOURNAL_STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {statusLabel(s)}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </>
        }
        moreActiveCount={Number(Boolean(filters.sourceType)) + Number(Boolean(filters.status))}
        active={active}
        onClearAll={reset}
      />

      <SectionCard flush>
        <DataTable<JournalEntry>
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={(r) => navigate(`${ROUTE_PATHS.financeJournals}/${r.id}`)}
          emptyTitle={hasFilters ? "Tidak ada jurnal yang cocok dengan filter ini" : "Belum ada jurnal"}
          emptyDescription={
            hasFilters
              ? "Ubah atau hapus filter untuk melihat jurnal lain."
              : "Posting dokumen dari halaman Posting atau catat jurnal manual."
          }
          emptyAction={
            hasFilters ? (
              <Button variant="secondary" size="sm" onClick={reset}>
                Hapus filter
              </Button>
            ) : canManage ? (
              <Button asChild>
                <Link to={ROUTE_PATHS.financeJournalNew}>Catat jurnal manual</Link>
              </Button>
            ) : undefined
          }
          columns={[
            {
              header: "Nomor",
              render: (r) => (
                <Link to={`${ROUTE_PATHS.financeJournals}/${r.id}`} className="font-medium text-primary hover:underline">
                  <DocNumber value={r.number} />
                </Link>
              ),
            },
            { header: "Tanggal", render: (r) => formatDate(r.date) },
            { header: "Memo", render: (r) => r.memo || "-" },
            { header: "Sumber", render: (r) => journalSourceLabel(r) },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
          ]}
        />
        <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
      </SectionCard>
    </Page>
  );
}
