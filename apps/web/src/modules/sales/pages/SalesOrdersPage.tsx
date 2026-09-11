import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  FilterBar,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  formatIDR,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { salesService } from "@/modules/sales/services/sales.service";
import { SALES_STATUSES, channelLabel as salesChannelLabel } from "@/modules/sales/types";
import type { SalesOrder } from "@/modules/sales/types";

const LIMIT = 20;

export default function SalesOrdersPage() {
  const can = useAuthStore((s) => s.can);
  const canCreate = can("sales.create");

  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [channel, setChannel] = useState("");
  const [page, setPage] = useState(1);
  const [exporting, setExporting] = useState<null | "xlsx" | "pdf">(null);
  const [exportError, setExportError] = useState<string | null>(null);

  const { data, loading, error: fetchError, reload } = useAsyncData(
    () =>
      salesService
        .list({ search, status, channel, page, limit: LIMIT })
        .then((res) => ({ items: res.items, total: res.meta.total })),
    [search, status, channel, page],
  );
  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const error = fetchError ?? exportError;

  function applySearch(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSearch(searchInput);
    setPage(1);
  }

  async function handleExport(format: "xlsx" | "pdf") {
    setExporting(format);
    setExportError(null);
    try {
      await salesService.downloadExport(format, { search, status, channel });
    } catch (err) {
      setExportError(toApiError(err).message);
    } finally {
      setExporting(null);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Operasional"
        title="Order Jual"
        description="Order penjualan reguler — klik nomor untuk melihat detail dan proses."
        actions={
          <div className="flex flex-wrap gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={exporting !== null}
              onClick={() => void handleExport("xlsx")}
            >
              {exporting === "xlsx" ? "Mengunduh…" : "Ekspor XLSX"}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={exporting !== null}
              onClick={() => void handleExport("pdf")}
            >
              {exporting === "pdf" ? "Mengunduh…" : "Ekspor PDF"}
            </Button>
            {canCreate ? (
              <Link to={`${ROUTE_PATHS.salesOrders}/new`}>
                <Button>Tambah Order</Button>
              </Link>
            ) : undefined}
          </div>
        }
      />

      <FilterBar>
        <form onSubmit={applySearch} className="flex flex-wrap items-end gap-4">
          <div className="min-w-52 flex-1">
            <FormField label="Cari">
              <TextInput
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="Nomor order…"
              />
            </FormField>
          </div>
          <FormField label="Status">
            <SelectInput
              value={status}
              onChange={(e) => {
                setStatus(e.target.value);
                setPage(1);
              }}
            >
              <option value="">Semua</option>
              {SALES_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {statusLabel(s)}
                </option>
              ))}
            </SelectInput>
          </FormField>
          <FormField label="Channel">
            <SelectInput
              value={channel}
              onChange={(e) => {
                setChannel(e.target.value);
                setPage(1);
              }}
            >
              <option value="">Semua</option>
              <option value="regular">Reguler</option>
              <option value="pos">POS</option>
            </SelectInput>
          </FormField>
          <Button type="submit" variant="secondary">
            Cari
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
            <DataTable<SalesOrder>
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada order jual"
              emptyDescription="Buat order baru atau ubah kata kunci pencarian."
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.salesOrders}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {r.number}
                    </Link>
                  ),
                },
                { header: "Tanggal", render: (r) => formatDate(r.orderDate) },
                { header: "Customer", render: (r) => r.partyName || "Walk-in" },
                {
                  header: "Channel",
                  render: (r) => (
                    <Badge tone={r.channel === "pos" ? "info" : "neutral"}>
                      {salesChannelLabel(r.channel)}
                    </Badge>
                  ),
                },
                {
                  header: "Total",
                  align: "right",
                  render: (r) => formatIDR(r.grandTotal),
                },
                {
                  header: "Status",
                  render: (r) => (
                    <Badge tone={statusTone(r.status)}>{statusLabel(r.status)}</Badge>
                  ),
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.salesOrders}/${r.id}`}
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
    </div>
  );
}
