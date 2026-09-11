import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
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
import { salesService } from "@/modules/sales/services/sales.service";
import {
  channelLabel,
  paymentTermsLabel,
} from "@/modules/sales/types";
import type { SalesOrder, SalesOrderItem } from "@/modules/sales/types";

export default function SalesOrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const can = useAuthStore((s) => s.can);

  const orderId = Number(id);
  const invalidId = !Number.isInteger(orderId) || orderId <= 0;

  const canEdit = can("sales.update");
  const canConfirm = can("sales.update");
  const canCancel = can("sales.archive");
  const canApprove = can("sales.approve");

  const [order, setOrder] = useState<SalesOrder | null>(null);
  const [busy, setBusy] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [creditOpen, setCreditOpen] = useState(false);
  const [creditDue, setCreditDue] = useState(todayWIB());
  const [creditError, setCreditError] = useState<string | null>(null);

  const { data: fetchedOrder, loading, error: fetchError, reload } = useAsyncData(
    () => (invalidId ? Promise.resolve(null) : salesService.get(orderId)),
    [orderId],
  );
  const error = invalidId ? "ID order tidak valid." : fetchError;

  // Hasil fetch disalin ke state tampilan; aksi mutasi (runAction) menimpa
  // state yang sama tanpa fetch ulang.
  const [appliedFetch, setAppliedFetch] = useState<SalesOrder | null>(null);
  if (fetchedOrder !== appliedFetch) {
    setAppliedFetch(fetchedOrder);
    setOrder(fetchedOrder);
  }

  async function runAction(fn: () => Promise<MutateResult<SalesOrder>>, fallback: string, fail: string) {
    setBusy(true);
    try {
      const res = await fn();
      setOrder(res.data);
      toast.fromServer(res.message, fallback, res.data.number);
    } catch (err) {
      toast.danger(fail, toApiError(err).message);
    } finally {
      setBusy(false);
    }
  }

  function handleConfirm() {
    void runAction(
      () => salesService.confirm(orderId),
      "Order dikonfirmasi",
      "Gagal mengonfirmasi",
    );
  }

  function handleCancel() {
    setCancelOpen(false);
    void runAction(
      () => salesService.cancel(orderId),
      "Order dibatalkan",
      "Gagal membatalkan",
    );
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

  if (loading) {
    return <p className="text-muted py-10 text-center text-sm">Memuat…</p>;
  }

  if (error || !order) {
    return (
      <div className="space-y-4">
        <PageHeader title="Detail Order Jual" />
        <Notice tone="danger" title="Gagal memuat order">
          {error ?? "Data tidak ditemukan."}{" "}
          <button
            type="button"
            onClick={() => reload()}
            className="cursor-pointer font-semibold text-brand hover:underline"
          >
            Coba lagi
          </button>
        </Notice>
        <Button variant="secondary" onClick={() => navigate(ROUTE_PATHS.salesOrders)}>
          Kembali ke daftar
        </Button>
      </div>
    );
  }

  const isDraft = order.status === "draft";
  // Batal backend: draf atau terkonfirmasi (service.go CancelOrder).
  const cancellable = order.status === "draft" || order.status === "confirmed";
  // Setujui kredit backend: terkonfirmasi + termin cod (ApproveCredit).
  const approvable = order.status === "confirmed" && order.paymentTerms === "cod";

  return (
    <div>
      <PageHeader
        eyebrow="Order Jual"
        title={order.number}
        description={`${formatDate(order.orderDate)} · ${channelLabel(order.channel)}`}
        actions={
          <>
            <Link to={`/print/sales/${order.id}`} target="_blank" rel="noreferrer">
              <Button variant="secondary">Cetak</Button>
            </Link>
            {isDraft && canEdit && (
              <Link to={`${ROUTE_PATHS.salesOrders}/${order.id}/edit`}>
                <Button variant="secondary">Ubah</Button>
              </Link>
            )}
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
                <Badge tone={statusTone(order.status)}>{statusLabel(order.status)}</Badge>
              </dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Customer</dt>
              <dd className="mt-1 text-sm text-ink">{order.partyName || "Walk-in"}</dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Channel</dt>
              <dd className="mt-1">
                <Badge tone={order.channel === "pos" ? "info" : "neutral"}>
                  {channelLabel(order.channel)}
                </Badge>
              </dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Termin</dt>
              <dd className="mt-1 text-sm text-ink">{paymentTermsLabel(order.paymentTerms)}</dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Jatuh Tempo</dt>
              <dd className="mt-1 text-sm text-ink">{formatDate(order.dueDate)}</dd>
            </div>
            <div>
              <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Kode Member</dt>
              <dd className="mt-1 text-sm text-ink">{order.memberCode || "-"}</dd>
            </div>
            {order.notes && (
              <div className="sm:col-span-3">
                <dt className="text-muted text-[0.8rem] font-semibold tracking-wide uppercase">Catatan</dt>
                <dd className="mt-1 text-sm text-ink">{order.notes}</dd>
              </div>
            )}
          </dl>
        </SectionCard>

        <SectionCard title="Item">
          <DataTable<SalesOrderItem>
            rows={order.items}
            rowKey={(r) => r.id}
            emptyTitle="Tidak ada item"
            columns={[
              {
                header: "Produk",
                render: (r) => (
                  <div>
                    <p className="font-medium">{r.productName}</p>
                    <p className="text-muted text-xs">{r.productCode}</p>
                  </div>
                ),
              },
              { header: "Qty", align: "right", render: (r) => formatNumber(r.qty) },
              { header: "Satuan", render: (r) => r.uom },
              { header: "Harga", align: "right", render: (r) => formatIDR(r.unitPrice) },
              { header: "Total Baris", align: "right", render: (r) => formatIDR(r.lineTotal) },
            ]}
          />
        </SectionCard>

        <SectionCard title="Total (dihitung server)">
          <dl className="mx-auto max-w-md space-y-2">
            <div className="flex justify-between text-sm">
              <dt className="text-muted">Subtotal</dt>
              <dd className="text-ink">{formatIDR(order.subtotal)}</dd>
            </div>
            <div className="flex justify-between text-sm">
              <dt className="text-muted">Diskon</dt>
              <dd className="text-ink">{formatIDR(order.discountTotal)}</dd>
            </div>
            <div className="flex justify-between text-sm">
              <dt className="text-muted">Pajak</dt>
              <dd className="text-ink">{formatIDR(order.taxTotal)}</dd>
            </div>
            <div className="flex justify-between border-t border-hairline pt-2 text-base font-bold">
              <dt className="text-ink">Grand Total</dt>
              <dd className="text-ink">{formatIDR(order.grandTotal)}</dd>
            </div>
          </dl>
        </SectionCard>

        {approvable && canApprove && (
          <SectionCard
            title="Persetujuan Kredit"
            description="Order COD terkonfirmasi bisa dialihkan ke termin tempo."
            actions={
              <Button size="sm" onClick={() => {
                setCreditError(null);
                setCreditDue(todayWIB());
                setCreditOpen(true);
              }}>
                Setujui Kredit
              </Button>
            }
          >
            <p className="text-muted text-sm">
              Menyetujui kredit mengubah termin menjadi net dengan jatuh tempo yang ditentukan.
            </p>
          </SectionCard>
        )}

        <ActionRow align="start">
          <Button variant="secondary" onClick={() => navigate(ROUTE_PATHS.salesOrders)}>
            Kembali ke daftar
          </Button>
        </ActionRow>
      </div>

      <ConfirmDialog
        open={cancelOpen}
        title="Batalkan order?"
        message={`Order ${order.number} akan dibatalkan dan tidak bisa diproses lagi.`}
        confirmLabel="Ya, batalkan"
        onConfirm={handleCancel}
        onCancel={() => setCancelOpen(false)}
      />

      <Modal
        open={creditOpen}
        title="Setujui Kredit"
        description={`Alihkan order ${order.number} dari COD ke termin tempo.`}
        size="sm"
        onClose={() => setCreditOpen(false)}
      >
        <form onSubmit={handleApproveCredit}>
          <FormField label="Jatuh Tempo" required htmlFor="f-credit-due" errorText={creditError ?? undefined}>
            <DateInput
              id="f-credit-due"
              value={creditDue}
              onChange={(e) => setCreditDue(e.target.value)}
            />
          </FormField>
          <div className="mt-6 flex justify-end gap-3">
            <Button variant="secondary" size="sm" onClick={() => setCreditOpen(false)}>
              Batal
            </Button>
            <Button type="submit" size="sm" disabled={busy}>
              Setujui
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
