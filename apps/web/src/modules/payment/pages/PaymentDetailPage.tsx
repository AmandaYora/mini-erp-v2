import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Button,
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
  FileDropzone,
  Money,
  Page,
  PageColumns,
} from "@/shared/components/patterns";
import type { DescriptionItem, RelatedDocument } from "@/shared/components/patterns";
import { NotFoundState } from "@/shared/components/feedback/not-found-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDateTime, formatIDR } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { usePageCrumb } from "@/shared/stores/page-meta.store";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { paymentService } from "@/modules/payment/services/payment.service";
import type { Payment, PaymentAllocation, PaymentProof } from "@/modules/payment/types";
import type { PaymentCorrectValues } from "@/modules/payment/schemas/payment.schema";
import { CancelPaymentModal } from "@/modules/payment/components/CancelPaymentModal";
import { CorrectPaymentModal } from "@/modules/payment/components/CorrectPaymentModal";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import {
  cancelGuardHint,
  directionLabel,
  isCorrectable,
  methodLabel,
  orderTypeLabel,
  paymentKindLabel,
  referenceLabel,
} from "@/modules/payment/types";
import { allocationHref, paymentRail } from "@/modules/payment/lib/payment-rail";

export default function PaymentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);
  const canCreate = can("payment.create");
  const canCancel = can("payment.archive");

  const paymentId = Number(id);
  const invalidId = !Number.isInteger(paymentId) || paymentId <= 0;

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [acting, setActing] = useState(false);
  const [correctOpen, setCorrectOpen] = useState(false);
  const [correcting, setCorrecting] = useState(false);
  const [correctError, setCorrectError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);

  const { data, loading, error, errorStatus, reload } = useAsyncData(async () => {
    if (invalidId) return null;
    const [p, pr] = await Promise.all([paymentService.get(paymentId), paymentService.proofs(paymentId)]);
    return { payment: p, proofs: pr };
  }, [paymentId]);
  const payment: Payment | null = data?.payment ?? null;
  const proofs: PaymentProof[] = data?.proofs ?? [];

  usePageCrumb(payment?.number);

  async function handleCancel(reason: string) {
    setActing(true);
    try {
      const res = await paymentService.cancel(paymentId, reason);
      toast.fromServer(res.message, "Pembayaran dibatalkan", res.data.number);
      setConfirmOpen(false);
      reload();
    } catch (err) {
      // Penjaga backend (COD/prepaid selesai) → arahkan ke Retur.
      const msg = toApiError(err).message;
      const hint = cancelGuardHint(msg);
      toast.danger("Gagal membatalkan", hint ? `${msg} ${hint}` : msg);
    } finally {
      setActing(false);
    }
  }

  // Koreksi salah input: server membuat pengganti lalu membatalkan yang ini → buka penggantinya.
  async function handleCorrect(values: PaymentCorrectValues) {
    setCorrecting(true);
    setCorrectError(null);
    try {
      const res = await paymentService.correct(paymentId, values);
      toast.fromServer(res.message, "Pembayaran dikoreksi", res.data.number);
      setCorrectOpen(false);
      void navigate(`${ROUTE_PATHS.payments}/${res.data.id}`);
    } catch (err) {
      setCorrectError(toApiError(err).message);
    } finally {
      setCorrecting(false);
    }
  }

  async function handleUpload(file: File) {
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

  if (invalidId || errorStatus === 404) {
    return (
      <NotFoundState
        title="Pembayaran tidak ditemukan"
        message="Pembayaran ini tidak ada atau sudah dihapus."
        backTo={ROUTE_PATHS.payments}
        backLabel="Ke daftar pembayaran"
      />
    );
  }

  if (loading && !payment && !error) return <DetailSkeleton />;

  if (error && !payment) {
    return (
      <Page>
        <PageHeader title="Pembayaran" />
        <ErrorState title="Gagal memuat pembayaran" message={error} onRetry={reload} />
      </Page>
    );
  }

  if (!payment) return null;

  const cancellable = payment.status === "active";
  const correctable = canCreate && canCancel && isCorrectable(payment);
  const menuItems = [
    ...(correctable
      ? [{ key: "correct", label: "Koreksi pembayaran", disabled: acting, onSelect: () => setCorrectOpen(true) }]
      : []),
    ...(canCancel && cancellable
      ? [
          {
            key: "cancel",
            label: "Batalkan pembayaran",
            destructive: true,
            disabled: acting,
            onSelect: () => setConfirmOpen(true),
          },
        ]
      : []),
  ];
  const allocTotal = payment.allocations.reduce((s, a) => s + a.amount, 0);
  const rail = paymentRail(payment);

  const related: RelatedDocument[] = [];
  for (const a of payment.allocations) {
    const to = allocationHref(a.orderType, a.orderId);
    if (to && !related.some((r) => r.to === to)) {
      related.push({ label: a.orderNumber || `#${a.orderId}`, kind: orderTypeLabel(a.orderType), to });
    }
  }

  const info: DescriptionItem[] = [
    { label: "Arah", value: directionLabel(payment.direction) },
    { label: "Metode", value: methodLabel(payment.method) },
    { label: "Jenis", value: paymentKindLabel(payment) },
    { label: "Tanggal bayar", value: formatDateTime(payment.paidAt) },
  ];
  if (payment.method !== "cash" && payment.reference) {
    info.push({ label: referenceLabel(payment.method), value: payment.reference });
  }
  info.push({ label: "Catatan", value: payment.notes, wide: true });

  return (
    <Page>
      <PageHeader
        title={payment.number}
        meta={<StatusBadge status={payment.status} />}
        subtitle={`${payment.partyName || "-"} · ${formatDateTime(payment.paidAt)}`}
        actions={
          <>
            <Button variant="secondary" asChild>
              <Link to={`/print/payments/${payment.id}`} target="_blank" rel="noreferrer">
                Cetak
              </Link>
            </Button>
            {menuItems.length > 0 && <ActionMenu items={menuItems} />}
          </>
        }
      >
        <DocumentRail steps={rail.steps} current={rail.current} stopped={rail.stopped} related={related} />
      </PageHeader>

      {(payment.correctsPaymentId ?? 0) > 0 && (
        <Notice tone="info" title="Koreksi dari pembayaran lain">
          Pembayaran ini menggantikan{" "}
          <Link to={`${ROUTE_PATHS.payments}/${payment.correctsPaymentId}`} className="font-medium text-primary hover:underline">
            {payment.correctsPaymentNumber || `#${payment.correctsPaymentId}`}
          </Link>
          , yang dibatalkan karena salah input.
        </Notice>
      )}

      {payment.status === "cancelled" && (
        <Notice tone="neutral" title={(payment.correctedById ?? 0) > 0 ? "Dikoreksi" : "Pembayaran ini sudah dibatalkan"}>
          {(payment.correctedById ?? 0) > 0 && (
            <>
              Dikoreksi oleh{" "}
              <Link to={`${ROUTE_PATHS.payments}/${payment.correctedById}`} className="font-medium text-primary hover:underline">
                {payment.correctedByNumber || `#${payment.correctedById}`}
              </Link>
              .{" "}
            </>
          )}
          {payment.cancelReason ? `Alasan: ${payment.cancelReason}` : "Alokasinya ke dokumen sudah terlepas."}
          {payment.cancelledAt ? ` (${formatDateTime(payment.cancelledAt)})` : ""}
        </Notice>
      )}

      {payment.pairId > 0 && (
        <Notice tone="info" title="Offset berpasangan">
          Membatalkan salah satu kaki otomatis membatalkan kaki lawannya.
        </Notice>
      )}

      <PageColumns
        main={
          <SectionCard
            flush
            title="Alokasi"
            description={`Dokumen yang dibayar. Total teralokasi ${formatIDR(allocTotal)}.`}
          >
            <DataTable<PaymentAllocation>
              rows={payment.allocations}
              rowKey={(a) => `${a.orderType}-${a.orderId}-${a.amount}`}
              emptyTitle="Tanpa alokasi"
              emptyDescription="Pembayaran ini belum dialokasikan ke dokumen."
              columns={[
                {
                  header: "Dokumen",
                  render: (a) => {
                    const to = allocationHref(a.orderType, a.orderId);
                    const label = <DocNumber value={a.orderNumber || `#${a.orderId}`} />;
                    return to ? (
                      <Link to={to} className="font-medium text-primary hover:underline">
                        {label}
                      </Link>
                    ) : (
                      label
                    );
                  },
                },
                { header: "Jenis", render: (a) => orderTypeLabel(a.orderType) },
                {
                  header: "Nominal",
                  align: "right",
                  render: (a) => <Money value={a.amount} className="font-medium" />,
                },
              ]}
            />
          </SectionCard>
        }
        aside={
          <>
            <SectionCard title="Ringkasan">
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between gap-3">
                  <dt className="text-muted-foreground">Jumlah</dt>
                  <dd className="font-display text-xl font-semibold text-foreground">
                    <Money value={payment.amount} />
                  </dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt className="text-muted-foreground">Teralokasi</dt>
                  <dd>
                    <Money value={allocTotal} />
                  </dd>
                </div>
                {payment.method === "cash" && payment.amountTendered > 0 && (
                  <>
                    <div className="flex justify-between gap-3 border-t border-border pt-2">
                      <dt className="text-muted-foreground">Uang diterima</dt>
                      <dd>
                        <Money value={payment.amountTendered} />
                      </dd>
                    </div>
                    <div className="flex justify-between gap-3">
                      <dt className="text-muted-foreground">Kembalian</dt>
                      <dd>
                        <Money value={payment.change} />
                      </dd>
                    </div>
                  </>
                )}
              </dl>
            </SectionCard>

            <SectionCard title="Pihak">
              <p className="text-base font-medium text-foreground">{payment.partyName || "-"}</p>
              {payment.partyType && (
                <p className="text-sm text-muted-foreground">
                  {payment.partyType === "supplier" ? "Pemasok" : "Pelanggan"}
                </p>
              )}
            </SectionCard>

            <SectionCard title="Info pembayaran">
              <DescriptionList columns={1} items={info} />
            </SectionCard>

            <SectionCard title="Bukti bayar" description="Foto atau slip transfer pembayaran ini.">
              <div className="flex flex-col gap-3">
                {proofs.length === 0 ? (
                  <p className="text-sm text-muted-foreground">Belum ada bukti bayar.</p>
                ) : (
                  <ul className="flex flex-col gap-2">
                    {proofs.map((p) => (
                      <li key={p.id} className="flex items-baseline justify-between gap-3 text-sm">
                        <a
                          href={p.url}
                          target="_blank"
                          rel="noreferrer"
                          className="min-w-0 truncate font-medium text-primary hover:underline"
                        >
                          {p.originalName}
                        </a>
                        <span className="tabular flex-none text-xs text-muted-foreground">
                          {(p.sizeBytes / 1024).toFixed(1)} KB
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
                {canCreate && cancellable && (
                  <FileDropzone
                    accept="image/*"
                    busy={uploading}
                    label="Seret bukti bayar ke sini atau pilih"
                    hint="Gambar slip atau kwitansi"
                    onFile={(f) => void handleUpload(f)}
                  />
                )}
              </div>
            </SectionCard>
          </>
        }
      />

      {confirmOpen && (
        <CancelPaymentModal
          number={payment.number}
          amount={payment.amount}
          submitting={acting}
          onClose={() => setConfirmOpen(false)}
          onSubmit={(r) => void handleCancel(r)}
        />
      )}

      {correctOpen && (
        <CorrectPaymentModal
          payment={payment}
          submitting={correcting}
          serverError={correctError}
          onClose={() => {
            setCorrectOpen(false);
            setCorrectError(null);
          }}
          onSubmit={(v) => void handleCorrect(v)}
        />
      )}
    </Page>
  );
}
