import { useState } from "react";
import type { FormEvent } from "react";
import {
  ActionRow,
  Button,
  FormField,
  Modal,
  SearchSelect,
} from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { mappingSchema } from "@/modules/finance/schemas/finance.schema";
import { mappingKeyLabel } from "@/modules/finance/types";
import type { Account } from "@/modules/finance/types";

interface MappingModalProps {
  open: boolean;
  mappingKey: string;
  currentAccountId: number | null;
  accounts: Account[];
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (accountId: number) => void;
}

// Ubah satu kunci pemetaan → PUT /account-mappings {key, accountId}.
// Backend menolak akun nonaktif dan tipe yang tak cocok (service.go).
export function MappingModal({
  open,
  mappingKey,
  currentAccountId,
  accounts,
  submitting,
  serverError,
  onClose,
  onSubmit,
}: MappingModalProps) {
  const [accountId, setAccountId] = useState<number | null>(currentAccountId);
  const [error, setError] = useState<string | undefined>(undefined);

  const options = accounts
    .filter((a) => a.status === "active")
    .map((a) => ({ value: a.id, label: `${a.code} — ${a.name}` }));

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = mappingSchema.safeParse({ key: mappingKey, accountId });
    if (!parsed.success) {
      setError(
        parsed.error.flatten().fieldErrors.accountId?.[0] ?? "Pilih akun dulu",
      );
      return;
    }
    setError(undefined);
    onSubmit(parsed.data.accountId);
  }

  return (
    <Modal
      open={open}
      title={`Pemetaan ${mappingKeyLabel(mappingKey)}`}
      description="Menentukan akun otomatis untuk posting jurnal dokumen."
      onClose={onClose}
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {serverError && <Notice tone="danger" title={serverError} />}

        <FormField label="Akun" required errorText={error}>
          <SearchSelect
            options={options}
            value={accountId}
            onChange={(v) =>
              setAccountId(
                typeof v === "number" ? v : v === null ? null : Number(v),
              )
            }
            placeholder="Pilih akun…"
          />
        </FormField>

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
            {submitting ? "Menyimpan…" : "Simpan Pemetaan"}
          </Button>
        </ActionRow>
      </form>
    </Modal>
  );
}
