import { useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, Modal, TextArea } from "@/shared/components/ui";
import { reopenPeriodSchema } from "@/modules/finance/schemas/finance.schema";

interface ReopenPeriodModalProps {
  periodLabel: string;
  scopeLabel: string;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}

// Membuka kembali periode terkunci wajib beralasan (POST /periods/reopen {year, month, reason}, FN-05);
// alasan tersimpan pada diff audit.
export function ReopenPeriodModal({ periodLabel, scopeLabel, submitting, onClose, onSubmit }: ReopenPeriodModalProps) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = reopenPeriodSchema.safeParse({ reason });
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
      title="Buka ulang periode?"
      description={`Periode ${periodLabel} (${scopeLabel}) akan dibuka kembali. Alasan dicatat pada jejak audit.`}
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Kembali
          </Button>
          <Button type="submit" form="reopen-period-form" loading={submitting}>
            Buka ulang
          </Button>
        </>
      }
    >
      <form id="reopen-period-form" onSubmit={handleSubmit} noValidate>
        <FormField label="Alasan" htmlFor="reopen-period-reason" required errorText={error}>
          <TextArea
            id="reopen-period-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            maxLength={255}
            placeholder="Mengapa periode ini dibuka kembali?"
          />
        </FormField>
      </form>
    </Modal>
  );
}
