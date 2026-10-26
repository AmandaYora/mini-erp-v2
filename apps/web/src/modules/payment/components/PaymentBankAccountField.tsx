import { FormField } from "@/shared/components/ui";
import { BankAccountSelect } from "@/modules/finance/components/BankAccountSelect";
import type { BankAccountsState } from "@/modules/finance/hooks/use-bank-accounts";
import { methodUsesBankAccount } from "@/modules/finance/lib/bank-accounts";

interface PaymentBankAccountFieldProps {
  bank: BankAccountsState;
  method: string;
  /** Id akun buku besar rekening terpilih (nilai efektif, sudah termasuk prefill). */
  value: number | null;
  onChange: (accountId: number | null) => void;
  /** "in" = uang masuk ("Ke rekening"), "out" = uang keluar ("Dari rekening"). */
  direction: "in" | "out";
  errorText?: string;
  disabled?: boolean;
  id?: string;
}

/**
 * Pilihan rekening perusahaan untuk pembayaran transfer/cek (ADR-0026). Tidak
 * tampil untuk tunai, atau selama perusahaan belum punya rekening (pembayaran
 * tetap ke akun pemetaan "bank" seperti sebelumnya).
 */
export function PaymentBankAccountField({ bank, method, value, onChange, direction, errorText, disabled, id }: PaymentBankAccountFieldProps) {
  if (!methodUsesBankAccount(method) || (!bank.required && value === null)) return null;
  return (
    <FormField
      label={direction === "in" ? "Ke rekening" : "Dari rekening"}
      htmlFor={id}
      required
      errorText={errorText}
      helperText={direction === "in" ? "Rekening perusahaan tempat uang masuk." : "Rekening perusahaan tempat uang keluar."}
    >
      <BankAccountSelect
        id={id}
        aria-label={direction === "in" ? "Ke rekening" : "Dari rekening"}
        options={bank.active}
        all={bank.all}
        value={value}
        onChange={onChange}
        disabled={disabled}
      />
    </FormField>
  );
}
