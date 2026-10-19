import { Link, useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  DateRangeFields,
  FormField,
  MultiToggle,
  PageHeader,
  Pagination,
  SearchInput,
  SectionCard,
  SelectInput,
  StatusBadge,
} from "@/shared/components/ui";
import { DocNumber, Page, Qty, Toolbar, type ActiveFilter } from "@/shared/components/patterns";
import { formatDate, statusLabel } from "@/shared/lib/format";
import { isAbort, toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import {
  DELIVERY_LIST_DEFAULTS,
  deliveryListFilterSchema,
  toDeliveryFilterParams,
} from "@/modules/delivery/schemas/delivery-list-filter.schema";
import { DELIVERY_STATUSES } from "@/modules/delivery/types";
import type { DeliveryNote } from "@/modules/delivery/types";
import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";

const LIMIT = 20;

// Kolom tabel hanya dari kunci noteView yang benar-benar ada: number,
// deliveryDate, orderNumber, status, items, documentKind, salesReturnId.
// Filter (semua server-side): search (nomor/penerima/sopir/plat), status CSV,
// salesOrderId, salesReturnId, documentKind, from/to.

export default function DeliveriesPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("delivery.create");

  const { filters, setFilters, reset } = useUrlFilters(deliveryListFilterSchema, DELIVERY_LIST_DEFAULTS);

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      deliveryService
        .list({ ...toDeliveryFilterParams(filters), page: filters.page, limit: LIMIT }, signal)
        .then((res) => ({ items: res.items, total: res.meta.total }))
        .catch((err) => {
          if (!isAbort(err)) {
            toast.danger("Gagal memuat pengiriman", toApiError(err).message);
          }
          throw err;
        }),
    [filters],
  );
  const items: DeliveryNote[] = data?.items ?? [];
  const total = data?.total ?? 0;

  const active: ActiveFilter[] = [];
  if (filters.salesOrderId > 0) {
    active.push({
      key: "salesOrderId",
      label: `Order jual #${filters.salesOrderId}`,
      onRemove: () => setFilters({ salesOrderId: 0 }),
    });
  }
  if (filters.documentKind) {
    active.push({
      key: "documentKind",
      label: filters.documentKind === "replacement" ? "Jenis: barang pengganti" : "Jenis: surat jalan order",
      onRemove: () => setFilters({ documentKind: "" }),
    });
  }
  if (filters.salesReturnId > 0) {
    active.push({
      key: "salesReturnId",
      label: `Retur penjualan #${filters.salesReturnId}`,
      onRemove: () => setFilters({ salesReturnId: 0 }),
    });
  }
  const hasFilters = active.length > 0 || Boolean(filters.search || filters.status || filters.from || filters.to);

  const createButton = canCreate ? (
    <Button asChild>
      <Link to={`${ROUTE_PATHS.deliveries}/new`}>Buat surat jalan</Link>
    </Button>
  ) : undefined;

  return (
    <Page>
      <PageHeader
        title="Pengiriman"
        description="Surat jalan per order jual. Klik baris untuk detail, konfirmasi, dan bukti kirim."
        actions={createButton}
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari surat jalan"
            value={filters.search}
            onSearch={(v) => setFilters({ search: v })}
            placeholder="Nomor, penerima, sopir, plat…"
          />
        }
        primary={
          <>
            <FormField label="Status">
              <MultiToggle
                aria-label="Status pengiriman"
                options={DELIVERY_STATUSES.map((s) => ({ value: s, label: statusLabel(s) }))}
                value={filters.status}
                onChange={(status) => setFilters({ status })}
              />
            </FormField>
            <DateRangeFields
              from={filters.from}
              to={filters.to}
              fromLabel="Kirim dari"
              toLabel="Kirim sampai"
              onChange={({ from, to }) => setFilters({ from, to })}
            />
          </>
        }
        more={
          <>
            <FormField label="Order jual">
              <SalesOrderSearchSelect
                value={filters.salesOrderId || null}
                allowClear
                placeholder="Semua order"
                onChange={(id) => setFilters({ salesOrderId: id ?? 0 })}
              />
            </FormField>
            <FormField label="Jenis">
              <SelectInput
                aria-label="Jenis dokumen"
                value={filters.documentKind}
                onChange={(e) => setFilters({ documentKind: e.target.value as typeof filters.documentKind })}
              >
                <option value="">Semua</option>
                <option value="order">Surat jalan order</option>
                <option value="replacement">Barang pengganti</option>
              </SelectInput>
            </FormField>
          </>
        }
        moreActiveCount={active.filter((a) => a.key !== "salesReturnId").length}
        active={active}
        onClearAll={reset}
      />

      <SectionCard flush>
        <DataTable<DeliveryNote>
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={(r) => navigate(`${ROUTE_PATHS.deliveries}/${r.id}`)}
          emptyTitle={hasFilters ? "Tidak ada yang cocok dengan filter ini" : "Belum ada surat jalan"}
          emptyDescription={
            hasFilters ? "Ubah atau hapus filter untuk melihat surat jalan lain." : "Buat surat jalan dari order jual terkonfirmasi."
          }
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
                <Link to={`${ROUTE_PATHS.deliveries}/${r.id}`} className="font-medium text-primary hover:underline">
                  <DocNumber value={r.number} />
                </Link>
              ),
            },
            { header: "Tanggal", render: (r) => formatDate(r.deliveryDate) },
            { header: "Order jual", render: (r) => (r.orderNumber ? <DocNumber value={r.orderNumber} /> : "–") },
            {
              header: "Jenis",
              render: (r) =>
                r.documentKind === "replacement" ? (
                  <Badge tone="info">Pengganti</Badge>
                ) : (
                  <Badge tone="neutral">Order</Badge>
                ),
            },
            { header: "Penerima", render: (r) => r.recipientName || "–" },
            { header: "Baris", align: "right", render: (r) => <Qty value={r.items.length} /> },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
          ]}
        />
        <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
      </SectionCard>
    </Page>
  );
}
