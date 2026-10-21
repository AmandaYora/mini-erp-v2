import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  DataTable,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  StatusBadge,
} from "@/shared/components/ui";
import { DocNumber, Page, Qty, SavedViews, Toolbar, type ActiveFilter } from "@/shared/components/patterns";
import { formatDate } from "@/shared/lib/format";
import { isAbort, toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { goodsreceiptService } from "@/modules/goodsreceipt/services/goodsreceipt.service";
import type { GoodsReceipt } from "@/modules/goodsreceipt/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useUrlFilters } from "@/shared/hooks/use-url-filters";
import { PurchaseOrderSearchSelect } from "@/modules/purchasing/components/PurchaseOrderSearchSelect";
import {
  GOODSRECEIPT_LIST_DEFAULTS,
  goodsreceiptListFilterSchema,
} from "@/modules/goodsreceipt/schemas/goodsreceipt-list-filter.schema";

const LIMIT = 20;

// Kolom tabel dari kunci receiptView: id, number, status (pending|posted|void),
// supplierDeliveryNumber, orderNumber, receivedAt, items. Penerimaan `pending`
// = tersimpan tetapi stok belum masuk (diselesaikan/dibatalkan di detail).
// Filter: backend List hanya menerima purchaseOrderId (tanpa search/tanggal/status/sortir).

export default function GoodsReceiptsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("goodsreceipt.create");

  const { filters, setFilters, reset } = useUrlFilters(goodsreceiptListFilterSchema, GOODSRECEIPT_LIST_DEFAULTS);

  const { data, loading, error, reload } = useAsyncData(
    (signal) =>
      goodsreceiptService
        .list(
          { purchaseOrderId: filters.purchaseOrderId || undefined, page: filters.page, limit: LIMIT },
          signal,
        )
        .then((res) => ({ items: res.items, total: res.meta.total }))
        .catch((err) => {
          // Batalan sendiri bukan kegagalan muat.
          if (!isAbort(err)) {
            const apiErr = toApiError(err);
            toast.danger("Gagal memuat penerimaan", apiErr.message);
          }
          throw err;
        }),
    [filters],
  );
  const items: GoodsReceipt[] = data?.items ?? [];
  const total = data?.total ?? 0;

  const active: ActiveFilter[] = [];
  if (filters.purchaseOrderId) {
    active.push({
      key: "po",
      label: "Order beli dipilih",
      onRemove: () => setFilters({ purchaseOrderId: 0 }),
    });
  }
  const filtered = active.length > 0;

  const createButton = canCreate ? (
    <Button asChild>
      <Link to={ROUTE_PATHS.goodsReceiptsNew}>Catat penerimaan</Link>
    </Button>
  ) : undefined;

  return (
    <Page>
      <PageHeader
        title="Penerimaan barang"
        description="Catatan penerimaan per purchase order. Klik baris untuk melihat detail."
        actions={createButton}
      />

      <Toolbar
        right={<SavedViews listKey="goods-receipts" columnsKey="goods-receipts" />}
        primary={
          <div className="min-w-64 flex-1 sm:max-w-md">
            <FormField label="Order beli">
              <PurchaseOrderSearchSelect
                value={filters.purchaseOrderId || null}
                allowClear
                placeholder="Semua order beli"
                onChange={(id) => setFilters({ purchaseOrderId: id ?? 0 })}
              />
            </FormField>
          </div>
        }
        active={active}
        onClearAll={reset}
      />

      <SectionCard flush>
        <DataTable<GoodsReceipt>
          columnsKey="goods-receipts"
          rows={items}
          rowKey={(r) => r.id}
          loading={loading}
          error={error}
          onRetry={reload}
          onRowClick={(r) => void navigate(`${ROUTE_PATHS.goodsReceipts}/${r.id}`)}
          emptyTitle={filtered ? "Tidak ada yang cocok dengan filter ini" : "Belum ada penerimaan"}
          emptyDescription={
            filtered
              ? "Ubah atau hapus filter order beli untuk melihat penerimaan lain."
              : "Catat penerimaan pertama dari purchase order yang sudah dikonfirmasi."
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
              header: "No. penerimaan",
              render: (r) => (
                <Link
                  to={`${ROUTE_PATHS.goodsReceipts}/${r.id}`}
                  className="font-medium text-primary hover:underline"
                >
                  <DocNumber value={r.number || `#${r.id}`} />
                </Link>
              ),
            },
            { header: "Tanggal terima", render: (r) => formatDate(r.receivedAt) },
            { header: "No. PO", render: (r) => (r.orderNumber ? <DocNumber value={r.orderNumber} /> : "-") },
            { header: "SJ pemasok", render: (r) => r.supplierDeliveryNumber || "-" },
            { header: "Baris", align: "right", render: (r) => <Qty value={r.lineCount} /> },
            { header: "Diterima oleh", render: (r) => r.receivedByName || "-" },
            { header: "Status", render: (r) => <StatusBadge status={r.status} /> },
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
