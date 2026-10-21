import { useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import {
  Button,
  ConfirmDialog,
  DataTable,
  DetailSkeleton,
  ErrorState,
  Modal,
  PageHeader,
  SectionCard,
  StatusBadge,
} from "@/shared/components/ui";
import {
  ActionMenu,
  DescriptionList,
  DocNumber,
  DocumentRail,
  Page,
  PageColumns,
  Qty,
} from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { formatDate, todayWIB } from "@/shared/lib/format";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { SettlementFields } from "@/modules/goodsreceipt/components/SettlementFields";
import { ReceiptPaymentProof } from "@/modules/goodsreceipt/components/ReceiptPaymentProof";
import {
  emptySettlementDraft,
  resolveSettlement,
  settlementFieldErrors,
} from "@/modules/goodsreceipt/lib/settlement-draft";
import type { SettlementDraft } from "@/modules/goodsreceipt/lib/settlement-draft";
import { goodsReceiptRail } from "@/modules/goodsreceipt/lib/goods-receipt-rail";
import { formatApiError, toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { goodsreceiptService } from "@/modules/goodsreceipt/services/goodsreceipt.service";
import type { GoodsReceipt, GoodsReceiptItem, PurchaseOrderOption, ReceiptPayment } from "@/modules/goodsreceipt/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

// Status: pending (tersimpan, stok belum masuk) → posted, atau void. Aksi pada
// receipt pending (izin goodsreceipt.create): "Posting ulang" =
// POST :id/complete (idempoten), "Batalkan" = POST :id/discard (stok yang
// sudah bergerak dibalik server).
// Dipesan vs diterima: receiptView TIDAK membawa qty pesanan maupun nama
// produk — keduanya dijoin best-effort dari detail PO via productId/variantId.

export default function GoodsReceiptDetailPage() {
  const { id } = useParams<{ id: string }>();

  const receiptId = Number(id);
  const validId = Number.isInteger(receiptId) && receiptId > 0;
  const can = useAuthStore((s) => s.can);
  const canAct = can("goodsreceipt.create");
  const canPay = can("payment.create");
  const canSwitchNet = can("purchasing.update");

  const location = useLocation();
  const [proofPayment, setProofPayment] = useState<ReceiptPayment | null>(
    (location.state as { payment?: ReceiptPayment } | null)?.payment ?? null,
  );
  const [acting, setActing] = useState(false);
  const [completeOpen, setCompleteOpen] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(false);
  const [settlement, setSettlement] = useState<SettlementDraft>(() => emptySettlementDraft(todayWIB()));
  const [settlementErrors, setSettlementErrors] = useState<Record<string, string>>({});
  const [actionError, setActionError] = useState<string | null>(null);

  const { data, loading, error, errorStatus, reload } = useAsyncData(
    async () => {
      if (!validId) return null;
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
  usePageCrumb(receipt ? receipt.number || `#${receipt.id}` : undefined);

  function openComplete() {
    setActionError(null);
    setSettlementErrors({});
    setSettlement({
      ...emptySettlementDraft(todayWIB()),
      mode: po?.paymentTerms === "cod" ? (canPay ? "pay_now" : canSwitchNet ? "switch_to_net" : "") : "",
    });
    setCompleteOpen(true);
  }

  async function handleComplete() {
    const settle =
      po?.paymentTerms === "cod"
        ? resolveSettlement(settlement)
        : ({ ok: true, payload: undefined } as const);
    if (!settle.ok) {
      setSettlementErrors(settle.errors);
      setActionError("Periksa isian pelunasan.");
      return;
    }
    setActionError(null);
    setSettlementErrors({});
    setActing(true);
    try {
      const res = await goodsreceiptService.complete(receiptId, settle.payload);
      setCompleteOpen(false);
      toast.fromServer(res.message, "Penerimaan diposting", res.data.number);
      if (res.data.payment) setProofPayment(res.data.payment);
      reload();
    } catch (err) {
      // Dialog tetap terbuka dengan pesan server (mis. stok masih gagal/izin).
      const apiErr = toApiError(err);
      setActionError(apiErr.message);
      setSettlementErrors(settlementFieldErrors(apiErr.errors));
      toast.danger("Gagal memposting penerimaan", apiErr.message);
    } finally {
      setActing(false);
    }
  }

  async function handleDiscard() {
    setActing(true);
    try {
      const res = await goodsreceiptService.discard(receiptId);
      setDiscardOpen(false);
      toast.fromServer(res.message, "Penerimaan dibatalkan", res.data.number);
      reload();
    } catch (err) {
      setDiscardOpen(false);
      toast.danger("Gagal membatalkan penerimaan", formatApiError(err));
    } finally {
      setActing(false);
    }
  }

  if (!validId || (!loading && errorStatus === 404)) {
    return (
      <NotFoundState
        title="Penerimaan tidak ditemukan"
        message="Penerimaan ini tidak ada atau bukan milik cabang aktif."
        backTo={ROUTE_PATHS.goodsReceipts}
        backLabel="Kembali ke daftar penerimaan"
      />
    );
  }

  if (loading && !receipt) {
    return (
      <Page>
        <DetailSkeleton />
      </Page>
    );
  }

  if (error || !receipt) {
    return (
      <Page>
        <PageHeader title="Penerimaan barang" />
        <ErrorState message={error ?? "Penerimaan tidak ditemukan"} onRetry={error ? reload : undefined} />
      </Page>
    );
  }

  const poLineOf = (it: GoodsReceiptItem) =>
    po?.items.find((l) => l.productId === it.productId && l.variantId === it.variantId) ?? null;

  const number = receipt.number || `#${receipt.id}`;
  const pending = receipt.status === "pending";
  const rail = goodsReceiptRail(
    receipt.status,
    receipt.orderNumber
      ? { id: receipt.purchaseOrderId, number: receipt.orderNumber, status: po?.status }
      : null,
    ROUTE_PATHS.purchaseOrders,
  );

  return (
    <Page>
      <PageHeader
        title={number}
        meta={<StatusBadge status={receipt.status} />}
        subtitle={`${receipt.orderNumber || "-"} · ${formatDate(receipt.receivedAt)}`}
        actions={
          pending && canAct ? (
            <>
              <ActionMenu
                items={[
                  {
                    key: "discard",
                    label: "Batalkan penerimaan",
                    destructive: true,
                    disabled: acting,
                    onSelect: () => setDiscardOpen(true),
                  },
                ]}
              />
              <Button onClick={openComplete} disabled={acting}>
                Posting ulang
              </Button>
            </>
          ) : undefined
        }
      >
        <DocumentRail steps={rail.steps} current={rail.current} stopped={rail.stopped} related={rail.related} />
      </PageHeader>

      {pending && (
        <Notice tone="warning" title="Penerimaan menunggu posting">
          Penerimaan ini tersimpan tetapi stok belum masuk. Posting ulang memasukkan stok (aman diulang);
          Batalkan membalik stok yang sudah bergerak.
        </Notice>
      )}

      <PageColumns
        main={
          <SectionCard title={`Barang (${receipt.items.length})`} flush>
            <DataTable<GoodsReceiptItem>
              rows={receipt.items}
              rowKey={(r) => r.id}
              emptyTitle="Tidak ada baris barang"
              stickyHeader={false}
              columns={[
                {
                  header: "Produk",
                  render: (r) => {
                    const match = poLineOf(r);
                    return match ? (
                      <div>
                        <span className="font-medium">{match.productName}</span>
                        <p className="text-xs text-muted-foreground">{match.productCode}</p>
                      </div>
                    ) : (
                      <span className="text-muted-foreground">
                        #{r.productId}/{r.variantId}
                      </span>
                    );
                  },
                },
                {
                  header: "Dipesan (PO)",
                  align: "right",
                  render: (r) => {
                    const match = poLineOf(r);
                    return match ? <Qty value={match.qty} unit={match.uom} /> : "-";
                  },
                },
                {
                  header: "Diterima",
                  align: "right",
                  render: (r) => <Qty value={r.qty} unit={r.uom} className="font-medium" />,
                },
                { header: "Lokasi", render: (r) => (r.locationId > 0 ? `#${r.locationId}` : "-") },
              ]}
            />
          </SectionCard>
        }
        aside={
          <>
          <SectionCard title="Info dokumen">
            <DescriptionList
              columns={1}
              items={[
                { label: "Nomor", value: <DocNumber value={number} copyable /> },
                {
                  label: "Order beli",
                  value: (
                    <Link
                      to={`${ROUTE_PATHS.purchaseOrders}/${receipt.purchaseOrderId}`}
                      className="text-primary hover:underline"
                    >
                      <DocNumber value={receipt.orderNumber || `#${receipt.purchaseOrderId}`} />
                    </Link>
                  ),
                },
                { label: "Pemasok", value: po?.partyName || "" },
                { label: "Tanggal terima", value: formatDate(receipt.receivedAt) },
                { label: "SJ pemasok", value: receipt.supplierDeliveryNumber },
                { label: "Diterima oleh", value: receipt.receivedByName },
                { label: "Catatan", value: receipt.notes.trim(), wide: true },
              ]}
            />
          </SectionCard>
          {proofPayment && canPay ? <ReceiptPaymentProof payment={proofPayment} /> : null}
          </>
        }
      />

      <Modal
        open={completeOpen}
        title="Posting ulang penerimaan"
        description={`Posting ${number} ke stok.`}
        onClose={() => {
          if (!acting) setCompleteOpen(false);
        }}
        actions={
          <>
            <Button variant="secondary" disabled={acting} onClick={() => setCompleteOpen(false)}>
              Batal
            </Button>
            <Button onClick={() => void handleComplete()} loading={acting}>
              Posting
            </Button>
          </>
        }
      >
        {actionError && <Notice tone="danger" title={actionError} className="mb-4" />}
        {po?.paymentTerms === "cod" ? (
          <SettlementFields
            value={settlement}
            errors={settlementErrors}
            canPay={canPay}
            canSwitchNet={canSwitchNet}
            onChange={setSettlement}
          />
        ) : (
          <p className="text-sm text-muted-foreground">Tidak ada isian tambahan untuk posting ulang.</p>
        )}
      </Modal>
      <ConfirmDialog
        open={discardOpen}
        title="Batalkan penerimaan"
        message={`Batalkan ${number}? Stok yang sudah bergerak akan dibalik.`}
        confirmLabel="Batalkan penerimaan"
        tone="danger"
        onConfirm={() => void handleDiscard()}
        onCancel={() => setDiscardOpen(false)}
      />
    </Page>
  );
}
