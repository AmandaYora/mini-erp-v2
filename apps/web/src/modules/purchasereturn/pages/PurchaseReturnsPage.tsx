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
import { DocNumber, Money, Page, Toolbar } from "@/shared/components/patterns";
import type { ActiveFilter } from "@/shared/components/patterns";
import { formatDate, statusLabel } from "@/shared/lib/format";
import { fromCsv } from "@/shared/lib/list-query";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { PurchaseOrderSearchSelect } from "@/modules/purchasing/components/PurchaseOrderSearchSelect";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { purchaseReturnService } from "@/modules/purchasereturn/services/purchase-return.service";
import { PURCHASE_RETURN_STATUSES } from "@/modules/purchasereturn/types";
import {
  PURCHASERETURN_LIST_DEFAULTS,
  purchasereturnListFilterSchema,
  toPurchaseReturnFilterParams,
} from "@/modules/purchasereturn/schemas/purchase-return-list-filter.schema";
import type { PurchaseReturn } from "@/modules/purchasereturn/types";

const LIMIT = 20;

export default function PurchaseReturnsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("purchasereturn.create");

  const { filters, setFilters, reset } = useUrlFilters(purchasereturnListFilterSchema, PURCHASERETURN_LIST_DEFAULTS);

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      purchaseReturnService
        .list({ ...toPurchaseReturnFilterParams(filters), page: filters.page, limit: LIMIT }, signal)
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [filters],
  );
  const items = data?.items ?? null;
  const total = data?.total ?? 0;

  const newReturnTo = `${ROUTE_PATHS.purchaseReturns}/new`;
  const hasFilters =
    filters.search !== "" || filters.status !== "" || filters.purchaseOrderId > 0 || filters.from !== "" || filters.to !== "";

  const active: ActiveFilter[] = [];
  for (const s of fromCsv(filters.status)) {
    active.push({
      key: `status-${s}`,
      label: `Status: ${statusLabel(s)}`,
      onRemove: () => setFilters({ status: fromCsv(filters.status).filter((x) => x !== s).join(",") }),
    });
  }
  if (filters.purchaseOrderId > 0)
    active.push({ key: "so", label: "Order beli dipilih", onRemove: () => setFilters({ purchaseOrderId: 0 }) });
  if (filters.from || filters.to)
    active.push({
      key: "date",
      label: `Retur: ${filters.from ? formatDate(filters.from) : "…"} – ${filters.to ? formatDate(filters.to) : "…"}`,
      onRemove: () => setFilters({ from: "", to: "" }),
    });

  return (
    <Page>
      <PageHeader
        title="Retur Beli"
        description="Retur pembelian terhadap order beli terkonfirmasi. Klik baris untuk detail dan proses."
        actions={
          canCreate ? (
            <Button asChild>
              <Link to={newReturnTo}>Buat retur</Link>
            </Button>
          ) : undefined
        }
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari retur"
            value={filters.search}
            onSearch={(v) => setFilters({ search: v })}
            placeholder="Nomor retur, order, pemasok, alasan…"
          />
        }
        primary={
          <>
            <FormField label="Status">
              <MultiToggle
                aria-label="Status retur"
                options={PURCHASE_RETURN_STATUSES.map((s) => ({ value: s, label: statusLabel(s) }))}
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
          <FormField label="Order beli">
            <PurchaseOrderSearchSelect
              value={filters.purchaseOrderId || null}
              allowClear
              placeholder="Semua order"
              onChange={(id) => setFilters({ purchaseOrderId: id ?? 0 })}
            />
          </FormField>
        }
        moreActiveCount={filters.purchaseOrderId > 0 ? 1 : 0}
        active={active}
        onClearAll={reset}
      />

      <SectionCard flush>
        <DataTable<PurchaseReturn>
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={(r) => void navigate(`${ROUTE_PATHS.purchaseReturns}/${r.id}`)}
          emptyTitle={hasFilters ? "Tidak ada retur yang cocok" : "Belum ada retur pembelian"}
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
                <Link to={`${ROUTE_PATHS.purchaseReturns}/${r.id}`} className="font-medium text-primary hover:underline">
                  <DocNumber value={r.number} />
                </Link>
              ),
            },
            { header: "Tanggal", render: (r) => formatDate(r.returnDate) },
            {
              header: "Order beli",
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
