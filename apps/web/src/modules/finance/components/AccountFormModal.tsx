import { useState } from "react";
import type { FormEvent } from "react";
import {
  ActionRow,
  Button,
  FormField,
  Modal,
  SelectInput,
  TextInput,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import {
  accountCreateSchema,
  accountEditSchema,
} from "@/modules/finance/schemas/finance.schema";
import type {
  AccountCreateValues,
  AccountEditValues,
} from "@/modules/finance/schemas/finance.schema";
import { ACCOUNT_TYPES, accountTypeLabel } from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";

interface AccountFormModalProps {
  open: boolean;
  mode: "create" | "edit";
  initial?: Account | null;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: AccountCreateValues | AccountEditValues) => void;
}

// Code dikunci saat edit (backend UpdateAccount tidak menerima code —
// handler.go). Checkbox kas = akun dipakai sisi kas (pembayaran).
export function AccountFormModal({
  open,
  mode,
  initial,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: AccountFormModalProps) {
  const [code, setCode] = useState(initial?.code ?? "");
  const [name, setName] = useState(initial?.name ?? "");
  const [type, setType] = useState(initial?.type ?? "aset");
  const [isCash, setIsCash] = useState(initial?.isCash ?? false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (mode === "create") {
      const parsed = accountCreateSchema.safeParse({
        code: code.trim().toUpperCase(),
        name: name.trim(),
        type,
        isCash,
      });
      if (!parsed.success) {
        const mapped: Record<string, string> = {};
        for (const [k, v] of Object.entries(
          parsed.error.flatten().fieldErrors,
        )) {
          if (v?.[0]) mapped[k] = v[0];
        }
        setErrors(mapped);
        return;
      }
      setErrors({});
      onSubmit(parsed.data);
      return;
    }
    const parsed = accountEditSchema.safeParse({
      name: name.trim(),
      type,
      isCash,
    });
    if (!parsed.success) {
      const mapped: Record<string, string> = {};
      for (const [k, v] of Object.entries(parsed.error.flatten().fieldErrors)) {
        if (v?.[0]) mapped[k] = v[0];
      }
      setErrors(mapped);
      return;
    }
    setErrors({});
    onSubmit(parsed.data);
  }

  return (
    <Modal
      open={open}
      title={mode === "create" ? "Tambah Akun" : `Ubah Akun ${initial?.code ?? ""}`}
      description={
        mode === "create"
          ? "Kode akun tidak dapat diubah setelah dibuat."
          : "Kode akun terkunci. Tipe akun berjurnal tidak dapat diubah (aturan backend)."
      }
      onClose={onClose}
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}

        {mode === "create" ? (
          <FormField label="Kode" required errorText={errors.code}>
            <TextInput
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="cth. 6300"
            />
          </FormField>
        ) : (
          <FormField label="Kode">
            <TextInput value={initial?.code ?? ""} disabled />
          </FormField>
        )}

        <FormField label="Nama" required errorText={errors.name}>
          <TextInput
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="cth. Beban Listrik"
          />
        </FormField>

        <FormField label="Tipe" required errorText={errors.type}>
          <SelectInput value={type} onChange={(e) => setType(e.target.value)}>
            {ACCOUNT_TYPES.map((t) => (
              <option key={t} value={t}>
                {accountTypeLabel(t)}
              </option>
            ))}
          </SelectInput>
        </FormField>

        <label className="flex cursor-pointer items-center gap-2 text-sm text-ink">
          <input
            type="checkbox"
            checked={isCash}
            onChange={(e) => setIsCash(e.target.checked)}
          />
          Akun kas (muncul sebagai sumber/dana kas)
        </label>

        <ActionRow>
          <Button
            type="button"
            variant="secondary"
            onClick={onClose}
            disabled={submitting}
          >
            Batal
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? "Menyimpan…" : "Simpan"}
          </Button>
        </ActionRow>
      </form>
    </Modal>
  );
}
