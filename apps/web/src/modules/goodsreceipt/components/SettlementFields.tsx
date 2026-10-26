import { DateInput, FormField, SelectInput, TextInput } from "@/shared/components/ui";
import type { SettlementDraft } from "@/modules/goodsreceipt/lib/settlement-draft";
import { useBankAccounts } from "@/modules/finance/hooks/use-bank-accounts";
import { PaymentBankAccountField } from "@/modules/payment/components/PaymentBankAccountField";

interface SettlementFieldsProps {
  value: SettlementDraft;
  errors: Record<string, string>;
  canPay: boolean;
  canSwitchNet: boolean;
  onChange: (next: SettlementDraft) => void;
}

/** Pelunasan PO COD: bayar sekarang atau jadikan tempo. */
export function SettlementFields({ value, errors, canPay, canSwitchNet, onChange }: SettlementFieldsProps) {
  const patch = (p: Partial<SettlementDraft>) => onChange({ ...value, ...p });
  // Rekening sumber transfer/cek (ADR-0026): kosong = default perusahaan (server memakai yang sama).
  const bank = useBankAccounts();
  return (
    <div className="rounded-md border border-border p-4">
      <p className="text-sm font-semibold text-foreground">Pelunasan order COD</p>
      <p className="mt-1 text-xs text-muted-foreground">
        Wajib bila penerimaan ini menuntaskan PO COD yang masih ada sisa tagihan. Pada penerimaan
        parsial atau PO yang sudah lunas, server mengabaikan pilihan ini.
      </p>
      <div className="mt-3 grid gap-4 md:grid-cols-2">
        <FormField label="Cara Pelunasan" errorText={errors.mode}>
          <SelectInput
            value={value.mode}
            onChange={(e) => patch({ mode: e.target.value as SettlementDraft["mode"] })}
          >
            <option value="">Tanpa pelunasan</option>
            <option value="pay_now" disabled={!canPay}>
              Bayar sekarang{canPay ? "" : " (butuh izin pembayaran)"}
            </option>
            <option value="switch_to_net" disabled={!canSwitchNet}>
              Jadikan tempo (net){canSwitchNet ? "" : " (butuh izin ubah pembelian)"}
            </option>
          </SelectInput>
        </FormField>
        {value.mode === "switch_to_net" && (
          <FormField label="Jatuh Tempo" required errorText={errors.dueDate}>
            <DateInput value={value.dueDate} onChange={(e) => patch({ dueDate: e.target.value })} />
          </FormField>
        )}
        {value.mode === "pay_now" && (
          <>
            <FormField label="Metode" errorText={errors.method}>
              <SelectInput
                value={value.method}
                onChange={(e) => patch({ method: e.target.value as SettlementDraft["method"] })}
              >
                <option value="cash">Tunai</option>
                <option value="transfer">Transfer</option>
                <option value="check">Cek</option>
              </SelectInput>
            </FormField>
            <FormField label="Tanggal Bayar" errorText={errors.paymentDate}>
              <DateInput value={value.paymentDate} onChange={(e) => patch({ paymentDate: e.target.value })} />
            </FormField>
            <PaymentBankAccountField
              bank={bank}
              method={value.method}
              value={value.cashAccountId ?? bank.defaultAccountId}
              onChange={(id) => patch({ cashAccountId: id })}
              direction="out"
              errorText={errors.cashAccountId}
            />
            {value.method !== "cash" && (
              <FormField
                label={value.method === "check" ? "Nomor Cek" : "Referensi Transfer"}
                required
                errorText={errors.reference}
              >
                <TextInput value={value.reference} onChange={(e) => patch({ reference: e.target.value })} />
              </FormField>
            )}
            <FormField label="Catatan" errorText={errors.notes}>
              <TextInput
                value={value.notes}
                maxLength={255}
                placeholder="Opsional"
                onChange={(e) => patch({ notes: e.target.value })}
              />
            </FormField>
          </>
        )}
      </div>
    </div>
  );
}
