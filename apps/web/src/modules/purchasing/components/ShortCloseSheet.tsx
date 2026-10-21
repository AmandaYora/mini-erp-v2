import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, DataTable, FormField, FormSheet, TextArea } from "@/shared/components/ui";
import { DescriptionList, Money, Qty } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { purchaseOrderService } from "@/modules/purchasing/services/purchase-order.service";
import { correctionReasonSchema } from "@/modules/purchasing/schemas/purchase-correction.schema";
import { BalanceAfter } from "@/modules/purchasing/components/BalanceAfter";
import type { PurchaseCorrectionPreviewLine } from "@/modules/purchasing/types";

interface ShortCloseSheetProps {
  open: boolean;
  orderId: number;
  orderNumber: string;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onConfirm: (reason: string) => void;
}

/**
 * "Tutup sisa order": barang yang belum datang tidak akan dikirim pemasok.
 * Qty tiap baris dipotong menjadi qty yang sudah diterima, order selesai.
 * Hutang sudah dicatat per penerimaan, jadi tidak ada jurnal baru.
 */
export function ShortCloseSheet({ open, orderId, orderNumber, submitting, serverError, onClose, onConfirm }: ShortCloseSheetProps) {
  const formId = useId();
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState<string | undefined>();
  const preview = useAsyncData(
    async () => (open ? purchaseOrderService.previewShortClose(orderId) : null),
    [open, orderId],
  );
  const pv = preview.data;

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = correctionReasonSchema.safeParse(reason);
    if (!parsed.success) {
      setReasonError(parsed.error.issues[0]?.message);
      return;
    }
    setReasonError(undefined);
    onConfirm(parsed.data);
  }

  return (
    <FormSheet
      open={open}
      title={`Tutup sisa order ${orderNumber}`}
      description="Sisa barang yang belum diterima dibatalkan; order selesai dengan qty yang sudah diterima."
      size="lg"
      onClose={onClose}
      actions={
        <>
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Batal
          </Button>
          <Button type="submit" form={formId} variant="danger" loading={submitting} disabled={!pv}>
            Tutup sisa order
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}
        {preview.loading && !pv && <p className="text-sm text-muted-foreground">Menghitung pratinjau…</p>}
        {preview.error && <Notice tone="danger" title="Sisa order tidak bisa ditutup">{preview.error}</Notice>}
        {pv && (
          <>
            <DataTable<PurchaseCorrectionPreviewLine>
              rows={pv.lines}
              rowKey={(r) => r.itemId}
              stickyHeader={false}
              emptyTitle="Tidak ada sisa order"
              columns={[
                { header: "Produk", render: (r) => <span className="font-medium">{r.productName}</span> },
                { header: "Dipesan", align: "right", render: (r) => <Qty value={r.qty} unit={r.uom} /> },
                { header: "Diterima", align: "right", render: (r) => <Qty value={r.receivedQty} unit={r.uom} /> },
                {
                  header: "Subtotal",
                  align: "right",
                  render: (r) => (
                    <div>
                      <Money value={r.newLineTotal} className="font-medium" />
                      <p className="text-xs text-muted-foreground line-through">
                        <Money value={r.oldLineTotal} />
                      </p>
                    </div>
                  ),
                },
              ]}
            />
            <DescriptionList
              columns={2}
              items={[
                { label: "Total order sekarang", value: <Money value={pv.oldGrandTotal} /> },
                { label: "Total setelah ditutup", value: <Money value={pv.newGrandTotal} className="font-semibold" /> },
                { label: "Sudah dibayar", value: <Money value={pv.paid} /> },
                { label: "Saldo pemasok", value: <BalanceAfter outstanding={pv.outstanding} /> },
              ]}
            />
          </>
        )}
        <FormField label="Alasan" required errorText={reasonError}>
          <TextArea
            rows={2}
            maxLength={500}
            value={reason}
            placeholder="Mis. pemasok kehabisan stok, sisa tidak dikirim"
            onChange={(e) => setReason(e.target.value)}
          />
        </FormField>
      </form>
    </FormSheet>
  );
}
