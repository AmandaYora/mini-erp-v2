import { useState } from "react";
import type { FormEvent, ReactNode } from "react";
import { Button, FormField, Modal, TextArea } from "@/shared/components/ui";
import { correctionReasonSchema } from "@/modules/finance/schemas/corrections.schema";

interface CorrectionReasonModalProps {
  title: string;
  description: string;
  confirmLabel: string;
  submitting: boolean;
  /** Ringkasan apa yang akan dibukukan (angka utama), tampil di atas isian alasan. */
  children?: ReactNode;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}

// Konfirmasi koreksi harga modal: alasan wajib (maks 500), tercatat di jejak audit bersama
// nomor jurnal koreksinya. Dipakai Hitung Ulang HPP dan Revaluasi Harga Modal.
export function CorrectionReasonModal({
  title,
  description,
  confirmLabel,
  submitting,
  children,
  onClose,
  onSubmit,
}: CorrectionReasonModalProps) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = correctionReasonSchema.safeParse({ reason });
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
      title={title}
      description={description}
      onClose={() => (submitting ? undefined : onClose())}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Kembali
          </Button>
          <Button type="submit" form="correction-reason-form" variant="danger" loading={submitting}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      {children && <div className="mb-4">{children}</div>}
      <form id="correction-reason-form" onSubmit={handleSubmit} noValidate>
        <FormField label="Alasan koreksi" htmlFor="correction-reason" required errorText={error}>
          <TextArea
            id="correction-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            maxLength={500}
            placeholder="Contoh: harga saldo awal salah ketik, seharusnya 9.000 per pcs"
          />
        </FormField>
      </form>
    </Modal>
  );
}
