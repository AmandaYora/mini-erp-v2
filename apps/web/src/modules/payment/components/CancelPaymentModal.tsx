import { useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, Modal, TextArea } from "@/shared/components/ui";
import { formatIDR } from "@/shared/lib/format";
import { cancelPaymentSchema } from "@/modules/payment/schemas/payment.schema";

interface CancelPaymentModalProps {
  number: string;
  amount: number;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}

// Batal pembayaran membutuhkan alasan (POST /payments/:id/cancel {reason}, maks 255).
export function CancelPaymentModal({ number, amount, submitting, onClose, onSubmit }: CancelPaymentModalProps) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = cancelPaymentSchema.safeParse({ reason });
    if (!parsed.success) {
      setError(parsed.error.flatten().fieldErrors.reason?.[0] ?? "Alasan wajib diisi");
      return;
    }
    setError(undefined);
    onSubmit(parsed.data.reason);
  }

  return (
    <Modal
      open
      size="sm"
      title="Batalkan pembayaran"
      description={`Pembayaran ${number} sebesar ${formatIDR(amount)} dibatalkan dan alokasinya ke dokumen terlepas. Bila hanya salah input, gunakan Koreksi pembayaran.`}
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Kembali
          </Button>
          <Button type="submit" form="cancel-payment-form" variant="danger" loading={submitting}>
            Batalkan pembayaran
          </Button>
        </>
      }
    >
      <form id="cancel-payment-form" onSubmit={handleSubmit} noValidate>
        <FormField label="Alasan" htmlFor="cancel-payment-reason" required errorText={error}>
          <TextArea
            id="cancel-payment-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            maxLength={255}
            placeholder="Mengapa pembayaran ini dibatalkan?"
          />
        </FormField>
      </form>
    </Modal>
  );
}
