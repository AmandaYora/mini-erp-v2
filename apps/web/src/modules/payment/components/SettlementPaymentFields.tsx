import { FormField, SelectInput } from "@/shared/components/ui";
import { PaymentBankAccountField } from "@/modules/payment/components/PaymentBankAccountField";
import { PaymentCashAccountField } from "@/modules/payment/components/PaymentCashAccountField";
import type { SettlementPayMethod, SettlementPaymentState } from "@/modules/payment/hooks/use-settlement-payment";

interface SettlementPaymentFieldsProps {
  pay: SettlementPaymentState;
  direction: "in" | "out";
  disabled?: boolean;
  errors?: Record<string, string>;
}

/** Metode + rekening/kas untuk penyelesaian retur yang menggerakkan uang. */
export function SettlementPaymentFields({ pay, direction, disabled, errors = {} }: SettlementPaymentFieldsProps) {
  return (
    <>
      <FormField label="Metode pembayaran" required errorText={errors.paymentMethod}>
        <SelectInput
          aria-label="Metode pembayaran"
          value={pay.method}
          disabled={disabled}
          onChange={(e) => pay.setMethod(e.target.value as SettlementPayMethod)}
        >
          <option value="cash">Tunai</option>
          <option value="transfer">Transfer</option>
          <option value="check">Cek</option>
        </SelectInput>
      </FormField>
      <PaymentCashAccountField
        cash={pay.cash}
        method={pay.method}
        value={pay.kasValue}
        onChange={pay.setKas}
        direction={direction}
        errorText={errors.cashAccountId}
        disabled={disabled}
      />
      <PaymentBankAccountField
        bank={pay.bank}
        method={pay.method}
        value={pay.bankValue}
        onChange={pay.setBank}
        direction={direction}
        errorText={errors.cashAccountId}
        disabled={disabled}
      />
    </>
  );
}
