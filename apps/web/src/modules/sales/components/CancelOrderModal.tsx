import { useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, Modal, TextArea } from "@/shared/components/ui";
import { requiredReasonSchema } from "@/modules/sales/schemas/order-correction.schema";

interface CancelOrderModalProps {
  number: string;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}

// Batal order jual dengan alasan (POST /sales-orders/:id/cancel {reason}, maks 255).
// Server menolak bila masih ada surat jalan, pembayaran, atau retur aktif.
export function CancelOrderModal({ number, submitting, onClose, onSubmit }: CancelOrderModalProps) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = requiredReasonSchema.safeParse(reason);
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message ?? "Alasan wajib diisi");
      return;
    }
    setError(undefined);
    onSubmit(parsed.data);
  }

  return (
    <Modal
      open
      size="sm"
      title="Batalkan order"
      description={`Order ${number} dibatalkan dan tidak bisa diproses lagi. Bila masih ada surat jalan, pembayaran, atau retur aktif, batalkan dokumen itu dulu.`}
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Kembali
          </Button>
          <Button type="submit" form="cancel-order-form" variant="danger" loading={submitting}>
            Batalkan order
          </Button>
        </>
      }
    >
      <form id="cancel-order-form" onSubmit={handleSubmit} noValidate>
        <FormField label="Alasan" htmlFor="cancel-order-reason" required errorText={error}>
          <TextArea
            id="cancel-order-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            maxLength={255}
            placeholder="Mengapa order ini dibatalkan?"
          />
        </FormField>
      </form>
    </Modal>
  );
}
