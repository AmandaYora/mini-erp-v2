import { Link, useNavigate } from "react-router-dom";
import { ChevronDown } from "lucide-react";
import {
  Badge,
  Button,
  Checkbox,
  DataTable,
  DateRangeFields,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  FormField,
  MultiToggle,
  PageHeader,
  Pagination,
  SearchInput,
  SectionCard,
  SelectInput,
  StatusBadge,
} from "@/shared/components/ui";
import { DocNumber, Page, Qty, SavedViews, Toolbar, type ActiveFilter } from "@/shared/components/patterns";
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
import { DELIVERY_STATUSES, isUnorderedDirect } from "@/modules/delivery/types";
import type { DeliveryNote } from "@/modules/delivery/types";
import { SalesOrderSearchSelect } from "@/modules/sales/components/SalesOrderSearchSelect";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";

const LIMIT = 20;

// Filter tanda tangan penerima (alert kesehatan data A11: SJ bersopir tanpa status TTD).
const SIGNATURE_LABEL: Record<string, string> = { none: "belum diisi", signed: "bertanda tangan", missing: "tanpa TTD (beralasan)" };

// Kolom tabel hanya dari kunci noteView yang benar-benar ada: number,
// deliveryDate, orderNumber, status, items, documentKind, salesReturnId,
// origin, partyName. Filter (semua server-side): search (nomor/penerima/
// sopir/plat), status CSV, salesOrderId, salesReturnId, documentKind, from/to,
// origin (order|direct), unordered=1 (SJ Langsung belum diorder, ADR-0022).

export default function DeliveriesPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("delivery.create");
  const canCreateDirect = can("delivery.create_direct");

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
  if (filters.origin) {
    active.push({
      key: "origin",
      label: filters.origin === "direct" ? "Asal: SJ Langsung" : "Asal: SJ order",
      onRemove: () => setFilters({ origin: "" }),
    });
  }
  if (filters.unordered === "1") {
    active.push({ key: "unordered", label: "Belum diorder", onRemove: () => setFilters({ unordered: "" }) });
  }
  if (filters.signature) {
    active.push({
      key: "signature",
      label: `TTD: ${SIGNATURE_LABEL[filters.signature] ?? filters.signature}`,
      onRemove: () => setFilters({ signature: "" }),
    });
  }
  const hasFilters = active.length > 0 || Boolean(filters.search || filters.status || filters.from || filters.to);

  // "Buat surat jalan": tautan langsung bila hanya satu jalur; menu bercabang
  // bila pengguna juga boleh membuat SJ Langsung (tanpa order, ADR-0022).
  const createButton = canCreateDirect ? (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button>
          Buat surat jalan
          <ChevronDown aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56">
        {canCreate && (
          <DropdownMenuItem onSelect={() => navigate(ROUTE_PATHS.deliveriesNew)}>Dari order jual</DropdownMenuItem>
        )}
        <DropdownMenuItem onSelect={() => navigate(ROUTE_PATHS.deliveriesNewDirect)}>
          Tanpa order (SJ Langsung)
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  ) : canCreate ? (
    <Button asChild>
      <Link to={ROUTE_PATHS.deliveriesNew}>Buat surat jalan</Link>
    </Button>
  ) : undefined;

  return (
    <Page>
      <PageHeader
        title="Pengiriman"
        description="Surat jalan per order jual atau SJ Langsung tanpa order. Klik baris untuk detail, konfirmasi, dan bukti kirim."
        actions={createButton}
      />

      <Toolbar
        right={<SavedViews listKey="deliveries" columnsKey="deliveries" />}
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
            <FormField label="Asal">
              <SelectInput
                aria-label="Asal surat jalan"
                value={filters.origin}
                onChange={(e) => setFilters({ origin: e.target.value as typeof filters.origin })}
              >
                <option value="">Semua</option>
                <option value="order">SJ order</option>
                <option value="direct">SJ Langsung</option>
              </SelectInput>
            </FormField>
            <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
              <Checkbox
                aria-label="Belum diorder"
                checked={filters.unordered === "1"}
                onCheckedChange={(v) => setFilters({ unordered: v === true ? "1" : "" })}
              />
              Belum diorder
              <span className="text-xs text-muted-foreground">(SJ Langsung tanpa order)</span>
            </label>
          </>
        }
        moreActiveCount={active.filter((a) => a.key !== "salesReturnId").length}
        active={active}
        onClearAll={reset}
      />

      <SectionCard flush>
        <DataTable<DeliveryNote>
          columnsKey="deliveries"
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
            {
              header: "Order jual",
              // SJ Langsung tanpa order: tampilkan pelanggannya (snapshot di SJ).
              render: (r) =>
                r.orderNumber ? (
                  <DocNumber value={r.orderNumber} />
                ) : r.origin === "direct" ? (
                  <span>{r.partyName || "–"}</span>
                ) : (
                  "–"
                ),
            },
            {
              header: "Jenis",
              render: (r) => (
                <span className="inline-flex flex-wrap gap-1">
                  {r.documentKind === "replacement" ? (
                    <Badge tone="info">Pengganti</Badge>
                  ) : r.origin === "direct" ? (
                    <Badge tone="info">SJ Langsung</Badge>
                  ) : (
                    <Badge tone="neutral">Order</Badge>
                  )}
                  {isUnorderedDirect({ origin: r.origin, salesOrderId: r.salesOrderId }) && (
                    <Badge tone="warning">Belum diorder</Badge>
                  )}
                </span>
              ),
            },
            { header: "Penerima", render: (r) => r.recipientName || "–" },
            {
              header: "TTD",
              render: (r) =>
                r.status !== "confirmed" ? (
                  "–"
                ) : r.recipientSignatureStatus === "signed" ? (
                  <Badge tone="success">Bertanda tangan</Badge>
                ) : r.recipientSignatureStatus === "missing" ? (
                  <Badge tone="muted">Tanpa TTD</Badge>
                ) : r.driverName ? (
                  <Badge tone="warning">Belum diisi</Badge>
                ) : (
                  "–"
                ),
            },
            { header: "Baris", align: "right", render: (r) => <Qty value={r.items.length} /> },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
          ]}
        />
        <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
      </SectionCard>
    </Page>
  );
}
