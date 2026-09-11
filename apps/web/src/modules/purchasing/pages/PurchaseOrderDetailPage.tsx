import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  PageHeader,
  SectionCard,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  formatIDR,
  formatNumber,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { purchaseOrderService } from "@/modules/purchasing/services/purchase-order.service";
import type {
  PurchaseOrder,
  PurchaseOrderItem,
} from "@/modules/purchasing/types";

// Alur status: draft → confirmed → completed (via penerimaan) / cancelled.
// Tahap "completed" terjadi di modul penerimaan, bukan di sini — backend
// orderView TIDAK memiliki receivedQty, jadi progres dihitung dari status.
const FLOW = ["draft", "confirmed", "completed"] as const;

function FlowSteps({ status }: { status: string }) {
  if (status === "cancelled") {
    return (
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <Badge tone="danger">{statusLabel("cancelled")}</Badge>
        <span className="text-muted">— order dibatalkan, alur berhenti.</span>
      </div>
    );
  }
  const current = FLOW.indexOf(status as (typeof FLOW)[number]);
  return (
    <ol className="flex flex-wrap items-center gap-2 text-sm">
      {FLOW.map((s, i) => {
        const done = current < 0 ? false : i <= current;
        return (
          <li key={s} className="flex items-center gap-2">
            {i > 0 && <span className="text-muted">→</span>}
            <Badge tone={done ? statusTone(s) : "neutral"}>{statusLabel(s)}</Badge>
          </li>
        );
      })}
      {status === "completed" && (
        <span className="text-muted">(selesai via penerimaan barang)</span>
      )}
    </ol>
  );
}

function discountText(it: PurchaseOrderItem): string {
  const parts: string[] = [];
  if (it.discountPct > 0) parts.push(`${it.discountPct}%`);
  if (it.discountNominal > 0) parts.push(formatIDR(it.discountNominal));
  return parts.length > 0 ? parts.join(" + ") : "-";
}

export default function PurchaseOrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const orderId = Number(id);
  const canUpdate = can("purchasing.update");
  const canCancel = can("purchasing.archive");

  const [order, setOrder] = useState<PurchaseOrder | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [acting, setActing] = useState(false);

  const validId = Number.isInteger(orderId) && orderId > 0;
  const { data: fetchedOrder, loading, error: fetchError, reload } = useAsyncData(
    async () => {
      if (!validId) return null;
      try {
        return await purchaseOrderService.get(orderId);
      } catch (err) {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat order beli", apiErr.message);
        throw err;
      }
    },
    [orderId],
  );
  const error = !validId ? "ID order beli tidak valid." : fetchError;

  // Hasil fetch disalin ke state tampilan; aksi konfirmasi/batal menimpa
  // state yang sama tanpa fetch ulang.
  const [appliedFetch, setAppliedFetch] = useState<PurchaseOrder | null>(null);
  if (fetchedOrder !== appliedFetch) {
    setAppliedFetch(fetchedOrder);
    setOrder(fetchedOrder);
  }

  async function handleConfirm() {
    setConfirmOpen(false);
    setActing(true);
    try {
      const res = await purchaseOrderService.confirm(orderId);
      setOrder(res.data);
      toast.fromServer(res.message, "Purchase order dikonfirmasi", res.data.number);
    } catch (err) {
      toast.danger("Gagal mengonfirmasi", toApiError(err).message);
    } finally {
      setActing(false);
    }
  }

  async function handleCancel() {
    setCancelOpen(false);
    setActing(true);
    try {
      const res = await purchaseOrderService.cancel(orderId);
      setOrder(res.data);
      toast.fromServer(res.message, "Purchase order dibatalkan", res.data.number);
    } catch (err) {
      toast.danger("Gagal membatalkan", toApiError(err).message);
    } finally {
      setActing(false);
    }
  }

  if (loading && !order) {
    return (
      <div>
        <PageHeader eyebrow="Pembelian" title="Detail Order Beli" />
        <p className="text-muted py-8 text-center text-sm">Memuat…</p>
      </div>
    );
  }

  if (error || !order) {
    return (
      <div>
        <PageHeader eyebrow="Pembelian" title="Detail Order Beli" />
        <Notice tone="danger" title={error ?? "Order tidak ditemukan"} />
        <div className="mt-4 flex gap-3">
          <Button variant="secondary" onClick={() => void navigate(ROUTE_PATHS.purchaseOrders)}>
            Kembali ke Daftar
          </Button>
          {error && <Button onClick={() => reload()}>Coba lagi</Button>}
        </div>
      </div>
    );
  }

  const isDraft = order.status === "draft";
  const isConfirmed = order.status === "confirmed";
  const canEdit = isDraft && canUpdate;

  return (
    <div>
      <PageHeader
        eyebrow="Pembelian"
        title={order.number}
        description={`${formatDate(order.orderDate)} · ${order.partyName}`}
        actions={
          <>
            <Button variant="secondary" onClick={() => void navigate(ROUTE_PATHS.purchaseOrders)}>
              Daftar
            </Button>
            {canEdit && (
              <Button
                onClick={() =>
                  void navigate(`${ROUTE_PATHS.purchaseOrders}/${order.id}/edit`)
                }
              >
                Ubah
              </Button>
            )}
          </>
        }
      />

      <div className="space-y-4">
        <SectionCard title="Ringkasan" description="Header order dan alur status.">
          <div className="mb-4">
            <FlowSteps status={order.status} />
          </div>
          <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm md:grid-cols-4">
            <div>
              <dt className="text-muted">Nomor</dt>
              <dd className="font-semibold text-ink">{order.number}</dd>
            </div>
            <div>
              <dt className="text-muted">Status</dt>
              <dd>
                <Badge tone={statusTone(order.status)}>{statusLabel(order.status)}</Badge>
              </dd>
            </div>
            <div>
              <dt className="text-muted">Tanggal Order</dt>
              <dd className="text-ink">{formatDate(order.orderDate)}</dd>
            </div>
            <div>
              <dt className="text-muted">Jatuh Tempo</dt>
              <dd className="text-ink">{formatDate(order.dueDate)}</dd>
            </div>
            <div className="col-span-2">
              <dt className="text-muted">Supplier</dt>
              <dd className="text-ink">{order.partyName || "-"}</dd>
            </div>
            <div>
              <dt className="text-muted">Termin</dt>
              <dd className="text-ink">{order.paymentTerms === "cod" ? "COD" : "Net"}</dd>
            </div>
            <div>
              <dt className="text-muted">Pajak</dt>
              <dd className="text-ink">
                {order.taxType}
                {order.taxType !== "none" ? ` (${order.taxRate}%)` : ""}
              </dd>
            </div>
            {order.notes.trim() !== "" && (
              <div className="col-span-2 md:col-span-4">
                <dt className="text-muted">Catatan</dt>
                <dd className="text-ink">{order.notes}</dd>
              </div>
            )}
          </dl>
          <dl className="mt-4 max-w-md space-y-2 border-t border-hairline pt-4 text-sm">
            <div className="flex justify-between">
              <dt className="text-muted">Subtotal</dt>
              <dd className="text-ink">{formatIDR(order.subtotal)}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-muted">Total Diskon</dt>
              <dd className="text-ink">− {formatIDR(order.discountTotal)}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-muted">Total Pajak</dt>
              <dd className="text-ink">{formatIDR(order.taxTotal)}</dd>
            </div>
            <div className="flex justify-between text-base">
              <dt className="font-semibold text-ink">Grand Total</dt>
              <dd className="font-bold text-ink">{formatIDR(order.grandTotal)}</dd>
            </div>
          </dl>
        </SectionCard>

        <SectionCard
          title={`Barang (${order.items.length})`}
          description="Harga snapshot server — perubahan master produk tidak mengubah riwayat."
        >
          <DataTable<PurchaseOrderItem>
            rows={order.items}
            rowKey={(r) => r.id}
            emptyTitle="Tidak ada baris barang"
            columns={[
              {
                header: "Produk",
                render: (r) => (
                  <span>
                    <span className="font-medium">{r.productName}</span>{" "}
                    <span className="text-muted text-xs">{r.productCode}</span>
                  </span>
                ),
              },
              { header: "Satuan", render: (r) => r.uom },
              {
                header: "Qty",
                align: "right",
                render: (r) => formatNumber(r.qty),
              },
              {
                header: "Qty Dasar",
                align: "right",
                render: (r) => formatNumber(r.qtyBase),
              },
              {
                header: "Harga",
                align: "right",
                render: (r) => formatIDR(r.unitPrice),
              },
              { header: "Diskon", render: (r) => discountText(r) },
              {
                header: "Subtotal",
                align: "right",
                render: (r) => (
                  <span className="font-medium">{formatIDR(r.lineTotal)}</span>
                ),
              },
            ]}
          />
        </SectionCard>

        {(isDraft || isConfirmed) && (canUpdate || canCancel) && (
          <SectionCard title="Aksi" description="Transisi status order.">
            <div className="flex flex-wrap gap-3">
              {isDraft && canUpdate && (
                <Button onClick={() => setConfirmOpen(true)} disabled={acting}>
                  Konfirmasi
                </Button>
              )}
              {(isDraft || isConfirmed) && canCancel && (
                <Button
                  variant="danger"
                  onClick={() => setCancelOpen(true)}
                  disabled={acting}
                >
                  Batalkan
                </Button>
              )}
            </div>
            {!isDraft && (
              <p className="text-muted mt-3 text-sm">
                Order {statusLabel(order.status).toLowerCase()} — ubah & konfirmasi hanya untuk
                draf.
              </p>
            )}
          </SectionCard>
        )}
      </div>

      <ConfirmDialog
        open={confirmOpen}
        title="Konfirmasi Order"
        message={`Konfirmasi order ${order.number}? Order terkonfirmasi tidak bisa diubah lagi.`}
        confirmLabel="Konfirmasi"
        tone="primary"
        onConfirm={() => void handleConfirm()}
        onCancel={() => setConfirmOpen(false)}
      />
      <ConfirmDialog
        open={cancelOpen}
        title="Batalkan Order"
        message={`Batalkan order ${order.number}? Tindakan ini tidak bisa dibatalkan.`}
        confirmLabel="Batalkan Order"
        tone="danger"
        onConfirm={() => void handleCancel()}
        onCancel={() => setCancelOpen(false)}
      />
    </div>
  );
}
