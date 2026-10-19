import { useState } from "react";
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
import { DocNumber, Money, Page, Toolbar, type ActiveFilter } from "@/shared/components/patterns";
import { formatDate, statusLabel } from "@/shared/lib/format";
import { fromCsv } from "@/shared/lib/list-query";
import { isAbort, toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { PartySearchSelect } from "@/modules/party/components/PartySearchSelect";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { purchaseOrderService } from "@/modules/purchasing/services/purchase-order.service";
import {
  PURCHASE_LIST_DEFAULTS,
  PURCHASE_STATUSES,
  purchaseListFilterSchema,
  toPurchaseFilterParams,
} from "@/modules/purchasing/schemas/purchase-list-filter.schema";
import { paymentTermsLabel } from "@/modules/sales/types";
import { billingBadge, billingView } from "@/modules/payment/lib/order-billing";
import type { PurchaseOrder } from "@/modules/purchasing/types";

const LIMIT = 20;

export default function PurchaseOrdersPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("purchasing.create");
  const canExport = can("purchasing.export");

  const { filters, setFilters, reset } = useUrlFilters(purchaseListFilterSchema, PURCHASE_LIST_DEFAULTS);
  const [exporting, setExporting] = useState<null | "xlsx" | "pdf">(null);

  // Filter terapan -> param service; list dan ekspor memakai objek yang sama.
  const filterParams = toPurchaseFilterParams(filters);
  const { data, loading, error: fetchError, reload } = useAsyncData(
    async (signal) => {
      try {
        const res = await purchaseOrderService.list(
          { ...filterParams, page: filters.page, limit: LIMIT },
          signal,
        );
        return { items: res.items, total: res.meta.total };
      } catch (err) {
        if (!isAbort(err)) toast.danger("Gagal memuat order beli", toApiError(err).message);
        throw err;
      }
    },
    [filters],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  async function handleExport(format: "xlsx" | "pdf") {
    setExporting(format);
    try {
      await purchaseOrderService.downloadExport(format, filterParams);
    } catch (err) {
      toast.danger("Gagal mengunduh ekspor", toApiError(err).message);
    } finally {
      setExporting(null);
    }
  }

  const active: ActiveFilter[] = [];
  if (filters.search) {
    active.push({ key: "search", label: `Cari: ${filters.search}`, onRemove: () => setFilters({ search: "" }) });
  }
  for (const s of fromCsv(filters.status)) {
    active.push({
      key: `status-${s}`,
      label: `Status: ${statusLabel(s)}`,
      onRemove: () => setFilters({ status: fromCsv(filters.status).filter((x) => x !== s).join(",") }),
    });
  }
  if (filters.partyId) {
    active.push({ key: "party", label: "Pemasok dipilih", onRemove: () => setFilters({ partyId: 0 }) });
  }
  if (filters.paymentTerms) {
    active.push({
      key: "terms",
      label: `Termin: ${paymentTermsLabel(filters.paymentTerms)}`,
      onRemove: () => setFilters({ paymentTerms: "" }),
    });
  }
  if (filters.from || filters.to) {
    active.push({
      key: "range",
      label: `Tanggal: ${filters.from ? formatDate(filters.from) : "…"} – ${filters.to ? formatDate(filters.to) : "…"}`,
      onRemove: () => setFilters({ from: "", to: "" }),
    });
  }
  const moreActiveCount =
    (filters.partyId ? 1 : 0) + (filters.paymentTerms ? 1 : 0) + (filters.from || filters.to ? 1 : 0);
  const filtered = active.length > 0;

  const createButton = canCreate ? (
    <Button asChild>
      <Link to={`${ROUTE_PATHS.purchaseOrders}/new`}>Tambah order</Link>
    </Button>
  ) : undefined;

  return (
    <Page>
      <PageHeader
        title="Order beli"
        description="Daftar purchase order ke pemasok. Klik baris untuk melihat detail."
        actions={
          <>
            {canExport && (
              <>
                <Button
                  variant="secondary"
                  size="sm"
                  loading={exporting === "xlsx"}
                  disabled={exporting !== null}
                  onClick={() => void handleExport("xlsx")}
                >
                  Ekspor XLSX
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  loading={exporting === "pdf"}
                  disabled={exporting !== null}
                  onClick={() => void handleExport("pdf")}
                >
                  Ekspor PDF
                </Button>
              </>
            )}
            {createButton}
          </>
        }
      />

      <Toolbar
        search={
          <SearchInput
            aria-label="Cari order beli"
            value={filters.search}
            onSearch={(v) => setFilters({ search: v })}
            placeholder="Nomor, faktur, nama/telepon/email pemasok…"
          />
        }
        primary={
          <MultiToggle
            aria-label="Status order"
            options={PURCHASE_STATUSES.map((s) => ({ value: s, label: statusLabel(s) }))}
            value={filters.status}
            onChange={(status) => setFilters({ status })}
          />
        }
        more={
          <>
            <FormField label="Pemasok">
              <PartySearchSelect
                partyKind="supplier"
                value={filters.partyId || null}
                placeholder="Semua pemasok"
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
            <DateRangeFields
              from={filters.from}
              to={filters.to}
              onChange={({ from, to }) => setFilters({ from, to })}
            />
          </>
        }
        moreActiveCount={moreActiveCount}
        active={active}
        onClearAll={reset}
      />

      <SectionCard flush>
        <DataTable<PurchaseOrder>
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={fetchError}
          onRetry={reload}
          onRowClick={(r) => void navigate(`${ROUTE_PATHS.purchaseOrders}/${r.id}`)}
          sort={{ key: filters.sort, dir: filters.dir }}
          onSortChange={(s) => setFilters({ sort: s.key as typeof filters.sort, dir: s.dir })}
          emptyTitle={filtered ? "Tidak ada yang cocok dengan filter ini" : "Belum ada order beli"}
          emptyDescription={
            filtered
              ? "Ubah atau hapus filter untuk melihat order lain."
              : "Buat order beli pertama untuk mulai memesan ke pemasok."
          }
          emptyAction={
            filtered ? (
              <Button variant="secondary" onClick={reset}>
                Hapus filter
              </Button>
            ) : (
              createButton
            )
          }
          columns={[
            {
              header: "Nomor",
              sortKey: "number",
              render: (r) => (
                <Link
                  to={`${ROUTE_PATHS.purchaseOrders}/${r.id}`}
                  className="font-medium text-primary hover:underline"
                >
                  <DocNumber value={r.number} />
                </Link>
              ),
            },
            { header: "Tanggal", sortKey: "date", render: (r) => formatDate(r.orderDate) },
            { header: "Pemasok", render: (r) => r.partyName || "-" },
            {
              header: "Termin",
              render: (r) => (
                <div>
                  <div>{paymentTermsLabel(r.paymentTerms)}</div>
                  {r.dueDate ? (
                    <div className="text-xs text-muted-foreground">Jatuh tempo {formatDate(r.dueDate)}</div>
                  ) : null}
                </div>
              ),
            },
            {
              header: "Total",
              align: "right",
              sortKey: "total",
              render: (r) => <Money value={r.grandTotal} className="font-medium" />,
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
            {
              header: "Status",
              sortKey: "status",
              render: (r) => (
                <div className="flex flex-wrap items-center gap-1">
                  <StatusBadge status={r.status} />
                  {r.returnSummary === "full" ? <Badge tone="danger">Diretur penuh</Badge> : null}
                  {r.returnSummary === "partial" ? <Badge tone="warning">Diretur sebagian</Badge> : null}
                </div>
              ),
            },
          ]}
        />
        <Pagination
          page={filters.page}
          limit={LIMIT}
          total={total}
          onPageChange={(p) => setFilters({ page: p })}
        />
      </SectionCard>
    </Page>
  );
}
