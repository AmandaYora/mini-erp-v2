import { SelectInput } from "@/shared/components/ui";
import type { BankAccount } from "@/modules/finance/types";
import { bankAccountByAccountId, bankAccountLabel } from "@/modules/finance/lib/bank-accounts";

interface BankAccountSelectProps {
  id?: string;
  "aria-label"?: string;
  /** Rekening aktif yang boleh dipilih. */
  options: BankAccount[];
  /** Semua rekening (untuk label nilai lama yang sudah diarsipkan). */
  all?: BankAccount[];
  /** Id akun buku besar rekening terpilih; null = belum dipilih. */
  value: number | null;
  onChange: (accountId: number | null) => void;
  disabled?: boolean;
  error?: string;
  /** Teks opsi kosong; tanpa ini opsi kosong tidak ditampilkan. */
  emptyLabel?: string;
}

/** Pemilih rekening perusahaan (ADR-0026). Nilai = id akun buku besar rekening. */
export function BankAccountSelect({
  id,
  options,
  all,
  value,
  onChange,
  disabled,
  error,
  emptyLabel,
  "aria-label": ariaLabel,
}: BankAccountSelectProps) {
  const current = bankAccountByAccountId(all ?? options, value);
  const currentArchived = current && !options.some((o) => o.accountId === current.accountId) ? current : null;
  return (
    <SelectInput
      id={id}
      aria-label={ariaLabel}
      value={value === null ? "" : String(value)}
      disabled={disabled}
      error={error}
      onChange={(e) => onChange(e.target.value === "" ? null : Number(e.target.value))}
    >
      {(emptyLabel !== undefined || value === null) && <option value="">{emptyLabel ?? "Pilih rekening…"}</option>}
      {currentArchived && (
        <option value={currentArchived.accountId}>{bankAccountLabel(currentArchived)} (diarsipkan)</option>
      )}
      {options.map((b) => (
        <option key={b.id} value={b.accountId}>
          {bankAccountLabel(b)}
          {b.isDefault ? " · default" : ""}
        </option>
      ))}
    </SelectInput>
  );
}
