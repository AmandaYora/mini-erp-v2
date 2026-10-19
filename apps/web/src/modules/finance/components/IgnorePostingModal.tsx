import { useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, Modal, TextArea } from "@/shared/components/ui";
import { ignorePostingSchema } from "@/modules/finance/schemas/finance.schema";

interface IgnorePostingModalProps {
  number: string;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}

// Abaikan dokumen macet dari antrian posting (FN-01): alasan wajib, tersimpan di audit;
// dokumen bisa dipulihkan kapan saja dan tidak memblokir tutup periode.
export function IgnorePostingModal({ number, submitting, onClose, onSubmit }: IgnorePostingModalProps) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = ignorePostingSchema.safeParse({ reason });
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
      title="Abaikan dokumen?"
      description={`${number} dikeluarkan dari antrian posting dan tidak masuk pembukuan. Tidak lagi memblokir tutup periode (tampil sebagai peringatan). Bisa dipulihkan kapan saja.`}
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Kembali
          </Button>
          <Button type="submit" form="ignore-posting-form" variant="danger" loading={submitting}>
            Abaikan
          </Button>
        </>
      }
    >
      <form id="ignore-posting-form" onSubmit={handleSubmit} noValidate>
        <FormField label="Alasan" htmlFor="ignore-posting-reason" required errorText={error}>
          <TextArea
            id="ignore-posting-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            maxLength={255}
            placeholder="Mengapa dokumen ini tidak perlu dijurnal?"
          />
        </FormField>
      </form>
    </Modal>
  );
}
