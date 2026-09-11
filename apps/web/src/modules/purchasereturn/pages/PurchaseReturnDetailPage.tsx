import { useState } from "react";
import type { FormEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ActionRow,
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  DateInput,
  FormField,
  Modal,
  PageHeader,
  SectionCard,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  formatDate,
  formatIDR,
  formatNumber,
  statusLabel,
  statusTone,
  todayWIB,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import type { MutateResult } from "@/shared/services/http-client";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { purchaseReturnSettlementSchema } from "@/modules/purchasereturn/schemas/purchase-return.schema";
import { purchaseReturnService } from "@/modules/purchasereturn/services/purchase-return.service";
import { purchaseSettlementTypeLabel } from "@/modules/purchasereturn/types";
import type {
  PurchaseReturn,
  PurchaseReturnItem,
  PurchaseReturnSettlement,
} from "@/modules/purchasereturn/types";

export default function PurchaseReturnDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const returnId = Number(id);
  const invalidId = !Number.isInteger(returnId) || returnId <= 0;

  // RegisterRoutes backend: confirm/settle = purchasereturn.create,
  // cancel = purchasereturn.archive, context = purchasereturn.view.
  // TIDAK ada permission baru (Tahap D).
  const canConfirm = can("purchasereturn.create");
  const canCancel = can("purchasereturn.archive");
  const canSettle = can("purchasereturn.create");

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

  const { data: fetchedRet, loading, error: fetchError, reload } = useAsyncData(
    () => (invalidId ? Promise.resolve(null) : purchaseReturnService.get(returnId)),
    [returnId],
  );
  const error = invalidId ? "ID retur tidak valid." : fetchError;

  // Hasil fetch disalin ke state tampilan; aksi mutasi menimpa state yang
  // sama tanpa fetch ulang.
  const [appliedFetch, setAppliedFetch] = useState<PurchaseReturn | null>(null);
  if (fetchedRet !== appliedFetch) {
    setAppliedFetch(fetchedRet);
    setRet(fetchedRet);
  }

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
    void runAction(
      () => purchaseReturnService.cancel(returnId),
      "Retur dibatalkan",
      "Gagal membatalkan",
    );
  }

  function openSettle() {
    setSettleType("");
    setSettleDate(todayWIB());
    setSettleAmount(ret && ret.unsettledTotal > 0 ? String(ret.unsettledTotal) : "");
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

  if (loading) {
    return <p className="text-muted py-10 text-center text-sm">Memuat…</p>;
  }

  if (error || !ret) {
    return (
      <div className="space-y-4">
        <PageHeader title="Detail Retur Pembelian" />
        <Notice tone="danger" title="Gagal memuat retur">
          {error ?? "Data tidak ditemukan."}{" "}
          <button
            type="button"
            onClick={() => reload()}
            className="cursor-pointer font-semibold text-brand hover:underline"
          >
            Coba lagi
          </button>
        </Notice>
        <Button variant="secondary" onClick={() => navigate(ROUTE_PATHS.purchaseReturns)}>
          Kembali ke daftar
        </Button>
      </div>
    );
  }

  const isDraft = ret.status === "draft";
  // Cancel backend: draf langsung batal; terkonfirmasi dibalik (stok masuk
  // lagi) kecuali sudah ada refund aktif.
  const cancellable = ret.status === "draft" || ret.status === "confirmed";
  const showSettleButton = canSettle && ret.status === "confirmed" && ret.unsettledTotal > 0;

  return (
    <div>
      <PageHeader
        eyebrow="Retur Beli"
        title={ret.number}
        description={`${formatDate(ret.returnDate)} · Ref ${ret.orderNumber || "-"}`}
        actions={
          <>
            {isDraft && canConfirm && (
              <Button onClick={handleConfirm} disabled={busy}>
                Konfirmasi
              </Button>
            )}
            {cancellable && canCancel && (
              <Button variant="danger" onClick={() => setCancelOpen(true)} disabled={busy}>
                Batalkan
              </Button>
            )}
          </>
        }
      />

      <div className="space-y-4">
        <SectionCard title="Informasi">
          <dl className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Status</dt>
              <dd className="mt-1">
                <Badge tone={statusTone(ret.status)}>{statusLabel(ret.status)}</Badge>
              </dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Order Beli</dt>
              <dd className="mt-1 text-sm text-ink">{ret.orderNumber || "-"}</dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Tanggal</dt>
              <dd className="mt-1 text-sm text-ink">{formatDate(ret.returnDate)}</dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Subtotal</dt>
              <dd className="mt-1 text-sm text-ink">{formatIDR(ret.subtotal)}</dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Diskon</dt>
              <dd className="mt-1 text-sm text-ink">{formatIDR(ret.discountTotal)}</dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">PPN</dt>
              <dd className="mt-1 text-sm text-ink">{formatIDR(ret.taxTotal)}</dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Total</dt>
              <dd className="mt-1 text-sm font-bold text-ink">{formatIDR(ret.total)}</dd>
            </div>
            {ret.notes && (
              <div className="sm:col-span-2">
                <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Alasan / Catatan</dt>
                <dd className="mt-1 text-sm text-ink">{ret.notes}</dd>
              </div>
            )}
          </dl>
        </SectionCard>

        <SectionCard title="Item">
          <DataTable<PurchaseReturnItem>
            rows={ret.items}
            rowKey={(r) => r.id}
            emptyTitle="Tidak ada item"
            columns={[
              {
                header: "Produk",
                render: (r) => (
                  <p className="font-medium">
                    #{r.productId} <span className="text-muted font-normal">· {r.uom}</span>
                  </p>
                ),
              },
              { header: "Qty", align: "right", render: (r) => formatNumber(r.qty) },
              { header: "Harga", align: "right", render: (r) => formatIDR(r.unitPrice) },
              { header: "Diskon", align: "right", render: (r) => formatIDR(r.discountNominal) },
              { header: "PPN", align: "right", render: (r) => formatIDR(r.taxAmount) },
              { header: "Total Baris", align: "right", render: (r) => formatIDR(r.lineTotal) },
            ]}
          />
        </SectionCard>

        <SectionCard
          title="Penyelesaian"
          description="Memo cara nilai retur diselesaikan dengan supplier — uang riil tetap bergerak lewat modul pembayaran."
          actions={
            showSettleButton ? (
              <Button size="sm" onClick={openSettle} disabled={busy}>
                Catat penyelesaian
              </Button>
            ) : undefined
          }
        >
          <DataTable<PurchaseReturnSettlement>
            rows={ret.settlements}
            rowKey={(r) => r.id}
            emptyTitle="Belum ada penyelesaian"
            emptyDescription="Catat penyelesaian setelah retur dikonfirmasi."
            columns={[
              { header: "Tanggal", render: (r) => formatDate(r.date) },
              { header: "Jenis", render: (r) => purchaseSettlementTypeLabel(r.type) },
              { header: "Nominal", align: "right", render: (r) => formatIDR(r.amount) },
              { header: "Metode", render: (r) => r.paymentMethod || "-" },
              { header: "Referensi", render: (r) => r.referenceNumber || "-" },
              { header: "Catatan", render: (r) => r.notes || "-" },
            ]}
          />
          <p className="mt-3 text-right text-sm text-ink">
            Diselesaikan <span className="font-bold">{formatIDR(ret.settledTotal)}</span>
            {" · "}Sisa <span className="font-bold">{formatIDR(ret.unsettledTotal)}</span>
          </p>
        </SectionCard>

        <ActionRow align="start">
          <Button variant="secondary" onClick={() => navigate(ROUTE_PATHS.purchaseReturns)}>
            Kembali ke daftar
          </Button>
        </ActionRow>
      </div>

      <ConfirmDialog
        open={cancelOpen}
        title="Batalkan retur?"
        message={`Retur ${ret.number} akan dibatalkan${ret.status === "confirmed" ? " dan stok yang sudah keluar akan dimasukkan kembali" : ""}.`}
        confirmLabel="Ya, batalkan"
        onConfirm={handleCancel}
        onCancel={() => setCancelOpen(false)}
      />

      <Modal
        open={settleOpen}
        title="Catat Penyelesaian"
        description={`Sisa belum diselesaikan: ${formatIDR(ret.unsettledTotal)}. Total penyelesaian tidak boleh melebihi total retur.`}
        onClose={() => {
          if (!settleBusy) setSettleOpen(false);
        }}
      >
        <form onSubmit={handleSettle}>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField label="Jenis Penyelesaian" required>
              <SelectInput
                value={settleType}
                disabled={settleBusy}
                onChange={(e) => setSettleType(e.target.value)}
              >
                <option value="">Pilih…</option>
                <option value="collect_payment">Terima pembayaran</option>
                <option value="reduce_receivable">Kurangi utang</option>
                <option value="refund">Refund</option>
                <option value="customer_credit">Kredit supplier</option>
              </SelectInput>
            </FormField>
            <FormField label="Tanggal" helperText="Kosongkan untuk hari ini (server).">
              <DateInput
                value={settleDate}
                onChange={(e) => setSettleDate(e.target.value)}
                disabled={settleBusy}
              />
            </FormField>
            <FormField label="Nominal" required helperText={`Sisa: ${formatIDR(ret.unsettledTotal)}`}>
              <TextInput
                type="number"
                min={1}
                step={1}
                value={settleAmount}
                onChange={(e) => setSettleAmount(e.target.value)}
                placeholder="Nominal (Rp)"
                disabled={settleBusy}
              />
            </FormField>
            <FormField label="Metode Pembayaran">
              <TextInput
                value={settleMethod}
                maxLength={50}
                onChange={(e) => setSettleMethod(e.target.value)}
                placeholder="Tunai / transfer / …"
                disabled={settleBusy}
              />
            </FormField>
            <FormField label="Nomor Referensi">
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
          </div>
          {settleError && (
            <div className="mt-4">
              <Notice tone="danger" title={settleError} />
            </div>
          )}
          <div className="mt-6 flex justify-end gap-3">
            <Button
              variant="secondary"
              onClick={() => setSettleOpen(false)}
              disabled={settleBusy}
            >
              Batal
            </Button>
            <Button type="submit" disabled={settleBusy}>
              {settleBusy ? "Menyimpan…" : "Simpan"}
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
