import { useState } from "react";
import type { FormEvent } from "react";
import {
  ActionRow,
  Button,
  FormField,
  Modal,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { addressFormSchema } from "@/modules/party/schemas/address.schema";
import type { AddressFormValues } from "@/modules/party/schemas/address.schema";
import type { Address } from "@/modules/party/types";

interface AddressFormModalProps {
  open: boolean;
  /** null = tambah baru. */
  initial: Address | null;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: AddressFormValues) => void;
}

export function AddressFormModal({
  open,
  initial,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: AddressFormModalProps) {
  const [label, setLabel] = useState(initial?.label ?? "");
  const [recipient, setRecipient] = useState(initial?.recipient ?? "");
  const [phone, setPhone] = useState(initial?.phone ?? "");
  const [text, setText] = useState(initial?.text ?? "");
  const [sortOrder, setSortOrder] = useState(String(initial?.sortOrder ?? 0));
  const [isPrimary, setIsPrimary] = useState(initial?.isPrimary ?? false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = addressFormSchema.safeParse({
      label,
      recipient,
      phone,
      text,
      isPrimary,
      sortOrder,
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        const key = String(issue.path[0] ?? "");
        if (key) errs[key] ??= issue.message;
      }
      setErrors(errs);
      return;
    }
    setErrors({});
    onSubmit(parsed.data);
  }

  return (
    <Modal
      open={open}
      title={initial ? "Ubah Alamat" : "Tambah Alamat"}
      description="Tepat satu alamat utama per customer — menandai alamat ini sebagai utama akan melepas yang lain."
      size="lg"
      onClose={onClose}
    >
      <form onSubmit={handleSubmit}>
        <div className="space-y-4">
          {serverError && <Notice tone="danger" title={serverError} />}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField label="Label" htmlFor="addr-label" errorText={errors.label}>
              <TextInput
                id="addr-label"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="Rumah / Kantor / Toko"
                maxLength={100}
              />
            </FormField>
            <FormField label="Penerima" htmlFor="addr-recipient" errorText={errors.recipient}>
              <TextInput
                id="addr-recipient"
                value={recipient}
                onChange={(e) => setRecipient(e.target.value)}
                placeholder="Nama penerima"
                maxLength={200}
              />
            </FormField>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField label="Telepon" htmlFor="addr-phone" errorText={errors.phone}>
              <TextInput
                id="addr-phone"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="08…"
                maxLength={30}
              />
            </FormField>
            <FormField label="Urutan" htmlFor="addr-sort" errorText={errors.sortOrder}>
              <TextInput
                id="addr-sort"
                type="number"
                min={0}
                step={1}
                value={sortOrder}
                onChange={(e) => setSortOrder(e.target.value)}
              />
            </FormField>
          </div>
          <FormField label="Alamat lengkap" htmlFor="addr-text" required errorText={errors.text}>
            <TextArea
              id="addr-text"
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder="Jalan, nomor, RT/RW, kelurahan, kota…"
              rows={3}
              maxLength={1000}
            />
          </FormField>
          <label className="flex cursor-pointer items-center gap-2 text-sm text-ink">
            <input
              type="checkbox"
              className="h-4 w-4"
              checked={isPrimary}
              onChange={(e) => setIsPrimary(e.target.checked)}
            />
            Jadikan alamat utama
          </label>
          <ActionRow>
            <Button variant="secondary" onClick={onClose} disabled={submitting}>
              Batal
            </Button>
            <Button type="submit" variant="primary" disabled={submitting}>
              {submitting ? "Menyimpan…" : "Simpan"}
            </Button>
          </ActionRow>
        </div>
      </form>
    </Modal>
  );
}
