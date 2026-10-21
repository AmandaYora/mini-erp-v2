import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, Modal, TextArea } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { correctionReasonSchema } from "@/modules/purchasing/schemas/purchase-correction.schema";

interface ReasonDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  placeholder?: string;
  submitting: boolean;
  /** Pesan server (formatApiError) bila aksi ditolak. */
  serverError: string | null;
  onClose: () => void;
  onConfirm: (reason: string) => void;
}

/** Konfirmasi aksi yang tidak bisa diulang dengan alasan wajib (mis. batalkan order). */
export function ReasonDialog({
  open,
  title,
  message,
  confirmLabel,
  placeholder,
  submitting,
  serverError,
  onClose,
  onConfirm,
}: ReasonDialogProps) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | undefined>();
  const formId = useId();

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = correctionReasonSchema.safeParse(reason);
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message);
      return;
    }
    setError(undefined);
    onConfirm(parsed.data);
  }

  return (
    <Modal
      open={open}
      title={title}
      size="sm"
      onClose={onClose}
      actions={
        <>
          <Button type="button" variant="secondary" size="sm" onClick={onClose} disabled={submitting}>
            Kembali
          </Button>
          <Button type="submit" form={formId} variant="danger" size="sm" loading={submitting}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="flex flex-col gap-4">
        <p className="text-sm text-foreground">{message}</p>
        {serverError && <Notice tone="danger" title={serverError} />}
        <FormField label="Alasan" required errorText={error}>
          <TextArea
            rows={3}
            maxLength={500}
            value={reason}
            placeholder={placeholder}
            onChange={(e) => setReason(e.target.value)}
          />
        </FormField>
      </form>
    </Modal>
  );
}
