import { useState } from "react";
import { useBankAccounts } from "@/modules/finance/hooks/use-bank-accounts";
import type { BankAccountsState } from "@/modules/finance/hooks/use-bank-accounts";
import { useCashAccounts } from "@/modules/finance/hooks/use-cash-accounts";
import type { CashAccountsState } from "@/modules/finance/hooks/use-cash-accounts";
import { methodUsesBankAccount, paymentBankAccountError } from "@/modules/finance/lib/bank-accounts";
import { cashAccountForPayload, initialCashAccount, rememberCashAccount } from "@/modules/finance/lib/cash-accounts";

export type SettlementPayMethod = "cash" | "transfer" | "check";

function newSettlementRef(): string {
  try {
    return crypto.randomUUID();
  } catch {
    return `st-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

export interface SettlementPaymentState {
  method: SettlementPayMethod;
  setMethod: (m: SettlementPayMethod) => void;
  bank: BankAccountsState;
  cash: CashAccountsState;
  /** Rekening efektif transfer/cek (pilihan → default). */
  bankValue: number | null;
  setBank: (id: number | null) => void;
  /** Kas efektif tunai (pilihan → kas terakhir arah ini → Kas Tunai). */
  kasValue: number | null;
  setKas: (id: number | null) => void;
  /** Kunci idempoten satu penyelesaian (kirim ulang tidak membayar dua kali). */
  clientRef: string;
  /** Kembali ke tunai, tanpa pilihan, kunci baru. */
  reset: () => void;
  /** Pesan galat rekening wajib (cermin server); null = valid. */
  accountError: () => string | null;
  /** cashAccountId untuk body permintaan; mengingat kas tunai yang dipakai. */
  payloadAccount: () => number | undefined;
}

/**
 * Uang penyelesaian retur (refund / tambah bayar) dicatat sebagai pembayaran:
 * metode + rekening (transfer/cek) atau kas (tunai), sama seperti form pembayaran.
 */
export function useSettlementPayment(direction: "in" | "out"): SettlementPaymentState {
  const bank = useBankAccounts();
  const cash = useCashAccounts();
  const [method, setMethod] = useState<SettlementPayMethod>("cash");
  const [bankChoice, setBank] = useState<number | null>(null);
  const [kasChoice, setKas] = useState<number | null>(null);
  const [clientRef, setClientRef] = useState(newSettlementRef);
  const bankValue = bankChoice ?? bank.defaultAccountId;
  const kasValue = kasChoice ?? initialCashAccount(cash.all, direction);
  return {
    method,
    setMethod,
    bank,
    cash,
    bankValue,
    setBank,
    kasValue,
    setKas,
    clientRef,
    reset: () => {
      setClientRef(newSettlementRef());
      setMethod("cash");
      setBank(null);
      setKas(null);
    },
    accountError: () => paymentBankAccountError(bank, method, bankValue),
    payloadAccount: () => {
      if (method === "cash") {
        const kas = cashAccountForPayload(cash.all, kasValue);
        rememberCashAccount(direction, kas ?? null);
        return kas;
      }
      return methodUsesBankAccount(method) && bankValue !== null ? bankValue : undefined;
    },
  };
}
