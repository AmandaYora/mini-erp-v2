import { useState } from "react";
import type { FormEvent } from "react";
import { Button, DataTable, FormField, LoadingState, Modal, TextArea } from "@/shared/components/ui";
import { Money, Qty } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { formatIDR } from "@/shared/lib/format";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { salesService } from "@/modules/sales/services/sales.service";
import { requiredReasonSchema } from "@/modules/sales/schemas/order-correction.schema";
import { shortCloseSummary } from "@/modules/sales/lib/order-correction";
import type { ShortCloseLine } from "@/modules/sales/types";

interface ShortCloseModalProps {
  orderId: number;
  number: string;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}

// "Tutup sisa order": order yang baru terkirim sebagian ditutup — qty tiap baris
// dipotong ke yang sudah terkirim, total dihitung ulang, order selesai. Angka di
// sini dari pratinjau server (builder yang sama dengan eksekusinya).
export function ShortCloseModal({ orderId, number, submitting, onClose, onSubmit }: ShortCloseModalProps) {
  const { data: plan, loading, error } = useAsyncData(() => salesService.closeShortPreview(orderId), [orderId]);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState<string | undefined>(undefined);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = requiredReasonSchema.safeParse(reason);
    if (!parsed.success) {
      setReasonError(parsed.error.issues[0]?.message ?? "Alasan wajib diisi");
      return;
    }
    setReasonError(undefined);
    onSubmit(parsed.data);
  }

  const summary = plan ? shortCloseSummary(plan) : null;

  return (
    <Modal
      open
      size="lg"
      title="Tutup sisa order"
      description={`Sisa ${number} yang belum dikirim tidak akan dikirim lagi. Order ditagih sebesar barang yang sudah terkirim, lalu selesai.`}
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Kembali
          </Button>
          <Button type="submit" form="short-close-form" variant="danger" loading={submitting} disabled={!plan}>
            Tutup sisa order
          </Button>
        </>
      }
    >
      {loading && <LoadingState />}
      {error && (
        <Notice tone="danger" title="Sisa order belum bisa ditutup">
          {error}
        </Notice>
      )}
      {plan && summary && (
        <form id="short-close-form" onSubmit={handleSubmit} noValidate className="space-y-4">
          <DataTable<ShortCloseLine>
            rows={plan.lines}
            rowKey={(r) => r.itemId}
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
                      {r.removed ? " · dihapus dari order" : ""}
                    </p>
                  </div>
                ),
              },
              { header: "Dipesan", align: "right", render: (r) => <Qty value={r.orderedQty} unit={r.uom} /> },
              { header: "Terkirim", align: "right", render: (r) => <Qty value={r.deliveredQty} unit={r.uom} /> },
              {
                header: "Total lama",
                align: "right",
                render: (r) => <Money value={r.oldLineTotal} className="text-muted-foreground" />,
              },
              {
                header: "Total baru",
                align: "right",
                render: (r) => <Money value={r.removed ? 0 : r.newLineTotal} className="font-medium" />,
              },
            ]}
          />

          <dl className="grid grid-cols-2 gap-x-6 gap-y-1.5 rounded-lg border border-border p-3.5 text-sm">
            <dt className="text-muted-foreground">Total order sekarang</dt>
            <dd className="text-right tabular">{formatIDR(plan.oldGrandTotal)}</dd>
            <dt className="text-muted-foreground">Total setelah ditutup</dt>
            <dd className="text-right font-semibold tabular">{formatIDR(plan.newGrandTotal)}</dd>
            <dt className="text-muted-foreground">Sudah dibayar</dt>
            <dd className="text-right tabular">{formatIDR(plan.paidAmount)}</dd>
            <dt className="text-muted-foreground">Sisa tagihan</dt>
            <dd className="text-right tabular">{formatIDR(plan.outstandingAfter)}</dd>
          </dl>

          {plan.refundDueAfter > 0 && (
            <Notice tone="warning" title="Pelanggan sudah membayar lebih">
              Setelah ditutup, kelebihan bayar {formatIDR(plan.refundDueAfter)} harus dikembalikan ke pelanggan
              atau disimpan sebagai saldo. Order tetap bisa ditutup; kelebihan ini tampil di ringkasan tagihan.
            </Notice>
          )}
          {summary.removed > 0 && (
            <p className="text-sm text-muted-foreground">
              {summary.removed} baris tanpa kiriman dihapus dari order; isinya tetap tercatat di jejak audit.
            </p>
          )}

          <FormField label="Alasan" htmlFor="short-close-reason" required errorText={reasonError}>
            <TextArea
              id="short-close-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={2}
              maxLength={255}
              placeholder="Mis. stok habis, pelanggan membatalkan sisa"
            />
          </FormField>
        </form>
      )}
    </Modal>
  );
}
