import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ChevronDown } from "lucide-react";
import {
  Badge,
  Button,
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
import { DocNumber, Money, Page, Toolbar, type ActiveFilter } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, statusLabel } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { billingBadge, billingView } from "@/modules/payment/lib/order-billing";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { salesService } from "@/modules/sales/services/sales.service";
import {
  PAYMENT_STATUSES,
  SALES_LIST_DEFAULTS,
  salesListFilterSchema,
  toSalesFilterParams,
} from "@/modules/sales/schemas/sales-list-filter.schema";
import {
  SALES_STATUSES,
  channelLabel as salesChannelLabel,
  paymentTermsLabel,
} from "@/modules/sales/types";
import type { SalesOrder } from "@/modules/sales/types";

const LIMIT = 20;

const PAYMENT_STATUS_LABEL = {
  unpaid: "Belum bayar",
  partial: "Bayar sebagian",
  paid: "Lunas",
  overdue: "Jatuh tempo",
} as const;

export default function SalesOrdersPage() {
  const can = useAuthStore((s) => s.can);
  const navigate = useNavigate();
  const canCreate = can("sales.create");
  const canExport = can("sales.export");

  const { filters, setFilters, reset } = useUrlFilters(salesListFilterSchema, SALES_LIST_DEFAULTS);
  const [exporting, setExporting] = useState<null | "xlsx" | "pdf">(null);
  const [exportError, setExportError] = useState<string | null>(null);

  // Filter terapan -> param service; list dan ekspor memakai objek yang sama.
  const filterParams = toSalesFilterParams(filters);
  const { data, loading, error: fetchError, reload } = useAsyncData(
    (signal) =>
      salesService
        .list({ ...filterParams, page: filters.page, limit: LIMIT }, signal)
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [filters],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  async function handleExport(format: "xlsx" | "pdf") {
    setExporting(format);
    setExportError(null);
    try {
      await salesService.downloadExport(format, filterParams);
    } catch (err) {
      setExportError(toApiError(err).message);
    } finally {
      setExporting(null);
    }
  }

  const active: ActiveFilter[] = [];
  if (filters.partyId) {
    active.push({
      key: "partyId",
      label: `Pelanggan #${filters.partyId}`,
      onRemove: () => setFilters({ partyId: 0 }),
    });
  }
  if (filters.channel) {
    active.push({
      key: "channel",
      label: `Kanal: ${salesChannelLabel(filters.channel)}`,
      onRemove: () => setFilters({ channel: "" }),
    });
  }
  if (filters.paymentTerms) {
    active.push({
      key: "paymentTerms",
      label: `Termin: ${paymentTermsLabel(filters.paymentTerms)}`,
      onRemove: () => setFilters({ paymentTerms: "" }),
    });
  }
  if (filters.paymentStatus) {
    active.push({
      key: "paymentStatus",
      label: `Pembayaran: ${PAYMENT_STATUS_LABEL[filters.paymentStatus as keyof typeof PAYMENT_STATUS_LABEL]}`,
      onRemove: () => setFilters({ paymentStatus: "" }),
    });
  }
  const hasFilters = active.length > 0 || Boolean(filters.search || filters.status || filters.from || filters.to);

  const createLink = canCreate ? (
    <Button asChild>
      <Link to={`${ROUTE_PATHS.salesOrders}/new`}>Tambah order</Link>
    </Button>
  ) : undefined;

  return (
    <Page>
      <PageHeader
        title="Order Jual"
        description="Order penjualan reguler dan POS. Klik baris untuk melihat detail dan memprosesnya."
        actions={
          <>
            {canExport && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="secondary" size="sm" loading={exporting !== null}>
                    Ekspor <ChevronDown aria-hidden />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="min-w-40">
                  <DropdownMenuItem disabled={exporting !== null} onSelect={() => void handleExport("xlsx")}>
                    Ekspor XLSX
                  </DropdownMenuItem>
                  <DropdownMenuItem disabled={exporting !== null} onSelect={() => void handleExport("pdf")}>
                    Ekspor PDF
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )}
            {createLink}
          </>
        }
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari order jual"
            value={filters.search}
            onSearch={(v) => setFilters({ search: v })}
            placeholder="Nomor, nama, telepon, email…"
          />
        }
        primary={
          <>
            <FormField label="Status">
              <MultiToggle
                aria-label="Status order"
                options={SALES_STATUSES.map((s) => ({ value: s, label: statusLabel(s) }))}
                value={filters.status}
                onChange={(status) => setFilters({ status })}
              />
            </FormField>
            <DateRangeFields
              from={filters.from}
              to={filters.to}
              onChange={({ from, to }) => setFilters({ from, to })}
            />
          </>
        }
        more={
          <>
            <FormField label="Pelanggan">
              <PartySearchSelect
                partyKind="customer"
                value={filters.partyId || null}
                placeholder="Semua pelanggan"
                onChange={(id) => setFilters({ partyId: id ?? 0 })}
              />
            </FormField>
            <FormField label="Termin">
              <SelectInput
                aria-label="Termin"
                value={filters.paymentTerms}
                onChange={(e) => setFilters({ paymentTerms: e.target.value as typeof filters.paymentTerms })}
              >
                <option value="">Semua</option>
                {(["cod", "net", "prepaid"] as const).map((t) => (
                  <option key={t} value={t}>
                    {paymentTermsLabel(t)}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <FormField label="Pembayaran">
              <SelectInput
                aria-label="Pembayaran"
                value={filters.paymentStatus}
                onChange={(e) => setFilters({ paymentStatus: e.target.value as typeof filters.paymentStatus })}
              >
                <option value="">Semua</option>
                {PAYMENT_STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {PAYMENT_STATUS_LABEL[s]}
                  </option>
                ))}
              </SelectInput>
            </FormField>
            <FormField label="Kanal">
              <SelectInput
                aria-label="Kanal"
                value={filters.channel}
                onChange={(e) => setFilters({ channel: e.target.value as typeof filters.channel })}
              >
                <option value="">Semua</option>
                <option value="regular">Reguler</option>
                <option value="pos">POS</option>
              </SelectInput>
            </FormField>
          </>
        }
        moreActiveCount={active.length}
        active={active}
        onClearAll={reset}
      />

      {exportError && (
        <Notice tone="danger" title="Ekspor gagal">
          {exportError}
        </Notice>
      )}

      <SectionCard flush>
        <DataTable<SalesOrder>
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={fetchError}
          onRetry={reload}
          onRowClick={(r) => navigate(`${ROUTE_PATHS.salesOrders}/${r.id}`)}
          sort={{ key: filters.sort, dir: filters.dir }}
          onSortChange={(s) => setFilters({ sort: s.key as typeof filters.sort, dir: s.dir })}
          emptyTitle={hasFilters ? "Tidak ada yang cocok dengan filter ini" : "Belum ada order jual"}
          emptyDescription={
            hasFilters ? "Ubah atau hapus filter untuk melihat order lain." : "Buat order jual pertama untuk memulai."
          }
          emptyAction={
            hasFilters ? (
              <Button variant="secondary" size="sm" onClick={reset}>
                Hapus filter
              </Button>
            ) : (
              createLink
            )
          }
          columns={[
            {
              header: "Nomor",
              sortKey: "number",
              render: (r) => (
                <Link to={`${ROUTE_PATHS.salesOrders}/${r.id}`} className="font-medium text-primary hover:underline">
                  <DocNumber value={r.number} />
                </Link>
              ),
            },
            { header: "Tanggal", sortKey: "date", render: (r) => formatDate(r.orderDate) },
            { header: "Pelanggan", render: (r) => r.partyName || "Tunai" },
            {
              header: "Kanal",
              render: (r) => (
                <Badge tone={r.channel === "pos" ? "info" : "neutral"}>{salesChannelLabel(r.channel)}</Badge>
              ),
            },
            {
              header: "Total",
              align: "right",
              sortKey: "total",
              render: (r) => <Money value={r.grandTotal} />,
            },
            {
              header: "Dibayar",
              align: "right",
              render: (r) => (billingView(r).available ? <Money value={billingView(r).paid} /> : "-"),
            },
            {
              header: "Sisa",
              align: "right",
              render: (r) => (billingView(r).available ? <Money value={billingView(r).outstanding} /> : "-"),
            },
            {
              header: "Pembayaran",
              render: (r) => {
                const b = billingView(r);
                if (!b.available || r.status === "draft" || r.status === "cancelled") return "-";
                const badge = billingBadge(b);
                return <Badge tone={badge.tone}>{badge.label}</Badge>;
              },
            },
            { header: "Dibuat oleh", render: (r) => r.createdByName || "-" },
            { header: "Status", sortKey: "status", render: (r) => <StatusBadge status={r.status} /> },
          ]}
        />
        <Pagination page={filters.page} limit={LIMIT} total={total} onPageChange={(p) => setFilters({ page: p })} />
      </SectionCard>
    </Page>
  );
}
