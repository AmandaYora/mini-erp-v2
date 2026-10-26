import { useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, Modal } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { BankAccountSelect } from "@/modules/finance/components/BankAccountSelect";
import type { BankAccount } from "@/modules/finance/types";

interface ReceiptAccountModalProps {
  number: string;
  current: number | null;
  options: BankAccount[];
  all: BankAccount[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (accountId: number) => void;
}

// Ganti rekening tujuan tagihan (ADR-0026) — juga untuk order selesai yang
// masih menunggu transfer. Pembayaran yang sudah tercatat tidak berubah.
export function ReceiptAccountModal({ number, current, options, all, submitting, serverError, onClose, onSubmit }: ReceiptAccountModalProps) {
  const [value, setValue] = useState<number | null>(current);
  const [error, setError] = useState<string | null>(null);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (value === null) {
      setError("Pilih rekening.");
      return;
    }
    setError(null);
    onSubmit(value);
  }

  return (
    <Modal
      open
      title="Ubah rekening tujuan"
      description={`Rekening yang dicetak di nota ${number} dan terisi otomatis untuk pembayaran transfernya.`}
      size="sm"
      onClose={onClose}
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
        {serverError && <Notice tone="danger" title={serverError} />}
        <FormField label="Rekening tujuan" htmlFor="f-receipt-change" required errorText={error ?? undefined}>
          <BankAccountSelect id="f-receipt-change" options={options} all={all} value={value} onChange={setValue} />
        </FormField>
        <div className="flex justify-end gap-2">
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" loading={submitting} disabled={value === null || value === current}>
            Simpan
          </Button>
        </div>
      </form>
    </Modal>
  );
}
