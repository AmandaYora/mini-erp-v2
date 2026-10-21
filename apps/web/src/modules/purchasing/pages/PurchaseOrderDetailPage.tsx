import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  DetailSkeleton,
  ErrorState,
  PageHeader,
  SectionCard,
  StatusBadge,
} from "@/shared/components/ui";
import {
  ActionMenu,
  DescriptionList,
  DocNumber,
  DocumentRail,
  Money,
  Page,
  PageColumns,
  Qty,
  type ActionMenuItem,
} from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { formatDate, formatIDR } from "@/shared/lib/format";
import { formatApiError } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { purchaseOrderService } from "@/modules/purchasing/services/purchase-order.service";
import { purchaseOrderRail } from "@/modules/purchasing/lib/purchase-order-rail";
import { purchaseTermsLabel } from "@/modules/purchasing/types";
import type { PurchaseOrder, PurchaseOrderItem } from "@/modules/purchasing/types";
import { OrderBillingCard } from "@/modules/payment/components/OrderBillingCard";
import { OrderHistoryCard } from "@/modules/payment/components/OrderHistoryCard";
import { OrderPaymentModal } from "@/modules/payment/components/OrderPaymentModal";
import { useOrderHistory } from "@/modules/payment/hooks/useOrderHistory";
import {
  billingBadge,
  billingView,
  canCreateReceipt,
  canCreateReturn,
  canRecordPayment,
  isPrepaidBlocked,
} from "@/modules/payment/lib/order-billing";
import { hasFulfilledDocument, lineKey } from "@/modules/payment/lib/order-history";
import { ReasonDialog } from "@/modules/purchasing/components/ReasonDialog";
import { HeaderEditSheet } from "@/modules/purchasing/components/HeaderEditSheet";
import { ShortCloseSheet } from "@/modules/purchasing/components/ShortCloseSheet";
import { PriceCorrectionSheet } from "@/modules/purchasing/components/PriceCorrectionSheet";
import { CorrectionHistoryCard } from "@/modules/purchasing/components/CorrectionHistoryCard";
import type { PurchaseHeaderValues, PriceCorrectionLineValues } from "@/modules/purchasing/schemas/purchase-correction.schema";

/** Info turunan baris barang (diskon, pajak, catatan) untuk baris kedua redup. */
function lineSubText(it: PurchaseOrderItem): string[] {
  const parts: string[] = [];
  const disc: string[] = [];
  if (it.discountPct > 0) disc.push(`${it.discountPct}%`);
  if (it.discountNominal > 0) disc.push(formatIDR(it.discountNominal));
  if (disc.length > 0) {
    parts.push(`Diskon ${disc.join(" + ")}${it.discountReason ? ` (${it.discountReason})` : ""}`);
  }
  parts.push(
    it.taxable
      ? `Pajak${it.taxRate > 0 ? ` ${it.taxRate}%` : ""} ${formatIDR(it.taxAmount)} · DPP ${formatIDR(it.taxBase)}`
      : "Tanpa pajak",
  );
  if (it.note) parts.push(`Catatan: ${it.note}`);
  return parts;
}

/** " (oleh Rani, 21 Okt 2026)" — siapa & kapan sebuah order dibatalkan/ditutup. */
function stampText(name: string | undefined, at: string | undefined): string {
  const parts = [name ? `oleh ${name}` : "", at ? formatDate(at) : ""].filter((p) => p !== "");
  return parts.length > 0 ? ` (${parts.join(", ")})` : "";
}

export default function PurchaseOrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const orderId = Number(id);
  const canUpdate = can("purchasing.update");
  const canCancel = can("purchasing.archive");
  const canReceive = can("goodsreceipt.create");
  const canPay = can("payment.create");
  const canReturn = can("purchasereturn.create");

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [paymentOpen, setPaymentOpen] = useState(false);
  // Panel koreksi yang terbuka (satu sekaligus) dan galat server-nya.
  const [sheet, setSheet] = useState<"header" | "shortClose" | "price" | null>(null);
  const [sheetBusy, setSheetBusy] = useState(false);
  const [sheetError, setSheetError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  // Naik tiap dokumen terkait berubah → riwayat dimuat ulang.
  const [historyVersion, setHistoryVersion] = useState(0);

  const validId = Number.isInteger(orderId) && orderId > 0;
  const {
    data: order,
    loading,
    error,
    errorStatus,
    reload,
  } = useAsyncData(
    async () => (validId ? purchaseOrderService.get(orderId) : null),
    [orderId],
  );
  usePageCrumb(order?.number);

  const history = useOrderHistory("purchase", orderId, {
    partyId: order?.partyId ?? 0,
    // Riwayat mencari semua alokasi aktif, termasuk offset retur.
    paidAmount: (order?.paidAmount ?? 0) + (order?.returnOffset ?? 0),
    enabled: order !== null && order.status !== "draft",
    version: historyVersion,
  });

  function refresh() {
    reload();
    setHistoryVersion((v) => v + 1);
  }

  // Respons konfirmasi/batal tidak membawa ringkasan pembayaran: muat ulang dari GET.
  async function runAction(
    fn: () => Promise<{ data: PurchaseOrder; message: string }>,
    fallback: string,
    fail: string,
  ) {
    setActing(true);
    setActionError(null);
    try {
      const res = await fn();
      toast.fromServer(res.message, fallback, res.data.number);
      refresh();
    } catch (err) {
      const msg = formatApiError(err);
      // Pesan penjaga server (penerimaan/pembayaran/retur aktif) tampil menetap.
      setActionError(`${fail}: ${msg}`);
      toast.danger(fail, msg);
    } finally {
      setActing(false);
    }
  }

  function handleConfirm() {
    setConfirmOpen(false);
    void runAction(
      () => purchaseOrderService.confirm(orderId),
      "Order dikonfirmasi",
      "Gagal mengonfirmasi",
    );
  }

  /** Aksi koreksi dari panel/dialog: galat server tampil di panel itu sendiri. */
  async function runSheet(
    fn: () => Promise<{ data: PurchaseOrder; message: string }>,
    fallback: string,
    close: () => void,
  ) {
    setSheetBusy(true);
    setSheetError(null);
    try {
      const res = await fn();
      toast.fromServer(res.message, fallback, res.data.number);
      close();
      refresh();
    } catch (err) {
      setSheetError(formatApiError(err));
    } finally {
      setSheetBusy(false);
    }
  }

  function openSheet(kind: "header" | "shortClose" | "price") {
    setSheetError(null);
    setSheet(kind);
  }

  function handleCancel(reason: string) {
    void runSheet(() => purchaseOrderService.cancel(orderId, reason), "Order dibatalkan", () => setCancelOpen(false));
  }

  function handleHeader(v: PurchaseHeaderValues) {
    void runSheet(
      () =>
        purchaseOrderService.updateHeader(orderId, {
          dueDate: v.dueDate || undefined,
          paymentTermDays: v.paymentTermDays,
          notes: v.notes,
          supplierInvoiceNumber: v.supplierInvoiceNumber,
          supplierInvoiceDate: v.supplierInvoiceDate || undefined,
          taxInvoiceNumber: v.taxInvoiceNumber,
          taxInvoiceDate: v.taxInvoiceDate || undefined,
          assignedUserId: v.assignedUserId,
          reason: v.reason || undefined,
        }),
      "Info dokumen disimpan",
      () => setSheet(null),
    );
  }

  function handleShortClose(reason: string) {
    void runSheet(() => purchaseOrderService.shortClose(orderId, reason), "Sisa order ditutup", () => setSheet(null));
  }

  function handlePriceCorrection(lines: PriceCorrectionLineValues[], reason: string) {
    void runSheet(
      () => purchaseOrderService.correctPrices(orderId, { lines, reason }),
      "Harga beli dikoreksi",
      () => setSheet(null),
    );
  }

  if (!validId || (!loading && errorStatus === 404)) {
    return (
      <NotFoundState
        title="Order beli tidak ditemukan"
        message="Order ini tidak ada atau bukan milik cabang aktif."
        backTo={ROUTE_PATHS.purchaseOrders}
        backLabel="Kembali ke daftar order beli"
      />
    );
  }

  if (loading && !order) {
    return (
      <Page>
        <DetailSkeleton />
      </Page>
    );
  }

  if (error || !order) {
    return (
      <Page>
        <PageHeader title="Order beli" />
        <ErrorState message={error ?? "Order tidak ditemukan"} onRetry={error ? reload : undefined} />
      </Page>
    );
  }

  const billing = billingView(order);
  const badge = billingBadge(billing);
  const isDraft = order.status === "draft";
  const isConfirmed = order.status === "confirmed";
  const isCompleted = order.status === "completed";
  // Order terkonfirmasi tanpa penerimaan/pembayaran/retur masih diubah lewat form penuh.
  const canEdit = canUpdate && (isDraft || (isConfirmed && order.hasDependents === false));
  const canEditHeader = canUpdate && ((isConfirmed && order.hasDependents === true) || isCompleted);
  const canCorrectPrice = canUpdate && ((isConfirmed && order.hasDependents === true) || isCompleted);
  const fulfilled = hasFulfilledDocument(history.data?.events ?? []);
  const showReceive = canReceive && canCreateReceipt(order);
  const showPay = canPay && canRecordPayment(order);
  const showReturn = canReturn && canCreateReturn(order, fulfilled);
  const prepaidBlocked = isConfirmed && isPrepaidBlocked(order);
  const progress = history.data?.progress ?? null;
  const hiddenNotes: string[] = [];
  if (history.data?.hidden.docs) hiddenNotes.push("Penerimaan disembunyikan (tanpa izin Penerimaan Barang)");
  if (history.data?.hidden.payments) hiddenNotes.push("Pembayaran disembunyikan (tanpa izin Pembayaran)");
  if (history.data?.hidden.returns) hiddenNotes.push("Retur disembunyikan (tanpa izin Retur Beli)");

  const rail = purchaseOrderRail(order.status, history.data?.events ?? []);

  const goReceive = () => void navigate(`${ROUTE_PATHS.goodsReceiptsNew}?from=${order.id}`);
  const goReturn = () => void navigate(`${ROUTE_PATHS.purchaseReturns}/new?from=${order.id}`);

  // Satu aksi utama menurut status; maksimal dua sekunder; sisanya di menu.
  const menuItems: ActionMenuItem[] = [];
  if (showReturn) menuItems.push({ key: "return", label: "Retur ke pemasok", onSelect: goReturn, disabled: acting });
  if (canEditHeader) {
    menuItems.push({ key: "header", label: "Ubah info dokumen", onSelect: () => openSheet("header"), disabled: acting });
  }
  if (canCorrectPrice) {
    menuItems.push({ key: "price", label: "Koreksi harga", onSelect: () => openSheet("price"), disabled: acting });
  }
  if (isConfirmed && canCancel && fulfilled) {
    menuItems.push({
      key: "short-close",
      label: "Tutup sisa order",
      onSelect: () => openSheet("shortClose"),
      destructive: true,
      disabled: acting,
    });
  }
  if ((isDraft || isConfirmed) && canCancel) {
    menuItems.push({
      key: "cancel",
      label: "Batalkan order",
      onSelect: () => {
        setSheetError(null);
        setCancelOpen(true);
      },
      destructive: true,
      disabled: acting,
    });
  }

  const payButton = showPay ? (
    <Button
      key="pay"
      variant={showReceive && isConfirmed ? "secondary" : "primary"}
      onClick={() => setPaymentOpen(true)}
      disabled={acting}
    >
      Bayar ke pemasok
    </Button>
  ) : null;

  const actions = (
    <>
      {canEdit && (
        <Button variant="secondary" onClick={() => void navigate(`${ROUTE_PATHS.purchaseOrders}/${order.id}/edit`)}>
          Ubah
        </Button>
      )}
      {isConfirmed && showReceive && payButton}
      <ActionMenu items={menuItems} />
      {isDraft && canUpdate && (
        <Button onClick={() => setConfirmOpen(true)} disabled={acting}>
          Konfirmasi order
        </Button>
      )}
      {isConfirmed && showReceive && (
        <Button onClick={goReceive} disabled={acting}>
          Terima barang
        </Button>
      )}
      {!(isConfirmed && showReceive) && payButton}
    </>
  );

  return (
    <Page>
      <PageHeader
        title={order.number}
        meta={
          <>
            <StatusBadge status={order.status} />
            {billing.available && !isDraft && order.status !== "cancelled" && (
              <Badge tone={badge.tone}>{badge.label}</Badge>
            )}
          </>
        }
        subtitle={`${order.partyName || "-"} · ${formatDate(order.orderDate)}`}
        actions={actions}
      >
        <DocumentRail steps={rail.steps} current={rail.current} stopped={rail.stopped} related={rail.related} />
      </PageHeader>

      {actionError && (
        <Notice tone="danger" title="Aksi ditolak">
          {actionError}
        </Notice>
      )}
      {order.status === "cancelled" && order.cancelReason && (
        <Notice tone="neutral" title="Order dibatalkan">
          {order.cancelReason}
          {stampText(order.cancelledByName, order.cancelledAt)}
        </Notice>
      )}
      {order.shortCloseReason && (
        <Notice tone="neutral" title="Sisa order ditutup">
          {order.shortCloseReason}
          {stampText(order.shortClosedByName, order.shortClosedAt)}
        </Notice>
      )}
      {prepaidBlocked && (
        <Notice tone="info" title="Order prabayar">
          Penerimaan barang baru bisa dicatat setelah tagihan lunas.
        </Notice>
      )}

      <PageColumns
        main={
          <>
            <SectionCard title={`Barang (${order.items.length})`} flush>
              <DataTable<PurchaseOrderItem>
                rows={order.items}
                rowKey={(r) => r.id}
                emptyTitle="Tidak ada baris barang"
                stickyHeader={false}
                columns={[
                  {
                    header: "Produk",
                    render: (r) => (
                      <div>
                        <p>
                          <span className="font-medium">{r.productName}</span>{" "}
                          <span className="text-xs text-muted-foreground">
                            {r.productCode}
                            {r.variantName ? ` · ${r.variantName}` : ""}
                            {!r.stockTracked ? " · tanpa stok" : ""}
                          </span>
                        </p>
                        {lineSubText(r).map((t) => (
                          <p key={t} className="text-xs text-muted-foreground">
                            {t}
                          </p>
                        ))}
                      </div>
                    ),
                  },
                  {
                    header: "Qty",
                    align: "right",
                    render: (r) => (
                      <div>
                        <Qty value={r.qty} unit={r.uom} />
                        {r.uomFactor !== 1 && (
                          <p className="text-xs text-muted-foreground">
                            = <Qty value={r.qtyBase} unit={r.baseUom} />
                          </p>
                        )}
                      </div>
                    ),
                  },
                  ...(progress !== null
                    ? [
                        {
                          header: "Diterima",
                          align: "right" as const,
                          render: (r: PurchaseOrderItem) => {
                            const p = progress.get(lineKey(r));
                            return (
                              <div>
                                <p>
                                  <Qty value={p?.done ?? 0} /> / <Qty value={r.qty} />
                                </p>
                                {(p?.pending ?? 0) > 0 && (
                                  <p className="text-xs text-muted-foreground">
                                    +<Qty value={p?.pending ?? 0} /> menunggu
                                  </p>
                                )}
                              </div>
                            );
                          },
                        },
                      ]
                    : []),
                  { header: "Harga", align: "right", render: (r) => <Money value={r.unitPrice} /> },
                  {
                    header: "Subtotal",
                    align: "right",
                    render: (r) => <Money value={r.lineTotal} className="font-medium" />,
                  },
                ]}
              />
            </SectionCard>

            <CorrectionHistoryCard corrections={order.corrections ?? []} />

            {!isDraft && (
              <OrderHistoryCard
                events={history.data?.events ?? []}
                loading={history.loading}
                warnings={history.data?.warnings ?? (history.error ? [history.error] : [])}
                hiddenNotes={hiddenNotes}
              />
            )}
          </>
        }
        aside={
          <>
            <OrderBillingCard
              title="Ringkasan tagihan"
              billing={billing}
              subtotal={order.subtotal}
              discountTotal={order.discountTotal}
              taxTotal={order.taxTotal}
              grandTotal={order.grandTotal}
            />
            <SectionCard title="Info dokumen">
              <DescriptionList
                columns={1}
                items={[
                  { label: "Nomor", value: <DocNumber value={order.number} copyable /> },
                  { label: "Pemasok", value: order.partyName || "-" },
                  { label: "Tanggal order", value: formatDate(order.orderDate) },
                  {
                    label: "Jatuh tempo",
                    value: (
                      <span className={billing.overdue ? "font-semibold text-destructive" : undefined}>
                        {formatDate(billing.dueDate !== "" ? billing.dueDate : order.dueDate)}
                        {billing.overdue ? " (terlewat)" : ""}
                      </span>
                    ),
                  },
                  {
                    label: "Termin",
                    value: `${purchaseTermsLabel(order.paymentTerms)}${
                      order.paymentTermDays > 0 ? ` · ${order.paymentTermDays} hari` : ""
                    }`,
                  },
                  {
                    label: "Faktur pemasok",
                    value:
                      order.supplierInvoiceNumber.trim() !== ""
                        ? `${order.supplierInvoiceNumber}${
                            order.supplierInvoiceDate ? ` · ${formatDate(order.supplierInvoiceDate)}` : ""
                          }`
                        : "",
                  },
                  {
                    label: "Faktur pajak",
                    value:
                      order.taxInvoiceNumber.trim() !== ""
                        ? `${order.taxInvoiceNumber}${
                            order.taxInvoiceDate ? ` · ${formatDate(order.taxInvoiceDate)}` : ""
                          }`
                        : "",
                  },
                  { label: "Penanggung jawab", value: order.assignedUserName },
                  { label: "Dibuat oleh", value: order.createdByName },
                  {
                    label: "Pajak",
                    value: `${order.taxType}${order.taxType !== "none" ? ` (${order.taxRate}%)` : ""}`,
                  },
                  { label: "Catatan", value: order.notes.trim(), wide: true },
                ]}
              />
            </SectionCard>
          </>
        }
      />

      <ConfirmDialog
        open={confirmOpen}
        title="Konfirmasi order"
        message={`Konfirmasi order ${order.number}? Order terkonfirmasi tidak bisa diubah lagi.`}
        confirmLabel="Konfirmasi"
        tone="primary"
        onConfirm={() => void handleConfirm()}
        onCancel={() => setConfirmOpen(false)}
      />
      <ReasonDialog
        key={cancelOpen ? "cancel-open" : "cancel-closed"}
        open={cancelOpen}
        title="Batalkan order"
        message={`Batalkan order ${order.number}? Order yang sudah punya penerimaan terposting tidak bisa dibatalkan: kembalikan barang lewat retur, atau tutup sisa order.`}
        confirmLabel="Batalkan order"
        placeholder="Mis. order dobel, pemasok batal kirim"
        submitting={sheetBusy}
        serverError={sheetError}
        onClose={() => setCancelOpen(false)}
        onConfirm={handleCancel}
      />
      {sheet === "header" && (
        <HeaderEditSheet
          open
          order={order}
          submitting={sheetBusy}
          serverError={sheetError}
          onClose={() => setSheet(null)}
          onSubmit={handleHeader}
        />
      )}
      {sheet === "shortClose" && (
        <ShortCloseSheet
          open
          orderId={order.id}
          orderNumber={order.number}
          submitting={sheetBusy}
          serverError={sheetError}
          onClose={() => setSheet(null)}
          onConfirm={handleShortClose}
        />
      )}
      {sheet === "price" && (
        <PriceCorrectionSheet
          open
          order={order}
          submitting={sheetBusy}
          serverError={sheetError}
          onClose={() => setSheet(null)}
          onConfirm={handlePriceCorrection}
        />
      )}

      <OrderPaymentModal
        open={paymentOpen}
        onClose={() => setPaymentOpen(false)}
        onPaid={() => {
          setPaymentOpen(false);
          refresh();
        }}
        orderType="purchase"
        orderId={order.id}
        orderNumber={order.number}
        partyId={order.partyId > 0 ? order.partyId : 0}
        outstanding={billing.outstanding}
      />
    </Page>
  );
}
