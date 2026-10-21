import { useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
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
  FileDropzone,
  Page,
  PageColumns,
  Qty,
  ReasonDialog,
} from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { formatDate, formatDateTime, formatIDR, todayWIB } from "@/shared/lib/format";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { formatApiError, toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { deliveryRail } from "@/modules/delivery/lib/delivery-rail";
import { evidenceChecklist, evidenceRequired, missingEvidence } from "@/modules/delivery/lib/evidence";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import {
  deliveryConfirmSchema,
  toConfirmPayload,
} from "@/modules/delivery/schemas/delivery-confirm.schema";
import type {
  DeliveryItem,
  DeliveryNote,
  DeliveryProof,
  DeliveryProofPurpose,
  SalesOrderOption,
} from "@/modules/delivery/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

// Alur: draft → confirmed (stok keluar saat confirm) / cancelled.
// - Konfirmasi: POST /:id/confirm, gate delivery.create, hanya draft.
// - Batalkan: POST /:id/cancel {reason} (alasan wajib, tampil di Info dokumen),
//   gate delivery.archive, hanya draft —
//   tombol disembunyikan untuk status lain (backend menolak confirmed dengan
//   "gunakan retur").
// - Bukti kirim: GET /:id/proofs + upload POST /:id/proof (multipart "file").
// Dipesan vs dikirim: noteView TIDAK membawa qty pesanan maupun nama produk —
// keduanya dijoin best-effort dari detail SO via productId/variantId.

function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return "-";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

/** Label Indonesia untuk status tanda terima ("" = belum diisi). */
function signatureLabel(status: string | null | undefined): string {
  if (status === "signed") return "Ditandatangani";
  if (status === "missing") return "Tidak ada";
  return "-";
}

// Hasil verifikasi Tahap D (JANGAN ditebak ulang): noteView membawa
// documentKind ("order"|"replacement") + salesReturnId (delivery/
// presentation/handler.go: noteView). Surat jalan pengganti dibuat dari
// retur tukar yang dikonfirmasi — pendapatannya TIDAK dijurnal otomatis.
// Field didefinisikan lokal di sini (tanpa menyentuh delivery/types.ts).
interface ReplacementInfo {
  documentKind?: string;
  salesReturnId?: number | null;
}

export default function DeliveryDetailPage() {
  const { id } = useParams<{ id: string }>();
  const can = useAuthStore((s) => s.can);

  const noteId = Number(id);
  const canConfirm = can("delivery.create");
  const canCancel = can("delivery.archive");
  const canUpload = can("delivery.create");
  const canPay = can("payment.create");
  const canSwitchNet = can("sales.approve");

  const [acting, setActing] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [recipientName, setRecipientName] = useState("");
  const [sigStatus, setSigStatus] = useState<"" | "signed" | "missing">("");
  const [sigReason, setSigReason] = useState("");
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const [confirmFields, setConfirmFields] = useState<Record<string, string>>({});
  // Pelunasan hanya untuk order COD (server menolaknya untuk net/prepaid).
  // "" = tanpa pelunasan (SJ parsial / order sudah lunas — server mengabaikan).
  const [settleMode, setSettleMode] = useState<"" | "pay_now" | "switch_to_net">("");
  const [payMethod, setPayMethod] = useState<"cash" | "transfer" | "check">("cash");
  const [payDate, setPayDate] = useState(todayWIB());
  const [payRef, setPayRef] = useState("");
  const [payTendered, setPayTendered] = useState("");
  const [dueDate, setDueDate] = useState("");

  const { data, loading, error, errorStatus, reload } = useAsyncData(
    async () => {
      if (!Number.isInteger(noteId) || noteId <= 0) {
        throw new Error("ID surat jalan tidak valid.");
      }
      try {
        const n = await deliveryService.get(noteId);
        const [proofRows, orderDetail] = await Promise.all([
          deliveryService.proofs(noteId).catch(() => [] as DeliveryProof[]),
          deliveryService
            .salesOrderDetail(n.salesOrderId)
            .catch(() => null as SalesOrderOption | null),
        ]);
        return { note: n, proofs: proofRows, so: orderDetail };
      } catch (err) {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat surat jalan", apiErr.message);
        throw err;
      }
    },
    [noteId],
  );
  const note: DeliveryNote | null = data?.note ?? null;
  const proofs: DeliveryProof[] = data?.proofs ?? [];
  const signedDocs = proofs.filter((p) => p.purpose === "delivery_signed_document");
  const photos = proofs.filter((p) => p.purpose !== "delivery_signed_document");
  // Peringatan lunak dari pembuatan draf (DL-04): dibawa lewat state navigasi.
  const createWarnings = ((useLocation().state as { warnings?: string[] } | null)?.warnings ?? []).filter(Boolean);
  const so: SalesOrderOption | null = data?.so ?? null;
  usePageCrumb(note?.number);

  function openConfirm() {
    setRecipientName("");
    setSigStatus("");
    setSigReason("");
    setConfirmError(null);
    setConfirmFields({});
    // COD yang bisa dibayar di tempat: usulkan pay_now (server mengabaikannya
    // bila SJ ini bukan pengiriman penuntas atau order sudah lunas).
    const cod = so?.paymentTerms === "cod";
    setSettleMode(cod ? (canPay ? "pay_now" : canSwitchNet ? "switch_to_net" : "") : "");
    setPayMethod("cash");
    setPayDate(todayWIB());
    setPayRef("");
    setPayTendered("");
    setDueDate("");
    setConfirmOpen(true);
  }

  async function handleConfirm() {
    // Validasi klien = cermin validateRecipient + planSettlement backend.
    const parsed = deliveryConfirmSchema.safeParse({
      recipientName,
      recipientSignatureStatus: sigStatus,
      recipientSignatureMissingReason: sigReason,
      settlement:
        settleMode === ""
          ? null
          : {
              mode: settleMode,
              method: payMethod,
              paymentDate: payDate,
              reference: payRef,
              amountTendered: payTendered,
              dueDate,
            },
    });
    if (!parsed.success) {
      const mapped: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        const key = issue.path.filter((p) => typeof p !== "number").join(".");
        mapped[key || "_"] ??= issue.message;
      }
      setConfirmFields(mapped);
      setConfirmError("Periksa kembali isian konfirmasi.");
      return;
    }
    if (needsEvidence && (recipientName.trim() === "" || sigStatus === "")) {
      setConfirmFields({
        recipientName: "Nama penerima dan status tanda tangan wajib diisi untuk pengiriman reguler.",
        ...(sigStatus === "" ? { recipientSignatureStatus: "Pilih status tanda tangan" } : {}),
      });
      setConfirmError("Lengkapi bukti serah terima dulu.");
      return;
    }
    setConfirmError(null);
    setConfirmFields({});
    setActing(true);
    try {
      // Konfirmasi ulang (replay) dijawab 200 oleh server — diperlakukan sebagai sukses.
      const res = await deliveryService.confirm(noteId, toConfirmPayload(parsed.data));
      setConfirmOpen(false);
      toast.fromServer(res.message, "Surat jalan dikonfirmasi", res.data.number);
      reload();
    } catch (err) {
      // Dialog tetap terbuka: pesan server (mis. "order tunai belum lunas",
      // 403 izin pelunasan) ditampilkan di dalamnya agar bisa diperbaiki.
      const apiErr = toApiError(err);
      setConfirmError(apiErr.message);
      const mapped: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) {
        mapped[fe.field.replace(/^settlement\./, "")] ??= fe.message;
      }
      setConfirmFields(mapped);
      toast.danger("Gagal mengonfirmasi", apiErr.message);
    } finally {
      setActing(false);
    }
  }

  async function handleCancel(reason: string) {
    setActing(true);
    try {
      const res = await deliveryService.cancel(noteId, reason);
      setCancelOpen(false);
      toast.fromServer(res.message, "Surat jalan dibatalkan", res.data.number);
      reload();
    } catch (err) {
      toast.danger("Gagal membatalkan", formatApiError(err));
    } finally {
      setActing(false);
    }
  }

  async function uploadFile(file: File, purpose: DeliveryProofPurpose) {
    if (!note) return;
    setUploading(true);
    try {
      const res = await deliveryService.uploadProof(note.id, file, purpose);
      toast.fromServer(res.message, "Bukti kirim diunggah");
      reload();
    } catch (err) {
      toast.danger("Gagal mengunggah bukti", formatApiError(err));
    } finally {
      setUploading(false);
    }
  }

  if (!Number.isInteger(noteId) || noteId <= 0 || (!loading && errorStatus === 404)) {
    return (
      <NotFoundState
        title="Surat jalan tidak ditemukan"
        message="Surat jalan ini tidak ada atau bukan milik cabang aktif."
        backTo={ROUTE_PATHS.deliveries}
        backLabel="Kembali ke daftar surat jalan"
      />
    );
  }

  if (loading && !note) return <DetailSkeleton />;

  if (error || !note) {
    return (
      <Page>
        <PageHeader title="Detail Surat Jalan" />
        <ErrorState message={error ?? "Surat jalan tidak ditemukan"} onRetry={error ? reload : undefined} />
      </Page>
    );
  }

  const isDraft = note.status === "draft";
  // Dokumen reguler: sopir, petugas, penerima+tanda tangan, dan foto wajib (kecuali dimatikan perusahaan).
  const needsEvidence = evidenceRequired(so?.channel, (note as DeliveryNote & { documentKind?: string }).documentKind);
  const evidence = evidenceChecklist({
    driverName: note.driverName,
    warehouseStaffName: note.warehouseStaffName,
    recipientName,
    signatureStatus: sigStatus,
    signedDocumentCount: signedDocs.length,
  });
  const evidenceMissing = needsEvidence ? missingEvidence(evidence) : [];
  const replInfo = note as DeliveryNote & ReplacementInfo;
  const isReplacement = replInfo.documentKind === "replacement";
  const replacementReturnId =
    typeof replInfo.salesReturnId === "number" && replInfo.salesReturnId > 0
      ? replInfo.salesReturnId
      : null;
  const soLineOf = (it: DeliveryItem) =>
    so?.items.find(
      (l) => l.productId === it.productId && l.variantId === it.variantId,
    ) ?? null;

  const rail = deliveryRail({
    status: note.status,
    dispatchedAt: note.dispatchedAt,
    salesOrderId: note.salesOrderId,
    orderNumber: note.orderNumber,
    orderStatus: so?.status ?? null,
    salesReturnId: replacementReturnId,
  });
  const driverMissing = evidenceMissing.some((m) => m.key === "driverName" || m.key === "warehouseStaffName");
  // Siklus SJ pengganti dimiliki retur penjualan (server menolak konfirmasi/batal
  // generik): tombolnya ada di halaman retur.
  const showConfirm = isDraft && canConfirm && !isReplacement;
  const showCancel = isDraft && canCancel && !isReplacement;

  return (
    <Page>
      <PageHeader
        title={note.number}
        meta={
          <>
            <StatusBadge status={note.status} />
            {isReplacement && <Badge tone="info">Pengganti</Badge>}
          </>
        }
        subtitle={`${so?.partyName || "Walk-in"} · ${formatDate(note.deliveryDate)}`}
        actions={
          <>
            <Button variant="secondary" asChild>
              <Link to={`/print/deliveries/${note.id}`} target="_blank" rel="noreferrer">
                Cetak
              </Link>
            </Button>
            <ActionMenu
              items={
                showCancel
                  ? [
                      {
                        key: "cancel",
                        label: "Batalkan surat jalan",
                        destructive: true,
                        disabled: acting,
                        onSelect: () => setCancelOpen(true),
                      },
                    ]
                  : []
              }
            />
            {showConfirm && (
              <Button onClick={openConfirm} disabled={acting}>
                Konfirmasi
              </Button>
            )}
          </>
        }
      >
        <DocumentRail steps={rail.steps} current={rail.current} stopped={rail.stopped} related={rail.related} />
      </PageHeader>

      {isReplacement && (
        <Notice tone="info" title="Surat jalan pengganti retur tukar">
          Barang pengganti dari {replacementReturnId !== null ? `Retur #${replacementReturnId}` : "retur terkait"} —
          pendapatan tidak dijurnal otomatis untuk pengiriman ini.
          {isDraft && (
            <>
              {" "}Konfirmasi pengiriman dilakukan dari{" "}
              {replacementReturnId !== null ? (
                <Link to={`/sales-returns/${replacementReturnId}`} className="underline">
                  halaman retur
                </Link>
              ) : (
                "halaman retur"
              )}
              .
            </>
          )}
        </Notice>
      )}
      {isDraft && driverMissing && (
        <Notice tone="warning" title="Dokumen belum lengkap">
          Sopir dan/atau petugas gudang belum terisi pada surat jalan ini. Pengiriman reguler tidak bisa
          dikonfirmasi tanpa keduanya; surat jalan draf tidak bisa diubah, jadi batalkan lalu buat ulang.
        </Notice>
      )}

      {isDraft && so?.channel !== "pos" && (
        <Notice tone="warning" title="Stok belum ditahan oleh surat jalan draf">
          Stok baru dikurangi saat surat jalan dikonfirmasi. Penjualan lain bisa memakainya lebih dulu; bila stok
          tidak cukup saat konfirmasi, surat jalan tetap draf dan Anda bisa menambah stok, mengganti lokasi, lalu
          mengonfirmasi ulang, atau membatalkannya.
          {createWarnings.length > 1 && (
            <ul className="mt-2 list-disc pl-5">
              {createWarnings.slice(1).map((w) => (
                <li key={w}>{w}</li>
              ))}
            </ul>
          )}
        </Notice>
      )}

      <PageColumns
        main={
          <>
            <SectionCard title={`Barang (${note.items.length})`} flush>
              <DataTable<DeliveryItem>
                rows={note.items}
                rowKey={(r) => r.id}
                emptyTitle="Tidak ada baris barang"
                columns={[
                  {
                    header: "Produk",
                    render: (r) => {
                      const match = soLineOf(r);
                      return match ? (
                        <div>
                          <p className="font-medium">{match.productName}</p>
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
                    header: "Dipesan (SO)",
                    align: "right",
                    render: (r) => {
                      const match = soLineOf(r);
                      return match ? <Qty value={match.qty} unit={match.uom} /> : "–";
                    },
                  },
                  {
                    header: "Dikirim",
                    align: "right",
                    render: (r) => <Qty value={r.qty} unit={r.uom} className="font-medium" />,
                  },
                  { header: "Lokasi", render: (r) => `#${r.locationId}` },
                ]}
              />
            </SectionCard>
          </>
        }
        aside={
          <>
            <SectionCard title="Dokumen & serah terima">
              <DescriptionList
                columns={1}
                items={[
                  { label: "Supir", value: note.driverName },
                  { label: "Plat kendaraan", value: note.vehiclePlate },
                  { label: "Petugas gudang", value: note.warehouseStaffName },
                  { label: "Penerima", value: note.recipientName },
                  { label: "Status tanda tangan", value: signatureLabel(note.recipientSignatureStatus) },
                  ...(note.recipientSignatureStatus === "missing"
                    ? [{ label: "Alasan tanda tangan hilang", value: note.recipientSignatureMissingReason }]
                    : []),
                  {
                    label: "Diberangkatkan",
                    value: note.dispatchedAt
                      ? `${formatDateTime(note.dispatchedAt)}${note.dispatchedBy ? ` · #${note.dispatchedBy}` : ""}`
                      : null,
                  },
                  {
                    label: "Dikonfirmasi",
                    value: note.confirmedAt
                      ? `${formatDateTime(note.confirmedAt)}${note.confirmedBy ? ` · #${note.confirmedBy}` : ""}`
                      : null,
                  },
                  { label: "Catatan turun barang", value: note.dropLocationNote, wide: true },
                ]}
              />
              <ProofSection
                title="Surat jalan fisik bertanda tangan"
                required
                empty="Belum ada scan/foto surat jalan fisik."
                files={signedDocs}
                canUpload={canUpload}
                uploading={uploading}
                dropLabel="Unggah scan/foto surat jalan"
                dropHint="Wajib sebelum konfirmasi. Hanya gambar, maksimal 6 MB."
                onFile={(f) => void uploadFile(f, "delivery_signed_document")}
              />
              <ProofSection
                title="Foto lokasi pengiriman"
                empty="Belum ada foto lokasi (opsional)."
                files={photos}
                canUpload={canUpload}
                uploading={uploading}
                dropLabel="Unggah foto lokasi (opsional)"
                dropHint="Hanya gambar, maksimal 6 MB."
                onFile={(f) => void uploadFile(f, "delivery_location_photo")}
              />
            </SectionCard>

            <SectionCard title="Info dokumen">
              <DescriptionList
                columns={1}
                items={[
                  { label: "Nomor", value: <DocNumber value={note.number} copyable /> },
                  {
                    label: "Order jual",
                    value: (
                      <Link to={`${ROUTE_PATHS.salesOrders}/${note.salesOrderId}`} className="text-primary hover:underline">
                        <DocNumber value={note.orderNumber || `#${note.salesOrderId}`} />
                      </Link>
                    ),
                  },
                  { label: "Pelanggan", value: so?.partyName },
                  { label: "Tanggal kirim", value: formatDate(note.deliveryDate) },
                  ...cancellationItems(note),
                  ...(isReplacement
                    ? [{ label: "Retur terkait", value: replacementReturnId !== null ? `Retur #${replacementReturnId}` : null }]
                    : []),
                  { label: "Catatan", value: note.notes, wide: true },
                ]}
              />
            </SectionCard>
          </>
        }
      />

      <Modal
        open={confirmOpen}
        title="Konfirmasi Surat Jalan"
        description={
          needsEvidence
            ? `Konfirmasi ${note.number}? Stok keluar dicatat dan tidak bisa dibatalkan (koreksi via retur). Pengiriman reguler wajib dilengkapi bukti serah terima di bawah.`
            : `Konfirmasi ${note.number}? Stok keluar dicatat dan tidak bisa dibatalkan (koreksi via retur). Isian penerima opsional — kosongkan semua untuk konfirmasi polos.`
        }
        onClose={() => {
          if (!acting) setConfirmOpen(false);
        }}
        actions={
          <>
            <Button
              variant="secondary"
              disabled={acting}
              onClick={() => setConfirmOpen(false)}
            >
              Batal
            </Button>
            <Button onClick={() => void handleConfirm()} loading={acting}>
              Konfirmasi
            </Button>
          </>
        }
      >
        {confirmError && (
          <div className="mb-4">
            <Notice tone="danger" title={confirmError} />
          </div>
        )}
        {needsEvidence && (
          <div className="mb-4 rounded-md border border-border p-4">
            <p className="text-sm font-semibold text-foreground">Kelengkapan dokumen pengiriman</p>
            <p className="text-muted-foreground mt-1 text-xs">
              Server menolak konfirmasi bila ada yang kurang (kecuali perusahaan menonaktifkan aturan ini).
            </p>
            <ul className="mt-3 space-y-2">
              {evidence.map((c) => {
                const serverMsg =
                  c.key === "recipientName"
                    ? confirmFields.recipientName
                    : confirmFields[c.key];
                return (
                  <li key={c.key} className="text-sm">
                    <span className={c.ok ? "text-success" : "text-destructive"}>{c.ok ? "Terpenuhi" : "Belum"}</span>
                    <span className="text-foreground"> — {c.label}</span>
                    {!c.ok && <p className="text-muted-foreground text-xs">{c.hint}</p>}
                    {serverMsg && <p className="text-destructive text-xs">Server: {serverMsg}</p>}
                  </li>
                );
              })}
            </ul>
            {canUpload && (
              <FileDropzone
                className="mt-3"
                accept="image/*"
                busy={uploading}
                label={`Unggah scan/foto surat jalan bertanda tangan (${signedDocs.length})`}
                hint="Wajib. Hanya gambar, maksimal 6 MB."
                onFile={(f) => void uploadFile(f, "delivery_signed_document")}
              />
            )}
          </div>
        )}
        <div className="grid gap-4 md:grid-cols-2">
          <FormField label="Nama Penerima" required={needsEvidence} errorText={confirmFields.recipientName}>
            <TextInput
              value={recipientName}
              maxLength={150}
              onChange={(e) => setRecipientName(e.target.value)}
              placeholder={needsEvidence ? "Nama penerima" : "Nama penerima (opsional)"}
            />
          </FormField>
          <FormField label="Status Tanda Tangan" required={needsEvidence} errorText={confirmFields.recipientSignatureStatus}>
            <SelectInput
              value={sigStatus}
              onChange={(e) =>
                setSigStatus(e.target.value as "" | "signed" | "missing")
              }
            >
              <option value="">{needsEvidence ? "Pilih status…" : "Tanpa status"}</option>
              <option value="signed">Ditandatangani</option>
              <option value="missing">Tidak ada</option>
            </SelectInput>
          </FormField>
        </div>
        {sigStatus === "missing" && (
          <div className="mt-4">
            <FormField
              label="Alasan Tanda Tangan Hilang"
              required
              errorText={confirmFields.recipientSignatureMissingReason}
            >
              <TextArea
                rows={2}
                value={sigReason}
                onChange={(e) => setSigReason(e.target.value)}
                placeholder="Alasan tidak ada tanda tangan"
              />
            </FormField>
          </div>
        )}
        {so?.paymentTerms === "cod" && (
          <div className="mt-5 rounded-md border border-border p-4">
            <p className="text-sm font-semibold text-foreground">Pelunasan order COD</p>
            <p className="text-muted-foreground mt-1 text-xs">
              Wajib bila surat jalan ini menuntaskan order COD yang masih ada sisa tagihan
              {so.grandTotal ? ` (total ${formatIDR(so.grandTotal)})` : ""}. Pada pengiriman
              parsial atau order yang sudah lunas, server mengabaikan pilihan ini.
            </p>
            <div className="mt-3 grid gap-4 md:grid-cols-2">
              <FormField label="Cara Pelunasan" errorText={confirmFields.settlement ?? confirmFields.mode}>
                <SelectInput
                  value={settleMode}
                  onChange={(e) =>
                    setSettleMode(e.target.value as "" | "pay_now" | "switch_to_net")
                  }
                >
                  <option value="">Tanpa pelunasan</option>
                  <option value="pay_now" disabled={!canPay}>
                    Bayar sekarang{canPay ? "" : " (butuh izin pembayaran)"}
                  </option>
                  <option value="switch_to_net" disabled={!canSwitchNet}>
                    Jadikan tempo (net){canSwitchNet ? "" : " (butuh izin persetujuan kredit)"}
                  </option>
                </SelectInput>
              </FormField>
              {settleMode === "switch_to_net" && (
                <FormField label="Jatuh Tempo" required errorText={confirmFields.dueDate}>
                  <DateInput value={dueDate} onChange={(e) => setDueDate(e.target.value)} />
                </FormField>
              )}
              {settleMode === "pay_now" && (
                <>
                  <FormField label="Metode" errorText={confirmFields.method}>
                    <SelectInput
                      value={payMethod}
                      onChange={(e) => setPayMethod(e.target.value as "cash" | "transfer" | "check")}
                    >
                      <option value="cash">Tunai</option>
                      <option value="transfer">Transfer</option>
                      <option value="check">Cek</option>
                    </SelectInput>
                  </FormField>
                  <FormField label="Tanggal Bayar" errorText={confirmFields.paymentDate}>
                    <DateInput value={payDate} onChange={(e) => setPayDate(e.target.value)} />
                  </FormField>
                  {payMethod !== "cash" && (
                    <FormField
                      label={payMethod === "check" ? "Nomor Cek" : "Referensi Transfer"}
                      required
                      errorText={confirmFields.reference}
                    >
                      <TextInput value={payRef} onChange={(e) => setPayRef(e.target.value)} />
                    </FormField>
                  )}
                  {payMethod === "cash" && (
                    <FormField
                      label="Uang Diterima (Rp)"
                      helperText="Opsional, untuk catatan kembalian."
                      errorText={confirmFields.amountTendered}
                    >
                      <TextInput
                        type="number"
                        min={0}
                        step={1}
                        value={payTendered}
                        onChange={(e) => setPayTendered(e.target.value)}
                      />
                    </FormField>
                  )}
                </>
              )}
            </div>
          </div>
        )}
      </Modal>
      <ReasonDialog
        open={cancelOpen}
        title="Batalkan surat jalan"
        description={`${note.number} dibatalkan dan kuota order dilepas. Alasan dicatat pada surat jalan dan jejak audit.`}
        confirmLabel="Batalkan surat jalan"
        placeholder="Mis. salah jumlah, salah lokasi gudang"
        submitting={acting}
        onClose={() => setCancelOpen(false)}
        onSubmit={(reason) => void handleCancel(reason)}
      />
    </Page>
  );
}

/** Satu zona bukti kirim (DL-01): daftar berkas + zona unggah per tujuan. */
function ProofSection({
  title,
  required,
  empty,
  files,
  canUpload,
  uploading,
  dropLabel,
  dropHint,
  onFile,
}: {
  title: string;
  required?: boolean;
  empty: string;
  files: DeliveryProof[];
  canUpload: boolean;
  uploading: boolean;
  dropLabel: string;
  dropHint: string;
  onFile: (f: File) => void;
}) {
  return (
    <div className="mt-5 border-t border-border pt-4">
      <h3 className="text-sm font-semibold text-foreground">
        {title} ({files.length})
        {required ? <span className="ml-2 text-xs font-normal text-muted-foreground">wajib untuk konfirmasi</span> : null}
      </h3>
      {files.length === 0 ? (
        <p className="mt-1 text-xs text-muted-foreground">{empty}</p>
      ) : (
        <ul className="mt-3 grid grid-cols-2 gap-3">
          {files.map((p) => (
            <li key={p.id} className="min-w-0">
              <a href={p.url} target="_blank" rel="noreferrer" className="block hover:no-underline">
                {p.mime.startsWith("image/") && (
                  <img
                    src={p.url}
                    alt={p.originalName || `Bukti #${p.id}`}
                    loading="lazy"
                    className="aspect-video w-full rounded-md border border-border object-cover"
                  />
                )}
                <span className="mt-1 block truncate text-xs font-medium text-primary hover:underline">
                  {p.originalName || `Bukti #${p.id}`}
                </span>
                <span className="block text-2xs text-muted-foreground">
                  {p.mime || "-"} · {formatBytes(p.sizeBytes)}
                </span>
              </a>
            </li>
          ))}
        </ul>
      )}
      {canUpload && (
        <FileDropzone className="mt-3" accept="image/*" busy={uploading} label={dropLabel} hint={dropHint} onFile={onFile} />
      )}
    </div>
  );
}
