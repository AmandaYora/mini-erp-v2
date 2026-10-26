import { useId, useState } from "react";
import type { FormEvent } from "react";
import { Button, DateInput, FormField, FormSheet, SelectInput, TextArea, TextInput } from "@/shared/components/ui";
import { Money } from "@/shared/components/patterns";
import { Notice } from "@/shared/components/feedback/notice";
import { RupiahInput } from "@/modules/payment/components/RupiahInput";
import { cashChange } from "@/modules/payment/lib/fifo";
import { paymentCorrectSchema } from "@/modules/payment/schemas/payment.schema";
import type { PaymentCorrectValues } from "@/modules/payment/schemas/payment.schema";
import type { Payment } from "@/modules/payment/types";
import { orderTypeLabel, referenceLabel } from "@/modules/payment/types";
import { PaymentBankAccountField } from "@/modules/payment/components/PaymentBankAccountField";
import { paymentBankAccountError } from "@/modules/finance/lib/bank-accounts";
import { useBankAccounts } from "@/modules/finance/hooks/use-bank-accounts";
import { initialPaymentAccount, methodUsesBankAccount } from "@/modules/finance/lib/bank-accounts";

interface CorrectPaymentModalProps {
  payment: Payment;
  submitting: boolean;
  serverError: string | null;
  onClose: () => void;
  onSubmit: (values: PaymentCorrectValues) => void;
}

/**
 * Panel koreksi pembayaran (salah input). Terisi nilai pembayaran saat ini; pihak dan dokumen
 * tetap. Backend membuat pembayaran pengganti, membagi nominal baru ke dokumen yang sama sesuai
 * urutan, lalu membatalkan pembayaran asli dengan alasan ini (POST /payments/:id/correct).
 */
export function CorrectPaymentModal({ payment, submitting, serverError, onClose, onSubmit }: CorrectPaymentModalProps) {
  const [method, setMethod] = useState(payment.method);
  const [amount, setAmount] = useState(String(payment.amount));
  const [paidAt, setPaidAt] = useState(payment.paidAt.slice(0, 10));
  const [reference, setReference] = useState(payment.reference ?? "");
  const [tendered, setTendered] = useState(payment.amountTendered > 0 ? String(payment.amountTendered) : "");
  const [notes, setNotes] = useState(payment.notes ?? "");
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const formId = useId();
  // Rekening transfer/cek (ADR-0026): awalnya rekening pembayaran asli bila masih aktif.
  const bank = useBankAccounts();
  const [cashAccountId, setCashAccountId] = useState<number | null>(null);
  const effectiveAccount = cashAccountId ?? initialPaymentAccount(bank.all, payment.cashAccountId ?? null);

  const amountNum = Number(amount);
  const tenderedNum = Number(tendered);

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = paymentCorrectSchema.safeParse({
      method,
      amount,
      paidAt: paidAt.trim() ? paidAt.trim() : undefined,
      reference: method !== "cash" && reference.trim() ? reference.trim() : undefined,
      amountTendered: method === "cash" && tendered.trim() ? tendered.trim() : undefined,
      notes: notes.trim() ? notes.trim() : undefined,
      reason,
      cashAccountId: methodUsesBankAccount(method) && effectiveAccount !== null ? effectiveAccount : undefined,
    });
    const bankError = paymentBankAccountError(bank, method, effectiveAccount);
    if (!parsed.success || bankError) {
      const fieldErrors: Record<string, string[] | undefined> = parsed.success ? {} : parsed.error.flatten().fieldErrors;
      const mapped: Record<string, string> = {};
      for (const [k, v] of Object.entries(fieldErrors)) {
        if (v && v.length > 0 && v[0]) mapped[k] = v[0];
      }
      if (bankError) mapped.cashAccountId = bankError;
      setErrors(mapped);
      return;
    }
    setErrors({});
    onSubmit(parsed.data);
  }

  return (
    <FormSheet
      open
      title="Koreksi pembayaran"
      description={`Pembayaran ${payment.number} diganti pembayaran baru dengan data di bawah, lalu dibatalkan. Pihak dan dokumen yang dibayar tetap.`}
      size="lg"
      onClose={onClose}
      actions={
        <>
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Batal
          </Button>
          <Button type="submit" form={formId} loading={submitting}>
            Simpan koreksi
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
        {serverError && <Notice tone="danger" title={serverError} />}

        <div className="rounded-md border border-border px-3 py-2 text-sm">
          <p className="mb-1 font-medium text-foreground">Dokumen yang dibayar</p>
          <ul className="flex flex-col gap-0.5">
            {payment.allocations.map((a) => (
              <li key={`${a.orderType}-${a.orderId}`} className="flex justify-between gap-3">
                <span>
                  {a.orderNumber || `#${a.orderId}`}{" "}
                  <span className="text-muted-foreground">({orderTypeLabel(a.orderType)})</span>
                </span>
                <Money value={a.amount} />
              </li>
            ))}
          </ul>
          <p className="mt-1 text-xs text-muted-foreground">
            Nominal baru dibagi ke dokumen ini sesuai urutan. Order tunai/COD yang sudah selesai harus tetap lunas.
          </p>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField label="Metode" required errorText={errors.method}>
            <SelectInput
              aria-label="Metode"
              value={method}
              onChange={(e) => {
                setMethod(e.target.value);
                if (e.target.value !== "cash") setTendered("");
              }}
            >
              <option value="cash">Tunai</option>
              <option value="transfer">Transfer</option>
              <option value="check">Cek</option>
            </SelectInput>
          </FormField>
          <FormField label="Tanggal bayar" errorText={errors.paidAt}>
            <DateInput aria-label="Tanggal bayar" value={paidAt} onChange={(e) => setPaidAt(e.target.value)} />
          </FormField>
        </div>

        <FormField label="Nominal" required errorText={errors.amount}>
          <RupiahInput
            value={amount}
            onValueChange={setAmount}
            invalid={Boolean(errors.amount)}
            aria-label="Nominal koreksi"
          />
        </FormField>

        {method === "cash" ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField label="Uang diterima" errorText={errors.amountTendered} helperText="Opsional.">
              <RupiahInput
                value={tendered}
                onValueChange={setTendered}
                invalid={Boolean(errors.amountTendered)}
                aria-label="Uang diterima"
              />
            </FormField>
            <div className="flex flex-col justify-end pb-2 text-sm">
              <span className="text-xs text-muted-foreground">Kembalian</span>
              <Money
                className="font-semibold text-foreground"
                value={
                  Number.isFinite(tenderedNum) && Number.isFinite(amountNum) ? cashChange(tenderedNum, amountNum) : 0
                }
              />
            </div>
          </div>
        ) : (
          <>
            <PaymentBankAccountField
              bank={bank}
              method={method}
              value={effectiveAccount}
              onChange={setCashAccountId}
              direction={payment.direction === "out" ? "out" : "in"}
              errorText={errors.cashAccountId}
            />
            <FormField label={referenceLabel(method)} errorText={errors.reference} helperText="Opsional, maksimal 100 karakter.">
              <TextInput
                aria-label={referenceLabel(method)}
                value={reference}
                onChange={(e) => setReference(e.target.value)}
                maxLength={100}
              />
            </FormField>
          </>
        )}

        <FormField label="Catatan" errorText={errors.notes}>
          <TextArea value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </FormField>

        <FormField
          label="Alasan koreksi"
          htmlFor="correct-payment-reason"
          required
          errorText={errors.reason}
          helperText="Tercatat sebagai alasan batal pembayaran asli dan di jejak audit."
        >
          <TextArea
            id="correct-payment-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={2}
            maxLength={255}
            placeholder="cth. Kasir salah pilih metode, seharusnya transfer"
          />
        </FormField>
      </form>
    </FormSheet>
  );
}
