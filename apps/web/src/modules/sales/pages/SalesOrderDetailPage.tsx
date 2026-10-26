import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import {
  Badge,
  Button,
  DataTable,
  DateInput,
  DetailSkeleton,
  ErrorState,
  FormField,
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
  Money,
  Page,
  PageColumns,
  Qty,
  type ActionMenuItem,
} from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { formatDate, formatDateTime, formatIDR, formatNumber, todayWIB } from "@/shared/lib/format";
import { formatApiError, toApiError } from "@/shared/services/http-client";
import type { MutateResult } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS, posReceiptPrintPath } from "@/app/routes/route-paths";
import { salesService } from "@/modules/sales/services/sales.service";
import { salesOrderRail } from "@/modules/sales/lib/sales-order-rail";
import { editAction, headerEditPayload } from "@/modules/sales/lib/order-correction";
import type { HeaderEditValues } from "@/modules/sales/schemas/order-correction.schema";
import { CancelOrderModal } from "@/modules/sales/components/CancelOrderModal";
import { HeaderEditModal } from "@/modules/sales/components/HeaderEditModal";
import { ReceiptAccountModal } from "@/modules/sales/components/ReceiptAccountModal";
import { useBankAccounts } from "@/modules/finance/hooks/use-bank-accounts";
import { bankAccountByAccountId, bankAccountLabel } from "@/modules/finance/lib/bank-accounts";
import { ShortCloseModal } from "@/modules/sales/components/ShortCloseModal";
import { channelLabel, originLabel, paymentTermsLabel, pricingSourceLabel } from "@/modules/sales/types";
import type { SalesOrder, SalesOrderItem } from "@/modules/sales/types";
import { OrderBillingCard } from "@/modules/payment/components/OrderBillingCard";
import { OrderHistoryCard } from "@/modules/payment/components/OrderHistoryCard";
import { OrderPaymentModal } from "@/modules/payment/components/OrderPaymentModal";
import { useOrderHistory } from "@/modules/payment/hooks/useOrderHistory";
import {
  billingBadge,
  billingView,
  canCreateDelivery,
  canCreateReturn,
  canRecordPayment,
  canSwitchToNet,
  isPrepaidBlocked,
  printButtonLabel,
} from "@/modules/payment/lib/order-billing";
import { hasFulfilledDocument, lineKey } from "@/modules/payment/lib/order-history";

export default function SalesOrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const can = useAuthStore((s) => s.can);
  // Baru saja dibuat dari surat jalan (W-06): tawarkan langkah berikutnya sekali.
  const justFromDeliveries = (location.state as { fromDeliveries?: boolean } | null)?.fromDeliveries === true;

  const orderId = Number(id);
  const invalidId = !Number.isInteger(orderId) || orderId <= 0;

  const canEdit = can("sales.update");
  const canConfirm = can("sales.update");
  const canCancel = can("sales.archive");
  const canApprove = can("sales.approve");
  const canDeliver = can("delivery.create");
  const canPay = can("payment.create");
  const canReturn = can("salesreturn.create");
  const bank = useBankAccounts();
  const [receiptOpen, setReceiptOpen] = useState(false);
  const [receiptError, setReceiptError] = useState<string | null>(null);

  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [creditOpen, setCreditOpen] = useState(false);
  const [creditDue, setCreditDue] = useState(todayWIB());
  const [creditError, setCreditError] = useState<string | null>(null);
  const [paymentOpen, setPaymentOpen] = useState(false);
  const [headerOpen, setHeaderOpen] = useState(false);
  const [headerError, setHeaderError] = useState<string | null>(null);
  const [shortCloseOpen, setShortCloseOpen] = useState(false);
  // Naik tiap dokumen terkait berubah → riwayat dimuat ulang.
  const [historyVersion, setHistoryVersion] = useState(0);

  const {
    data: order,
    loading,
    error: fetchError,
    errorStatus,
    reload,
  } = useAsyncData(
    () => (invalidId ? Promise.resolve(null) : salesService.get(orderId)),
    [orderId],
  );

  usePageCrumb(order?.number);

  const history = useOrderHistory("sales", orderId, {
    partyId: order?.partyId ?? 0,
    paidAmount: order?.paidAmount ?? 0,
    enabled: order !== null && order.status !== "draft",
    version: historyVersion,
  });

  function refresh() {
    reload();
    setHistoryVersion((v) => v + 1);
  }

  // Respons mutasi (confirm/cancel/approve-credit) tidak membawa ringkasan
  // pembayaran, jadi order dimuat ulang dari GET setelah aksi berhasil.
  async function runAction(fn: () => Promise<MutateResult<SalesOrder>>, fallback: string, fail: string) {
    setBusy(true);
    setActionError(null);
    try {
      const res = await fn();
      toast.fromServer(res.message, fallback, res.data.number);
      refresh();
    } catch (err) {
      const msg = formatApiError(err);
      // Pesan penjaga server (mis. ada surat jalan/pembayaran aktif) tampil
      // menetap di halaman, bukan hanya toast sesaat.
      setActionError(`${fail}: ${msg}`);
      toast.danger(fail, msg);
    } finally {
      setBusy(false);
    }
  }

  function handleConfirm() {
    void runAction(() => salesService.confirm(orderId), "Order dikonfirmasi", "Gagal mengonfirmasi");
  }

  function handleCancel(reason: string) {
    setCancelOpen(false);
    void runAction(() => salesService.cancel(orderId, reason), "Order dibatalkan", "Gagal membatalkan");
  }

  function handleShortClose(reason: string) {
    setShortCloseOpen(false);
    void runAction(() => salesService.closeShort(orderId, reason), "Sisa order ditutup", "Gagal menutup sisa order");
  }

  // Ubah info order: pesan server tampil di dalam dialog agar isian tidak hilang.
  async function handleHeaderEdit(o: SalesOrder, initial: HeaderEditValues, values: HeaderEditValues) {
    setBusy(true);
    setHeaderError(null);
    try {
      const res = await salesService.update(o.id, headerEditPayload(o, initial, values));
      toast.fromServer(res.message, "Info order disimpan", res.data.number);
      setHeaderOpen(false);
      refresh();
    } catch (err) {
      const msg = toApiError(err).message;
      setHeaderError(msg);
      toast.danger("Gagal menyimpan info order", msg);
    } finally {
      setBusy(false);
    }
  }

  async function handleReceiptAccount(o: SalesOrder, accountId: number) {
    setBusy(true);
    setReceiptError(null);
    try {
      const res = await salesService.setReceiptAccount(o.id, accountId);
      toast.fromServer(res.message, "Rekening tujuan disimpan", res.data.number);
      setReceiptOpen(false);
      refresh();
    } catch (err) {
      const msg = toApiError(err).message;
      setReceiptError(msg);
      toast.danger("Gagal menyimpan rekening tujuan", msg);
    } finally {
      setBusy(false);
    }
  }

  function handleApproveCredit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (creditDue.trim() === "") {
      setCreditError("Jatuh tempo wajib diisi.");
      return;
    }
    setCreditError(null);
    setCreditOpen(false);
    void runAction(
      () => salesService.approveCredit(orderId, creditDue.trim()),
      "Kredit disetujui",
      "Gagal menyetujui kredit",
    );
  }

  if (invalidId || (!loading && errorStatus === 404)) {
    return (
      <NotFoundState
        title="Order jual tidak ditemukan"
        message="Order ini tidak ada atau bukan milik cabang aktif."
        backTo={ROUTE_PATHS.salesOrders}
        backLabel="Kembali ke daftar order"
      />
    );
  }

  if (loading && !order) return <DetailSkeleton />;

  if (fetchError || !order) {
    return (
      <Page>
        <PageHeader title="Detail Order Jual" />
        <ErrorState title="Gagal memuat order" message={fetchError ?? "Data tidak ditemukan."} onRetry={reload} />
      </Page>
    );
  }

  const billing = billingView(order);
  const badge = billingBadge(billing);
  const isDraft = order.status === "draft";
  // Batal backend: draf atau terkonfirmasi (service.go CancelOrder).
  const cancellable = order.status === "draft" || order.status === "confirmed";
  const events = history.data?.events ?? [];
  const fulfilled = hasFulfilledDocument(events);
  const showDelivery = canDeliver && canCreateDelivery(order);
  const showPay = canPay && canRecordPayment(order);
  const showReturn = canReturn && canCreateReturn(order, fulfilled);
  const showCredit = canApprove && canSwitchToNet(order);
  const showConfirm = isDraft && canConfirm;
  // Draf dan order terkonfirmasi bisa dikoreksi; mode (form lengkap atau info
  // saja) ditentukan server dari dokumen turunan (editMode di GET detail).
  const edit = canEdit ? editAction(order) : null;
  const showEdit = edit !== null;
  const editLabel = isDraft ? "Ubah" : edit === "header" ? "Ubah info order" : "Ubah order";
  const editUrl = `${ROUTE_PATHS.salesOrders}/${order.id}/edit`;
  // Tutup sisa order: terkonfirmasi, sudah ada kiriman terkonfirmasi, tanpa draf SJ.
  const showShortClose = canCancel && order.status === "confirmed" && order.canCloseShort === true;
  const isPos = order.channel === "pos";
  const prepaidBlocked = order.status === "confirmed" && order.channel !== "pos" && isPrepaidBlocked(order);
  const progress = history.data?.progress ?? null;
  const hiddenNotes: string[] = [];
  if (history.data?.hidden.docs) hiddenNotes.push("Surat jalan disembunyikan (tanpa izin Surat Jalan)");
  if (history.data?.hidden.payments) hiddenNotes.push("Pembayaran disembunyikan (tanpa izin Pembayaran)");
  if (history.data?.hidden.returns) hiddenNotes.push("Retur disembunyikan (tanpa izin Retur Jual)");

  const fromDelivery = order.origin === "delivery";
  const rail = salesOrderRail({ status: order.status, channel: order.channel, origin: order.origin, billing, events });
  const customer = order.partyName || "Walk-in";
  const printUrl = `/print/sales/${order.id}`;
  const deliveryUrl = `${ROUTE_PATHS.deliveries}/new?from=${order.id}`;

  // Satu tombol primary: langkah berikut yang paling mungkin menurut status.
  const primary: "confirm" | "delivery" | "pay" | null = showConfirm
    ? "confirm"
    : showDelivery
      ? "delivery"
      : showPay
        ? "pay"
        : null;

  // Maksimal dua tombol sekunder terlihat; sisanya masuk menu "⋯".
  const secondary: ("print" | "pay" | "edit")[] = ["print"];
  if (showPay && primary !== "pay") secondary.push("pay");
  if (showEdit) secondary.push("edit");
  const visibleSecondary = secondary.slice(0, 2);

  const menuItems: ActionMenuItem[] = [];
  if (secondary.includes("pay") && !visibleSecondary.includes("pay")) {
    menuItems.push({ key: "pay", label: "Catat pembayaran", onSelect: () => setPaymentOpen(true), disabled: busy });
  }
  if (secondary.includes("edit") && !visibleSecondary.includes("edit")) {
    menuItems.push({
      key: "edit",
      label: editLabel,
      disabled: busy,
      onSelect: () => (edit === "header" ? openHeaderEdit() : navigate(editUrl)),
    });
  }
  if (showShortClose) {
    menuItems.push({
      key: "close-short",
      label: "Tutup sisa order",
      destructive: true,
      disabled: busy,
      onSelect: () => setShortCloseOpen(true),
    });
  }
  if (showReturn) {
    menuItems.push({
      key: "return",
      label: "Retur / tukar",
      onSelect: () => navigate(`${ROUTE_PATHS.salesReturns}/new?from=${order.id}`),
      disabled: busy,
    });
  }
  if (showCredit) {
    menuItems.push({
      key: "credit",
      label: "Setujui kredit (jadikan tempo)",
      disabled: busy,
      onSelect: () => {
        setCreditError(null);
        setCreditDue(todayWIB());
        setCreditOpen(true);
      },
    });
  }
  if (cancellable && canCancel) {
    menuItems.push({
      key: "cancel",
      label: "Batalkan order",
      destructive: true,
      disabled: busy,
      onSelect: () => setCancelOpen(true),
    });
  }

  const showBillingBadge = billing.available && !isDraft && order.status !== "cancelled";

  function openHeaderEdit() {
    setHeaderError(null);
    setHeaderOpen(true);
  }

  return (
    <Page>
      <PageHeader
        title={order.number}
        meta={
          <>
            <StatusBadge status={order.status} />
            {fromDelivery && <Badge tone="info">Dari SJ</Badge>}
            {showBillingBadge && <Badge tone={badge.tone}>{badge.label}</Badge>}
          </>
        }
        subtitle={`${customer} · ${formatDate(order.orderDate)} · ${channelLabel(order.channel)}`}
        actions={
          <>
            {visibleSecondary.includes("print") && (
              <Button variant="secondary" asChild>
                <Link to={printUrl} target="_blank" rel="noreferrer">
                  {printButtonLabel(billing)}
                </Link>
              </Button>
            )}
            {isPos && order.status !== "draft" && order.status !== "cancelled" && (
              <Button variant="secondary" asChild>
                <Link to={posReceiptPrintPath(order.id)} target="_blank" rel="noreferrer">
                  Cetak struk
                </Link>
              </Button>
            )}
            {visibleSecondary.includes("pay") && (
              <Button variant="secondary" onClick={() => setPaymentOpen(true)} disabled={busy}>
                Catat pembayaran
              </Button>
            )}
            {visibleSecondary.includes("edit") &&
              (edit === "header" ? (
                <Button variant="secondary" onClick={openHeaderEdit} disabled={busy}>
                  {editLabel}
                </Button>
              ) : (
                <Button variant="secondary" asChild>
                  <Link to={editUrl}>{editLabel}</Link>
                </Button>
              ))}
            <ActionMenu items={menuItems} />
            {primary === "confirm" && (
              <Button onClick={handleConfirm} loading={busy}>
                Konfirmasi order
              </Button>
            )}
            {primary === "delivery" && (
              <Button asChild>
                <Link to={deliveryUrl}>Terbitkan surat jalan</Link>
              </Button>
            )}
            {primary === "pay" && (
              <Button onClick={() => setPaymentOpen(true)} disabled={busy}>
                Catat pembayaran
              </Button>
            )}
          </>
        }
      >
        <DocumentRail steps={rail.steps} current={rail.current} stopped={rail.stopped} related={rail.related} />
      </PageHeader>

      {actionError && (
        <Notice tone="danger" title="Aksi ditolak">
          {actionError}
        </Notice>
      )}
      {justFromDeliveries && fromDelivery && (
        <Notice tone="success" title="Order dari surat jalan selesai">
          <p>Surat jalan sudah tertaut ke order ini. Langkah berikutnya:</p>
          <div className="mt-2 flex flex-wrap gap-2">
            <Button variant="secondary" size="sm" asChild>
              <Link to={printUrl} target="_blank" rel="noreferrer">
                Cetak invoice
              </Link>
            </Button>
            {showPay && (
              <Button variant="secondary" size="sm" onClick={() => setPaymentOpen(true)} disabled={busy}>
                Catat pembayaran
              </Button>
            )}
          </div>
        </Notice>
      )}
      {order.status === "cancelled" && order.cancelReason && (
        <Notice tone="warning" title="Order dibatalkan">
          {order.cancelReason}
          {order.cancelledAt
            ? ` (${[order.cancelledByName, formatDateTime(order.cancelledAt)].filter((t) => t).join(", ")})`
            : ""}
        </Notice>
      )}
      {order.shortClosedAt && (
        <Notice tone="info" title="Sisa order ditutup">
          Order ditagih sebesar barang yang sudah terkirim. Alasan: {order.shortCloseReason || "-"} (
          {[order.shortClosedByName, formatDateTime(order.shortClosedAt)].filter((t) => t).join(", ")})
        </Notice>
      )}
      {prepaidBlocked && (
        <Notice tone="info" title="Order prabayar">
          Surat jalan baru bisa diterbitkan setelah tagihan lunas.
        </Notice>
      )}
      {!prepaidBlocked && order.status === "confirmed" && order.paymentTerms === "cod" && billing.outstanding > 0 && (
        <Notice tone="info" title="Order COD">
          Pelunasan dicatat saat surat jalan penuntas dikonfirmasi, atau lewat Catat pembayaran.
        </Notice>
      )}

      <PageColumns
        main={
          <>
            <SectionCard title={`Barang (${order.items.length})`} flush>
              <DataTable<SalesOrderItem>
                rows={order.items}
                rowKey={(r) => r.id}
                emptyTitle="Tidak ada barang"
                columns={[
                  {
                    header: "Produk",
                    render: (r) => (
                      <div>
                        <p className="font-medium">{r.productName}</p>
                        <p className="text-xs text-muted-foreground">
                          {r.productCode}
                          {r.variantName ? ` · ${r.variantName}` : ""}
                          {r.itemType && r.itemType !== "goods" ? ` · ${r.itemType}` : ""}
                          {!r.stockTracked ? " · tanpa stok" : ""}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {pricingSourceLabel(r.pricingSource)}
                          {r.basePrice !== r.unitPrice && r.basePrice > 0 ? ` · Dasar ${formatIDR(r.basePrice)}` : ""}
                          {r.taxable ? ` · Pajak ${formatIDR(r.taxAmount)} (DPP ${formatIDR(r.taxBase)})` : " · Tanpa pajak"}
                        </p>
                        {r.discountReason && (
                          <p className="text-xs text-muted-foreground">Alasan diskon: {r.discountReason}</p>
                        )}
                        {r.note && <p className="text-xs text-muted-foreground">Catatan: {r.note}</p>}
                      </div>
                    ),
                  },
                  {
                    header: "Qty",
                    align: "right",
                    render: (r) => (
                      <div>
                        <Qty value={r.qty} unit={r.uom} />
                        {r.baseUom && r.baseUom !== r.uom && (
                          <p className="text-xs text-muted-foreground">
                            = {formatNumber(r.qtyBase)} {r.baseUom}
                          </p>
                        )}
                      </div>
                    ),
                  },
                  ...(progress !== null && order.channel !== "pos"
                    ? [
                        {
                          header: "Terkirim",
                          align: "right" as const,
                          render: (r: SalesOrderItem) => {
                            const p = progress.get(lineKey(r));
                            return (
                              <div>
                                <p className="tabular">
                                  {formatNumber(p?.done ?? 0)} / {formatNumber(r.qty)}
                                </p>
                                {(p?.pending ?? 0) > 0 && (
                                  <p className="text-xs text-muted-foreground">+{formatNumber(p?.pending ?? 0)} draf</p>
                                )}
                              </div>
                            );
                          },
                        },
                      ]
                    : []),
                  { header: "Harga", align: "right", render: (r) => <Money value={r.unitPrice} /> },
                  {
                    header: "Diskon",
                    align: "right",
                    render: (r) => {
                      const parts: string[] = [];
                      if (r.discountPct > 0) parts.push(`${formatNumber(r.discountPct)}%`);
                      if (r.discountNominal > 0) parts.push(formatIDR(r.discountNominal));
                      return <span className="tabular">{parts.length > 0 ? parts.join(" + ") : "–"}</span>;
                    },
                  },
                  { header: "Total", align: "right", render: (r) => <Money value={r.lineTotal} className="font-medium" /> },
                ]}
              />
            </SectionCard>

            {!isDraft && (
              <OrderHistoryCard
                events={events}
                loading={history.loading}
                warnings={history.data?.warnings ?? (history.error ? [history.error] : [])}
                hiddenNotes={hiddenNotes}
                paymentsUnavailable={order.partyId <= 0 && !history.data?.hidden.payments}
              />
            )}
          </>
        }
        aside={
          <>
            <OrderBillingCard
              billing={billing}
              subtotal={order.subtotal}
              discountTotal={order.discountTotal}
              taxTotal={order.taxTotal}
              grandTotal={order.grandTotal}
              returns={
                order.netTotal !== undefined
                  ? {
                      adjustment: order.returnAdjustment ?? 0,
                      netTotal: order.netTotal,
                      refundDue: order.refundDue ?? 0,
                      exchangeChargeDue: order.exchangeChargeDue ?? 0,
                    }
                  : undefined
              }
            />
            <SectionCard title="Pelanggan">
              <DescriptionList
                columns={1}
                items={[
                  { label: "Nama", value: customer },
                  { label: "Kode member", value: order.memberCode },
                ]}
              />
            </SectionCard>
            <SectionCard title="Info dokumen">
              <DescriptionList
                columns={1}
                items={[
                  { label: "Nomor", value: <DocNumber value={order.number} copyable /> },
                  { label: "Tanggal order", value: formatDate(order.orderDate) },
                  { label: "Kanal", value: channelLabel(order.channel) },
                  ...(order.origin ? [{ label: "Asal", value: originLabel(order.origin) }] : []),
                  { label: "Termin", value: paymentTermsLabel(order.paymentTerms) },
                  {
                    label: "Jatuh tempo",
                    value: (
                      <span className={billing.overdue ? "font-semibold text-destructive" : undefined}>
                        {formatDate(billing.dueDate !== "" ? billing.dueDate : order.dueDate)}
                        {billing.overdue ? " (terlewat)" : ""}
                      </span>
                    ),
                  },
                  ...(order.clientRef
                    ? [{ label: "Referensi klien", value: <span className="font-mono text-xs">{order.clientRef}</span> }]
                    : []),
                  { label: "Dibuat oleh", value: order.createdByName },
                  { label: "Penanggung jawab", value: order.assignedUserName },
                  ...((order.receiptAccountId ?? 0) > 0 || bank.active.length > 0
                    ? [
                        {
                          label: "Rekening tujuan",
                          value: (
                            <span className="inline-flex flex-wrap items-center gap-2">
                              <span>
                                {(() => {
                                  const b = bankAccountByAccountId(bank.all, order.receiptAccountId);
                                  return b ? bankAccountLabel(b) : "-";
                                })()}
                              </span>
                              {canEdit && order.status !== "cancelled" && bank.active.length > 0 && (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  aria-label="Ubah rekening tujuan"
                                  onClick={() => {
                                    setReceiptError(null);
                                    setReceiptOpen(true);
                                  }}
                                >
                                  Ubah
                                </Button>
                              )}
                            </span>
                          ),
                        },
                      ]
                    : []),
                  ...(order.creditApprovedAt
                    ? [
                        {
                          label: "Kredit disetujui",
                          value: `${order.creditApprovedByName || "-"} · ${formatDate(order.creditApprovedAt)}`,
                        },
                      ]
                    : []),
                  ...(order.shipToAddress
                    ? [
                        {
                          label: order.shipToAddressId > 0 ? "Alamat kirim" : "Alamat kirim (sekali pakai)",
                          value: [order.shipToRecipient, order.shipToPhone, order.shipToAddress]
                            .filter((t) => t && t.trim() !== "")
                            .join(" · "),
                          wide: true,
                        },
                      ]
                    : []),
                  { label: "Catatan", value: order.notes, wide: true },
                ]}
              />
            </SectionCard>
          </>
        }
      />

      {cancelOpen && (
        <CancelOrderModal
          number={order.number}
          submitting={busy}
          onClose={() => setCancelOpen(false)}
          onSubmit={handleCancel}
        />
      )}

      {headerOpen && (
        <HeaderEditModal
          order={order}
          lockReason={order.editLockReason}
          canPickUser={can("users.view")}
          submitting={busy}
          serverError={headerError}
          onClose={() => setHeaderOpen(false)}
          onSubmit={(initial, values) => void handleHeaderEdit(order, initial, values)}
        />
      )}

      {receiptOpen && (
        <ReceiptAccountModal
          number={order.number}
          current={(order.receiptAccountId ?? 0) > 0 ? (order.receiptAccountId ?? null) : null}
          options={bank.active}
          all={bank.all}
          submitting={busy}
          serverError={receiptError}
          onClose={() => setReceiptOpen(false)}
          onSubmit={(id) => void handleReceiptAccount(order, id)}
        />
      )}

      {shortCloseOpen && (
        <ShortCloseModal
          orderId={order.id}
          number={order.number}
          submitting={busy}
          onClose={() => setShortCloseOpen(false)}
          onSubmit={handleShortClose}
        />
      )}

      <Modal
        open={creditOpen}
        title="Setujui kredit"
        description={`Alihkan order ${order.number} dari COD ke termin tempo.`}
        size="sm"
        onClose={() => setCreditOpen(false)}
      >
        <form onSubmit={handleApproveCredit}>
          <FormField label="Jatuh tempo" required htmlFor="f-credit-due" errorText={creditError ?? undefined}>
            <DateInput id="f-credit-due" value={creditDue} onChange={(e) => setCreditDue(e.target.value)} />
          </FormField>
          <div className="mt-6 flex justify-end gap-3">
            <Button variant="secondary" size="sm" onClick={() => setCreditOpen(false)}>
              Batal
            </Button>
            <Button type="submit" size="sm" loading={busy}>
              Setujui
            </Button>
          </div>
        </form>
      </Modal>

      <OrderPaymentModal
        open={paymentOpen}
        onClose={() => setPaymentOpen(false)}
        onPaid={() => {
          setPaymentOpen(false);
          refresh();
        }}
        orderType="sales"
        orderId={order.id}
        orderNumber={order.number}
        partyId={order.partyId > 0 ? order.partyId : 0}
        outstanding={billing.outstanding}
        billAccountId={(order.receiptAccountId ?? 0) > 0 ? (order.receiptAccountId ?? null) : null}
      />
    </Page>
  );
}
