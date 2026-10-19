import { useState } from "react";
import type { FormEvent } from "react";
import { Button, Checkbox, FormField, FormSheet, SelectInput, TextInput } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { accountCreateSchema, accountEditSchema } from "@/modules/finance/schemas/finance.schema";
import type { AccountCreateValues, AccountEditValues } from "@/modules/finance/schemas/finance.schema";
import { ACCOUNT_TYPES, accountTypeLabel } from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";

interface AccountFormSheetProps {
  mode: "create" | "edit";
  initial?: Account | null;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: AccountCreateValues | AccountEditValues) => void;
}

// Kode dikunci saat edit (backend UpdateAccount tidak menerima kode). "Akun kas" = dipakai sisi kas.
export function AccountFormSheet({ mode, initial, submitting, serverError, onClose, onSubmit }: AccountFormSheetProps) {
  const [code, setCode] = useState(initial?.code ?? "");
  const [name, setName] = useState(initial?.name ?? "");
  const [type, setType] = useState(initial?.type ?? "aset");
  const [isCash, setIsCash] = useState(initial?.isCash ?? false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  function fail(fieldErrors: Record<string, string[] | undefined>) {
    const mapped: Record<string, string> = {};
    for (const [k, v] of Object.entries(fieldErrors)) if (v?.[0]) mapped[k] = v[0];
    setErrors(mapped);
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (mode === "create") {
      const parsed = accountCreateSchema.safeParse({ code: code.trim().toUpperCase(), name: name.trim(), type, isCash });
      if (!parsed.success) return fail(parsed.error.flatten().fieldErrors);
      setErrors({});
      onSubmit(parsed.data);
      return;
    }
    const parsed = accountEditSchema.safeParse({ name: name.trim(), type, isCash });
    if (!parsed.success) return fail(parsed.error.flatten().fieldErrors);
    setErrors({});
    onSubmit(parsed.data);
  }

  return (
    <FormSheet
      open
      title={mode === "create" ? "Tambah akun" : `Ubah akun ${initial?.code ?? ""}`}
      description={
        mode === "create"
          ? "Kode akun tidak dapat diubah setelah dibuat."
          : "Kode akun terkunci. Tipe akun berjurnal tidak dapat diubah (aturan backend)."
      }
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="account-form" loading={submitting}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="account-form" onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
        {serverError && <Notice tone="danger" title={serverError} />}
        {mode === "create" ? (
          <FormField label="Kode" htmlFor="account-code" required errorText={errors.code}>
            <TextInput id="account-code" value={code} onChange={(e) => setCode(e.target.value)} placeholder="cth. 6300" />
          </FormField>
        ) : (
          <FormField label="Kode" htmlFor="account-code">
            <TextInput id="account-code" value={initial?.code ?? ""} disabled readOnly />
          </FormField>
        )}
        <FormField label="Nama" htmlFor="account-name" required errorText={errors.name}>
          <TextInput id="account-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="cth. Beban Listrik" />
        </FormField>
        <FormField label="Tipe" htmlFor="account-type" required errorText={errors.type}>
          <SelectInput id="account-type" value={type} onChange={(e) => setType(e.target.value)}>
            {ACCOUNT_TYPES.map((t) => (
              <option key={t} value={t}>
                {accountTypeLabel(t)}
              </option>
            ))}
          </SelectInput>
        </FormField>
        <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
          <Checkbox checked={isCash} onCheckedChange={(v) => setIsCash(v === true)} />
          Akun kas (muncul sebagai sumber dana kas)
        </label>
      </form>
    </FormSheet>
  );
}
