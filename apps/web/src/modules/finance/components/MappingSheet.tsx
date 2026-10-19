import { useState } from "react";
import type { FormEvent } from "react";
import { Button, FormField, FormSheet, SearchSelect } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { mappingSchema } from "@/modules/finance/schemas/finance.schema";
import { MAPPING_ACCOUNT_TYPES, mappingKeyLabel } from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";

interface MappingSheetProps {
  mappingKey: string;
  currentAccountId: number | null;
  accounts: Account[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (accountId: number) => void;
}

// Ubah satu kunci pemetaan → PUT /account-mappings {key, accountId}. Backend menolak akun nonaktif dan tipe tak cocok.
export function MappingSheet({ mappingKey, currentAccountId, accounts, submitting, serverError, onClose, onSubmit }: MappingSheetProps) {
  const [accountId, setAccountId] = useState<number | null>(currentAccountId);
  const [error, setError] = useState<string | undefined>(undefined);

  const allowed = MAPPING_ACCOUNT_TYPES[mappingKey];
  const options = accounts
    .filter((a) => a.status === "active" && (!allowed || (allowed as readonly string[]).includes(a.type)))
    .map((a) => ({ value: a.id, label: `${a.code} — ${a.name}` }));

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = mappingSchema.safeParse({ key: mappingKey, accountId });
    if (!parsed.success) {
      setError(parsed.error.flatten().fieldErrors.accountId?.[0] ?? "Pilih akun dulu");
      return;
    }
    setError(undefined);
    onSubmit(parsed.data.accountId);
  }

  return (
    <FormSheet
      open
      title={`Pemetaan ${mappingKeyLabel(mappingKey)}`}
      description="Menentukan akun otomatis untuk posting jurnal dokumen."
      onClose={onClose}
      actions={
        <>
          <Button variant="secondary" disabled={submitting} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="mapping-form" loading={submitting}>
            Simpan pemetaan
          </Button>
        </>
      }
    >
      <form id="mapping-form" onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
        {serverError && <Notice tone="danger" title={serverError} />}
        <FormField label="Akun" required errorText={error}>
          <SearchSelect
            options={options}
            value={accountId}
            onChange={(v) => setAccountId(typeof v === "number" ? v : v === null ? null : Number(v))}
            placeholder="Pilih akun…"
          />
        </FormField>
      </form>
    </FormSheet>
  );
}
