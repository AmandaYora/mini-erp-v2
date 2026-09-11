import { useState } from "react";
import type { ChangeEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
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
  formatDateTime,
  formatIDR,
  statusLabel,
  statusTone,
} from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { paymentService } from "@/modules/payment/services/payment.service";
import type { Payment, PaymentProof } from "@/modules/payment/types";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import {
  directionLabel,
  methodLabel,
  orderTypeLabel,
} from "@/modules/payment/types";

export default function PaymentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("payment.create");
  const canCancel = can("payment.archive");

  const paymentId = Number(id);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [acting, setActing] = useState(false);
  const [uploading, setUploading] = useState(false);

  const { data, loading, error, reload } = useAsyncData(
    async () => {
      if (!Number.isInteger(paymentId) || paymentId <= 0) {
        throw new Error("ID pembayaran tidak valid.");
      }
      try {
        const [p, pr] = await Promise.all([
          paymentService.get(paymentId),
          paymentService.proofs(paymentId),
        ]);
        return { payment: p, proofs: pr };
      } catch (err) {
        const apiErr = toApiError(err);
        toast.danger("Gagal memuat pembayaran", apiErr.message);
        throw err;
      }
    },
    [paymentId],
  );
  const payment: Payment | null = data?.payment ?? null;
  const proofs: PaymentProof[] = data?.proofs ?? [];

  async function handleCancel() {
    setConfirmOpen(false);
    setActing(true);
    try {
      const res = await paymentService.cancel(paymentId);
      toast.fromServer(res.message, "Pembayaran dibatalkan", res.data.number);
      reload();
    } catch (err) {
      toast.danger("Gagal membatalkan", toApiError(err).message);
    } finally {
      setActing(false);
    }
  }

  async function handleUpload(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setUploading(true);
    try {
      const res = await paymentService.uploadProof(paymentId, file);
      toast.fromServer(res.message, "Bukti bayar diunggah");
      reload();
    } catch (err) {
      toast.danger("Gagal mengunggah bukti", toApiError(err).message);
    } finally {
      setUploading(false);
    }
  }

  if (loading && !payment && !error) {
    return <p className="text-muted py-8 text-center text-sm">Memuat…</p>;
  }

  if (error && !payment) {
    return (
      <div>
        <PageHeader
          eyebrow="Pembayaran"
          title="Detail Pembayaran"
          actions={
            <Button
              variant="secondary"
              onClick={() => void navigate(ROUTE_PATHS.payments)}
            >
              Kembali
            </Button>
          }
        />
        <Notice tone="danger" title={error}>
          <button
            type="button"
            onClick={() => reload()}
            className="cursor-pointer font-semibold text-brand hover:underline"
          >
            Coba lagi
          </button>
        </Notice>
      </div>
    );
  }

  if (!payment) return null;

  const cancellable = payment.status === "active";
  const allocTotal = payment.allocations.reduce((s, a) => s + a.amount, 0);

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        eyebrow="Pembayaran"
        title={payment.number}
        description={`${payment.partyName || "-"} • ${formatIDR(payment.amount)}`}
        actions={
          <>
            <Link to={`/print/payments/${payment.id}`} target="_blank" rel="noreferrer">
              <Button variant="secondary">Cetak</Button>
            </Link>
            <Button
              variant="secondary"
              onClick={() => void navigate(ROUTE_PATHS.payments)}
            >
              Kembali
            </Button>
            {canCancel && cancellable && (
              <Button
                variant="danger"
                disabled={acting}
                onClick={() => setConfirmOpen(true)}
              >
                Batalkan
              </Button>
            )}
          </>
        }
      />

      <SectionCard title="Informasi">
        <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <dt className="text-muted text-[0.8rem]">Pihak</dt>
            <dd className="mt-0.5 font-medium text-ink">
              {payment.partyName || "-"}
            </dd>
          </div>
          <div>
            <dt className="text-muted text-[0.8rem]">Arah</dt>
            <dd className="mt-0.5 font-medium text-ink">
              {directionLabel(payment.direction)}
            </dd>
          </div>
          <div>
            <dt className="text-muted text-[0.8rem]">Metode</dt>
            <dd className="mt-0.5 font-medium text-ink">
              {methodLabel(payment.method)}
            </dd>
          </div>
          <div>
            <dt className="text-muted text-[0.8rem]">Tanggal Bayar</dt>
            <dd className="mt-0.5 font-medium text-ink">
              {formatDateTime(payment.paidAt)}
            </dd>
          </div>
          <div>
            <dt className="text-muted text-[0.8rem]">Jumlah</dt>
            <dd className="mt-0.5 font-medium text-ink">
              {formatIDR(payment.amount)}
            </dd>
          </div>
          <div>
            <dt className="text-muted text-[0.8rem]">Status</dt>
            <dd className="mt-0.5">
              <Badge tone={statusTone(payment.status)}>
                {statusLabel(payment.status)}
              </Badge>
            </dd>
          </div>
          <div className="sm:col-span-2">
            <dt className="text-muted text-[0.8rem]">Catatan</dt>
            <dd className="mt-0.5 text-ink">{payment.notes || "-"}</dd>
          </div>
        </dl>
      </SectionCard>

      <SectionCard
        title="Alokasi"
        description={`Total teralokasi ${formatIDR(allocTotal)}.`}
      >
        <DataTable
          rows={payment.allocations}
          rowKey={(a) => `${a.orderType}-${a.orderId}-${a.amount}`}
          emptyTitle="Tanpa alokasi"
          columns={[
            {
              header: "Nomor Dokumen",
              render: (a) => a.orderNumber || `#${a.orderId}`,
            },
            { header: "Tipe", render: (a) => orderTypeLabel(a.orderType) },
            {
              header: "Nominal",
              align: "right",
              render: (a) => (
                <span className="font-medium">{formatIDR(a.amount)}</span>
              ),
            },
          ]}
        />
      </SectionCard>

      <SectionCard
        title="Bukti Bayar"
        description="Foto/slip transfer pembayaran ini."
        actions={
          canCreate && cancellable ? (
            <label className="inline-flex cursor-pointer items-center justify-center rounded-md border border-hairline bg-surface px-3 py-1.5 text-sm font-medium text-ink hover:bg-surface-subtle">
              {uploading ? "Mengunggah…" : "Unggah Bukti"}
              <input
                type="file"
                accept="image/*"
                className="hidden"
                disabled={uploading}
                onChange={(e) => void handleUpload(e)}
              />
            </label>
          ) : undefined
        }
      >
        <DataTable<PaymentProof>
          rows={proofs}
          rowKey={(p) => p.id}
          emptyTitle="Belum ada bukti bayar"
          emptyDescription="Unggah foto slip/kwitansi sebagai bukti."
          columns={[
            {
              header: "Nama File",
              render: (p) => (
                <a
                  href={p.url}
                  target="_blank"
                  rel="noreferrer"
                  className="font-medium text-brand hover:underline"
                >
                  {p.originalName}
                </a>
              ),
            },
            { header: "Tipe", render: (p) => p.mime || "-" },
            {
              header: "Ukuran",
              align: "right",
              render: (p) => `${(p.sizeBytes / 1024).toFixed(1)} KB`,
            },
          ]}
        />
      </SectionCard>

      <p className="text-sm">
        <Link
          to={ROUTE_PATHS.payments}
          className="font-medium text-brand hover:underline"
        >
          ← Kembali ke daftar pembayaran
        </Link>
      </p>

      <ConfirmDialog
        open={confirmOpen}
        title="Batalkan Pembayaran"
        message={`Batalkan pembayaran ${payment.number} sebesar ${formatIDR(payment.amount)}? Alokasi ke dokumen ikut terlepas.`}
        confirmLabel={acting ? "Memproses…" : "Ya, Batalkan"}
        onConfirm={() => void handleCancel()}
        onCancel={() => setConfirmOpen(false)}
      />
    </div>
  );
}
