import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, Modal, TextArea } from "@/shared/components/ui";
import { reasonSchema } from "./reason-schema";

export interface ReasonDialogProps {
  open: boolean;
  title: string;
  /** Apa yang terjadi bila dikonfirmasi (dokumen apa, efek stok/status). */
  description: string;
  /** Label tombol aksi, mis. "Batalkan surat jalan". */
  confirmLabel: string;
  placeholder?: string;
  submitting?: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}

/**
 * Dialog konfirmasi yang meminta alasan wajib sebelum aksi koreksi (batal/buang dokumen).
 * Alasan disimpan pada dokumen dan jejak audit oleh backend. Domain-agnostik: judul, keterangan,
 * dan label tombol datang dari pemanggil.
 */
export function ReasonDialog({ open, ...rest }: ReasonDialogProps) {
  // Isi dialog dipasang ulang setiap kali dibuka, jadi alasan lama tidak tersisa.
  return open ? <ReasonDialogBody {...rest} /> : null;
}

function ReasonDialogBody({
  title,
  description,
  confirmLabel,
  placeholder = "Tulis alasan singkat, mis. salah input jumlah",
  submitting = false,
  onClose,
  onSubmit,
}: Omit<ReasonDialogProps, "open">) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);
  const id = useId();
  const formId = `${id}-form`;
  const fieldId = `${id}-reason`;

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = reasonSchema.safeParse({ reason });
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
      onClose={submitting ? () => undefined : onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Kembali
          </Button>
          <Button type="submit" form={formId} variant="danger" loading={submitting}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} noValidate>
        <FormField label="Alasan" htmlFor={fieldId} required errorText={error}>
          <TextArea
            id={fieldId}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            maxLength={255}
            placeholder={placeholder}
            autoFocus
          />
        </FormField>
      </form>
    </Modal>
  );
}
