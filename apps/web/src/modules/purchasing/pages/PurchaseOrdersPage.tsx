import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
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
import { formatDate, formatIDR, statusLabel, statusTone } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { purchaseOrderService } from "@/modules/purchasing/services/purchase-order.service";
import type { PurchaseOrder } from "@/modules/purchasing/types";

const LIMIT = 20;

const STATUS_OPTIONS = [
  { value: "", label: "Semua status" },
  { value: "draft", label: "Draf" },
  { value: "confirmed", label: "Terkonfirmasi" },
  { value: "completed", label: "Selesai" },
  { value: "cancelled", label: "Dibatalkan" },
];

export default function PurchaseOrdersPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("purchasing.create");

  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [exporting, setExporting] = useState<null | "xlsx" | "pdf">(null);
  const [exportError, setExportError] = useState<string | null>(null);

  const { data, loading, error: fetchError, reload } = useAsyncData(
    async () => {
      try {
        const res = await purchaseOrderService.list({ search, status, page, limit: LIMIT });
        return { items: res.items, total: res.meta.total };
      } catch (err) {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat order beli", apiErr.message);
        throw err;
      }
    },
    [search, status, page],
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
      await purchaseOrderService.downloadExport(format, { search, status });
    } catch (err) {
      const apiErr = toApiError(err);
      setExportError(apiErr.message);
      toast.danger("Gagal mengunduh ekspor", apiErr.message);
    } finally {
      setExporting(null);
    }
  }

  return (
    <div>
      <PageHeader
        eyebrow="Pembelian"
        title="Order Beli"
        description="Daftar purchase order — klik nomor untuk melihat detail."
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
              <Button onClick={() => void navigate(`${ROUTE_PATHS.purchaseOrders}/new`)}>
                Tambah Order
              </Button>
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
                placeholder="Nomor / nama supplier…"
              />
            </FormField>
          </div>
          <div className="min-w-44">
            <FormField label="Status">
              <SelectInput
                value={status}
                onChange={(e) => {
                  setStatus(e.target.value);
                  setPage(1);
                }}
              >
                {STATUS_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </SelectInput>
            </FormField>
          </div>
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
            <DataTable<PurchaseOrder>
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada order beli"
              emptyDescription="Buat order baru atau ubah kata kunci pencarian."
              columns={[
                {
                  header: "Nomor",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.purchaseOrders}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      {r.number}
                    </Link>
                  ),
                },
                { header: "Tanggal", render: (r) => formatDate(r.orderDate) },
                { header: "Supplier", render: (r) => r.partyName || "-" },
                {
                  header: "Total",
                  align: "right",
                  render: (r) => (
                    <span className="font-medium">{formatIDR(r.grandTotal)}</span>
                  ),
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
                      to={`${ROUTE_PATHS.purchaseOrders}/${r.id}`}
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
