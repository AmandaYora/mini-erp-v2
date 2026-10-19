import { useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, Modal, TextArea } from "@/shared/components/ui";
import { cancelExpenseSchema } from "@/modules/finance/schemas/finance.schema";

interface CancelExpenseModalProps {
  number: string;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}

// Batal biaya membutuhkan alasan (POST /expenses/:id/cancel {reason}, maks 255) — dialog kecil.
export function CancelExpenseModal({ number, submitting, onClose, onSubmit }: CancelExpenseModalProps) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = cancelExpenseSchema.safeParse({ reason });
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
      title="Batalkan biaya"
      description={`Biaya ${number} dibatalkan dan jurnalnya dibalik. Alasan dicatat pada biaya dan jejak audit.`}
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Kembali
          </Button>
          <Button type="submit" form="cancel-expense-form" variant="danger" loading={submitting}>
            Batalkan biaya
          </Button>
        </>
      }
    >
      <form id="cancel-expense-form" onSubmit={handleSubmit} noValidate>
        <FormField label="Alasan" htmlFor="cancel-expense-reason" required errorText={error}>
          <TextArea
            id="cancel-expense-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            maxLength={255}
            placeholder="Mengapa biaya ini dibatalkan?"
          />
        </FormField>
      </form>
    </Modal>
  );
}
