import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Button,
  ConfirmDialog,
  DataTable,
  DateInput,
  DetailSkeleton,
  ErrorState,
  FormField,
  FormSheet,
  PageHeader,
  SectionCard,
  SelectInput,
  StatusBadge,
  TextInput,
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
} from "@/shared/components/patterns";
import type { ActionMenuItem, RelatedDocument } from "@/shared/components/patterns";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDate, formatIDR, todayWIB } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import type { MutateResult } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { purchaseReturnSettlementSchema } from "@/modules/purchasereturn/schemas/purchase-return.schema";
import { purchaseReturnService } from "@/modules/purchasereturn/services/purchase-return.service";
import { purchaseReturnRail } from "@/modules/purchasereturn/lib/purchase-return-rail";
import { purchaseSettlementTypeLabel } from "@/modules/purchasereturn/types";
import type {
  PurchaseReturn,
  PurchaseReturnItem,
  PurchaseReturnSettlement,
} from "@/modules/purchasereturn/types";

function SummaryRow({ label, children, strong }: { label: string; children: React.ReactNode; strong?: boolean }) {
  return (
    <div className={`flex items-baseline justify-between gap-3 text-sm ${strong ? "border-t border-border pt-3" : ""}`}>
      <dt className={strong ? "font-semibold text-foreground" : "text-muted-foreground"}>{label}</dt>
      <dd className={strong ? "font-display text-xl font-semibold text-foreground" : "text-foreground"}>{children}</dd>
    </div>
  );
}

export default function PurchaseReturnDetailPage() {
  const { id } = useParams<{ id: string }>();
  const can = useAuthStore((s) => s.can);
  const settleFormId = useId();

  const returnId = Number(id);
  const invalidId = !Number.isInteger(returnId) || returnId <= 0;

  // RegisterRoutes backend: confirm/settle = purchasereturn.create,
  // cancel = purchasereturn.archive, context = purchasereturn.view.
  // TIDAK ada permission baru (Tahap D).
  const canConfirm = can("purchasereturn.create");
  const canCancel = can("purchasereturn.archive");
  const canSettle = can("purchasereturn.create");
  const canRefund = can("purchasereturn.refund");

  const [ret, setRet] = useState<PurchaseReturn | null>(null);
  const [busy, setBusy] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);

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
  } = useAsyncData(() => (invalidId ? Promise.resolve(null) : purchaseReturnService.get(returnId)), [returnId]);

  // Hasil fetch disalin ke state tampilan; aksi mutasi menimpa state yang
  // sama tanpa fetch ulang.
  const [appliedFetch, setAppliedFetch] = useState<PurchaseReturn | null>(null);
  if (fetchedRet !== appliedFetch) {
    setAppliedFetch(fetchedRet);
    setRet(fetchedRet);
  }

  usePageCrumb(ret?.number);

  async function runAction(fn: () => Promise<MutateResult<PurchaseReturn>>, fallback: string, fail: string) {
    setBusy(true);
    try {
      const res = await fn();
      setRet(res.data);
      toast.fromServer(res.message, fallback, res.data.number);
    } catch (err) {
      toast.danger(fail, toApiError(err).message);
    } finally {
      setBusy(false);
    }
  }

  function handleConfirm() {
    void runAction(
      () => purchaseReturnService.confirm(returnId),
      "Retur dikonfirmasi, stok keluar kembali",
      "Gagal mengonfirmasi",
    );
  }

  function handleCancel() {
    setCancelOpen(false);
    void runAction(() => purchaseReturnService.cancel(returnId), "Retur dibatalkan", "Gagal membatalkan");
  }

  function openSettle() {
    setSettleType("");
    setSettleDate(todayWIB());
    setSettleAmount(ret && ret.externalRemaining > 0 ? String(ret.externalRemaining) : "");
    setSettleMethod("");
    setSettleRef("");
    setSettleNotes("");
    setSettleError(null);
    setSettleOpen(true);
  }

  function handleSettle(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = purchaseReturnSettlementSchema.safeParse({
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
    purchaseReturnService
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
      .catch((err) => setSettleError(toApiError(err).message))
      .finally(() => setSettleBusy(false));
  }

  if (invalidId || errorStatus === 404) {
    return (
      <NotFoundState
        title="Retur tidak ditemukan"
        message="Retur pembelian ini tidak ada atau sudah dihapus."
        backTo={ROUTE_PATHS.purchaseReturns}
        backLabel="Ke daftar retur beli"
      />
    );
  }
  if (loading && !ret) return <DetailSkeleton />;
  if (fetchError && !ret) {
    return (
      <Page>
        <PageHeader title="Retur beli" />
        <ErrorState title="Gagal memuat retur" message={fetchError} onRetry={reload} />
      </Page>
    );
  }
  if (!ret) return null;

  const isDraft = ret.status === "draft";
  // Cancel backend: draf langsung batal; terkonfirmasi dibalik (stok masuk
  // lagi) kecuali sudah ada refund aktif.
  const cancellable = ret.status === "draft" || ret.status === "confirmed";
  // K-04c: tombol hanya bila server menyatakan masih ada kelebihan yang sah
  // diselesaikan (retur yang habis diserap tagihan tidak punya apa-apa lagi).
  const allowedTypes = ret.allowedSettlementTypes ?? [];
  const showSettleButton = canSettle && ret.status === "confirmed" && allowedTypes.length > 0;
  const rail = purchaseReturnRail(ret);

  const showConfirm = isDraft && canConfirm;
  const menuItems: ActionMenuItem[] = [
    ...(showConfirm && showSettleButton
      ? [{ key: "settle", label: "Catat penyelesaian", onSelect: openSettle, disabled: busy }]
      : []),
    ...(cancellable && canCancel
      ? [{ key: "cancel", label: "Batalkan retur", destructive: true, disabled: busy, onSelect: () => setCancelOpen(true) }]
      : []),
  ];

  const related: RelatedDocument[] = ret.purchaseOrderId
    ? [
        {
          label: ret.orderNumber || `#${ret.purchaseOrderId}`,
          kind: "Order beli asal",
          to: `${ROUTE_PATHS.purchaseOrders}/${ret.purchaseOrderId}`,
        },
      ]
    : [];

  return (
    <Page>
      <PageHeader
        title={ret.number}
        meta={<StatusBadge status={ret.status} />}
        subtitle={`${formatDate(ret.returnDate)} · Ref ${ret.orderNumber || "-"}`}
        actions={
          <>
            <ActionMenu items={menuItems} />
            {showConfirm ? (
              <Button onClick={handleConfirm} disabled={busy}>
                Konfirmasi retur
              </Button>
            ) : showSettleButton ? (
              <Button onClick={openSettle} disabled={busy}>
                Catat penyelesaian
              </Button>
            ) : null}
          </>
        }
      >
        <DocumentRail steps={rail.steps} current={rail.current} stopped={rail.stopped} related={related} />
      </PageHeader>

      <PageColumns
        main={
          <>
            <SectionCard flush title={`Barang diretur (${ret.items.length})`}>
              <DataTable<PurchaseReturnItem>
                rows={ret.items}
                rowKey={(r) => r.id}
                emptyTitle="Tidak ada item"
                columns={[
                  {
                    header: "Produk",
                    render: (r) => (
                      <p className="font-medium">
                        #{r.productId} <span className="font-normal text-muted-foreground">· {r.uom}</span>
                      </p>
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

            <SectionCard
              flush
              title="Penyelesaian"
              description="Cara kelebihan nilai retur diselesaikan dengan supplier. Refund bergerak lewat modul pembayaran; saldo supplier dijurnal sebagai uang muka supplier."
            >
              <DataTable<PurchaseReturnSettlement>
                rows={ret.settlements}
                rowKey={(r) => r.id}
                emptyTitle="Belum ada penyelesaian"
                emptyDescription={
                  ret.status === "confirmed" && allowedTypes.length === 0
                    ? "Seluruh nilai retur sudah mengurangi hutang — tidak ada yang perlu diselesaikan."
                    : "Catat penyelesaian setelah retur dikonfirmasi."
                }
                columns={[
                  { header: "Tanggal", render: (r) => formatDate(r.date) },
                  {
                    header: "Jenis",
                    render: (r) => (
                      <div>
                        <p>{purchaseSettlementTypeLabel(r.type)}</p>
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
                {ret.payableReduction > 0 && (
                  <SummaryRow label="Pengurang utang">
                    <Money value={ret.payableReduction} />
                  </SummaryRow>
                )}
                <div className="border-t border-border pt-3">
                  <SummaryRow label="Diselesaikan">
                    <Money value={ret.settledTotal} />
                  </SummaryRow>
                  <SummaryRow label="Sisa kelebihan">
                    <Money value={ret.externalRemaining} className={ret.externalRemaining > 0 ? "text-warning" : undefined} />
                  </SummaryRow>
                </div>
              </dl>
            </SectionCard>

            <SectionCard title="Info dokumen">
              <DescriptionList
                columns={1}
                items={[
                  {
                    label: "Order beli",
                    value: ret.orderNumber ? (
                      <Link
                        to={`${ROUTE_PATHS.purchaseOrders}/${ret.purchaseOrderId}`}
                        className="text-primary hover:underline"
                      >
                        <DocNumber value={ret.orderNumber} />
                      </Link>
                    ) : null,
                  },
                  { label: "Tanggal retur", value: formatDate(ret.returnDate) },
                  { label: "Alasan", value: ret.reason, wide: true },
                  { label: "Catatan", value: ret.notes, wide: true },
                ]}
              />
            </SectionCard>
          </>
        }
      />

      <ConfirmDialog
        open={cancelOpen}
        title="Batalkan retur?"
        message={`Retur ${ret.number} akan dibatalkan${ret.status === "confirmed" ? " dan stok yang sudah keluar akan dimasukkan kembali" : ""}. Bila retur sudah punya pembayaran/refund aktif, batalkan pembayarannya dulu (server menolak).`}
        confirmLabel="Ya, batalkan"
        onConfirm={handleCancel}
        onCancel={() => setCancelOpen(false)}
      />

      <FormSheet
        open={settleOpen}
        title="Catat penyelesaian"
        description={`Kelebihan yang bisa diselesaikan: ${formatIDR(ret.externalRemaining)}. Nilai retur lebih dulu mengurangi hutang ke supplier; hanya kelebihannya yang bisa direfund atau dijadikan saldo supplier.`}
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
            <SelectInput value={settleType} disabled={settleBusy} onChange={(e) => setSettleType(e.target.value)}>
              <option value="">Pilih…</option>
              {allowedTypes.includes("refund") && canRefund && (
                <option value="refund">{purchaseSettlementTypeLabel("refund")}</option>
              )}
              {allowedTypes.includes("supplier_credit") && (
                <option value="supplier_credit">{purchaseSettlementTypeLabel("supplier_credit")}</option>
              )}
            </SelectInput>
          </FormField>
          <FormField label="Nominal (Rp)" required helperText={`Maksimal: ${formatIDR(ret.externalRemaining)}`}>
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
              {ret.externalRemaining > 0 && (
                <Button
                  variant="secondary"
                  disabled={settleBusy}
                  onClick={() => setSettleAmount(String(ret.externalRemaining))}
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
