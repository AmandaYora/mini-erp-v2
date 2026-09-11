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
  TextArea,
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
import { salesReturnSettlementSchema } from "@/modules/salesreturn/schemas/sales-return.schema";
import { salesReturnService } from "@/modules/salesreturn/services/sales-return.service";
import {
  replacementStatusLabel,
  returnModeLabel,
  settlementTypeLabel,
} from "@/modules/salesreturn/types";
import type {
  ReplacementDeliveryNote,
  ReturnSettlement,
  SalesReturn,
  SalesReturnItem,
  SalesReturnReplacementItem,
} from "@/modules/salesreturn/types";

export default function SalesReturnDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const returnId = Number(id);
  const invalidId = !Number.isInteger(returnId) || returnId <= 0;

  // RegisterRoutes backend: confirm/cancel/dispatch/confirm-pengganti/settle =
  // salesreturn.create (preview juga), cancel = salesreturn.archive, context =
  // salesreturn.view. TIDAK ada permission baru (Tahap D).
  const canConfirm = can("salesreturn.create");
  const canCancel = can("salesreturn.archive");
  const canAct = can("salesreturn.create");

  const [ret, setRet] = useState<SalesReturn | null>(null);
  const [busy, setBusy] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);

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
    () => (invalidId ? Promise.resolve(null) : salesReturnService.get(returnId)),
    [returnId],
  );
  const error = invalidId ? "ID retur tidak valid." : fetchError;

  // Hasil fetch disalin ke state tampilan; aksi mutasi menimpa state yang
  // sama tanpa fetch ulang.
  const [appliedFetch, setAppliedFetch] = useState<SalesReturn | null>(null);
  if (fetchedRet !== appliedFetch) {
    setAppliedFetch(fetchedRet);
    setRet(fetchedRet);
  }

  async function runAction(fn: () => Promise<MutateResult<SalesReturn>>, fallback: string, fail: string) {
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
      () => salesReturnService.confirm(returnId),
      "Retur dikonfirmasi, stok masuk kembali",
      "Gagal mengonfirmasi",
    );
  }

  function handleCancel() {
    setCancelOpen(false);
    void runAction(
      () => salesReturnService.cancel(returnId),
      "Retur dibatalkan",
      "Gagal membatalkan",
    );
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
      .catch((err) => setDispatchError(toApiError(err).message))
      .finally(() => setDispatchBusy(false));
  }

  function openConfirmReplacement() {
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
    setConfirmError(null);
    setConfirmBusy(true);
    const bare =
      recipientName.trim() === "" && sigStatus === "" && sigReason.trim() === "";
    salesReturnService
      .confirmReplacement(
        returnId,
        replDelivery.id,
        bare
          ? undefined
          : {
              recipientName:
                recipientName.trim() === "" ? undefined : recipientName.trim(),
              recipientSignatureStatus: sigStatus === "" ? undefined : sigStatus,
              recipientSignatureMissingReason:
                sigReason.trim() === "" ? undefined : sigReason.trim(),
            },
      )
      .then((res) => {
        toast.fromServer(res.message, "Penerimaan pengganti dikonfirmasi", res.data.number);
        setConfirmOpen(false);
        reload();
      })
      .catch((err) => setConfirmError(toApiError(err).message))
      .finally(() => setConfirmBusy(false));
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
      .catch((err) => setSettleError(toApiError(err).message))
      .finally(() => setSettleBusy(false));
  }

  if (loading) {
    return <p className="text-muted py-10 text-center text-sm">Memuat…</p>;
  }

  if (error || !ret) {
    return (
      <div className="space-y-4">
        <PageHeader title="Detail Retur Penjualan" />
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
        <Button variant="secondary" onClick={() => navigate(ROUTE_PATHS.salesReturns)}>
          Kembali ke daftar
        </Button>
      </div>
    );
  }

  const isDraft = ret.status === "draft";
  // Cancel backend: draf langsung batal; terkonfirmasi dibalik (stok keluar
  // lagi) kecuali sudah ada refund aktif.
  const cancellable = ret.status === "draft" || ret.status === "confirmed";
  const isExchange = ret.returnMode === "exchange";
  const canDispatch =
    canAct && isExchange && ret.status === "confirmed" && ret.replacementDeliveryStatus === "pending";
  const canConfirmRepl =
    canAct && isExchange && ret.replacementDeliveryStatus === "dispatched";
  const canSettle = canAct && ret.status === "confirmed" && ret.unsettledTotal > 0;

  return (
    <div>
      <PageHeader
        eyebrow="Retur Jual"
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
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Mode</dt>
              <dd className="mt-1">
                <Badge tone={isExchange ? "info" : "neutral"}>{returnModeLabel(ret.returnMode)}</Badge>
              </dd>
            </div>
            {isExchange && (
              <div>
                <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Pengiriman Pengganti</dt>
                <dd className="mt-1">
                  <Badge tone={statusTone(ret.replacementDeliveryStatus)}>
                    {replacementStatusLabel(ret.replacementDeliveryStatus)}
                  </Badge>
                </dd>
              </div>
            )}
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Order Jual</dt>
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
          <DataTable<SalesReturnItem>
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

        {isExchange && (
          <SectionCard
            title={`Barang Pengganti (${ret.replacementItems.length})`}
            description="Dikirim via surat jalan pengganti setelah retur dikonfirmasi. Stok keluar saat penerimaan dikonfirmasi."
            actions={
              <>
                {canDispatch && (
                  <Button size="sm" onClick={openDispatch} disabled={busy}>
                    Kirim barang pengganti
                  </Button>
                )}
                {canConfirmRepl && (
                  <Button size="sm" onClick={openConfirmReplacement} disabled={busy}>
                    Konfirmasi terima pengganti
                  </Button>
                )}
              </>
            }
          >
            <DataTable<SalesReturnReplacementItem>
              rows={ret.replacementItems}
              rowKey={(r) => r.id}
              emptyTitle="Tidak ada barang pengganti"
              columns={[
                {
                  header: "Produk",
                  render: (r) => (
                    <p className="font-medium">
                      #{r.productId} <span className="text-muted font-normal">· {r.uom}</span>
                    </p>
                  ),
                },
                { header: "Lokasi", render: (r) => `#${r.locationId}` },
                { header: "Qty", align: "right", render: (r) => formatNumber(r.qty) },
                { header: "Harga", align: "right", render: (r) => formatIDR(r.unitPrice) },
                { header: "Total Baris", align: "right", render: (r) => formatIDR(r.lineTotal) },
              ]}
            />
          </SectionCard>
        )}

        <SectionCard
          title="Penyelesaian"
          description="Memo cara nilai retur diselesaikan — uang riil tetap bergerak lewat modul pembayaran."
          actions={
            canSettle ? (
              <Button size="sm" onClick={openSettle} disabled={busy}>
                Catat penyelesaian
              </Button>
            ) : undefined
          }
        >
          <DataTable<ReturnSettlement>
            rows={ret.settlements}
            rowKey={(r) => r.id}
            emptyTitle="Belum ada penyelesaian"
            emptyDescription="Catat penyelesaian setelah retur dikonfirmasi."
            columns={[
              { header: "Tanggal", render: (r) => formatDate(r.date) },
              { header: "Jenis", render: (r) => settlementTypeLabel(r.type) },
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
          <Button variant="secondary" onClick={() => navigate(ROUTE_PATHS.salesReturns)}>
            Kembali ke daftar
          </Button>
        </ActionRow>
      </div>

      <ConfirmDialog
        open={cancelOpen}
        title="Batalkan retur?"
        message={`Retur ${ret.number} akan dibatalkan${ret.status === "confirmed" ? " dan stok yang sudah masuk akan dikeluarkan kembali" : ""}.`}
        confirmLabel="Ya, batalkan"
        onConfirm={handleCancel}
        onCancel={() => setCancelOpen(false)}
      />

      <Modal
        open={dispatchOpen}
        title="Kirim Barang Pengganti"
        description={`Buat surat jalan pengganti untuk ${ret.number}. Semua isian opsional — kosongkan semua untuk kirim dengan bawaan server.`}
        onClose={() => {
          if (!dispatchBusy) setDispatchOpen(false);
        }}
      >
        <form onSubmit={handleDispatch}>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField label="Tanggal Kirim">
              <DateInput
                value={dispatchDate}
                onChange={(e) => setDispatchDate(e.target.value)}
                disabled={dispatchBusy}
              />
            </FormField>
            <FormField label="Supir">
              <TextInput
                value={driverName}
                maxLength={100}
                onChange={(e) => setDriverName(e.target.value)}
                placeholder="Nama supir (opsional)"
                disabled={dispatchBusy}
              />
            </FormField>
            <FormField label="Plat Kendaraan">
              <TextInput
                value={vehiclePlate}
                maxLength={20}
                onChange={(e) => setVehiclePlate(e.target.value)}
                placeholder="Plat (opsional)"
                disabled={dispatchBusy}
              />
            </FormField>
            <FormField label="Petugas Gudang">
              <TextInput
                value={warehouseStaff}
                maxLength={100}
                onChange={(e) => setWarehouseStaff(e.target.value)}
                placeholder="Petugas (opsional)"
                disabled={dispatchBusy}
              />
            </FormField>
          </div>
          <div className="mt-4">
            <FormField label="Catatan Turun Barang">
              <TextArea
                rows={2}
                value={dropNote}
                onChange={(e) => setDropNote(e.target.value)}
                placeholder="Catatan turun barang (opsional)"
                disabled={dispatchBusy}
              />
            </FormField>
          </div>
          <div className="mt-4">
            <FormField label="Catatan">
              <TextArea
                rows={2}
                value={dispatchNotes}
                onChange={(e) => setDispatchNotes(e.target.value)}
                placeholder="Catatan pengiriman (opsional)"
                disabled={dispatchBusy}
              />
            </FormField>
          </div>
          {dispatchError && (
            <div className="mt-4">
              <Notice tone="danger" title={dispatchError} />
            </div>
          )}
          <div className="mt-6 flex justify-end gap-3">
            <Button
              variant="secondary"
              onClick={() => setDispatchOpen(false)}
              disabled={dispatchBusy}
            >
              Batal
            </Button>
            <Button type="submit" disabled={dispatchBusy}>
              {dispatchBusy ? "Mengirim…" : "Kirim"}
            </Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={confirmOpen}
        title="Konfirmasi Terima Pengganti"
        description={
          replDelivery
            ? `Konfirmasi penerimaan ${replDelivery.number}? Stok pengganti keluar dan jalur tukar ditutup. Isian penerima opsional — kosongkan semua untuk konfirmasi polos.`
            : "Mencari surat jalan pengganti…"
        }
        onClose={() => {
          if (!confirmBusy) setConfirmOpen(false);
        }}
      >
        <form onSubmit={handleConfirmReplacement}>
          {replLookupError && (
            <div className="mb-4">
              <Notice tone="danger" title={replLookupError} />
            </div>
          )}
          {replDelivery === null && !replLookupError && (
            <p className="text-muted py-4 text-center text-sm">Mencari surat jalan pengganti…</p>
          )}
          {confirmError && (
            <div className="mb-4">
              <Notice tone="danger" title={confirmError} />
            </div>
          )}
          {replDelivery !== null && (
            <div className="grid gap-4 md:grid-cols-2">
              <FormField label="Nama Penerima">
                <TextInput
                  value={recipientName}
                  maxLength={150}
                  onChange={(e) => setRecipientName(e.target.value)}
                  placeholder="Nama penerima (opsional)"
                  disabled={confirmBusy}
                />
              </FormField>
              <FormField label="Status Tanda Tangan">
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
          {replDelivery !== null && sigStatus === "missing" && (
            <div className="mt-4">
              <FormField label="Alasan Tanda Tangan Hilang" required>
                <TextArea
                  rows={2}
                  value={sigReason}
                  onChange={(e) => setSigReason(e.target.value)}
                  placeholder="Alasan tidak ada tanda tangan"
                  disabled={confirmBusy}
                />
              </FormField>
            </div>
          )}
          <div className="mt-6 flex justify-end gap-3">
            <Button
              variant="secondary"
              onClick={() => setConfirmOpen(false)}
              disabled={confirmBusy}
            >
              Batal
            </Button>
            <Button type="submit" disabled={confirmBusy || replDelivery === null}>
              {confirmBusy ? "Mengonfirmasi…" : "Konfirmasi"}
            </Button>
          </div>
        </form>
      </Modal>

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
                <option value="collect_payment">Tagih pembayaran</option>
                <option value="reduce_receivable">Kurangi piutang</option>
                <option value="refund">Refund</option>
                <option value="customer_credit">Kredit pelanggan</option>
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
