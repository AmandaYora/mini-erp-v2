import { FormField, SelectInput } from "@/shared/components/ui";
import type { CashAccountsState } from "@/modules/finance/hooks/use-cash-accounts";
import { cashAccountByAccountId, cashAccountLabel } from "@/modules/finance/lib/cash-accounts";

interface PaymentCashAccountFieldProps {
  cash: CashAccountsState;
  method: string;
  /** Id akun kas terpilih (nilai efektif, sudah termasuk prefill). */
  value: number | null;
  onChange: (accountId: number | null) => void;
  /** "in" = uang masuk ("Ke kas"), "out" = uang keluar ("Dari kas"). */
  direction: "in" | "out";
  errorText?: string;
  disabled?: boolean;
  id?: string;
}

/**
 * Pilihan akun kas untuk pembayaran tunai (mis. bayar pemasok dari Kas Besar).
 * Tidak tampil untuk transfer/cek, atau selama hanya ada satu kas (tunai tetap
 * ke Kas Tunai seperti sebelumnya).
 */
export function PaymentCashAccountField({ cash, method, value, onChange, direction, errorText, disabled, id }: PaymentCashAccountFieldProps) {
  if (method !== "cash" || !cash.shown) return null;
  const label = direction === "in" ? "Ke kas" : "Dari kas";
  const current = cashAccountByAccountId(cash.all, value);
  const currentArchived = current && current.status !== "active" ? current : null;
  return (
    <FormField
      label={label}
      htmlFor={id}
      required
      errorText={errorText}
      helperText={direction === "in" ? "Akun kas tempat uang tunai masuk." : "Akun kas tempat uang tunai keluar."}
    >
      <SelectInput
        id={id}
        aria-label={label}
        value={value === null ? "" : String(value)}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value === "" ? null : Number(e.target.value))}
      >
        {value === null && <option value="">Pilih kas…</option>}
        {currentArchived && (
          <option value={currentArchived.accountId}>{cashAccountLabel(currentArchived)} (diarsipkan)</option>
        )}
        {cash.active.map((a) => (
          <option key={a.accountId} value={a.accountId}>
            {cashAccountLabel(a)}
          </option>
        ))}
      </SelectInput>
    </FormField>
  );
}
