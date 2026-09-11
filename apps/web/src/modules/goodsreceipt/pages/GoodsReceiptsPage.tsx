import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  DataTable,
  FilterBar,
  FormField,
  PageHeader,
  Pagination,
  SectionCard,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { goodsreceiptService } from "@/modules/goodsreceipt/services/goodsreceipt.service";
import { ReceiveModal } from "@/modules/goodsreceipt/components/ReceiveModal";
import type { GoodsReceipt } from "@/modules/goodsreceipt/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

const LIMIT = 20;

// Kolom tabel hanya dari kunci receiptView yang benar-benar ada: id,
// orderNumber, receivedAt, items. Backend TIDAK menyediakan nomor dokumen,
// supplier, maupun status penerimaan — kolom-kolom itu sengaja tidak ada.
// Filter: backend List hanya menerima purchaseOrderId (tanpa search).

export default function GoodsReceiptsPage() {
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("goodsreceipt.create");

  const [poInput, setPoInput] = useState("");
  const [purchaseOrderId, setPurchaseOrderId] = useState<number | null>(null);
  const [page, setPage] = useState(1);
  const [modalOpen, setModalOpen] = useState(false);

  const { data, loading, error, reload } = useAsyncData(
    () =>
      goodsreceiptService
        .list({
          purchaseOrderId: purchaseOrderId ?? undefined,
          page,
          limit: LIMIT,
        })
        .then((res) => ({ items: res.items, total: res.meta.total }))
        .catch((err) => {
          const apiErr = toApiError(err);
          toast.danger("Gagal memuat penerimaan", apiErr.message);
          throw err;
        }),
    [purchaseOrderId, page],
  );
  const items: GoodsReceipt[] = data?.items ?? [];
  const total = data?.total ?? 0;

  function applyFilter(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const n = Number(poInput.trim());
    setPurchaseOrderId(poInput.trim() !== "" && Number.isInteger(n) && n > 0 ? n : null);
    setPage(1);
  }

  return (
    <div>
      <PageHeader
        eyebrow="Pembelian"
        title="Penerimaan Barang"
        description="Catatan penerimaan per purchase order — klik ID untuk melihat detail."
        actions={
          canCreate ? (
            <Button onClick={() => setModalOpen(true)}>
              Catat Penerimaan
            </Button>
          ) : undefined
        }
      />

      <FilterBar>
        <form onSubmit={applyFilter} className="flex flex-wrap items-end gap-4">
          <div className="min-w-52 flex-1">
            <FormField
              label="Purchase Order ID"
              helperText="Backend hanya mendukung filter purchaseOrderId."
            >
              <TextInput
                value={poInput}
                onChange={(e) => setPoInput(e.target.value)}
                placeholder="cth: 12 (kosongkan = semua)"
                inputMode="numeric"
              />
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
            <DataTable<GoodsReceipt>
              rows={items}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada penerimaan"
              emptyDescription="Catat penerimaan baru atau ubah filter purchase order."
              columns={[
                {
                  header: "ID",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.goodsReceipts}/${r.id}`}
                      className="font-medium text-brand hover:underline"
                    >
                      #{r.id}
                    </Link>
                  ),
                },
                {
                  header: "Tanggal Terima",
                  render: (r) => formatDate(r.receivedAt),
                },
                { header: "No. PO", render: (r) => r.orderNumber || "-" },
                {
                  header: "Baris",
                  align: "right",
                  render: (r) => formatNumber(r.items.length),
                },
                {
                  header: "Aksi",
                  align: "right",
                  render: (r) => (
                    <Link
                      to={`${ROUTE_PATHS.goodsReceipts}/${r.id}`}
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

      <ReceiveModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onCreated={(id) => {
          setModalOpen(false);
          reload();
          navigate(`${ROUTE_PATHS.goodsReceipts}/${id}`);
        }}
      />
    </div>
  );
}
