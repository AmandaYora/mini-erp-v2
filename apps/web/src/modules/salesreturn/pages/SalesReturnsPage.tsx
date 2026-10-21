import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  DataTable,
  DateRangeFields,
  FormField,
  MultiToggle,
  PageHeader,
  Pagination,
  SearchInput,
  SectionCard,
  StatusBadge,
} from "@/shared/components/ui";
import { DocNumber, Money, Page, SavedViews, Toolbar } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { formatDate, statusLabel } from "@/shared/lib/format";
import { fromCsv } from "@/shared/lib/list-query";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { salesReturnService } from "@/modules/salesreturn/services/sales-return.service";
import { SALES_RETURN_STATUSES } from "@/modules/salesreturn/types";
import {
  SALESRETURN_LIST_DEFAULTS,
  salesreturnListFilterSchema,
  toSalesReturnFilterParams,
} from "@/modules/salesreturn/schemas/sales-return-list-filter.schema";
import type { SalesReturn } from "@/modules/salesreturn/types";

const LIMIT = 20;

export default function SalesReturnsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("salesreturn.create");

  const { filters, setFilters, reset } = useUrlFilters(salesreturnListFilterSchema, SALESRETURN_LIST_DEFAULTS);

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      salesReturnService
        .list({ ...toSalesReturnFilterParams(filters), page: filters.page, limit: LIMIT }, signal)
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [filters],
  );
  const items = data?.items ?? null;
  const total = data?.total ?? 0;

  const newReturnTo = `${ROUTE_PATHS.salesReturns}/new`;
  const hasFilters =
    filters.search !== "" || filters.status !== "" || filters.salesOrderId > 0 || filters.from !== "" || filters.to !== "";

  const active: ActiveFilter[] = [];
  for (const s of fromCsv(filters.status)) {
    active.push({
      key: `status-${s}`,
      label: `Status: ${statusLabel(s)}`,
      onRemove: () => setFilters({ status: fromCsv(filters.status).filter((x) => x !== s).join(",") }),
    });
  }
  if (filters.salesOrderId > 0)
    active.push({ key: "so", label: "Order jual dipilih", onRemove: () => setFilters({ salesOrderId: 0 }) });
  if (filters.from || filters.to)
    active.push({
      key: "date",
      label: `Retur: ${filters.from ? formatDate(filters.from) : "…"} – ${filters.to ? formatDate(filters.to) : "…"}`,
      onRemove: () => setFilters({ from: "", to: "" }),
    });

  return (
    <Page>
      <PageHeader
        title="Retur Jual"
        description="Retur penjualan terhadap order jual terkonfirmasi. Klik baris untuk detail dan proses."
        actions={
          canCreate ? (
            <Button asChild>
              <Link to={newReturnTo}>Buat retur</Link>
            </Button>
          ) : undefined
        }
      />

      <Toolbar
        right={<SavedViews listKey="sales-returns" columnsKey="sales-returns" />}
        search={
          <SearchInput
            aria-label="Cari retur"
            value={filters.search}
            onSearch={(v) => setFilters({ search: v })}
            placeholder="Nomor retur, order, pelanggan, alasan…"
          />
        }
        primary={
          <>
            <FormField label="Status">
              <MultiToggle
                aria-label="Status retur"
                options={SALES_RETURN_STATUSES.map((s) => ({ value: s, label: statusLabel(s) }))}
                value={filters.status}
                onChange={(status) => setFilters({ status })}
              />
            </FormField>
            <DateRangeFields
              from={filters.from}
              to={filters.to}
              fromLabel="Retur dari"
              toLabel="Retur sampai"
              onChange={({ from, to }) => setFilters({ from, to })}
            />
          </>
        }
        more={
          <FormField label="Order jual">
            <SalesOrderSearchSelect
              value={filters.salesOrderId || null}
              allowClear
              placeholder="Semua order"
              onChange={(id) => setFilters({ salesOrderId: id ?? 0 })}
            />
          </FormField>
        }
        moreActiveCount={filters.salesOrderId > 0 ? 1 : 0}
        active={active}
        onClearAll={reset}
      />

      <SectionCard flush>
        <DataTable<SalesReturn>
          columnsKey="sales-returns"
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={(r) => void navigate(`${ROUTE_PATHS.salesReturns}/${r.id}`)}
          emptyTitle={hasFilters ? "Tidak ada retur yang cocok" : "Belum ada retur penjualan"}
          emptyDescription={
            hasFilters ? "Ubah atau hapus filter untuk melihat retur lain." : "Retur yang dibuat akan muncul di sini."
          }
          emptyAction={
            !hasFilters && canCreate ? (
              <Button asChild>
                <Link to={newReturnTo}>Buat retur pertama</Link>
              </Button>
            ) : undefined
          }
          columns={[
            {
              header: "Nomor",
              render: (r) => (
                <Link to={`${ROUTE_PATHS.salesReturns}/${r.id}`} className="font-medium text-primary hover:underline">
                  <DocNumber value={r.number} />
                </Link>
              ),
            },
            { header: "Tanggal", render: (r) => formatDate(r.returnDate) },
            {
              header: "Order jual",
              render: (r) => (r.orderNumber ? <DocNumber value={r.orderNumber} /> : "-"),
            },
            { header: "Total", align: "right", render: (r) => <Money value={r.total} /> },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
          ]}
        />
        <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
      </SectionCard>
    </Page>
  );
}
