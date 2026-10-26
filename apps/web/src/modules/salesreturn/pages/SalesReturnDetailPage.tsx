import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Badge,
  Button,
  Checkbox,
  DataTable,
  DateInput,
  DetailSkeleton,
  ErrorState,
  FormField,
  FormSheet,
  Modal,
  PageHeader,
  SectionCard,
  SelectInput,
  StatusBadge,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import {
  ActionMenu,
  cancellationItems,
  DescriptionList,
  DocNumber,
  DocumentRail,
  Money,
  Page,
  PageColumns,
  Qty,
  ReasonDialog,
} from "@/shared/components/patterns";
import type { ActionMenuItem, RelatedDocument } from "@/shared/components/patterns";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatIDR, statusTone, todayWIB } from "@/shared/lib/format";
import { formatApiError, toApiError } from "@/shared/services/http-client";
import type { MutateResult } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { salesReturnSettlementSchema } from "@/modules/salesreturn/schemas/sales-return.schema";
import { salesReturnService } from "@/modules/salesreturn/services/sales-return.service";
import { salesReturnRail } from "@/modules/salesreturn/lib/sales-return-rail";
import { useBankAccounts } from "@/modules/finance/hooks/use-bank-accounts";
import { methodUsesBankAccount } from "@/modules/finance/lib/bank-accounts";
import { PaymentBankAccountField } from "@/modules/payment/components/PaymentBankAccountField";
import { paymentBankAccountError } from "@/modules/finance/lib/bank-accounts";
import {
  replacementStatusLabel,
  returnConditionLabel,
  returnDifferenceKind,
  returnDifferenceLabel,
  returnModeLabel,
  settlementTypeLabel,
} from "@/modules/salesreturn/types";
import type {
  ReplacementDeliveryNote,
  ReplacementPayNow,
  ReturnSettlement,
  SalesReturn,
  SalesReturnItem,
  SalesReturnReplacementItem,
} from "@/modules/salesreturn/types";

function SummaryRow({ label, children, strong }: { label: string; children: React.ReactNode; strong?: boolean }) {
  return (
    <div className={`flex items-baseline justify-between gap-3 text-sm ${strong ? "border-t border-border pt-3" : ""}`}>
      <dt className={strong ? "font-semibold text-foreground" : "text-muted-foreground"}>{label}</dt>
      <dd className={strong ? "font-display text-xl font-semibold text-foreground" : "text-foreground"}>{children}</dd>
    </div>
  );
}

export default function SalesReturnDetailPage() {
  const { id } = useParams<{ id: string }>();
  const can = useAuthStore((s) => s.can);
  const settleFormId = useId();
  const dispatchFormId = useId();
  const confirmFormId = useId();

  const returnId = Number(id);
  const invalidId = !Number.isInteger(returnId) || returnId <= 0;

  // RegisterRoutes backend: confirm/cancel/dispatch/confirm-pengganti/settle =
  // salesreturn.create (preview juga), cancel & batal kirim pengganti =
  // salesreturn.archive (alasan wajib), context =
  // salesreturn.view. TIDAK ada permission baru (Tahap D).
  const canConfirm = can("salesreturn.create");
  const canCancel = can("salesreturn.archive");
  const canAct = can("salesreturn.create");
  const canRefund = can("salesreturn.refund");

  const [ret, setRet] = useState<SalesReturn | null>(null);
  const [busy, setBusy] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [cancelReplOpen, setCancelReplOpen] = useState(false);

  // Kirim barang pengganti (dispatch).
  const [dispatchOpen, setDispatchOpen] = useState(false);
  const [dispatchDate, setDispatchDate] = useState(todayWIB());
  const [dispatchNotes, setDispatchNotes] = useState("");
  const [driverName, setDriverName] = useState("");
  const [vehiclePlate, setVehiclePlate] = useState("");
  const [warehouseStaff, setWarehouseStaff] = useState("");
  const [dropNote, setDropNote] = useState("");
  const [dispatchError, setDispatchError] = useState<string | null>(null);
  const [dispatchBusy, setDispatchBusy] = useState(false);

  // Konfirmasi terima pengganti.
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [replDelivery, setReplDelivery] = useState<ReplacementDeliveryNote | null>(null);
  const [replLookupError, setReplLookupError] = useState<string | null>(null);
  const [recipientName, setRecipientName] = useState("");
  const [sigStatus, setSigStatus] = useState<"" | "signed" | "missing">("");
  const [sigReason, setSigReason] = useState("");
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const [confirmBusy, setConfirmBusy] = useState(false);
  // Tagih selisih tukar saat serah (K-04e).
  const [payNow, setPayNow] = useState(false);
  const [payMethod, setPayMethod] = useState<"cash" | "transfer" | "check">("cash");
  // Rekening transfer/cek tambah bayar (ADR-0026): null = default perusahaan.
  const bank = useBankAccounts();
  const [payAccount, setPayAccount] = useState<number | null>(null);
  const effectivePayAccount = payAccount ?? bank.defaultAccountId;
  const [payAmount, setPayAmount] = useState("");
  const [payRef, setPayRef] = useState("");
  const [payTendered, setPayTendered] = useState("");

  // Penyelesaian (settlement).
  const [settleOpen, setSettleOpen] = useState(false);
  const [settleType, setSettleType] = useState("");
  const [settleDate, setSettleDate] = useState(todayWIB());
  const [settleAmount, setSettleAmount] = useState("");
  const [settleMethod, setSettleMethod] = useState("");
  const [settleRef, setSettleRef] = useState("");
  const [settleNotes, setSettleNotes] = useState("");
  const [settleError, setSettleError] = useState<string | null>(null);
  const [settleBusy, setSettleBusy] = useState(false);

  const {
    data: fetchedRet,
    loading,
    error: fetchError,
    errorStatus,
    reload,
  } = useAsyncData(() => (invalidId ? Promise.resolve(null) : salesReturnService.get(returnId)), [returnId]);

  // Hasil fetch disalin ke state tampilan; aksi mutasi menimpa state yang
  // sama tanpa fetch ulang.
  const [appliedFetch, setAppliedFetch] = useState<SalesReturn | null>(null);
  if (fetchedRet !== appliedFetch) {
    setAppliedFetch(fetchedRet);
    setRet(fetchedRet);
  }

  usePageCrumb(ret?.number);

  // SJ pengganti yang sedang jalan (retur tukar "dispatched"): statusnya
  // menentukan apakah pengiriman masih bisa ditarik (hanya draf).
  const replDispatched = ret?.returnMode === "exchange" && ret.replacementDeliveryStatus === "dispatched";
  const { data: replNote } = useAsyncData(async () => {
    if (!ret || !replDispatched) return null;
    try {
      return await salesReturnService.findReplacementDelivery(ret.salesOrderId, ret.id);
    } catch {
      return null; // tanpa izin lihat SJ: aksi tarik pengiriman disembunyikan
    }
  }, [ret?.id, ret?.salesOrderId, replDispatched]);

  async function runAction(fn: () => Promise<MutateResult<SalesReturn>>, fallback: string, fail: string): Promise<boolean> {
    setBusy(true);
    try {
      const res = await fn();
      setRet(res.data);
      toast.fromServer(res.message, fallback, res.data.number);
      return true;
    } catch (err) {
      toast.danger(fail, formatApiError(err));
      return false;
    } finally {
      setBusy(false);
    }
  }

  function handleConfirm() {
    void runAction(
      () => salesReturnService.confirm(returnId),
      "Retur dikonfirmasi, stok masuk kembali",
      "Gagal mengonfirmasi",
    );
  }

  async function handleCancel(reason: string) {
    if (await runAction(() => salesReturnService.cancel(returnId, reason), "Retur dibatalkan", "Gagal membatalkan")) {
      setCancelOpen(false);
    }
  }

  async function handleCancelReplacement(reason: string) {
    const ok = await runAction(
      () => salesReturnService.cancelReplacement(returnId, reason),
      "Pengiriman pengganti dibatalkan",
      "Gagal membatalkan pengiriman pengganti",
    );
    if (ok) setCancelReplOpen(false);
  }

  function openDispatch() {
    setDispatchDate(todayWIB());
    setDispatchNotes("");
    setDriverName("");
    setVehiclePlate("");
    setWarehouseStaff("");
    setDropNote("");
    setDispatchError(null);
    setDispatchOpen(true);
  }

  function handleDispatch(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setDispatchError(null);
    setDispatchBusy(true);
    salesReturnService
      .dispatchReplacement(returnId, {
        deliveryDate: dispatchDate.trim() || undefined,
        notes: dispatchNotes.trim() || undefined,
        driverName: driverName.trim() || undefined,
        vehiclePlate: vehiclePlate.trim() || undefined,
        warehouseStaffName: warehouseStaff.trim() || undefined,
        dropLocationNote: dropNote.trim() || undefined,
      })
      .then((res) => {
        toast.fromServer(res.message, "Barang pengganti dikirim", res.data.number);
        setDispatchOpen(false);
        reload();
      })
      .catch((err) => setDispatchError(formatApiError(err)))
      .finally(() => setDispatchBusy(false));
  }

  function openConfirmReplacement() {
    setPayNow(false);
    setPayMethod("cash");
    setPayAmount(ret && ret.chargeRemaining > 0 ? String(ret.chargeRemaining) : "");
    setPayRef("");
    setPayTendered("");
    setRecipientName("");
    setSigStatus("");
    setSigReason("");
    setConfirmError(null);
    setReplLookupError(null);
    setReplDelivery(null);
    setConfirmOpen(true);
    // ID surat jalan pengganti TIDAK ada di view retur — resolve via daftar
    // surat jalan SO yang sama (documentKind replacement + salesReturnId).
    if (ret) {
      salesReturnService
        .findReplacementDelivery(ret.salesOrderId, ret.id)
        .then(setReplDelivery)
        .catch((err) => setReplLookupError(toApiError(err).message));
    }
  }

  function handleConfirmReplacement(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (replDelivery === null) {
      setConfirmError("Surat jalan pengganti belum ditemukan — coba tutup dan buka lagi.");
      return;
    }
    // Cermin validateRecipient backend: status hanya signed|missing, bila
    // status diisi maka nama wajib, bila missing maka alasan wajib. Semua
    // kosong = konfirmasi polos.
    if (sigStatus !== "" && recipientName.trim() === "") {
      setConfirmError("Nama penerima wajib diisi bila ada status tanda terima.");
      return;
    }
    if (sigStatus === "missing" && sigReason.trim() === "") {
      setConfirmError("Alasan wajib diisi bila tanda tangan tidak ada.");
      return;
    }
    let settlement: ReplacementPayNow | undefined;
    if (payNow) {
      const amount = Number(payAmount);
      if (!Number.isFinite(amount) || amount <= 0) {
        setConfirmError("Nominal tagihan selisih harus lebih dari 0.");
        return;
      }
      if (ret && amount > ret.chargeRemaining) {
        setConfirmError(`Nominal melebihi selisih yang harus dibayar (sisa ${formatIDR(ret.chargeRemaining)}).`);
        return;
      }
      const tendered = payTendered.trim() === "" ? undefined : Number(payTendered);
      if (tendered !== undefined && (!Number.isFinite(tendered) || tendered < amount)) {
        setConfirmError("Uang diterima kurang dari nominal pembayaran.");
        return;
      }
      const bankError = paymentBankAccountError(bank, payMethod, effectivePayAccount);
      if (bankError) {
        setConfirmError(bankError);
        return;
      }
      settlement = {
        mode: "pay_now",
        amount,
        method: payMethod,
        reference: payRef.trim() === "" ? undefined : payRef.trim(),
        amountTendered: payMethod === "cash" ? tendered : undefined,
        cashAccountId: methodUsesBankAccount(payMethod) && effectivePayAccount !== null ? effectivePayAccount : undefined,
      };
    }
    setConfirmError(null);
    setConfirmBusy(true);
    const bare = recipientName.trim() === "" && sigStatus === "" && sigReason.trim() === "" && !settlement;
    salesReturnService
      .confirmReplacement(
        returnId,
        replDelivery.id,
        bare
          ? undefined
          : {
              recipientName: recipientName.trim() === "" ? undefined : recipientName.trim(),
              recipientSignatureStatus: sigStatus === "" ? undefined : sigStatus,
              recipientSignatureMissingReason: sigReason.trim() === "" ? undefined : sigReason.trim(),
              settlement,
            },
      )
      .then((res) => {
        toast.fromServer(res.message, "Penerimaan pengganti dikonfirmasi", res.data.number);
        setConfirmOpen(false);
        reload();
      })
      .catch((err) => setConfirmError(formatApiError(err)))
      .finally(() => setConfirmBusy(false));
  }

  /** Batas nominal per jenis: tagih = selisih terbuka; refund/saldo = sisa eksternal. */
  function settleLimit(type: string): number {
    if (!ret) return 0;
    return type === "collect_payment" ? ret.chargeRemaining : ret.externalRemaining;
  }

  function openSettle() {
    setSettleType("");
    setSettleDate(todayWIB());
    setSettleAmount("");
    setSettleMethod("");
    setSettleRef("");
    setSettleNotes("");
    setSettleError(null);
    setSettleOpen(true);
  }

  function handleSettle(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = salesReturnSettlementSchema.safeParse({
      type: settleType,
      date: settleDate.trim(),
      amount: settleAmount,
      paymentMethod: settleMethod.trim(),
      referenceNumber: settleRef.trim(),
      notes: settleNotes.trim(),
    });
    if (!parsed.success) {
      setSettleError(parsed.error.issues[0]?.message ?? "Form belum valid.");
      return;
    }
    setSettleError(null);
    setSettleBusy(true);
    salesReturnService
      .addSettlement(returnId, {
        type: parsed.data.type,
        date: parsed.data.date || undefined,
        amount: parsed.data.amount,
        paymentMethod: parsed.data.paymentMethod || undefined,
        referenceNumber: parsed.data.referenceNumber || undefined,
        notes: parsed.data.notes || undefined,
      })
      .then((res) => {
        setRet(res.data);
        toast.fromServer(res.message, "Penyelesaian dicatat");
        setSettleOpen(false);
      })
      .catch((err) => setSettleError(formatApiError(err)))
      .finally(() => setSettleBusy(false));
  }

  if (invalidId || errorStatus === 404) {
    return (
      <NotFoundState
        title="Retur tidak ditemukan"
        message="Retur penjualan ini tidak ada atau sudah dihapus."
        backTo={ROUTE_PATHS.salesReturns}
        backLabel="Ke daftar retur jual"
      />
    );
  }
  if (loading && !ret) return <DetailSkeleton />;
  if (fetchError && !ret) {
    return (
      <Page>
        <PageHeader title="Retur jual" />
        <ErrorState title="Gagal memuat retur" message={fetchError} onRetry={reload} />
      </Page>
    );
  }
  if (!ret) return null;

  const isDraft = ret.status === "draft";
  // Cancel backend: draf langsung batal; terkonfirmasi dibalik (stok keluar
  // lagi) kecuali sudah ada refund aktif.
  const cancellable = ret.status === "draft" || ret.status === "confirmed";
  const isExchange = ret.returnMode === "exchange";
  const diffKind = returnDifferenceKind(ret.differenceAmount);
  const canDispatch =
    canAct && isExchange && ret.status === "confirmed" && ret.replacementDeliveryStatus === "pending";
  const canConfirmRepl = canAct && isExchange && ret.replacementDeliveryStatus === "dispatched";
  // Pengiriman pengganti yang salah ditarik lewat retur selama SJ-nya masih draf.
  const canCancelRepl = canCancel && replDispatched && replNote?.status === "draft";
  // K-04c: server menghitung jenis yang masih sah dari arah selisih; retur yang
  // seluruhnya diserap tagihan (atau impas) tidak punya apa-apa lagi untuk dicatat.
  const allowedTypes = ret.allowedSettlementTypes ?? [];
  const canSettle = canAct && ret.status === "confirmed" && allowedTypes.length > 0;
  // K-04e: tagih selisih tukar saat serah terima bila pengganti lebih mahal.
  const canPayNow = isExchange && ret.chargeRemaining > 0 && can("payment.create");
  const rail = salesReturnRail(ret);

  // Aksi berikut yang paling mungkin menurut status = satu tombol primary; sisanya masuk menu.
  const flow: { key: string; label: string; run: () => void }[] = [];
  if (isDraft && canConfirm) flow.push({ key: "confirm", label: "Konfirmasi retur", run: handleConfirm });
  if (canDispatch) flow.push({ key: "dispatch", label: "Kirim barang pengganti", run: openDispatch });
  if (canConfirmRepl) flow.push({ key: "receive", label: "Konfirmasi terima pengganti", run: openConfirmReplacement });
  if (canSettle) flow.push({ key: "settle", label: "Catat penyelesaian", run: openSettle });
  const primary = flow[0];
  const menuItems: ActionMenuItem[] = [
    ...flow.slice(1).map((f) => ({ key: f.key, label: f.label, onSelect: f.run, disabled: busy })),
    ...(canCancelRepl
      ? [
          {
            key: "cancel-replacement",
            label: "Batalkan pengiriman pengganti",
            destructive: true,
            disabled: busy,
            onSelect: () => setCancelReplOpen(true),
          },
        ]
      : []),
    ...(cancellable && canCancel
      ? [{ key: "cancel", label: "Batalkan retur", destructive: true, disabled: busy, onSelect: () => setCancelOpen(true) }]
      : []),
  ];

  const related: RelatedDocument[] = ret.salesOrderId
    ? [
        {
          label: ret.orderNumber || `#${ret.salesOrderId}`,
          kind: "Order jual asal",
          to: `${ROUTE_PATHS.salesOrders}/${ret.salesOrderId}`,
        },
      ]
    : [];

  return (
    <Page>
      <PageHeader
        title={ret.number}
        meta={
          <>
            <StatusBadge status={ret.status} />
            {isExchange && <Badge tone="info">{returnModeLabel(ret.returnMode)}</Badge>}
          </>
        }
        subtitle={`${formatDate(ret.returnDate)} · Ref ${ret.orderNumber || "-"}`}
        actions={
          <>
            <ActionMenu items={menuItems} />
            {primary && (
              <Button onClick={primary.run} disabled={busy}>
                {primary.label}
              </Button>
            )}
          </>
        }
      >
        <DocumentRail steps={rail.steps} current={rail.current} stopped={rail.stopped} related={related} />
      </PageHeader>

      <PageColumns
        main={
          <>
            <SectionCard flush title={`Barang diretur (${ret.items.length})`}>
              <DataTable<SalesReturnItem>
                rows={ret.items}
                rowKey={(r) => r.id}
                emptyTitle="Tidak ada item"
                columns={[
                  {
                    header: "Produk",
                    render: (r) => (
                      <div>
                        <p className="font-medium">
                          #{r.productId} <span className="font-normal text-muted-foreground">· {r.uom}</span>
                        </p>
                        <p className="text-xs text-muted-foreground">{returnConditionLabel(r.condition)}</p>
                      </div>
                    ),
                  },
                  { header: "Qty", align: "right", render: (r) => <Qty value={r.qty} /> },
                  { header: "Harga", align: "right", render: (r) => <Money value={r.unitPrice} /> },
                  { header: "Diskon", align: "right", render: (r) => <Money value={r.discountNominal} /> },
                  { header: "PPN", align: "right", render: (r) => <Money value={r.taxAmount} /> },
                  { header: "Total", align: "right", render: (r) => <Money value={r.lineTotal} className="font-medium" /> },
                ]}
              />
            </SectionCard>

            {isExchange && (
              <SectionCard
                flush
                title={`Barang pengganti (${ret.replacementItems.length})`}
                description="Dikirim via surat jalan pengganti setelah retur dikonfirmasi. Stok keluar saat penerimaan dikonfirmasi."
              >
                <DataTable<SalesReturnReplacementItem>
                  rows={ret.replacementItems}
                  rowKey={(r) => r.id}
                  emptyTitle="Tidak ada barang pengganti"
                  columns={[
                    {
                      header: "Produk",
                      render: (r) => (
                        <div>
                          <p className="font-medium">
                            #{r.productId} <span className="font-normal text-muted-foreground">· {r.uom}</span>
                          </p>
                          <p className="text-xs text-muted-foreground">Lokasi #{r.locationId}</p>
                        </div>
                      ),
                    },
                    { header: "Qty", align: "right", render: (r) => <Qty value={r.qty} /> },
                    { header: "Harga", align: "right", render: (r) => <Money value={r.unitPrice} /> },
                    { header: "Dasar pajak", align: "right", render: (r) => <Money value={r.taxBase} /> },
                    { header: "PPN", align: "right", render: (r) => <Money value={r.taxAmount} /> },
                    { header: "Total", align: "right", render: (r) => <Money value={r.lineTotal} className="font-medium" /> },
                  ]}
                />
              </SectionCard>
            )}

            <SectionCard
              flush
              title="Penyelesaian"
              description="Cara nilai retur diselesaikan. Refund & tagihan selisih bergerak lewat modul pembayaran; saldo pelanggan dijurnal sebagai uang muka pelanggan."
            >
              <DataTable<ReturnSettlement>
                rows={ret.settlements}
                rowKey={(r) => r.id}
                emptyTitle="Belum ada penyelesaian"
                emptyDescription={
                  ret.status === "confirmed" && allowedTypes.length === 0
                    ? "Tidak ada nilai yang perlu diselesaikan — retur ini sudah mengurangi piutang order (atau tukar impas)."
                    : "Catat penyelesaian setelah retur dikonfirmasi."
                }
                columns={[
                  { header: "Tanggal", render: (r) => formatDate(r.date) },
                  {
                    header: "Jenis",
                    render: (r) => (
                      <div>
                        <p>{settlementTypeLabel(r.type)}</p>
                        {(r.paymentMethod || r.referenceNumber || r.notes) && (
                          <p className="text-xs text-muted-foreground">
                            {[r.paymentMethod, r.referenceNumber, r.notes].filter(Boolean).join(" · ")}
                          </p>
                        )}
                      </div>
                    ),
                  },
                  { header: "Nominal", align: "right", render: (r) => <Money value={r.amount} /> },
                ]}
              />
            </SectionCard>
          </>
        }
        aside={
          <>
            <SectionCard title="Ringkasan">
              <dl className="space-y-2">
                <SummaryRow label="Subtotal">
                  <Money value={ret.subtotal} />
                </SummaryRow>
                <SummaryRow label="Diskon">
                  <Money value={ret.discountTotal} />
                </SummaryRow>
                <SummaryRow label="PPN">
                  <Money value={ret.taxTotal} />
                </SummaryRow>
                <SummaryRow label="Total retur" strong>
                  <Money value={ret.total} />
                </SummaryRow>
                {ret.receivableReduction > 0 && (
                  <SummaryRow label="Pengurang piutang">
                    <Money value={ret.receivableReduction} />
                  </SummaryRow>
                )}
                {isExchange && (
                  <>
                    <SummaryRow label="Total pengganti">
                      <Money value={ret.replacementTotal} />
                    </SummaryRow>
                    <SummaryRow label="Selisih">
                      <span className="inline-flex flex-wrap items-center justify-end gap-2">
                        <Money value={Math.abs(ret.differenceAmount)} />
                        <Badge tone={diffKind === "refund" ? "success" : diffKind === "charge" ? "warning" : "neutral"}>
                          {returnDifferenceLabel(ret.differenceAmount)}
                        </Badge>
                      </span>
                    </SummaryRow>
                  </>
                )}
                <div className="border-t border-border pt-3">
                  <SummaryRow label="Diselesaikan">
                    <Money value={ret.settledTotal} />
                  </SummaryRow>
                  {ret.externalRemaining > 0 && (
                    <SummaryRow label="Sisa refund / saldo">
                      <Money value={ret.externalRemaining} className="text-warning" />
                    </SummaryRow>
                  )}
                  {ret.chargeRemaining > 0 && (
                    <SummaryRow label="Selisih belum ditagih">
                      <Money value={ret.chargeRemaining} className="text-warning" />
                    </SummaryRow>
                  )}
                </div>
              </dl>
            </SectionCard>

            <SectionCard title="Info dokumen">
              <DescriptionList
                columns={1}
                items={[
                  {
                    label: "Order jual",
                    value: ret.orderNumber ? (
                      <Link to={`${ROUTE_PATHS.salesOrders}/${ret.salesOrderId}`} className="text-primary hover:underline">
                        <DocNumber value={ret.orderNumber} />
                      </Link>
                    ) : null,
                  },
                  { label: "Tanggal retur", value: formatDate(ret.returnDate) },
                  { label: "Mode", value: returnModeLabel(ret.returnMode) },
                  ...(isExchange
                    ? [
                        {
                          label: "Pengiriman pengganti",
                          value: (
                            <Badge tone={statusTone(ret.replacementDeliveryStatus)}>
                              {replacementStatusLabel(ret.replacementDeliveryStatus)}
                            </Badge>
                          ),
                        },
                      ]
                    : []),
                  ...(replDispatched && replNote
                    ? [
                        {
                          label: "Surat jalan pengganti",
                          value: (
                            <Link to={`${ROUTE_PATHS.deliveries}/${replNote.id}`} className="text-primary hover:underline">
                              <DocNumber value={replNote.number} />
                            </Link>
                          ),
                        },
                      ]
                    : []),
                  ...cancellationItems(ret),
                  { label: "Alasan", value: ret.reason, wide: true },
                  { label: "Catatan", value: ret.notes, wide: true },
                ]}
              />
            </SectionCard>
          </>
        }
      />

      <ReasonDialog
        open={cancelOpen}
        title="Batalkan retur"
        description={`Retur ${ret.number} dibatalkan${ret.status === "confirmed" ? " dan stok yang sudah masuk dikeluarkan kembali" : ""}.${isExchange ? " Retur tukar yang barang penggantinya sudah dikirim harus dibatalkan pengirimannya dulu (selama surat jalannya masih draf)." : ""} Bila sudah ada refund aktif, batalkan pembayarannya dulu. Alasan dicatat pada retur dan jejak audit.`}
        confirmLabel="Batalkan retur"
        placeholder="Mis. salah pilih order, jumlah retur salah"
        submitting={busy}
        onClose={() => setCancelOpen(false)}
        onSubmit={(reason) => void handleCancel(reason)}
      />

      <ReasonDialog
        open={cancelReplOpen}
        title="Batalkan pengiriman pengganti"
        description={`Surat jalan pengganti ${replNote?.number ?? ""} (draf) dibatalkan dan barang pengganti kembali menunggu dikirim. Setelah itu kirim ulang pengganti yang benar, atau batalkan retur. Alasan dicatat pada surat jalan, retur, dan jejak audit.`}
        confirmLabel="Batalkan pengiriman pengganti"
        placeholder="Mis. salah supir, salah tanggal kirim"
        submitting={busy}
        onClose={() => setCancelReplOpen(false)}
        onSubmit={(reason) => void handleCancelReplacement(reason)}
      />

      <Modal
        open={dispatchOpen}
        title="Kirim barang pengganti"
        description={`Buat surat jalan pengganti untuk ${ret.number}. Supir dan petugas gudang wajib diisi (sama seperti surat jalan biasa), kecuali bukti pengiriman dimatikan di pengaturan perusahaan. Retur tukar dari kasir POS tidak lewat sini: barang pengganti diserahkan langsung saat retur dikonfirmasi.`}
        onClose={() => {
          if (!dispatchBusy) setDispatchOpen(false);
        }}
        actions={
          <>
            <Button variant="secondary" onClick={() => setDispatchOpen(false)} disabled={dispatchBusy}>
              Batal
            </Button>
            <Button type="submit" form={dispatchFormId} loading={dispatchBusy}>
              {dispatchBusy ? "Mengirim…" : "Kirim"}
            </Button>
          </>
        }
      >
        <form id={dispatchFormId} onSubmit={handleDispatch} className="flex flex-col gap-4">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField label="Tanggal kirim">
              <DateInput value={dispatchDate} onChange={(e) => setDispatchDate(e.target.value)} disabled={dispatchBusy} />
            </FormField>
            <FormField label="Supir" required>
              <TextInput
                value={driverName}
                maxLength={100}
                onChange={(e) => setDriverName(e.target.value)}
                placeholder="Nama supir"
                disabled={dispatchBusy}
              />
            </FormField>
            <FormField label="Plat kendaraan">
              <TextInput
                value={vehiclePlate}
                maxLength={20}
                onChange={(e) => setVehiclePlate(e.target.value)}
                placeholder="Plat (opsional)"
                disabled={dispatchBusy}
              />
            </FormField>
            <FormField label="Petugas gudang" required>
              <TextInput
                value={warehouseStaff}
                maxLength={100}
                onChange={(e) => setWarehouseStaff(e.target.value)}
                placeholder="Nama petugas gudang"
                disabled={dispatchBusy}
              />
            </FormField>
          </div>
          <FormField label="Catatan turun barang">
            <TextArea
              rows={2}
              value={dropNote}
              onChange={(e) => setDropNote(e.target.value)}
              placeholder="Catatan turun barang (opsional)"
              disabled={dispatchBusy}
            />
          </FormField>
          <FormField label="Catatan">
            <TextArea
              rows={2}
              value={dispatchNotes}
              onChange={(e) => setDispatchNotes(e.target.value)}
              placeholder="Catatan pengiriman (opsional)"
              disabled={dispatchBusy}
            />
          </FormField>
          {dispatchError && <Notice tone="danger" title={dispatchError} />}
        </form>
      </Modal>

      <Modal
        open={confirmOpen}
        title="Konfirmasi terima pengganti"
        description={
          replDelivery
            ? `Konfirmasi penerimaan ${replDelivery.number}? Stok pengganti keluar dan jalur tukar ditutup. Isian penerima opsional — kosongkan semua untuk konfirmasi polos.`
            : "Mencari surat jalan pengganti…"
        }
        onClose={() => {
          if (!confirmBusy) setConfirmOpen(false);
        }}
        actions={
          <>
            <Button variant="secondary" onClick={() => setConfirmOpen(false)} disabled={confirmBusy}>
              Batal
            </Button>
            <Button type="submit" form={confirmFormId} loading={confirmBusy} disabled={replDelivery === null}>
              {confirmBusy ? "Mengonfirmasi…" : "Konfirmasi"}
            </Button>
          </>
        }
      >
        <form id={confirmFormId} onSubmit={handleConfirmReplacement} className="flex flex-col gap-4">
          {replLookupError && <Notice tone="danger" title={replLookupError} />}
          {confirmError && <Notice tone="danger" title={confirmError} />}
          {replDelivery !== null && (
            <div className="grid gap-4 md:grid-cols-2">
              <FormField label="Nama penerima">
                <TextInput
                  value={recipientName}
                  maxLength={150}
                  onChange={(e) => setRecipientName(e.target.value)}
                  placeholder="Nama penerima (opsional)"
                  disabled={confirmBusy}
                />
              </FormField>
              <FormField label="Status tanda tangan">
                <SelectInput
                  value={sigStatus}
                  disabled={confirmBusy}
                  onChange={(e) => setSigStatus(e.target.value as "" | "signed" | "missing")}
                >
                  <option value="">Tanpa status</option>
                  <option value="signed">Ditandatangani</option>
                  <option value="missing">Tidak ada</option>
                </SelectInput>
              </FormField>
            </div>
          )}
          {replDelivery !== null && canPayNow && (
            <div className="rounded-md border border-border p-3">
              <label htmlFor="repl-pay-now" className="flex cursor-pointer items-center gap-2.5 text-sm font-medium">
                <Checkbox
                  id="repl-pay-now"
                  checked={payNow}
                  disabled={confirmBusy}
                  onCheckedChange={(v) => setPayNow(v === true)}
                />
                Tagih selisih sekarang ({formatIDR(ret.chargeRemaining)})
              </label>
              <p className="mt-1 text-xs text-muted-foreground">
                Pembayaran dicatat bersamaan dengan konfirmasi: bila pembayaran gagal, surat jalan tetap draf dan bisa diulang.
              </p>
              {payNow && (
                <div className="mt-3 grid gap-3 md:grid-cols-2">
                  <FormField label="Metode bayar">
                    <SelectInput
                      value={payMethod}
                      disabled={confirmBusy}
                      onChange={(e) => setPayMethod(e.target.value as "cash" | "transfer" | "check")}
                    >
                      <option value="cash">Tunai</option>
                      <option value="transfer">Transfer</option>
                      <option value="check">Cek</option>
                    </SelectInput>
                  </FormField>
                  <FormField label="Nominal (Rp)" required>
                    <TextInput
                      type="number"
                      min={1}
                      step={1}
                      value={payAmount}
                      onChange={(e) => setPayAmount(e.target.value)}
                      disabled={confirmBusy}
                      className="tabular text-right"
                    />
                  </FormField>
                  <PaymentBankAccountField
                    bank={bank}
                    method={payMethod}
                    value={effectivePayAccount}
                    onChange={setPayAccount}
                    direction="in"
                    disabled={confirmBusy}
                  />
                  <FormField label="Nomor referensi">
                    <TextInput
                      value={payRef}
                      maxLength={100}
                      onChange={(e) => setPayRef(e.target.value)}
                      placeholder="No. transfer / cek (opsional)"
                      disabled={confirmBusy}
                    />
                  </FormField>
                  {payMethod === "cash" && (
                    <FormField label="Uang diterima (Rp)">
                      <TextInput
                        type="number"
                        min={0}
                        step={1}
                        value={payTendered}
                        onChange={(e) => setPayTendered(e.target.value)}
                        placeholder="Opsional"
                        disabled={confirmBusy}
                        className="tabular text-right"
                      />
                    </FormField>
                  )}
                </div>
              )}
            </div>
          )}
          {replDelivery !== null && sigStatus === "missing" && (
            <FormField label="Alasan tanda tangan tidak ada" required>
              <TextArea
                rows={2}
                value={sigReason}
                onChange={(e) => setSigReason(e.target.value)}
                placeholder="Alasan tidak ada tanda tangan"
                disabled={confirmBusy}
              />
            </FormField>
          )}
        </form>
      </Modal>

      <FormSheet
        open={settleOpen}
        title="Catat penyelesaian"
        description={
          ret.chargeRemaining > 0
            ? `Selisih tukar yang belum ditagih: ${formatIDR(ret.chargeRemaining)}.`
            : `Sisa yang bisa direfund atau dijadikan saldo pelanggan: ${formatIDR(ret.externalRemaining)}. Nilai retur lebih dulu mengurangi piutang order.`
        }
        onClose={() => {
          if (!settleBusy) setSettleOpen(false);
        }}
        actions={
          <>
            <Button variant="secondary" onClick={() => setSettleOpen(false)} disabled={settleBusy}>
              Batal
            </Button>
            <Button type="submit" form={settleFormId} loading={settleBusy}>
              {settleBusy ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id={settleFormId} onSubmit={handleSettle} className="flex flex-col gap-4">
          {settleError && <Notice tone="danger" title={settleError} />}
          <FormField label="Jenis penyelesaian" required>
            <SelectInput
              value={settleType}
              disabled={settleBusy}
              onChange={(e) => {
                setSettleType(e.target.value);
                const limit = settleLimit(e.target.value);
                setSettleAmount(limit > 0 ? String(limit) : "");
              }}
            >
              <option value="">Pilih…</option>
              {allowedTypes.includes("collect_payment") && (
                <option value="collect_payment">{settlementTypeLabel("collect_payment")}</option>
              )}
              {allowedTypes.includes("refund") && canRefund && (
                <option value="refund">{settlementTypeLabel("refund")}</option>
              )}
              {allowedTypes.includes("customer_credit") && (
                <option value="customer_credit">{settlementTypeLabel("customer_credit")}</option>
              )}
            </SelectInput>
          </FormField>
          <FormField
            label="Nominal (Rp)"
            required
            helperText={settleType ? `Maksimal: ${formatIDR(settleLimit(settleType))}` : "Pilih jenis penyelesaian dulu."}
          >
            <div className="flex gap-2">
              <TextInput
                type="number"
                min={1}
                step={1}
                value={settleAmount}
                onChange={(e) => setSettleAmount(e.target.value)}
                placeholder="Nominal (Rp)"
                disabled={settleBusy}
                className="tabular text-right"
              />
              {settleType !== "" && settleLimit(settleType) > 0 && (
                <Button
                  variant="secondary"
                  disabled={settleBusy}
                  onClick={() => setSettleAmount(String(settleLimit(settleType)))}
                >
                  Isi sisa
                </Button>
              )}
            </div>
          </FormField>
          <FormField label="Tanggal" helperText="Kosongkan untuk hari ini (server).">
            <DateInput value={settleDate} onChange={(e) => setSettleDate(e.target.value)} disabled={settleBusy} />
          </FormField>
          <FormField label="Metode pembayaran">
            <TextInput
              value={settleMethod}
              maxLength={50}
              onChange={(e) => setSettleMethod(e.target.value)}
              placeholder="Tunai / transfer / …"
              disabled={settleBusy}
            />
          </FormField>
          <FormField label="Nomor referensi">
            <TextInput
              value={settleRef}
              maxLength={100}
              onChange={(e) => setSettleRef(e.target.value)}
              placeholder="No. bukti / referensi"
              disabled={settleBusy}
            />
          </FormField>
          <FormField label="Catatan">
            <TextInput
              value={settleNotes}
              maxLength={255}
              onChange={(e) => setSettleNotes(e.target.value)}
              placeholder="Catatan (opsional)"
              disabled={settleBusy}
            />
          </FormField>
        </form>
      </FormSheet>
    </Page>
  );
}
