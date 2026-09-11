import { useState } from "react";
import type { ChangeEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
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
  formatDateTime,
  formatNumber,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { deliveryService } from "@/modules/delivery/services/delivery.service";
import type {
  DeliveryItem,
  DeliveryNote,
  DeliveryProof,
  SalesOrderOption,
} from "@/modules/delivery/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";

// Alur: draft → confirmed (stok keluar saat confirm) / cancelled.
// - Konfirmasi: POST /:id/confirm, gate delivery.create, hanya draft.
// - Batalkan: POST /:id/cancel, gate delivery.archive, hanya draft —
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

function textOrDash(v: string | null | undefined): string {
  return v && v.trim() !== "" ? v : "-";
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
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const noteId = Number(id);
  const canConfirm = can("delivery.create");
  const canCancel = can("delivery.archive");
  const canUpload = can("delivery.create");

  const [acting, setActing] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [recipientName, setRecipientName] = useState("");
  const [sigStatus, setSigStatus] = useState<"" | "signed" | "missing">("");
  const [sigReason, setSigReason] = useState("");
  const [confirmError, setConfirmError] = useState<string | null>(null);

  const { data, loading, error, reload } = useAsyncData(
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
  const so: SalesOrderOption | null = data?.so ?? null;

  function openConfirm() {
    setRecipientName("");
    setSigStatus("");
    setSigReason("");
    setConfirmError(null);
    setConfirmOpen(true);
  }

  async function handleConfirm() {
    // Cermin validateRecipient backend: status hanya signed|missing, bila
    // status diisi maka nama wajib, bila missing maka alasan wajib. Semua
    // kosong = konfirmasi polos (body kosong tetap valid).
    if (sigStatus !== "" && recipientName.trim() === "") {
      setConfirmError("Nama penerima wajib diisi bila ada status tanda terima.");
      return;
    }
    if (sigStatus === "missing" && sigReason.trim() === "") {
      setConfirmError("Alasan wajib diisi bila tanda tangan tidak ada.");
      return;
    }
    setConfirmError(null);
    setConfirmOpen(false);
    setActing(true);
    try {
      const bare =
        recipientName.trim() === "" &&
        sigStatus === "" &&
        sigReason.trim() === "";
      const res = await deliveryService.confirm(
        noteId,
        bare
          ? undefined
          : {
              recipientName:
                recipientName.trim() === "" ? undefined : recipientName.trim(),
              recipientSignatureStatus: sigStatus === "" ? undefined : sigStatus,
              recipientSignatureMissingReason:
                sigReason.trim() === "" ? undefined : sigReason.trim(),
            },
      );
      toast.fromServer(res.message, "Surat jalan dikonfirmasi", res.data.number);
      reload();
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
      const res = await deliveryService.cancel(noteId);
      toast.fromServer(res.message, "Surat jalan dibatalkan", res.data.number);
      reload();
    } catch (err) {
      toast.danger("Gagal membatalkan", toApiError(err).message);
    } finally {
      setActing(false);
    }
  }

  async function handleFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file || !note) return;
    setUploading(true);
    try {
      const res = await deliveryService.uploadProof(note.id, file);
      toast.fromServer(res.message, "Bukti kirim diunggah");
      reload();
    } catch (err) {
      toast.danger("Gagal mengunggah bukti", toApiError(err).message);
    } finally {
      setUploading(false);
    }
  }

  if (loading && !note) {
    return (
      <div>
        <PageHeader eyebrow="Operasional" title="Detail Surat Jalan" />
        <p className="text-muted py-8 text-center text-sm">Memuat…</p>
      </div>
    );
  }

  if (error || !note) {
    return (
      <div>
        <PageHeader eyebrow="Operasional" title="Detail Surat Jalan" />
        <Notice tone="danger" title={error ?? "Surat jalan tidak ditemukan"} />
        <div className="mt-4 flex gap-3">
          <Button
            variant="secondary"
            onClick={() => void navigate(ROUTE_PATHS.deliveries)}
          >
            Kembali ke Daftar
          </Button>
          {error && <Button onClick={() => reload()}>Coba lagi</Button>}
        </div>
      </div>
    );
  }

  const isDraft = note.status === "draft";
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

  return (
    <div>
      <PageHeader
        eyebrow="Operasional"
        title={note.number}
        description={`${note.orderNumber || "-"} · ${formatDate(note.deliveryDate)}`}
        actions={
          <>
            <Link
              to={`/print/deliveries/${note.id}`}
              target="_blank"
              rel="noreferrer"
            >
              <Button variant="secondary">Cetak</Button>
            </Link>
            <Button
              variant="secondary"
              onClick={() => void navigate(ROUTE_PATHS.deliveries)}
            >
              Daftar
            </Button>
          </>
        }
      />

      <div className="space-y-4">
        {isReplacement && (
          <Notice tone="info" title="Surat jalan pengganti retur tukar">
            Barang pengganti dari {replacementReturnId !== null ? `Retur #${replacementReturnId}` : "retur terkait"} —
            pendapatan tidak dijurnal otomatis untuk pengiriman ini.
          </Notice>
        )}
        <SectionCard
          title="Ringkasan"
          actions={
            <>
              {isReplacement && <Badge tone="info">Pengganti</Badge>}
              <Badge tone={statusTone(note.status)}>
                {statusLabel(note.status)}
              </Badge>
            </>
          }
        >
          <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm md:grid-cols-4">
            <div>
              <dt className="text-muted">Nomor</dt>
              <dd className="font-semibold text-ink">{note.number}</dd>
            </div>
            <div>
              <dt className="text-muted">No. SO</dt>
              <dd className="font-semibold text-ink">
                <Link
                  to={`${ROUTE_PATHS.salesOrders}/${note.salesOrderId}`}
                  className="text-brand hover:underline"
                >
                  {note.orderNumber || `#${note.salesOrderId}`}
                </Link>
              </dd>
            </div>
            <div>
              <dt className="text-muted">Tanggal Kirim</dt>
              <dd className="text-ink">{formatDate(note.deliveryDate)}</dd>
            </div>
            <div>
              <dt className="text-muted">Customer</dt>
              <dd className="text-ink">{so?.partyName || "-"}</dd>
            </div>
            {isReplacement && (
              <div>
                <dt className="text-muted">Retur Terkait</dt>
                <dd className="font-semibold text-ink">
                  {replacementReturnId !== null ? `Retur #${replacementReturnId}` : "-"}
                </dd>
              </div>
            )}
            {note.notes.trim() !== "" && (
              <div className="col-span-2 md:col-span-4">
                <dt className="text-muted">Catatan</dt>
                <dd className="text-ink">{note.notes}</dd>
              </div>
            )}
          </dl>
        </SectionCard>

        <SectionCard
          title="Dokumen & Serah Terima"
          description="Supir, kendaraan, petugas, dan bukti terima dari backend."
        >
          <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm md:grid-cols-4">
            <div>
              <dt className="text-muted">Supir</dt>
              <dd className="text-ink">{textOrDash(note.driverName)}</dd>
            </div>
            <div>
              <dt className="text-muted">Plat Kendaraan</dt>
              <dd className="text-ink">{textOrDash(note.vehiclePlate)}</dd>
            </div>
            <div>
              <dt className="text-muted">Petugas Gudang</dt>
              <dd className="text-ink">{textOrDash(note.warehouseStaffName)}</dd>
            </div>
            <div>
              <dt className="text-muted">Penerima</dt>
              <dd className="text-ink">{textOrDash(note.recipientName)}</dd>
            </div>
            <div>
              <dt className="text-muted">Status Tanda Tangan</dt>
              <dd className="text-ink">
                {signatureLabel(note.recipientSignatureStatus)}
              </dd>
            </div>
            {note.recipientSignatureStatus === "missing" && (
              <div>
                <dt className="text-muted">Alasan Tanda Tangan Hilang</dt>
                <dd className="text-ink">
                  {textOrDash(note.recipientSignatureMissingReason)}
                </dd>
              </div>
            )}
            <div>
              <dt className="text-muted">Diberangkatkan</dt>
              <dd className="text-ink">
                {note.dispatchedAt
                  ? `${formatDateTime(note.dispatchedAt)}${note.dispatchedBy ? ` · #${note.dispatchedBy}` : ""}`
                  : "-"}
              </dd>
            </div>
            <div>
              <dt className="text-muted">Dikonfirmasi</dt>
              <dd className="text-ink">
                {note.confirmedAt
                  ? `${formatDateTime(note.confirmedAt)}${note.confirmedBy ? ` · #${note.confirmedBy}` : ""}`
                  : "-"}
              </dd>
            </div>
            {note.dropLocationNote.trim() !== "" && (
              <div className="col-span-2 md:col-span-4">
                <dt className="text-muted">Catatan Turun Barang</dt>
                <dd className="text-ink">{note.dropLocationNote}</dd>
              </div>
            )}
          </dl>
        </SectionCard>

        <SectionCard
          title={`Barang (${note.items.length})`}
          description="Qty dipesan dari SO (best-effort) vs qty surat jalan ini."
        >
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
                header: "Qty Dipesan (SO)",
                align: "right",
                render: (r) => {
                  const match = soLineOf(r);
                  return match ? formatNumber(match.qty) : "-";
                },
              },
              {
                header: "Qty Kirim",
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

        <SectionCard
          title={`Bukti Kirim (${proofs.length})`}
          description="Foto bukti pengiriman (hanya gambar)."
          actions={
            canUpload ? (
              <label
                className={`inline-flex cursor-pointer items-center justify-center rounded-md bg-brand px-4 py-2 text-[0.95rem] font-medium text-white transition-colors hover:bg-brand-hover ${uploading ? "pointer-events-none opacity-50" : ""}`}
              >
                {uploading ? "Mengunggah…" : "Unggah Foto"}
                <input
                  type="file"
                  accept="image/*"
                  className="hidden"
                  disabled={uploading}
                  onChange={(e) => void handleFile(e)}
                />
              </label>
            ) : undefined
          }
        >
          {proofs.length === 0 ? (
            <p className="text-muted py-4 text-center text-sm">
              Belum ada bukti kirim.
            </p>
          ) : (
            <ul className="divide-y divide-hairline">
              {proofs.map((p) => (
                <li
                  key={p.id}
                  className="flex flex-wrap items-center justify-between gap-3 py-3"
                >
                  <div>
                    <a
                      href={p.url}
                      target="_blank"
                      rel="noreferrer"
                      className="font-medium text-brand hover:underline"
                    >
                      {p.originalName || `Bukti #${p.id}`}
                    </a>
                    <p className="text-muted text-xs">
                      {p.mime || "-"} · {formatBytes(p.sizeBytes)}
                    </p>
                  </div>
                  <a
                    href={p.url}
                    target="_blank"
                    rel="noreferrer"
                    className="font-medium text-brand hover:underline"
                  >
                    Lihat
                  </a>
                </li>
              ))}
            </ul>
          )}
        </SectionCard>

        {isDraft && (canConfirm || canCancel) && (
          <SectionCard
            title="Aksi"
            description="Konfirmasi menggerakkan stok keluar; pembatalan hanya untuk draf."
          >
            <div className="flex flex-wrap gap-3">
              {canConfirm && (
                <Button onClick={openConfirm} disabled={acting}>
                  Konfirmasi
                </Button>
              )}
              {canCancel && (
                <Button
                  variant="danger"
                  onClick={() => setCancelOpen(true)}
                  disabled={acting}
                >
                  Batalkan
                </Button>
              )}
            </div>
          </SectionCard>
        )}
      </div>

      <Modal
        open={confirmOpen}
        title="Konfirmasi Surat Jalan"
        description={`Konfirmasi ${note.number}? Stok keluar dicatat dan tidak bisa dibatalkan (koreksi via retur). Isian penerima opsional — kosongkan semua untuk konfirmasi polos.`}
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
            <Button onClick={() => void handleConfirm()} disabled={acting}>
              {acting ? "Mengonfirmasi…" : "Konfirmasi"}
            </Button>
          </>
        }
      >
        {confirmError && (
          <div className="mb-4">
            <Notice tone="danger" title={confirmError} />
          </div>
        )}
        <div className="grid gap-4 md:grid-cols-2">
          <FormField label="Nama Penerima">
            <TextInput
              value={recipientName}
              maxLength={150}
              onChange={(e) => setRecipientName(e.target.value)}
              placeholder="Nama penerima (opsional)"
            />
          </FormField>
          <FormField label="Status Tanda Tangan">
            <SelectInput
              value={sigStatus}
              onChange={(e) =>
                setSigStatus(e.target.value as "" | "signed" | "missing")
              }
            >
              <option value="">Tanpa status</option>
              <option value="signed">Ditandatangani</option>
              <option value="missing">Tidak ada</option>
            </SelectInput>
          </FormField>
        </div>
        {sigStatus === "missing" && (
          <div className="mt-4">
            <FormField label="Alasan Tanda Tangan Hilang" required>
              <TextArea
                rows={2}
                value={sigReason}
                onChange={(e) => setSigReason(e.target.value)}
                placeholder="Alasan tidak ada tanda tangan"
              />
            </FormField>
          </div>
        )}
      </Modal>
      <ConfirmDialog
        open={cancelOpen}
        title="Batalkan Surat Jalan"
        message={`Batalkan ${note.number}? Tindakan ini tidak bisa dibatalkan.`}
        confirmLabel="Batalkan Surat Jalan"
        tone="danger"
        onConfirm={() => void handleCancel()}
        onCancel={() => setCancelOpen(false)}
      />
    </div>
  );
}
