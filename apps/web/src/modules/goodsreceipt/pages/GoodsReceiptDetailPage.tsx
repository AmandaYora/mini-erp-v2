import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Button,
  DataTable,
  PageHeader,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatNumber } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { goodsreceiptService } from "@/modules/goodsreceipt/services/goodsreceipt.service";
import type {
  GoodsReceipt,
  GoodsReceiptItem,
  PurchaseOrderOption,
} from "@/modules/goodsreceipt/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

// Read-only: RegisterRoutes goodsreceipt (handler.go) hanya mendaftarkan GET
// list, POST create, GET :id — tidak ada endpoint konfirmasi/pembatalan, jadi
// halaman ini tidak menampilkan aksi status apa pun.
// Dipesan vs diterima: receiptView TIDAK membawa qty pesanan maupun nama
// produk — keduanya dijoin best-effort dari detail PO via productId/variantId.

export default function GoodsReceiptDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const receiptId = Number(id);

  const { data, loading, error, reload } = useAsyncData(
    async () => {
      if (!Number.isInteger(receiptId) || receiptId <= 0) {
        throw new Error("ID penerimaan tidak valid.");
      }
      try {
        const rec = await goodsreceiptService.get(receiptId);
        // Nama produk + qty pesanan pelengkap — halaman tetap berguna tanpanya.
        const orderDetail = await goodsreceiptService
          .purchaseOrderDetail(rec.purchaseOrderId)
          .catch(() => null as PurchaseOrderOption | null);
        return { receipt: rec, po: orderDetail };
      } catch (err) {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat penerimaan", apiErr.message);
        throw err;
      }
    },
    [receiptId],
  );
  const receipt: GoodsReceipt | null = data?.receipt ?? null;
  const po: PurchaseOrderOption | null = data?.po ?? null;

  if (loading && !receipt) {
    return (
      <div>
        <PageHeader eyebrow="Pembelian" title="Detail Penerimaan" />
        <p className="text-muted py-8 text-center text-sm">Memuat…</p>
      </div>
    );
  }

  if (error || !receipt) {
    return (
      <div>
        <PageHeader eyebrow="Pembelian" title="Detail Penerimaan" />
        <Notice tone="danger" title={error ?? "Penerimaan tidak ditemukan"} />
        <div className="mt-4 flex gap-3">
          <Button
            variant="secondary"
            onClick={() => void navigate(ROUTE_PATHS.goodsReceipts)}
          >
            Kembali ke Daftar
          </Button>
          {error && <Button onClick={() => reload()}>Coba lagi</Button>}
        </div>
      </div>
    );
  }

  const poLineOf = (it: GoodsReceiptItem) =>
    po?.items.find(
      (l) => l.productId === it.productId && l.variantId === it.variantId,
    ) ?? null;

  return (
    <div>
      <PageHeader
        eyebrow="Pembelian"
        title={`Penerimaan #${receipt.id}`}
        description={`${receipt.orderNumber || "-"} · ${formatDate(receipt.receivedAt)}`}
        actions={
          <Button
            variant="secondary"
            onClick={() => void navigate(ROUTE_PATHS.goodsReceipts)}
          >
            Daftar
          </Button>
        }
      />

      <div className="space-y-4">
        <SectionCard
          title="Ringkasan"
          description="Penerimaan bersifat final — tidak ada aksi konfirmasi/pembatalan."
        >
          <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm md:grid-cols-4">
            <div>
              <dt className="text-muted">ID</dt>
              <dd className="font-semibold text-ink">#{receipt.id}</dd>
            </div>
            <div>
              <dt className="text-muted">No. PO</dt>
              <dd className="font-semibold text-ink">
                <Link
                  to={`${ROUTE_PATHS.purchaseOrders}/${receipt.purchaseOrderId}`}
                  className="text-brand hover:underline"
                >
                  {receipt.orderNumber || `#${receipt.purchaseOrderId}`}
                </Link>
              </dd>
            </div>
            <div>
              <dt className="text-muted">Tanggal Terima</dt>
              <dd className="text-ink">{formatDate(receipt.receivedAt)}</dd>
            </div>
            <div>
              <dt className="text-muted">Supplier</dt>
              <dd className="text-ink">{po?.partyName || "-"}</dd>
            </div>
            {receipt.notes.trim() !== "" && (
              <div className="col-span-2 md:col-span-4">
                <dt className="text-muted">Catatan</dt>
                <dd className="text-ink">{receipt.notes}</dd>
              </div>
            )}
          </dl>
        </SectionCard>

        <SectionCard
          title={`Barang (${receipt.items.length})`}
          description="Qty dipesan dari PO (best-effort) vs qty yang diterima baris ini."
        >
          <DataTable<GoodsReceiptItem>
            rows={receipt.items}
            rowKey={(r) => r.id}
            emptyTitle="Tidak ada baris barang"
            columns={[
              {
                header: "Produk",
                render: (r) => {
                  const match = poLineOf(r);
                  return match ? (
                    <span>
                      <span className="font-medium">{match.productName}</span>{" "}
                      <span className="text-muted text-xs">
                        {match.productCode}
                      </span>
                    </span>
                  ) : (
                    <span className="text-muted">
                      #{r.productId}/{r.variantId}
                    </span>
                  );
                },
              },
              { header: "Satuan", render: (r) => r.uom },
              {
                header: "Qty Dipesan (PO)",
                align: "right",
                render: (r) => {
                  const match = poLineOf(r);
                  return match ? formatNumber(match.qty) : "-";
                },
              },
              {
                header: "Qty Diterima",
                align: "right",
                render: (r) => (
                  <span className="font-medium">{formatNumber(r.qty)}</span>
                ),
              },
              {
                header: "Lokasi",
                render: (r) => `#${r.locationId}`,
              },
            ]}
          />
        </SectionCard>
      </div>
    </div>
  );
}
